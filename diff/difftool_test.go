package diff

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aryann/difflib"
	"github.com/stretchr/testify/require"

	"github.com/databus23/helm-diff/v3/manifest"
)

// The printDiffToolReport tests below re-run this test binary as a stand-in for
// the external diff tool (the TestHelperProcess pattern from os/exec), so they do
// not depend on POSIX tools like diff or echo and also run on Windows.
const (
	diffToolHelperEnv      = "GO_WANT_DIFF_TOOL_HELPER"
	diffToolHelperBehavior = "HELM_DIFF_TOOL_HELPER_BEHAVIOR"
)

// diffToolHelperCommand returns the command line for the stand-in diff tool and
// selects its behavior, which is implemented by TestDiffToolHelperProcess below.
func diffToolHelperCommand(t *testing.T, behavior string) string {
	t.Helper()
	t.Setenv(diffToolHelperEnv, "1")
	t.Setenv(diffToolHelperBehavior, behavior)

	binary, err := os.Executable()
	require.NoError(t, err)

	return fmt.Sprintf("%q -test.run=^TestDiffToolHelperProcess$ --", binary)
}

// TestDiffToolHelperProcess is not a real test. It is the implementation of the
// stand-in diff tool: invoked as `<test binary> -test.run=... -- <old> <new>`,
// it selects its behavior via HELM_DIFF_TOOL_HELPER_BEHAVIOR.
func TestDiffToolHelperProcess(t *testing.T) {
	if os.Getenv(diffToolHelperEnv) != "1" {
		return
	}

	// Everything after "--" is the file paths appended by printDiffToolReport.
	var files []string
	for i, arg := range os.Args {
		if arg == "--" {
			files = os.Args[i+1:]
			break
		}
	}

	switch os.Getenv(diffToolHelperBehavior) {
	case "cat": // print the contents of both files, like `cat old new`
		for _, file := range files {
			content, err := os.ReadFile(file)
			if err != nil {
				fmt.Fprintf(os.Stderr, "helper cat: %v\n", err)
				os.Exit(3)
			}
			_, _ = os.Stdout.Write(content)
		}
	case "args": // print the file arguments, one per line
		for _, file := range files {
			fmt.Println(file)
		}
	case "diff": // differences found: print to stdout and exit 1 like diff tools do
		fmt.Println("helper: differences found")
		os.Exit(1)
	case "fail": // simulate a broken tool
		fmt.Fprintln(os.Stderr, "helper: boom")
		os.Exit(3)
	}

	os.Exit(0)
}

// captureStderr runs fn with os.Stderr redirected to a pipe and returns what
// was written to it.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stderr
	reader, writer, err := os.Pipe()
	require.NoError(t, err)

	os.Stderr = writer
	defer func() { os.Stderr = original }()

	fn()

	require.NoError(t, writer.Close())
	content, err := io.ReadAll(reader)
	require.NoError(t, err)

	return string(content)
}

func TestSplitDiffToolCommand(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected []string
		wantErr  bool
	}{
		{
			name:     "empty",
			command:  "   ",
			expected: nil,
		},
		{
			name:     "simple",
			command:  "diff -u -N",
			expected: []string{"diff", "-u", "-N"},
		},
		{
			name:     "collapses repeated whitespace",
			command:  "  diff \t -u  ",
			expected: []string{"diff", "-u"},
		},
		{
			name:     "double quoted argument keeps spaces",
			command:  `"/opt/my tools/diff" --color=always`,
			expected: []string{"/opt/my tools/diff", "--color=always"},
		},
		{
			name:     "single quoted argument keeps spaces",
			command:  `'/opt/my tools/diff' -u`,
			expected: []string{"/opt/my tools/diff", "-u"},
		},
		{
			name:     "quotes inside an argument",
			command:  `git --no-pager diff --src-prefix="a b/"`,
			expected: []string{"git", "--no-pager", "diff", "--src-prefix=a b/"},
		},
		{
			name:     "empty quotes keep an empty argument",
			command:  `tool "" arg`,
			expected: []string{"tool", "", "arg"},
		},
		{
			name:    "unclosed double quote",
			command: `diff -u "foo`,
			wantErr: true,
		},
		{
			name:    "unclosed single quote",
			command: `diff -u 'foo`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, err := splitDiffToolCommand(tt.command)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expected, args)
		})
	}
}

func TestDiffToolCommandResolution(t *testing.T) {
	t.Run("empty when nothing is configured", func(t *testing.T) {
		t.Setenv(DiffToolEnvVar, "")
		require.Empty(t, diffToolCommand(""))
	})

	t.Run("environment variable is used when no command is given", func(t *testing.T) {
		t.Setenv(DiffToolEnvVar, "colordiff -u")
		require.Equal(t, "colordiff -u", diffToolCommand(""))
	})

	t.Run("explicit command wins over the environment variable", func(t *testing.T) {
		t.Setenv(DiffToolEnvVar, "colordiff -u")
		require.Equal(t, "difft", diffToolCommand("difft"))
	})
}

func TestWriteDiffToolSides(t *testing.T) {
	report := &Report{
		Entries: []ReportEntry{
			{
				Key:        "default, nginx, Deployment (apps)",
				Kind:       "Deployment",
				ChangeType: changeTypeModify,
				Diffs: []difflib.DiffRecord{
					{Payload: "kind: Deployment", Delta: difflib.Common},
					{Payload: "  replicas: 2", Delta: difflib.LeftOnly},
					{Payload: "  replicas: 3", Delta: difflib.RightOnly},
				},
			},
		},
	}

	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old")
	newPath := filepath.Join(dir, "new")
	require.NoError(t, writeDiffToolSides(report, oldPath, newPath))

	oldContent, err := os.ReadFile(oldPath)
	require.NoError(t, err)
	newContent, err := os.ReadFile(newPath)
	require.NoError(t, err)

	require.Equal(t, "---\n# Resource: default, nginx, Deployment (apps)\n# Change: MODIFY\nkind: Deployment\n  replicas: 2\n", string(oldContent))
	require.Equal(t, "---\n# Resource: default, nginx, Deployment (apps)\n# Change: MODIFY\nkind: Deployment\n  replicas: 3\n", string(newContent))
}

func TestWriteDiffToolSidesSuppressedKind(t *testing.T) {
	report := &Report{
		Entries: []ReportEntry{
			{
				Key:             "default, mysecret, Secret (v1)",
				Kind:            "Secret",
				SuppressedKinds: []string{"Secret"},
				ChangeType:      changeTypeModify,
				Diffs: []difflib.DiffRecord{
					{Payload: "kind: Secret", Delta: difflib.Common},
					{Payload: "  password: aGkK", Delta: difflib.LeftOnly},
					{Payload: "  password: Ynll", Delta: difflib.RightOnly},
				},
			},
		},
	}

	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old")
	newPath := filepath.Join(dir, "new")
	require.NoError(t, writeDiffToolSides(report, oldPath, newPath))

	oldContent, err := os.ReadFile(oldPath)
	require.NoError(t, err)
	newContent, err := os.ReadFile(newPath)
	require.NoError(t, err)

	require.NotContains(t, string(oldContent), "aGkK", "suppressed kinds must not leak their content")
	require.NotContains(t, string(newContent), "Ynll", "suppressed kinds must not leak their content")
	require.Contains(t, string(oldContent), "Changes suppressed on sensitive content of type Secret")
	require.Equal(t, string(oldContent), string(newContent), "suppressed entries must be identical on both sides")
}

func TestWriteDiffToolSidesSuppressedLines(t *testing.T) {
	report := &Report{
		Entries: []ReportEntry{
			// A MODIFY entry whose every changed line was filtered out by
			// --suppress-output-line-regex (flipped to MODIFY_SUPPRESSED).
			{
				Key:        "default, nginx, Deployment (apps)",
				Kind:       "Deployment",
				ChangeType: changeTypeModifySuppressed,
			},
			// An ADD entry can end up with empty diffs the same way.
			{
				Key:        "default, api, Deployment (apps)",
				Kind:       "Deployment",
				ChangeType: changeTypeAdd,
			},
		},
	}

	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old")
	newPath := filepath.Join(dir, "new")
	require.NoError(t, writeDiffToolSides(report, oldPath, newPath))

	oldContent, err := os.ReadFile(oldPath)
	require.NoError(t, err)
	newContent, err := os.ReadFile(newPath)
	require.NoError(t, err)

	require.Equal(t, string(oldContent), string(newContent),
		"fully suppressed entries must be identical on both sides so the tool reports no change")
	require.Equal(t, 2, strings.Count(string(oldContent), "# Changes suppressed by --suppress-output-line-regex"),
		"each fully suppressed entry carries the placeholder")
}

func TestPrintDiffToolReport(t *testing.T) {
	report := &Report{
		diffToolCommand: diffToolHelperCommand(t, "cat"),
		Entries: []ReportEntry{
			{
				Key:        "default, nginx, Deployment (apps)",
				Kind:       "Deployment",
				ChangeType: changeTypeModify,
				Diffs: []difflib.DiffRecord{
					{Payload: "kind: Deployment", Delta: difflib.Common},
					{Payload: "  replicas: 2", Delta: difflib.LeftOnly},
					{Payload: "  replicas: 3", Delta: difflib.RightOnly},
				},
			},
		},
	}

	var buf bytes.Buffer
	printDiffToolReport(report, &buf)

	output := buf.String()
	require.Contains(t, output, "  replicas: 2")
	require.Contains(t, output, "  replicas: 3")
	require.Contains(t, output, "# Change: MODIFY", "change types must be visible to the tool user")
}

func TestPrintDiffToolReportEmpty(t *testing.T) {
	report := &Report{diffToolCommand: diffToolHelperCommand(t, "args"), Entries: []ReportEntry{}}

	var buf bytes.Buffer
	printDiffToolReport(report, &buf)

	require.Empty(t, buf.String())
}

func TestPrintDiffToolReportPassesBothFiles(t *testing.T) {
	report := &Report{
		diffToolCommand: diffToolHelperCommand(t, "args"),
		Entries: []ReportEntry{
			{
				Key:        "default, nginx, Deployment (apps)",
				Kind:       "Deployment",
				ChangeType: changeTypeModify,
				Diffs:      []difflib.DiffRecord{{Payload: "kind: Deployment", Delta: difflib.Common}},
			},
		},
	}

	var buf bytes.Buffer
	printDiffToolReport(report, &buf)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.Len(t, lines, 2, "the external command must receive exactly the two file paths")
	require.NotEqual(t, lines[0], lines[1], "both sides must be distinct files")
	require.Equal(t, "old.yaml", filepath.Base(lines[0]))
	require.Equal(t, "new.yaml", filepath.Base(lines[1]))
}

func TestPrintDiffToolReportExitCodeOneIsNotAnError(t *testing.T) {
	report := &Report{
		diffToolCommand: diffToolHelperCommand(t, "diff"),
		Entries: []ReportEntry{
			{
				Key:        "default, nginx, Deployment (apps)",
				Kind:       "Deployment",
				ChangeType: changeTypeModify,
				Diffs:      []difflib.DiffRecord{{Payload: "kind: Deployment", Delta: difflib.Common}},
			},
		},
	}

	var buf bytes.Buffer
	stderr := captureStderr(t, func() { printDiffToolReport(report, &buf) })

	require.Contains(t, buf.String(), "helper: differences found", "tool output on stdout is preserved")
	require.NotContains(t, stderr, "Error:", "exit code 1 means differences found and is not a failure")
}

func TestPrintDiffToolReportCommandFailure(t *testing.T) {
	report := &Report{
		diffToolCommand: diffToolHelperCommand(t, "fail"),
		Entries: []ReportEntry{
			{
				Key:        "default, nginx, Deployment (apps)",
				Kind:       "Deployment",
				ChangeType: changeTypeModify,
				Diffs:      []difflib.DiffRecord{{Payload: "kind: Deployment", Delta: difflib.Common}},
			},
		},
	}

	var buf bytes.Buffer
	stderr := captureStderr(t, func() { printDiffToolReport(report, &buf) })

	require.Contains(t, stderr, "helper: boom", "the tool's own stderr is passed through")
	require.Contains(t, stderr, "Error: diff tool", "the failure is reported")
	require.Contains(t, stderr, "failed")
	require.Empty(t, buf.String(), "a failed tool produces no report output")
}

func TestPrintDiffToolReportUnclosedQuote(t *testing.T) {
	report := &Report{
		diffToolCommand: `diff -u "foo`,
		Entries: []ReportEntry{
			{
				Key:        "default, nginx, Deployment (apps)",
				Kind:       "Deployment",
				ChangeType: changeTypeModify,
				Diffs:      []difflib.DiffRecord{{Payload: "kind: Deployment", Delta: difflib.Common}},
			},
		},
	}

	var buf bytes.Buffer
	stderr := captureStderr(t, func() { printDiffToolReport(report, &buf) })

	require.Contains(t, stderr, "unclosed", "an unclosed quote must not be silently dropped")
	require.Empty(t, buf.String())
}

func TestManifestsDiffToolOutput(t *testing.T) {
	t.Setenv(DiffToolEnvVar, "")

	old := map[string]*manifest.MappingResult{
		"default, nginx, Deployment (apps)": {
			Name:    "default, nginx, Deployment (apps)",
			Kind:    "Deployment",
			Content: "kind: Deployment\nspec:\n  replicas: 2\n",
		},
	}
	updated := map[string]*manifest.MappingResult{
		"default, nginx, Deployment (apps)": {
			Name:    "default, nginx, Deployment (apps)",
			Kind:    "Deployment",
			Content: "kind: Deployment\nspec:\n  replicas: 3\n",
		},
	}

	t.Run("the command alone selects the external tool", func(t *testing.T) {
		var buf bytes.Buffer
		opts := &Options{OutputFormat: outputFormatDiff, DiffToolCommand: diffToolHelperCommand(t, "cat")}

		require.True(t, Manifests(old, updated, opts, &buf))
		require.Contains(t, buf.String(), "  replicas: 2")
		require.Contains(t, buf.String(), "  replicas: 3")
		require.Contains(t, buf.String(), "# Change: MODIFY", "expected output from the external tool")
		require.NotContains(t, buf.String(), "has changed:", "the built-in renderer must not run")
	})

	t.Run("the environment variable alone selects the external tool", func(t *testing.T) {
		t.Setenv(DiffToolEnvVar, diffToolHelperCommand(t, "cat"))
		var buf bytes.Buffer
		opts := &Options{OutputFormat: outputFormatDiff}

		require.True(t, Manifests(old, updated, opts, &buf))
		require.Contains(t, buf.String(), "# Change: MODIFY")
	})

	t.Run("the external tool wins over every built-in output", func(t *testing.T) {
		helper := diffToolHelperCommand(t, "cat")
		for _, format := range []string{"diff", "simple", "json", "template", "structured", "dyff"} {
			var buf bytes.Buffer
			opts := &Options{OutputFormat: format, DiffToolCommand: helper}

			require.True(t, Manifests(old, updated, opts, &buf))
			require.Contains(t, buf.String(), "# Change: MODIFY",
				"output %q must be overridden by the external diff command", format)
		}
	})

	t.Run("built-in output is used when no command is configured", func(t *testing.T) {
		var buf bytes.Buffer
		opts := &Options{OutputFormat: "simple"}

		require.True(t, Manifests(old, updated, opts, &buf))
		require.Contains(t, buf.String(), "to be changed.")
		require.NotContains(t, buf.String(), "# Change: MODIFY")
	})

	t.Run("honors suppress-output-line-regex", func(t *testing.T) {
		var buf bytes.Buffer
		opts := &Options{
			DiffToolCommand:           diffToolHelperCommand(t, "cat"),
			SuppressedOutputLineRegex: []string{"replicas"},
		}

		require.True(t, Manifests(old, updated, opts, &buf))
		require.NotContains(t, buf.String(), "replicas")
		require.Contains(t, buf.String(), "# Changes suppressed by --suppress-output-line-regex",
			"fully suppressed entries must leave a trace instead of vanishing")
	})

	t.Run("honors --suppress for secret kinds", func(t *testing.T) {
		oldSecret := map[string]*manifest.MappingResult{
			"default, mysecret, Secret (v1)": {
				Name:    "default, mysecret, Secret (v1)",
				Kind:    "Secret",
				Content: "kind: Secret\ndata:\n  password: aGkK\n",
			},
		}
		newSecret := map[string]*manifest.MappingResult{
			"default, mysecret, Secret (v1)": {
				Name:    "default, mysecret, Secret (v1)",
				Kind:    "Secret",
				Content: "kind: Secret\ndata:\n  password: Ynll\n",
			},
		}

		var buf bytes.Buffer
		opts := &Options{
			DiffToolCommand: diffToolHelperCommand(t, "cat"),
			SuppressedKinds: []string{"Secret"},
		}

		require.True(t, Manifests(oldSecret, newSecret, opts, &buf))
		require.Contains(t, buf.String(), "Changes suppressed on sensitive content of type Secret")
		require.NotContains(t, buf.String(), "aGkK")
		require.NotContains(t, buf.String(), "Ynll")
	})

	t.Run("redacts secrets end to end", func(t *testing.T) {
		oldSecret := map[string]*manifest.MappingResult{
			"default, mysecret, Secret (v1)": {
				Name:    "default, mysecret, Secret (v1)",
				Kind:    "Secret",
				Content: "kind: Secret\ndata:\n  password: aGkK\n",
			},
		}
		newSecret := map[string]*manifest.MappingResult{
			"default, mysecret, Secret (v1)": {
				Name:    "default, mysecret, Secret (v1)",
				Kind:    "Secret",
				Content: "kind: Secret\ndata:\n  password: Ynll\n",
			},
		}

		var buf bytes.Buffer
		opts := &Options{DiffToolCommand: diffToolHelperCommand(t, "cat")}

		require.True(t, Manifests(oldSecret, newSecret, opts, &buf))
		require.Contains(t, buf.String(), "-------- # (3 bytes)", "redaction of the old value must reach the tool's files")
		require.Contains(t, buf.String(), "++++++++ # (3 bytes)", "redaction of the new value must reach the tool's files")
		require.NotContains(t, buf.String(), "aGkK")
		require.NotContains(t, buf.String(), "Ynll")
	})

	t.Run("no output when there are no changes", func(t *testing.T) {
		var buf bytes.Buffer
		opts := &Options{DiffToolCommand: diffToolHelperCommand(t, "cat")}

		require.False(t, Manifests(old, old, opts, &buf))
		require.Empty(t, buf.String())
	})
}

func TestDiffToolEnabled(t *testing.T) {
	t.Run("disabled when nothing is configured", func(t *testing.T) {
		t.Setenv(DiffToolEnvVar, "")
		require.False(t, (&Options{OutputFormat: outputFormatDiff}).DiffTool())
	})

	t.Run("enabled by the command", func(t *testing.T) {
		t.Setenv(DiffToolEnvVar, "")
		require.True(t, (&Options{DiffToolCommand: "difft"}).DiffTool())
	})

	t.Run("enabled by the environment variable", func(t *testing.T) {
		t.Setenv(DiffToolEnvVar, "difft")
		require.True(t, (&Options{}).DiffTool())
	})

	t.Run("a blank command does not enable it", func(t *testing.T) {
		t.Setenv(DiffToolEnvVar, "   ")
		require.False(t, (&Options{DiffToolCommand: "  "}).DiffTool())
	})

	t.Run("the environment variable can be ignored in favor of an explicit flag", func(t *testing.T) {
		t.Setenv(DiffToolEnvVar, "difft")
		opts := &Options{}
		require.True(t, opts.DiffTool())

		opts.IgnoreDiffToolEnvVar()
		require.False(t, opts.DiffTool(), "an explicit flag must win over the environment")
	})

	t.Run("structured output is disabled while an external tool is used", func(t *testing.T) {
		t.Setenv(DiffToolEnvVar, "")
		opts := &Options{OutputFormat: "structured", DiffToolCommand: "difft"}
		require.True(t, opts.DiffTool())
		require.False(t, opts.StructuredOutput(),
			"the external tool needs the line diffs that structured output skips")
	})
}

func TestPrintDiffToolReportNoCommand(t *testing.T) {
	t.Setenv(DiffToolEnvVar, "")

	report := &Report{
		Entries: []ReportEntry{
			{
				Key:        "default, nginx, Deployment (apps)",
				Kind:       "Deployment",
				ChangeType: changeTypeModify,
				Diffs:      []difflib.DiffRecord{{Payload: "kind: Deployment", Delta: difflib.Common}},
			},
		},
	}

	var buf bytes.Buffer
	require.NotPanics(t, func() { printDiffToolReport(report, &buf) })
	require.Empty(t, buf.String(), "without a command there is nothing to render")
}

package diff

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/aryann/difflib"
	"github.com/stretchr/testify/require"

	"github.com/databus23/helm-diff/v3/manifest"
)

func TestSplitDiffToolCommand(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected []string
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, splitDiffToolCommand(tt.command))
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
				ChangeType: "MODIFY",
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

	require.Equal(t, "---\n# Source: default, nginx, Deployment (apps)\nkind: Deployment\n  replicas: 2\n", string(oldContent))
	require.Equal(t, "---\n# Source: default, nginx, Deployment (apps)\nkind: Deployment\n  replicas: 3\n", string(newContent))
}

func TestWriteDiffToolSidesSuppressedKind(t *testing.T) {
	report := &Report{
		Entries: []ReportEntry{
			{
				Key:             "default, mysecret, Secret (v1)",
				Kind:            "Secret",
				SuppressedKinds: []string{"Secret"},
				ChangeType:      "MODIFY",
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

func TestPrintDiffToolReport(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("relies on a POSIX diff implementation")
	}

	report := &Report{
		diffToolCommand: "diff -u -N",
		Entries: []ReportEntry{
			{
				Key:        "default, nginx, Deployment (apps)",
				Kind:       "Deployment",
				ChangeType: "MODIFY",
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
	require.Contains(t, output, "-  replicas: 2")
	require.Contains(t, output, "+  replicas: 3")
}

func TestPrintDiffToolReportEmpty(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("relies on a POSIX diff implementation")
	}

	report := &Report{diffToolCommand: "diff -u -N", Entries: []ReportEntry{}}

	var buf bytes.Buffer
	printDiffToolReport(report, &buf)

	require.Empty(t, buf.String())
}

func TestPrintDiffToolReportPassesBothFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("relies on a POSIX shell")
	}

	report := &Report{
		diffToolCommand: "echo",
		Entries: []ReportEntry{
			{
				Key:        "default, nginx, Deployment (apps)",
				Kind:       "Deployment",
				ChangeType: "MODIFY",
				Diffs:      []difflib.DiffRecord{{Payload: "kind: Deployment", Delta: difflib.Common}},
			},
		},
	}

	var buf bytes.Buffer
	printDiffToolReport(report, &buf)

	fields := bytes.Fields(buf.Bytes())
	require.Len(t, fields, 2, "the external command must receive exactly the two file paths")
	require.NotEqual(t, string(fields[0]), string(fields[1]), "both sides must be distinct files")
}

func TestManifestsDiffToolOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("relies on a POSIX diff implementation")
	}
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
		opts := &Options{OutputFormat: "diff", DiffToolCommand: "diff -u -N"}

		require.True(t, Manifests(old, updated, opts, &buf))
		require.Contains(t, buf.String(), "-  replicas: 2")
		require.Contains(t, buf.String(), "+  replicas: 3")
		require.Contains(t, buf.String(), "@@", "expected unified diff output from the external tool")
	})

	t.Run("the environment variable alone selects the external tool", func(t *testing.T) {
		t.Setenv(DiffToolEnvVar, "diff -u -N")
		var buf bytes.Buffer
		opts := &Options{OutputFormat: "diff"}

		require.True(t, Manifests(old, updated, opts, &buf))
		require.Contains(t, buf.String(), "@@")
	})

	t.Run("the external tool wins over every built-in output", func(t *testing.T) {
		for _, format := range []string{"diff", "simple", "json", "template", "structured", "dyff"} {
			var buf bytes.Buffer
			opts := &Options{OutputFormat: format, DiffToolCommand: "diff -u -N"}

			require.True(t, Manifests(old, updated, opts, &buf))
			require.Contains(t, buf.String(), "@@",
				"output %q must be overridden by the external diff command", format)
		}
	})

	t.Run("built-in output is used when no command is configured", func(t *testing.T) {
		var buf bytes.Buffer
		opts := &Options{OutputFormat: "simple"}

		require.True(t, Manifests(old, updated, opts, &buf))
		require.Contains(t, buf.String(), "to be changed.")
		require.NotContains(t, buf.String(), "@@")
	})

	t.Run("honors suppress-output-line-regex", func(t *testing.T) {
		var buf bytes.Buffer
		opts := &Options{
			DiffToolCommand:           "diff -u -N",
			SuppressedOutputLineRegex: []string{"replicas"},
		}

		Manifests(old, updated, opts, &buf)
		require.NotContains(t, buf.String(), "replicas")
	})

	t.Run("no output when there are no changes", func(t *testing.T) {
		var buf bytes.Buffer
		opts := &Options{DiffToolCommand: "diff -u -N"}

		require.False(t, Manifests(old, old, opts, &buf))
		require.Empty(t, buf.String())
	})
}

func TestDiffToolEnabled(t *testing.T) {
	t.Run("disabled when nothing is configured", func(t *testing.T) {
		t.Setenv(DiffToolEnvVar, "")
		require.False(t, (&Options{OutputFormat: "diff"}).DiffTool())
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
				ChangeType: "MODIFY",
				Diffs:      []difflib.DiffRecord{{Payload: "kind: Deployment", Delta: difflib.Common}},
			},
		},
	}

	var buf bytes.Buffer
	require.NotPanics(t, func() { printDiffToolReport(report, &buf) })
	require.Empty(t, buf.String(), "without a command there is nothing to render")
}

func TestPrintDiffToolReportCommandFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("relies on a POSIX shell")
	}

	report := &Report{
		diffToolCommand: "helm-diff-no-such-external-tool",
		Entries: []ReportEntry{
			{
				Key:        "default, nginx, Deployment (apps)",
				Kind:       "Deployment",
				ChangeType: "MODIFY",
				Diffs:      []difflib.DiffRecord{{Payload: "kind: Deployment", Delta: difflib.Common}},
			},
		},
	}

	var buf bytes.Buffer
	require.NotPanics(t, func() { printDiffToolReport(report, &buf) })
}

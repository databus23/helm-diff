package diff

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aryann/difflib"
)

const (
	// DiffToolEnvVar holds the diff tool command line when no flag is given.
	// Setting it is by itself a request to render the diff with that command.
	DiffToolEnvVar = "HELM_DIFF_TOOL"
)

// diffToolCommand returns the configured command, falling back to the
// environment variable. There is deliberately no default: helm-diff never picks a
// diff tool on the user's behalf, so an unset command is a configuration error
// rather than an invitation to guess.
func diffToolCommand(configured string) string {
	if strings.TrimSpace(configured) != "" {
		return configured
	}

	return strings.TrimSpace(os.Getenv(DiffToolEnvVar))
}

// splitDiffToolCommand splits a command line into a command and its arguments.
// Whitespace separates arguments unless quoted, so that paths containing spaces can
// be expressed as `"/opt/my tools/diff" -u`. The result is executed directly rather
// than through a shell, so the generated file paths cannot be expanded or injected.
func splitDiffToolCommand(command string) []string {
	var (
		args    []string
		current strings.Builder
		quote   rune
		started bool
	)

	for _, r := range command {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
			started = true
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			if started {
				args = append(args, current.String())
				current.Reset()
				started = false
			}
		default:
			current.WriteRune(r)
			started = true
		}
	}

	if started {
		args = append(args, current.String())
	}

	return args
}

func setupDiffToolReport(r *Report) {
	r.format.output = printDiffToolReport
}

// printDiffToolReport writes both sides of the report to temporary files and
// appends their paths as the last two arguments of the diff tool command,
// streaming its output to `to`.
func printDiffToolReport(r *Report, to io.Writer) {
	if len(r.Entries) == 0 {
		return
	}

	args := splitDiffToolCommand(diffToolCommand(r.diffToolCommand))
	if len(args) == 0 {
		// Unreachable: this printer is only installed once a command is configured.
		fmt.Fprintf(os.Stderr, "Error: no diff tool configured\n")
		return
	}

	oldFile, newFile, cleanup, err := createDiffToolFiles()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: unable to create temporary files for the diff tool: %v\n", err)
		return
	}
	defer cleanup()

	if err := writeDiffToolSides(r, oldFile, newFile); err != nil {
		fmt.Fprintf(os.Stderr, "Error: unable to write manifests for the diff tool: %v\n", err)
		return
	}

	cmd := exec.Command(args[0], append(args[1:], oldFile, newFile)...)
	cmd.Stdout = to
	cmd.Stderr = os.Stderr

	// Exit code 1 conventionally means "differences found", the expected case here.
	// Other failures are reported but must not abort helm-diff, whose own exit code
	// is derived from the report.
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return
		}
		fmt.Fprintf(os.Stderr, "Error: diff tool %q failed: %v\n", strings.Join(args, " "), err)
	}
}

func createDiffToolFiles() (oldFile, newFile string, cleanup func(), err error) {
	// Stable basenames in a private directory: diff tools label their output
	// with the file names, which randomized temp names would make unreadable.
	dir, err := os.MkdirTemp("", "helm-diff-tool")
	if err != nil {
		return "", "", nil, err
	}

	return filepath.Join(dir, "current.yaml"),
		filepath.Join(dir, "new.yaml"),
		func() { _ = os.RemoveAll(dir) },
		nil
}

// writeDiffToolSides reconstructs the old and the new manifests from the report
// entries rather than from the raw manifests, which keeps secret redaction and line
// suppression in effect for whatever the diff tool receives.
func writeDiffToolSides(r *Report, oldPath, newPath string) error {
	var current, next strings.Builder

	for _, entry := range r.Entries {
		header := "---\n# Source: " + entry.Key + "\n"
		_, _ = current.WriteString(header)
		_, _ = next.WriteString(header)

		if containsKind(entry.SuppressedKinds, entry.Kind) {
			// Identical placeholder on both sides: the tool must report no change
			// rather than receive the suppressed content.
			placeholder := fmt.Sprintf("# Changes suppressed on sensitive content of type %s\n", entry.Kind)
			_, _ = current.WriteString(placeholder)
			_, _ = next.WriteString(placeholder)
			continue
		}

		for _, record := range entry.Diffs {
			switch record.Delta {
			case difflib.Common:
				_, _ = current.WriteString(record.Payload + "\n")
				_, _ = next.WriteString(record.Payload + "\n")
			case difflib.LeftOnly:
				_, _ = current.WriteString(record.Payload + "\n")
			case difflib.RightOnly:
				_, _ = next.WriteString(record.Payload + "\n")
			}
		}
	}

	if err := os.WriteFile(oldPath, []byte(current.String()), 0o600); err != nil {
		return err
	}

	return os.WriteFile(newPath, []byte(next.String()), 0o600)
}

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

// diffToolCommand merges the --diff-tool flag with the HELM_DIFF_TOOL
// environment variable, the flag winning. Callers that must honor the
// explicit-flag precedence use Options.configuredDiffToolCommand instead;
// this free function deliberately knows nothing about that. There is
// deliberately no default: helm-diff never picks a diff tool on the user's
// behalf, so an unset command is a configuration error rather than an
// invitation to guess.
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
// An unclosed quote is an error: silently dropping it would hand the tool a
// different command line than the user typed.
func splitDiffToolCommand(command string) ([]string, error) {
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

	if quote != 0 {
		return nil, fmt.Errorf("unclosed %q quote in diff tool command %q", quote, command)
	}

	if started {
		args = append(args, current.String())
	}

	return args, nil
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

	args, err := splitDiffToolCommand(r.diffToolCommand)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
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
	// with the file names, so old.yaml/new.yaml pair up naturally where random
	// temp names would be unreadable.
	dir, err := os.MkdirTemp("", "helm-diff-tool")
	if err != nil {
		return "", "", nil, err
	}

	return filepath.Join(dir, "old.yaml"),
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
		// The tool only sees file content, so a header comment carries what the
		// built-in output would print around the diff: the resource the entry
		// refers to (the manifest content keeps its own "# Source:" template
		// path) and the change type, which would otherwise be invisible to the
		// tool user (ADD, REMOVE, MODIFY, ...).
		header := fmt.Sprintf("---\n# Resource: %s\n# Change: %s\n", entry.Key, entry.ChangeType)
		_, _ = current.WriteString(header)
		_, _ = next.WriteString(header)

		switch {
		case containsKind(entry.SuppressedKinds, entry.Kind):
			// Identical placeholder on both sides: the tool must report no change
			// rather than receive the suppressed content.
			placeholder := fmt.Sprintf("# Changes suppressed on sensitive content of type %s\n", entry.Kind)
			_, _ = current.WriteString(placeholder)
			_, _ = next.WriteString(placeholder)
		case len(entry.Diffs) == 0:
			// Every changed line was removed by --suppress-output-line-regex: such
			// entries arrive either flipped to MODIFY_SUPPRESSED or, for the other
			// change types (ADD/REMOVE/OWNERSHIP are not rewritten), with empty
			// diffs. Without a placeholder the tool would report no difference at
			// all while helm-diff still exits with "changes found".
			placeholder := "# Changes suppressed by --suppress-output-line-regex\n"
			_, _ = current.WriteString(placeholder)
			_, _ = next.WriteString(placeholder)
		default:
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
	}

	if err := os.WriteFile(oldPath, []byte(current.String()), 0o600); err != nil {
		return err
	}

	return os.WriteFile(newPath, []byte(next.String()), 0o600)
}

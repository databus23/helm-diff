package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func shouldRunFakeHelm() bool {
	if os.Getenv("HELM_DIFF_FAKE_HELM") != "1" {
		return false
	}
	if len(os.Args) < 2 {
		return false
	}
	return !strings.HasPrefix(os.Args[1], "-test.")
}

// printFakeHelmOutput prints the output for a fake helm invocation.
// A `helm version` call prints helm version build info, so that the version
// checks in cmd (see getHelmVersion) work against the fake helm. The version
// output can be overridden via HELM_DIFF_FAKE_VERSION_OUTPUT.
// Any other invocation prints HELM_DIFF_FAKE_OUTPUT.
func printFakeHelmOutput() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		if v := os.Getenv("HELM_DIFF_FAKE_VERSION_OUTPUT"); v != "" {
			fmt.Print(v)
		} else {
			fmt.Println(`version.BuildInfo{Version:"v3.18.0"}`)
		}
	} else {
		fmt.Print(os.Getenv("HELM_DIFF_FAKE_OUTPUT"))
	}
}

func TestMain(m *testing.M) {
	if shouldRunFakeHelm() {
		mode := os.Getenv("HELM_DIFF_FAKE_HELM_MODE")
		switch mode {
		case "error":
			fmt.Fprintln(os.Stderr, "error: chart not found")
			os.Exit(1)
		case "dual":
			countFile := os.Getenv("HELM_DIFF_FAKE_COUNT_FILE")
			data, err := os.ReadFile(countFile)
			if err != nil && !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "failed to read count file %q: %v\n", countFile, err)
				os.Exit(1)
			}
			count := 0
			if len(data) > 0 {
				if _, err := fmt.Sscanf(string(data), "%d", &count); err != nil {
					fmt.Fprintf(os.Stderr, "failed to parse count from %q: %v\n", string(data), err)
					os.Exit(1)
				}
			}
			count++
			if err := os.WriteFile(countFile, []byte(fmt.Sprintf("%d", count)), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "failed to write count file %q: %v\n", countFile, err)
				os.Exit(1)
			}
			if count == 1 {
				fmt.Print(os.Getenv("HELM_DIFF_FAKE_OUTPUT_1"))
			} else {
				fmt.Print(os.Getenv("HELM_DIFF_FAKE_OUTPUT_2"))
			}
		case "capture_args":
			argsFile := os.Getenv("HELM_DIFF_FAKE_ARGS_FILE")
			if argsFile != "" {
				f, err := os.OpenFile(argsFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
				if err == nil {
					_, _ = fmt.Fprintln(f, strings.Join(os.Args[1:], " "))
					_ = f.Close()
				}
			}
			printFakeHelmOutput()
		default:
			printFakeHelmOutput()
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestFakeHelmVersionOutput(t *testing.T) {
	exe, err := os.Executable()
	require.NoError(t, err)

	t.Run("custom version output via HELM_DIFF_FAKE_VERSION_OUTPUT", func(t *testing.T) {
		t.Setenv("HELM_DIFF_FAKE_HELM", "1")
		t.Setenv("HELM_DIFF_FAKE_HELM_MODE", "default")
		t.Setenv("HELM_DIFF_FAKE_VERSION_OUTPUT", `version.BuildInfo{Version:"v3.99.0"}`)

		out, err := exec.Command(exe, "version").CombinedOutput()
		require.NoError(t, err)
		require.Equal(t, `version.BuildInfo{Version:"v3.99.0"}`, string(out))
	})

	t.Run("default version build info", func(t *testing.T) {
		t.Setenv("HELM_DIFF_FAKE_HELM", "1")
		t.Setenv("HELM_DIFF_FAKE_HELM_MODE", "capture_args")

		out, err := exec.Command(exe, "version").CombinedOutput()
		require.NoError(t, err)
		require.Equal(t, "version.BuildInfo{Version:\"v3.18.0\"}\n", string(out))
	})

	t.Run("non-version invocations print HELM_DIFF_FAKE_OUTPUT", func(t *testing.T) {
		t.Setenv("HELM_DIFF_FAKE_HELM", "1")
		t.Setenv("HELM_DIFF_FAKE_HELM_MODE", "default")
		t.Setenv("HELM_DIFF_FAKE_OUTPUT", "manifest-output")

		out, err := exec.Command(exe, "get", "manifest").CombinedOutput()
		require.NoError(t, err)
		require.Equal(t, "manifest-output", string(out))
	})
}

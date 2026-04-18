package cmd

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestLocalCmdArgValidation(t *testing.T) {
	cases := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "no arguments",
			args:        []string{},
			expectError: true,
		},
		{
			name:        "one argument",
			args:        []string{"chart1"},
			expectError: true,
		},
		{
			name:        "three arguments",
			args:        []string{"chart1", "chart2", "chart3"},
			expectError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := localCmd()
			cmd.SetArgs(tc.args)
			err := cmd.Execute()

			if tc.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestLocalCmdExecution(t *testing.T) {
	manifestYAML := `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  key: value
`
	setupFakeHelm(t, "default", manifestYAML, "", "")

	chart1 := t.TempDir()
	chart2 := t.TempDir()

	cmd := localCmd()
	cmd.SetArgs([]string{chart1, chart2})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}
}

func TestLocalCmdNoChanges(t *testing.T) {
	manifestYAML := `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  key: value
`
	setupFakeHelm(t, "default", manifestYAML, "", "")

	chart1 := t.TempDir()
	chart2 := t.TempDir()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := localCmd()
	cmd.SetArgs([]string{chart1, chart2})

	err := cmd.Execute()
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Expected no error but got: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	if buf.String() != "" {
		t.Errorf("Expected no output when charts are identical, got: %q", buf.String())
	}
}

func TestLocalCmdWithChanges(t *testing.T) {
	manifest1 := `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  key: value1
`
	manifest2 := `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  key: value2
`
	setupFakeHelmDual(t, manifest1, manifest2)

	chart1 := t.TempDir()
	chart2 := t.TempDir()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := localCmd()
	cmd.SetArgs([]string{chart1, chart2})

	err := cmd.Execute()
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Expected no error but got: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "value1") || !strings.Contains(output, "value2") {
		t.Errorf("Expected diff output containing value1 and value2, got: %q", output)
	}
}

func TestLocalCmdDetailedExitCode(t *testing.T) {
	manifest1 := `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  key: value1
`
	manifest2 := `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  key: value2
`
	setupFakeHelmDual(t, manifest1, manifest2)

	chart1 := t.TempDir()
	chart2 := t.TempDir()

	cmd := localCmd()
	cmd.SetArgs([]string{chart1, chart2, "--detailed-exitcode"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Expected error with exit code 2 but got nil")
	}

	var diffErr Error
	if !errors.As(err, &diffErr) {
		t.Fatalf("Expected Error type but got %T: %v", err, err)
	}
	if diffErr.Code != 2 {
		t.Errorf("Expected exit code 2 but got %d", diffErr.Code)
	}
}

func TestLocalCmdDetailedExitCodeNoChanges(t *testing.T) {
	manifestYAML := `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  key: value
`
	setupFakeHelm(t, "default", manifestYAML, "", "")

	chart1 := t.TempDir()
	chart2 := t.TempDir()

	cmd := localCmd()
	cmd.SetArgs([]string{chart1, chart2, "--detailed-exitcode"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("Expected no error when no changes, but got: %v", err)
	}
}

func TestLocalCmdNamespace(t *testing.T) {
	manifestYAML := `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key: value
`
	argsFile := t.TempDir() + "/args"
	setupFakeHelm(t, "capture_args", manifestYAML, argsFile, "")

	chart1 := t.TempDir()
	chart2 := t.TempDir()

	cmd := localCmd()
	cmd.SetArgs([]string{chart1, chart2, "--namespace", "myns"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Expected no error but got: %v", err)
	}

	args1, _ := os.ReadFile(argsFile)
	if !strings.Contains(string(args1), "--namespace myns") {
		t.Errorf("Expected --namespace myns in helm template args, got: %q", string(args1))
	}
}

func TestLocalCmdHelmTemplateError(t *testing.T) {
	setupFakeHelm(t, "error", "", "", "")

	chart1 := t.TempDir()
	chart2 := t.TempDir()

	cmd := localCmd()
	cmd.SetArgs([]string{chart1, chart2})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Expected error when helm template fails but got nil")
	}
}

func setupFakeHelm(t *testing.T, mode, output, argsFile, countFile string) {
	t.Helper()
	t.Setenv("HELM_DIFF_FAKE_HELM", "1")
	t.Setenv("HELM_DIFF_FAKE_HELM_MODE", mode)
	t.Setenv("HELM_DIFF_FAKE_OUTPUT", output)
	t.Setenv("HELM_DIFF_FAKE_ARGS_FILE", argsFile)
	t.Setenv("HELM_DIFF_FAKE_COUNT_FILE", countFile)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HELM_BIN", exe)
}

func setupFakeHelmDual(t *testing.T, manifest1, manifest2 string) {
	t.Helper()
	countFile := t.TempDir() + "/call_count"
	setupFakeHelm(t, "dual", "", "", countFile)
	t.Setenv("HELM_DIFF_FAKE_OUTPUT_1", manifest1)
	t.Setenv("HELM_DIFF_FAKE_OUTPUT_2", manifest2)
}

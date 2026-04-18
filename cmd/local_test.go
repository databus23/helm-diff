package cmd

import (
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

	output, err := captureStdout(func() {
		cmd := localCmd()
		cmd.SetArgs([]string{chart1, chart2})

		if execErr := cmd.Execute(); execErr != nil {
			t.Errorf("Expected no error but got: %v", execErr)
		}
	})

	if err != nil {
		t.Fatalf("Failed to capture stdout: %v", err)
	}
	if output != "" {
		t.Errorf("Expected no output when charts are identical, got: %q", output)
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

	output, err := captureStdout(func() {
		cmd := localCmd()
		cmd.SetArgs([]string{chart1, chart2})

		if execErr := cmd.Execute(); execErr != nil {
			t.Errorf("Expected no error but got: %v", execErr)
		}
	})

	if err != nil {
		t.Fatalf("Failed to capture stdout: %v", err)
	}

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

	args1, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("Expected fake helm args file to be readable, but got: %v", err)
	}
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

func TestPrepareStdinValues(t *testing.T) {
	l := &local{
		valueFiles: valueFiles{"-", "-", "real-values.yaml"},
	}

	stdinContent := `key: stdin-value
`
	tmpStdin, err := os.CreateTemp("", "fake-stdin")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpStdin.Name())
	if _, err := tmpStdin.WriteString(stdinContent); err != nil {
		t.Fatal(err)
	}
	if err := tmpStdin.Close(); err != nil {
		t.Fatal(err)
	}

	oldStdin := os.Stdin
	f, err := os.Open(tmpStdin.Name())
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = f
	defer func() {
		os.Stdin = oldStdin
		f.Close()
	}()

	cleanup, err := l.prepareStdinValues()
	if err != nil {
		t.Fatalf("Expected no error but got: %v", err)
	}
	if cleanup == nil {
		t.Fatal("Expected cleanup function but got nil")
	}

	if l.valueFiles[0] == "-" {
		t.Error("Expected first valueFile to be replaced with temp file path")
	}
	if l.valueFiles[1] == "-" {
		t.Error("Expected second valueFile to be replaced with temp file path")
	}
	if l.valueFiles[0] != l.valueFiles[1] {
		t.Errorf("Expected both '-' entries to point to the same temp file, got %q and %q", l.valueFiles[0], l.valueFiles[1])
	}
	if _, err := os.Stat(l.valueFiles[0]); os.IsNotExist(err) {
		t.Errorf("Expected temp file %q to exist", l.valueFiles[0])
	}
	if l.valueFiles[2] != "real-values.yaml" {
		t.Errorf("Expected third valueFile to be unchanged, got %q", l.valueFiles[2])
	}

	data, err := os.ReadFile(l.valueFiles[0])
	if err != nil {
		t.Fatalf("Failed to read temp file: %v", err)
	}
	if string(data) != stdinContent {
		t.Errorf("Expected temp file to contain stdin content %q, got %q", stdinContent, string(data))
	}

	cleanup()

	if _, err := os.Stat(l.valueFiles[0]); !os.IsNotExist(err) {
		t.Errorf("Expected temp file %q to be removed after cleanup", l.valueFiles[0])
	}
}

func TestPrepareStdinValuesNoStdin(t *testing.T) {
	l := &local{
		valueFiles: valueFiles{"values1.yaml", "values2.yaml"},
	}

	cleanup, err := l.prepareStdinValues()
	if err != nil {
		t.Fatalf("Expected no error but got: %v", err)
	}
	if cleanup != nil {
		t.Error("Expected nil cleanup function when no stdin values")
	}
	if l.valueFiles[0] != "values1.yaml" || l.valueFiles[1] != "values2.yaml" {
		t.Errorf("Expected valueFiles to be unchanged, got %v", l.valueFiles)
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

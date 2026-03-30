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
	tmpDir := t.TempDir()
	fakeHelm := tmpDir + "/helm"
	manifestYAML := `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  key: value
`

	err := os.WriteFile(fakeHelm, []byte(`#!/bin/sh
cat <<EOF
`+manifestYAML+`
EOF
`), 0755)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("HELM_BIN", fakeHelm)

	chart1 := t.TempDir()
	chart2 := t.TempDir()

	cmd := localCmd()
	cmd.SetArgs([]string{chart1, chart2})

	err = cmd.Execute()
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}
}

func TestLocalCmdNoChanges(t *testing.T) {
	tmpDir := t.TempDir()
	fakeHelm := tmpDir + "/helm"
	manifestYAML := `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  key: value
`
	err := os.WriteFile(fakeHelm, []byte(`#!/bin/sh
cat <<'EOF'
`+manifestYAML+`
EOF
`), 0755)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("HELM_BIN", fakeHelm)

	chart1 := t.TempDir()
	chart2 := t.TempDir()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := localCmd()
	cmd.SetArgs([]string{chart1, chart2})

	err = cmd.Execute()
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
	tmpDir := t.TempDir()
	fakeHelm := tmpDir + "/helm"

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

	script := `#!/bin/sh
CALL_COUNT="` + tmpDir + `/call_count"
COUNT=$(cat "$CALL_COUNT" 2>/dev/null || echo "0")
COUNT=$((COUNT + 1))
echo "$COUNT" > "$CALL_COUNT"
if [ "$COUNT" = "1" ]; then
cat <<'MANIFEST1'
` + manifest1 + `
MANIFEST1
else
cat <<'MANIFEST2'
` + manifest2 + `
MANIFEST2
fi
`
	err := os.WriteFile(fakeHelm, []byte(script), 0755)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("HELM_BIN", fakeHelm)

	chart1 := t.TempDir()
	chart2 := t.TempDir()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := localCmd()
	cmd.SetArgs([]string{chart1, chart2})

	err = cmd.Execute()
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
	tmpDir := t.TempDir()
	fakeHelm := tmpDir + "/helm"

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

	script := `#!/bin/sh
CALL_COUNT="` + tmpDir + `/call_count"
COUNT=$(cat "$CALL_COUNT" 2>/dev/null || echo "0")
COUNT=$((COUNT + 1))
echo "$COUNT" > "$CALL_COUNT"
if [ "$COUNT" = "1" ]; then
cat <<'MANIFEST1'
` + manifest1 + `
MANIFEST1
else
cat <<'MANIFEST2'
` + manifest2 + `
MANIFEST2
fi
`
	err := os.WriteFile(fakeHelm, []byte(script), 0755)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("HELM_BIN", fakeHelm)

	chart1 := t.TempDir()
	chart2 := t.TempDir()

	cmd := localCmd()
	cmd.SetArgs([]string{chart1, chart2, "--detailed-exitcode"})

	err = cmd.Execute()
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
	tmpDir := t.TempDir()
	fakeHelm := tmpDir + "/helm"
	manifestYAML := `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  key: value
`
	err := os.WriteFile(fakeHelm, []byte(`#!/bin/sh
cat <<'EOF'
`+manifestYAML+`
EOF
`), 0755)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("HELM_BIN", fakeHelm)

	chart1 := t.TempDir()
	chart2 := t.TempDir()

	cmd := localCmd()
	cmd.SetArgs([]string{chart1, chart2, "--detailed-exitcode"})

	err = cmd.Execute()
	if err != nil {
		t.Errorf("Expected no error when no changes, but got: %v", err)
	}
}

func TestLocalCmdNamespace(t *testing.T) {
	tmpDir := t.TempDir()
	fakeHelm := tmpDir + "/helm"

	script := `#!/bin/sh
echo "$@" > ` + tmpDir + `/args
cat <<'EOF'
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key: value
EOF
`
	err := os.WriteFile(fakeHelm, []byte(script), 0755)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("HELM_BIN", fakeHelm)

	chart1 := t.TempDir()
	chart2 := t.TempDir()

	cmd := localCmd()
	cmd.SetArgs([]string{chart1, chart2, "--namespace", "myns"})

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("Expected no error but got: %v", err)
	}

	args1, _ := os.ReadFile(tmpDir + "/args")
	if !strings.Contains(string(args1), "--namespace myns") {
		t.Errorf("Expected --namespace myns in helm template args, got: %q", string(args1))
	}
}

func TestLocalCmdHelmTemplateError(t *testing.T) {
	tmpDir := t.TempDir()
	fakeHelm := tmpDir + "/helm"

	script := `#!/bin/sh
echo "error: chart not found" >&2
exit 1
`
	err := os.WriteFile(fakeHelm, []byte(script), 0755)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("HELM_BIN", fakeHelm)

	chart1 := t.TempDir()
	chart2 := t.TempDir()

	cmd := localCmd()
	cmd.SetArgs([]string{chart1, chart2})

	err = cmd.Execute()
	if err == nil {
		t.Fatal("Expected error when helm template fails but got nil")
	}
}

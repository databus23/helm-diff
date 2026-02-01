package cmd

import (
	"os"
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

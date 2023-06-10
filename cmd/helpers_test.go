package cmd

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func captureStdout(f func()) (string, error) {
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w

	defer func() {
		os.Stdout = old
	}()

	f()

	w.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func TestCaptureStdout(t *testing.T) {
	output, err := captureStdout(func() {
		_, _ = os.Stdout.Write([]byte("test"))
	})
	require.NoError(t, err)
	require.Equal(t, "test", output)
}

func TestIsDebug(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{
			name:     "HELM_DEBUG is true",
			envValue: "true",
			expected: true,
		},
		{
			name:     "HELM_DEBUG is false",
			envValue: "false",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HELM_DEBUG", tt.envValue)
			require.Equalf(t, tt.expected, isDebug(), "Expected %v but got %v", tt.expected, isDebug())
		})
	}
}

func TestDebugPrint(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected string
	}{
		{
			name:     "non-empty when HELM_DEBUG is true",
			envValue: "true",
			expected: "test\n",
		},
		{
			name:     "empty when HELM_DEBUG is false",
			envValue: "false",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HELM_DEBUG", tt.envValue)
			output, err := captureStdout(func() {
				debugPrint("test")
			})
			require.NoError(t, err)
			require.Equalf(t, tt.expected, output, "Expected %v but got %v", tt.expected, output)
		})
	}
}

func TestOutputWithRichError(t *testing.T) {
	tests := []struct {
		name           string
		envValue       string
		cmd            *exec.Cmd
		expected       string
		expectedStdout string
	}{
		{
			name:           "debug output in stdout when HELM_DEBUG is true",
			envValue:       "true",
			cmd:            exec.Command("echo", "test1"),
			expected:       "test1\n",
			expectedStdout: "Executing echo test1\n",
		},
		{
			name:           "non-debug output in stdout when HELM_DEBUG is false",
			envValue:       "false",
			cmd:            exec.Command("echo", "test2"),
			expected:       "test2\n",
			expectedStdout: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HELM_DEBUG", tt.envValue)
			var (
				stdoutString        string
				outBytes            []byte
				funcErr, captureErr error
			)
			stdoutString, captureErr = captureStdout(func() {
				outBytes, funcErr = outputWithRichError(tt.cmd)
			})
			require.NoError(t, captureErr)
			require.NoError(t, funcErr)
			require.Equalf(t, tt.expected, string(outBytes), "Expected %v but got %v", tt.expected, string(outBytes))
			require.Equalf(t, tt.expectedStdout, stdoutString, "Expected %v but got %v", tt.expectedStdout, stdoutString)
		})
	}
}

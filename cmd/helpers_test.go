package cmd

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"testing"

	"github.com/spf13/cobra"
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

func TestResolveStorageNamespace(t *testing.T) {
	cases := []struct {
		name             string
		storageNamespace string
		namespace        string
		expected         string
	}{
		{
			name:             "storage namespace set returns storage namespace",
			storageNamespace: "flux-system",
			namespace:        "prod-apps",
			expected:         "flux-system",
		},
		{
			name:             "storage namespace empty returns target namespace",
			storageNamespace: "",
			namespace:        "prod-apps",
			expected:         "prod-apps",
		},
		{
			name:             "both empty returns empty",
			storageNamespace: "",
			namespace:        "",
			expected:         "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actual := resolveStorageNamespace(tc.storageNamespace, tc.namespace)
			require.Equal(t, tc.expected, actual)
		})
	}
}

func TestStorageNamespaceEnvVarDefaults(t *testing.T) {
	newCommands := map[string]func() *cobra.Command{
		"upgrade":  newChartCommand,
		"revision": revisionCmd,
		"rollback": rollbackCmd,
	}

	t.Run("env vars populate flag defaults", func(t *testing.T) {
		t.Setenv("HELM_DIFF_STORAGE_NAMESPACE", "env-storage")
		t.Setenv("HELM_NAMESPACE", "env-target")

		for name, newCmd := range newCommands {
			t.Run(name, func(t *testing.T) {
				cmd := newCmd()
				storageNs, err := cmd.Flags().GetString("storage-namespace")
				require.NoError(t, err)
				require.Equal(t, "env-storage", storageNs)

				ns, err := cmd.Flags().GetString("namespace")
				require.NoError(t, err)
				require.Equal(t, "env-target", ns)
			})
		}
	})

	t.Run("explicit flags take precedence over env var defaults", func(t *testing.T) {
		t.Setenv("HELM_DIFF_STORAGE_NAMESPACE", "env-storage")
		t.Setenv("HELM_NAMESPACE", "env-target")

		cmd := newChartCommand()
		err := cmd.ParseFlags([]string{"--storage-namespace", "flag-storage", "-n", "flag-target"})
		require.NoError(t, err)

		storageNs, _ := cmd.Flags().GetString("storage-namespace")
		ns, _ := cmd.Flags().GetString("namespace")
		require.Equal(t, "flag-storage", storageNs)
		require.Equal(t, "flag-target", ns)
	})

	t.Run("empty env vars leave flags empty", func(t *testing.T) {
		t.Setenv("HELM_DIFF_STORAGE_NAMESPACE", "")
		t.Setenv("HELM_NAMESPACE", "")

		for name, newCmd := range newCommands {
			t.Run(name, func(t *testing.T) {
				cmd := newCmd()
				storageNs, _ := cmd.Flags().GetString("storage-namespace")
				require.Empty(t, storageNs)

				ns, _ := cmd.Flags().GetString("namespace")
				require.Empty(t, ns)
			})
		}
	})
}

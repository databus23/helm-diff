package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsRemoteAccessAllowed(t *testing.T) {
	cases := []struct {
		name     string
		cmd      diffCmd
		expected bool
	}{
		{
			name: "no flags",
			cmd: diffCmd{
				dryRunMode: "none",
			},
			expected: true,
		},
		{
			name: "legacy explicit dry-run=true flag",
			cmd: diffCmd{
				dryRunMode: "true",
			},
			expected: false,
		},
		{
			name: "legacy explicit dry-run=false flag",
			cmd: diffCmd{
				dryRunMode: "false",
			},
			expected: true,
		},
		{
			name: "legacy empty dry-run flag",
			cmd: diffCmd{
				dryRunMode: dryRunNoOptDefVal,
			},
			expected: false,
		},
		{
			name: "server-side dry-run flag",
			cmd: diffCmd{
				dryRunMode: "server",
			},
			expected: true,
		},
		{
			name: "client-side dry-run flag",
			cmd: diffCmd{
				dryRunMode: "client",
			},
			expected: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actual := tc.cmd.clusterAccessAllowed()
			if actual != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, actual)
			}
		})
	}
}

func TestPrepareEnvSettings_MultiFileKubeconfig(t *testing.T) {
	original := os.Getenv("KUBECONFIG")
	defer os.Setenv("KUBECONFIG", original)

	cases := []struct {
		name            string
		kubeconfig      string
		kubeContext     string
		wantKubeConfig  string
		wantKubeContext string
	}{
		{
			name:            "single file kubeconfig is preserved",
			kubeconfig:      "/path/to/config",
			kubeContext:     "",
			wantKubeConfig:  "/path/to/config",
			wantKubeContext: "",
		},
		{
			name:            "multi-file kubeconfig is cleared",
			kubeconfig:      "/path/to/file1" + string(filepath.ListSeparator) + "/path/to/file2",
			kubeContext:     "",
			wantKubeConfig:  "",
			wantKubeContext: "",
		},
		{
			name:            "multi-file kubeconfig with three files is cleared",
			kubeconfig:      "/a" + string(filepath.ListSeparator) + "/b" + string(filepath.ListSeparator) + "/c",
			kubeContext:     "",
			wantKubeConfig:  "",
			wantKubeContext: "",
		},
		{
			name:            "empty kubeconfig is preserved",
			kubeconfig:      "",
			kubeContext:     "",
			wantKubeConfig:  "",
			wantKubeContext: "",
		},
		{
			name:            "kube-context override is applied",
			kubeconfig:      "/path/to/config",
			kubeContext:     "my-context",
			wantKubeConfig:  "/path/to/config",
			wantKubeContext: "my-context",
		},
		{
			name:            "multi-file kubeconfig with kube-context override",
			kubeconfig:      "/path/to/file1" + string(filepath.ListSeparator) + "/path/to/file2",
			kubeContext:     "my-context",
			wantKubeConfig:  "",
			wantKubeContext: "my-context",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			os.Setenv("KUBECONFIG", tc.kubeconfig)

			env := prepareEnvSettings(tc.kubeContext)

			if env.KubeConfig != tc.wantKubeConfig {
				t.Errorf("KubeConfig: got %q, want %q", env.KubeConfig, tc.wantKubeConfig)
			}
			if env.KubeContext != tc.wantKubeContext {
				t.Errorf("KubeContext: got %q, want %q", env.KubeContext, tc.wantKubeContext)
			}
		})
	}
}

func TestPrepareEnvSettings_ConfigFlagsPointToCorrectFields(t *testing.T) {
	original := os.Getenv("KUBECONFIG")
	defer os.Setenv("KUBECONFIG", original)

	t.Run("config flags reflect kube-context override", func(t *testing.T) {
		os.Setenv("KUBECONFIG", "/some/config")
		env := prepareEnvSettings("my-override-context")

		if env.KubeContext != "my-override-context" {
			t.Errorf("env.KubeContext = %q, want %q", env.KubeContext, "my-override-context")
		}
	})

	t.Run("multi-file kubeconfig does not set ExplicitPath", func(t *testing.T) {
		multiPath := "/tmp/file1" + string(filepath.ListSeparator) + "/tmp/file2"
		os.Setenv("KUBECONFIG", multiPath)

		env := prepareEnvSettings("")

		if env.KubeConfig != "" {
			t.Errorf("env.KubeConfig = %q, want empty string for multi-file KUBECONFIG", env.KubeConfig)
		}

		getter := env.RESTClientGetter()
		rawConfig := getter.ToRawKubeConfigLoader()
		loadingRules := rawConfig.ConfigAccess()

		if loadingRules != nil {
			if explicitPath := loadingRules.GetExplicitFile(); explicitPath != "" {
				t.Errorf("ExplicitPath = %q, want empty string for multi-file KUBECONFIG", explicitPath)
			}
		}
	})

	t.Run("single file kubeconfig preserves ExplicitPath", func(t *testing.T) {
		os.Setenv("KUBECONFIG", "/tmp/single-config")

		env := prepareEnvSettings("")

		if env.KubeConfig != "/tmp/single-config" {
			t.Errorf("env.KubeConfig = %q, want %q", env.KubeConfig, "/tmp/single-config")
		}
	})
}

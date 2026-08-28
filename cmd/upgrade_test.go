package cmd

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
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

func TestServerSideFlagValidation(t *testing.T) {
	cases := []struct {
		name      string
		value     string
		expectErr bool
	}{
		{name: "true", value: "true", expectErr: false},
		{name: "false", value: "false", expectErr: false},
		{name: "auto", value: "auto", expectErr: false},
		{name: "invalid", value: "yes", expectErr: true},
		{name: "empty", value: "", expectErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			valid := slices.Contains(validServerSideVals, tc.value)
			if valid == tc.expectErr {
				t.Errorf("value %q: expected valid=%v, got valid=%v", tc.value, !tc.expectErr, valid)
			}
		})
	}
}

func TestValidateRevision(t *testing.T) {
	cases := []struct {
		name       string
		revision   int
		changed    bool
		dryRunMode string
		expectErr  bool
	}{
		{name: "unset", revision: 0, changed: false, dryRunMode: dryRunNone, expectErr: false},
		{name: "positive revision", revision: 2, changed: true, dryRunMode: dryRunNone, expectErr: false},
		{name: "positive revision with dry-run=server", revision: 2, changed: true, dryRunMode: dryRunServer, expectErr: false},
		{name: "explicit zero", revision: 0, changed: true, dryRunMode: dryRunNone, expectErr: true},
		{name: "negative revision", revision: -1, changed: true, dryRunMode: dryRunNone, expectErr: true},
		{name: "dry-run=client denies cluster access", revision: 2, changed: true, dryRunMode: dryRunNoOptDefVal, expectErr: true},
		{name: "dry-run=true denies cluster access", revision: 2, changed: true, dryRunMode: envTrue, expectErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := diffCmd{revision: tc.revision, dryRunMode: tc.dryRunMode}
			err := d.validateRevision(tc.changed)
			if (err != nil) != tc.expectErr {
				t.Errorf("expected error=%v, got %v", tc.expectErr, err)
			}
		})
	}
}

func TestGetStorageNamespace(t *testing.T) {
	cases := []struct {
		name             string
		namespace        string
		storageNamespace string
		expected         string
	}{
		{
			name:             "storage namespace defaults to target namespace when unset",
			namespace:        "target-ns",
			storageNamespace: "",
			expected:         "target-ns",
		},
		{
			name:             "storage namespace overrides target namespace when set",
			namespace:        "target-ns",
			storageNamespace: "flux-system",
			expected:         "flux-system",
		},
		{
			name:             "both empty returns empty",
			namespace:        "",
			storageNamespace: "",
			expected:         "",
		},
		{
			name:             "storage namespace set with empty target namespace",
			namespace:        "",
			storageNamespace: "flux-system",
			expected:         "flux-system",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := diffCmd{
				namespace:        tc.namespace,
				storageNamespace: tc.storageNamespace,
			}
			actual := d.getStorageNamespace()
			if actual != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, actual)
			}
		})
	}
}

func TestUpgradeCommand_StorageNamespaceFlag(t *testing.T) {
	cmd := newChartCommand()
	f := cmd.Flags()

	if f.Lookup("storage-namespace") == nil {
		t.Fatal("expected flag --storage-namespace to be registered")
	}

	if f.Lookup("namespace") == nil {
		t.Fatal("expected flag --namespace to be registered")
	}

	if f.ShorthandLookup("n") == nil {
		t.Fatal("expected shorthand flag -n to be registered")
	}

	err := cmd.ParseFlags([]string{"--storage-namespace", "flux-system", "-n", "prod-apps"})
	if err != nil {
		t.Fatalf("unexpected error parsing flags: %v", err)
	}

	storageNs, err := cmd.Flags().GetString("storage-namespace")
	if err != nil || storageNs != "flux-system" {
		t.Errorf("expected storage-namespace=flux-system, got %q (err: %v)", storageNs, err)
	}

	ns, err := cmd.Flags().GetString("namespace")
	if err != nil || ns != "prod-apps" {
		t.Errorf("expected namespace=prod-apps, got %q (err: %v)", ns, err)
	}
}

func TestUpgradeCommand_Execution_StorageNamespace(t *testing.T) {
	manifestYAML := `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: prod-apps
data:
  key: value
`

	t.Run("explicit flag separates storage and target namespace", func(t *testing.T) {
		argsFile := t.TempDir() + "/args"
		setupFakeHelm(t, "capture_args", manifestYAML, argsFile, "")

		chartDir := t.TempDir()
		cmd := newChartCommand()
		cmd.SetArgs([]string{"my-release", chartDir, "--storage-namespace", "flux-system", "-n", "prod-apps"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error executing upgrade command: %v", err)
		}

		data, err := os.ReadFile(argsFile)
		if err != nil {
			t.Fatalf("failed to read fake helm args: %v", err)
		}
		argsContent := string(data)

		// get manifest should use storage namespace
		if !strings.Contains(argsContent, "get manifest my-release --namespace flux-system") {
			t.Errorf("expected 'helm get manifest' to use --namespace flux-system, got:\n%s", argsContent)
		}
		// template should use target namespace
		if !strings.Contains(argsContent, "template my-release "+chartDir+" --namespace prod-apps") {
			t.Errorf("expected 'helm template' to use --namespace prod-apps, got:\n%s", argsContent)
		}
	})

	t.Run("env var sets storage namespace when flag is omitted", func(t *testing.T) {
		argsFile := t.TempDir() + "/args"
		setupFakeHelm(t, "capture_args", manifestYAML, argsFile, "")
		t.Setenv("HELM_DIFF_STORAGE_NAMESPACE", "flux-system-env")

		chartDir := t.TempDir()
		cmd := newChartCommand()
		cmd.SetArgs([]string{"my-release", chartDir, "-n", "prod-apps"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error executing upgrade command: %v", err)
		}

		data, err := os.ReadFile(argsFile)
		if err != nil {
			t.Fatalf("failed to read fake helm args: %v", err)
		}
		argsContent := string(data)

		// get manifest should use storage namespace from env var
		if !strings.Contains(argsContent, "get manifest my-release --namespace flux-system-env") {
			t.Errorf("expected 'helm get manifest' to use --namespace flux-system-env, got:\n%s", argsContent)
		}
		// template should use target namespace
		if !strings.Contains(argsContent, "template my-release "+chartDir+" --namespace prod-apps") {
			t.Errorf("expected 'helm template' to use --namespace prod-apps, got:\n%s", argsContent)
		}
	})

	t.Run("defaults to target namespace when storage namespace is omitted", func(t *testing.T) {
		argsFile := t.TempDir() + "/args"
		setupFakeHelm(t, "capture_args", manifestYAML, argsFile, "")
		t.Setenv("HELM_DIFF_STORAGE_NAMESPACE", "")

		chartDir := t.TempDir()
		cmd := newChartCommand()
		cmd.SetArgs([]string{"my-release", chartDir, "-n", "prod-apps"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error executing upgrade command: %v", err)
		}

		data, err := os.ReadFile(argsFile)
		if err != nil {
			t.Fatalf("failed to read fake helm args: %v", err)
		}
		argsContent := string(data)

		// get manifest should use target namespace as fallback
		if !strings.Contains(argsContent, "get manifest my-release --namespace prod-apps") {
			t.Errorf("expected 'helm get manifest' to fall back to --namespace prod-apps, got:\n%s", argsContent)
		}
		// template should use target namespace
		if !strings.Contains(argsContent, "template my-release "+chartDir+" --namespace prod-apps") {
			t.Errorf("expected 'helm template' to use --namespace prod-apps, got:\n%s", argsContent)
		}
	})
}

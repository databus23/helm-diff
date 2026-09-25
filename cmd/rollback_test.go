package cmd

import (
	"os"
	"strings"
	"testing"
)

func TestRollbackCommand_StorageNamespaceFlag(t *testing.T) {
	cmd := rollbackCmd()
	f := cmd.Flags()

	if f.Lookup("storage-namespace") == nil {
		t.Fatal("expected flag --storage-namespace to be registered on rollbackCmd")
	}

	if f.Lookup("namespace") == nil {
		t.Fatal("expected flag --namespace to be registered on rollbackCmd")
	}

	if f.ShorthandLookup("n") == nil {
		t.Fatal("expected shorthand flag -n to be registered on rollbackCmd")
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

func TestRollbackCommand_Execution_StorageNamespace(t *testing.T) {
	manifestYAML := `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: prod-apps
data:
  key: value
`

	t.Run("explicit flag passes storage namespace to helm get", func(t *testing.T) {
		argsFile := t.TempDir() + "/args"
		setupFakeHelm(t, "capture_args", manifestYAML, argsFile, "")

		cmd := rollbackCmd()
		cmd.SetArgs([]string{"my-release", "2", "--storage-namespace", "flux-system", "-n", "prod-apps"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error executing rollback command: %v", err)
		}

		data, err := os.ReadFile(argsFile)
		if err != nil {
			t.Fatalf("failed to read fake helm args: %v", err)
		}
		argsContent := string(data)

		if !strings.Contains(argsContent, "get manifest my-release --namespace flux-system") {
			t.Errorf("expected 'helm get manifest' for latest release to use --namespace flux-system, got:\n%s", argsContent)
		}
		if !strings.Contains(argsContent, "get manifest my-release --revision 2 --namespace flux-system") {
			t.Errorf("expected 'helm get manifest' for revision 2 to use --namespace flux-system, got:\n%s", argsContent)
		}
	})

	t.Run("env var sets storage namespace when flag is omitted", func(t *testing.T) {
		argsFile := t.TempDir() + "/args"
		setupFakeHelm(t, "capture_args", manifestYAML, argsFile, "")
		t.Setenv("HELM_DIFF_STORAGE_NAMESPACE", "flux-system-env")

		cmd := rollbackCmd()
		cmd.SetArgs([]string{"my-release", "2", "-n", "prod-apps"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error executing rollback command: %v", err)
		}

		data, err := os.ReadFile(argsFile)
		if err != nil {
			t.Fatalf("failed to read fake helm args: %v", err)
		}
		argsContent := string(data)

		if !strings.Contains(argsContent, "get manifest my-release --namespace flux-system-env") {
			t.Errorf("expected 'helm get manifest' to use env var --namespace flux-system-env, got:\n%s", argsContent)
		}
	})

	t.Run("defaults to target namespace when storage namespace is omitted", func(t *testing.T) {
		argsFile := t.TempDir() + "/args"
		setupFakeHelm(t, "capture_args", manifestYAML, argsFile, "")
		t.Setenv("HELM_DIFF_STORAGE_NAMESPACE", "")

		cmd := rollbackCmd()
		cmd.SetArgs([]string{"my-release", "2", "-n", "prod-apps"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error executing rollback command: %v", err)
		}

		data, err := os.ReadFile(argsFile)
		if err != nil {
			t.Fatalf("failed to read fake helm args: %v", err)
		}
		argsContent := string(data)

		if !strings.Contains(argsContent, "get manifest my-release --namespace prod-apps") {
			t.Errorf("expected 'helm get manifest' to fall back to target namespace --namespace prod-apps, got:\n%s", argsContent)
		}
	})
}

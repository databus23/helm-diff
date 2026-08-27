package cmd

import (
	"os"
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

func TestRollback_GetStorageNamespace(t *testing.T) {
	original := os.Getenv("HELM_DIFF_STORAGE_NAMESPACE")
	defer os.Setenv("HELM_DIFF_STORAGE_NAMESPACE", original)

	cases := []struct {
		name             string
		namespace        string
		storageNamespace string
		envVar           string
		expected         string
	}{
		{
			name:             "defaults to target namespace",
			namespace:        "target-ns",
			storageNamespace: "",
			envVar:           "",
			expected:         "target-ns",
		},
		{
			name:             "storage namespace flag set",
			namespace:        "target-ns",
			storageNamespace: "flux-system",
			envVar:           "",
			expected:         "flux-system",
		},
		{
			name:             "storage namespace env var set",
			namespace:        "target-ns",
			storageNamespace: "",
			envVar:           "flux-system-env",
			expected:         "flux-system-env",
		},
		{
			name:             "storage namespace flag overrides env var",
			namespace:        "target-ns",
			storageNamespace: "flux-system-flag",
			envVar:           "flux-system-env",
			expected:         "flux-system-flag",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			os.Setenv("HELM_DIFF_STORAGE_NAMESPACE", tc.envVar)
			r := rollback{
				namespace:        tc.namespace,
				storageNamespace: tc.storageNamespace,
			}
			if r.storageNamespace == "" && tc.envVar != "" {
				r.storageNamespace = os.Getenv("HELM_DIFF_STORAGE_NAMESPACE")
			}
			actual := r.getStorageNamespace()
			if actual != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, actual)
			}
		})
	}
}

package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestNamespacesStorage(t *testing.T) {
	cases := []struct {
		name       string
		namespaces namespaces
		expected   string
	}{
		{
			name:       "storage namespace defaults to target namespace when unset",
			namespaces: namespaces{namespace: "target-ns"},
			expected:   "target-ns",
		},
		{
			name:       "storage namespace overrides target namespace when set",
			namespaces: namespaces{namespace: "target-ns", storageNamespace: "flux-system"},
			expected:   "flux-system",
		},
		{
			name:       "both empty returns empty",
			namespaces: namespaces{},
			expected:   "",
		},
		{
			name:       "storage namespace set with empty target namespace",
			namespaces: namespaces{storageNamespace: "flux-system"},
			expected:   "flux-system",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, tc.namespaces.storage())
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
			require.Equal(t, tc.expected, resolveStorageNamespace(tc.storageNamespace, tc.namespace))
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

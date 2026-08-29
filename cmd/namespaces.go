package cmd

import (
	"os"

	"github.com/spf13/pflag"
)

// namespaces groups the target namespace and the storage namespace of a release.
//
// The target namespace is where the release resources are (or would be) deployed
// and where chart templates are rendered into. The storage namespace is where the
// helm release storage (Secret/ConfigMap) is located. GitOps tools like the FluxCD
// HelmRelease controller persist release records in a storage namespace (e.g.
// flux-system) that differs from the target namespace of the workloads.
type namespaces struct {
	namespace        string // target namespace (-n/--namespace, HELM_NAMESPACE)
	storageNamespace string // storage namespace (--storage-namespace, HELM_DIFF_STORAGE_NAMESPACE)
}

// storage returns the namespace of the helm release storage, falling back to the
// target namespace when no storage namespace was configured.
func (n *namespaces) storage() string {
	return resolveStorageNamespace(n.storageNamespace, n.namespace)
}

// addNamespaceFlags registers the -n/--namespace and --storage-namespace flags on f,
// binding them to n. Both flags default to their respective environment variables
// (HELM_NAMESPACE and HELM_DIFF_STORAGE_NAMESPACE), so a flag passed on the command
// line (including an explicit empty string) always takes precedence over the
// environment.
func addNamespaceFlags(f *pflag.FlagSet, n *namespaces) {
	f.StringVarP(&n.namespace, "namespace", "n", os.Getenv("HELM_NAMESPACE"), "namespace to assume the release to be installed into. Defaults to the current kube config namespace.")
	f.StringVar(&n.storageNamespace, "storage-namespace", os.Getenv("HELM_DIFF_STORAGE_NAMESPACE"), "namespace where the helm release storage (Secret/ConfigMap) is located. Defaults to the target namespace (-n/--namespace)")
}

// resolveStorageNamespace returns storageNamespace if non-empty, otherwise falls back to namespace.
func resolveStorageNamespace(storageNamespace, namespace string) string {
	if storageNamespace != "" {
		return storageNamespace
	}
	return namespace
}

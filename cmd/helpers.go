package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/client-go/util/homedir"
)

var (
	// DefaultHelmHome to hold default home path of .helm dir
	DefaultHelmHome = filepath.Join(homedir.HomeDir(), ".helm")
)

func isDebug() bool {
	return os.Getenv("HELM_DEBUG") == envTrue
}
func debugPrint(format string, a ...interface{}) {
	if isDebug() {
		fmt.Printf(format+"\n", a...)
	}
}

func outputWithRichError(cmd *exec.Cmd) ([]byte, error) {
	debugPrint("Executing %s", strings.Join(cmd.Args, " "))
	output, err := cmd.Output()
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return output, fmt.Errorf("%s: %s", exitError.Error(), string(exitError.Stderr))
	}
	return output, err
}

// resolveStorageNamespace returns storageNamespace if non-empty, otherwise falls back to namespace.
func resolveStorageNamespace(storageNamespace, namespace string) string {
	if storageNamespace != "" {
		return storageNamespace
	}
	return namespace
}

// resolveNamespaceFlags populates storageNamespace and namespace from their respective environment variables
// (HELM_DIFF_STORAGE_NAMESPACE and HELM_NAMESPACE) if the flags were not explicitly set on the command line.
func resolveNamespaceFlags(cmd *cobra.Command, storageNamespace, namespace *string) {
	if !cmd.Flags().Changed("storage-namespace") && *storageNamespace == "" {
		*storageNamespace = os.Getenv("HELM_DIFF_STORAGE_NAMESPACE")
	}
	if !cmd.Flags().Changed("namespace") && *namespace == "" {
		*namespace = os.Getenv("HELM_NAMESPACE")
	}
}

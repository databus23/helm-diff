package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"k8s.io/client-go/util/homedir"
)

const (
	helm2TestSuccessHook = "test-success"
	helm3TestHook        = "test"
)

var (
	// DefaultHelmHome to hold default home path of .helm dir
	DefaultHelmHome = filepath.Join(homedir.HomeDir(), ".helm")
)

func isDebug() bool {
	return os.Getenv("HELM_DEBUG") == "true"
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

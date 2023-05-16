package cmd

// This file contains functions that where blatantly copied from
// https://github.wdf.sap.corp/kubernetes/helm

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

/////////////// Source: cmd/helm/install.go /////////////////////////

type valueFiles []string

func (v *valueFiles) String() string {
	return fmt.Sprint(*v)
}

// Ensures all valuesFiles exist
func (v *valueFiles) Valid() error {
	errStr := ""
	for _, valuesFile := range *v {
		if strings.TrimSpace(valuesFile) != "-" {
			if _, err := os.Stat(valuesFile); os.IsNotExist(err) {
				errStr += err.Error()
			}
		}
	}

	if errStr == "" {
		return nil
	}

	return errors.New(errStr)
}

func (v *valueFiles) Type() string {
	return "valueFiles"
}

func (v *valueFiles) Set(value string) error {
	for _, filePath := range strings.Split(value, ",") {
		*v = append(*v, filePath)
	}
	return nil
}

/////////////// Source: cmd/helm/helm.go ////////////////////////////

func checkArgsLength(argsReceived int, requiredArgs ...string) error {
	expectedNum := len(requiredArgs)
	if argsReceived != expectedNum {
		arg := "arguments"
		if expectedNum == 1 {
			arg = "argument"
		}
		return fmt.Errorf("This command needs %v %s: %s", expectedNum, arg, strings.Join(requiredArgs, ", "))
	}
	return nil
}

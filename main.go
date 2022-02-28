package main

import (
	"os"

	"github.com/ksa-real/helm-diff/v3/cmd"
	_ "k8s.io/client-go/plugin/pkg/client/auth/azure"
	_ "k8s.io/client-go/plugin/pkg/client/auth/exec"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	_ "k8s.io/client-go/plugin/pkg/client/auth/openstack"
)

func main() {
	if err := cmd.New().Execute(); err != nil {
		switch e := err.(type) {
		case cmd.Error:
			os.Exit(e.Code)
		default:
			os.Exit(1)
		}
	}
}

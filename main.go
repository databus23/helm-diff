package main

import (
	"os"

	"github.com/databus23/helm-diff/cmd"
)

func main() {

	if err := cmd.New().Execute(); err != nil {
		os.Exit(1)
	}
}

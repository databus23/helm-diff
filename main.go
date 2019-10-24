package main

import (
	"os"

	"github.com/databus23/helm-diff/cmd"
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

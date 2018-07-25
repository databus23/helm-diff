package main

import (
	"os"
	"fmt"

	"github.com/databus23/helm-diff/cmd"
)

func main() {
	if err := cmd.New().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

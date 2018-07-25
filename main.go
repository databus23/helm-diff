package main

import (
	"fmt"
	"os"

	"github.com/databus23/helm-diff/cmd"
)

func main() {
	if err := cmd.New().Execute(); err != nil {
		fmt.Println(err)
		switch e := err.(type) {
		case cmd.Error:
			os.Exit(e.Code)
		default:
			os.Exit(1)
		}
	}
}

package cmd

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	if os.Getenv("HELM_DIFF_FAKE_HELM") == "1" {
		mode := os.Getenv("HELM_DIFF_FAKE_HELM_MODE")
		switch mode {
		case "error":
			fmt.Fprintln(os.Stderr, "error: chart not found")
			os.Exit(1)
		case "dual":
			countFile := os.Getenv("HELM_DIFF_FAKE_COUNT_FILE")
			data, _ := os.ReadFile(countFile)
			count := 0
			if len(data) > 0 {
				fmt.Sscanf(string(data), "%d", &count)
			}
			count++
			os.WriteFile(countFile, []byte(fmt.Sprintf("%d", count)), 0644)
			if count == 1 {
				fmt.Print(os.Getenv("HELM_DIFF_FAKE_OUTPUT_1"))
			} else {
				fmt.Print(os.Getenv("HELM_DIFF_FAKE_OUTPUT_2"))
			}
		case "capture_args":
			argsFile := os.Getenv("HELM_DIFF_FAKE_ARGS_FILE")
			if argsFile != "" {
				os.WriteFile(argsFile, []byte(strings.Join(os.Args[1:], " ")), 0644)
			}
			fmt.Print(os.Getenv("HELM_DIFF_FAKE_OUTPUT"))
		default:
			fmt.Print(os.Getenv("HELM_DIFF_FAKE_OUTPUT"))
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

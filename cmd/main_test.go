package cmd

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func shouldRunFakeHelm() bool {
	if os.Getenv("HELM_DIFF_FAKE_HELM") != "1" {
		return false
	}
	if len(os.Args) < 2 {
		return false
	}
	return !strings.HasPrefix(os.Args[1], "-test.")
}

func TestMain(m *testing.M) {
	if shouldRunFakeHelm() {
		mode := os.Getenv("HELM_DIFF_FAKE_HELM_MODE")
		switch mode {
		case "error":
			fmt.Fprintln(os.Stderr, "error: chart not found")
			os.Exit(1)
		case "dual":
			countFile := os.Getenv("HELM_DIFF_FAKE_COUNT_FILE")
			data, err := os.ReadFile(countFile)
			if err != nil && !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "failed to read count file %q: %v\n", countFile, err)
				os.Exit(1)
			}
			count := 0
			if len(data) > 0 {
				if _, err := fmt.Sscanf(string(data), "%d", &count); err != nil {
					fmt.Fprintf(os.Stderr, "failed to parse count from %q: %v\n", string(data), err)
					os.Exit(1)
				}
			}
			count++
			if err := os.WriteFile(countFile, []byte(fmt.Sprintf("%d", count)), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "failed to write count file %q: %v\n", countFile, err)
				os.Exit(1)
			}
			if count == 1 {
				fmt.Print(os.Getenv("HELM_DIFF_FAKE_OUTPUT_1"))
			} else {
				fmt.Print(os.Getenv("HELM_DIFF_FAKE_OUTPUT_2"))
			}
		case "capture_args":
			argsFile := os.Getenv("HELM_DIFF_FAKE_ARGS_FILE")
			if argsFile != "" {
				if err := os.WriteFile(argsFile, []byte(strings.Join(os.Args[1:], " ")), 0644); err != nil {
					fmt.Fprintf(os.Stderr, "failed to write fake helm args file %q: %v\n", argsFile, err)
					os.Exit(1)
				}
			}
			fmt.Print(os.Getenv("HELM_DIFF_FAKE_OUTPUT"))
		default:
			fmt.Print(os.Getenv("HELM_DIFF_FAKE_OUTPUT"))
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

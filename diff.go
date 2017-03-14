package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/aryann/difflib"
	"github.com/mgutz/ansi"
)

func diffManifests(oldIndex, newIndex map[string]string, to io.Writer) {
	for key, oldContent := range oldIndex {
		if newContent, ok := newIndex[key]; ok {
			if oldContent != newContent {
				// modified
				fmt.Fprintf(to, ansi.Color("%s has changed:", "yellow")+"\n", key)
				printDiff(oldContent, newContent, to)
			}
		} else {
			// removed
			fmt.Fprintf(to, ansi.Color("%s has been removed:", "yellow")+"\n", key)
			printDiff(oldContent, "", to)
		}
	}

	for key, newContent := range newIndex {
		if _, ok := oldIndex[key]; !ok {
			// added
			fmt.Fprintf(to, ansi.Color("%s has been added:", "yellow")+"\n", key)
			printDiff("", newContent, to)
		}
	}
}

func printDiff(before, after string, to io.Writer) {
	diffs := difflib.Diff(strings.Split(before, "\n"), strings.Split(after, "\n"))

	for _, diff := range diffs {
		text := diff.Payload

		switch diff.Delta {
		case difflib.RightOnly:
			fmt.Fprintf(to, "%s\n", ansi.Color("+ "+text, "green"))
		case difflib.LeftOnly:
			fmt.Fprintf(to, "%s\n", ansi.Color("- "+text, "red"))
		case difflib.Common:
			fmt.Fprintf(to, "%s\n", "  "+text)
		}
	}

}

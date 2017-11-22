package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/aryann/difflib"
	"github.com/mgutz/ansi"

	"github.com/databus23/helm-diff/manifest"
)

func diffManifests(oldIndex, newIndex map[string]*manifest.MappingResult, suppressedKinds []string, to io.Writer) {
	for key, oldContent := range oldIndex {
		if newContent, ok := newIndex[key]; ok {
			if oldContent.Content != newContent.Content {
				// modified
				fmt.Fprintf(to, ansi.Color("%s has changed:", "yellow")+"\n", key)
				printDiff(suppressedKinds, oldContent.Kind, oldContent.Content, newContent.Content, to)
			}
		} else {
			// removed
			fmt.Fprintf(to, ansi.Color("%s has been removed:", "yellow")+"\n", key)
			printDiff(suppressedKinds, oldContent.Kind, oldContent.Content, "", to)
		}
	}

	for key, newContent := range newIndex {
		if _, ok := oldIndex[key]; !ok {
			// added
			fmt.Fprintf(to, ansi.Color("%s has been added:", "yellow")+"\n", key)
			printDiff(suppressedKinds, newContent.Kind, "", newContent.Content, to)
		}
	}
}

func printDiff(suppressedKinds []string, kind, before, after string, to io.Writer) {
	diffs := difflib.Diff(strings.Split(before, "\n"), strings.Split(after, "\n"))

	for _, ckind := range suppressedKinds {
		if ckind == kind {
			str := fmt.Sprintf("+ Changes suppressed on sensitive content of type %s\n", kind)
			fmt.Fprintf(to, ansi.Color(str, "yellow"))
			return
		}
	}

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

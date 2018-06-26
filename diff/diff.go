package diff

import (
	"fmt"
	"io"
	"strings"

	"github.com/aryann/difflib"
	"github.com/mgutz/ansi"

	"github.com/databus23/helm-diff/manifest"
)


func DiffManifests(oldIndex, newIndex map[string]*manifest.MappingResult, suppressedKinds []string, to io.Writer) bool {
	seenAnyChanges := false
	emptyMapping := &manifest.MappingResult{}
	for key, oldContent := range oldIndex {
		if newContent, ok := newIndex[key]; ok {
			if oldContent.Content != newContent.Content {
				// modified
				fmt.Fprintf(to, ansi.Color("%s has changed:", "yellow")+"\n", key)
				diffs := generateDiff(oldContent, newContent)
				if len(diffs) > 0 {
					seenAnyChanges = true
				}
				printDiff(suppressedKinds, oldContent.Kind, diffs, to)
			}
		} else {
			// removed
			fmt.Fprintf(to, ansi.Color("%s has been removed:", "yellow")+"\n", key)
			diffs := generateDiff(oldContent, emptyMapping)
			if len(diffs) > 0 {
				seenAnyChanges = true
			}
			printDiff(suppressedKinds, oldContent.Kind, diffs, to)
		}
	}

	for key, newContent := range newIndex {
		if _, ok := oldIndex[key]; !ok {
			// added
			fmt.Fprintf(to, ansi.Color("%s has been added:", "yellow")+"\n", key)
			diffs := generateDiff(emptyMapping, newContent)
			if len(diffs) > 0 {
				seenAnyChanges = true
			}
			printDiff(suppressedKinds, newContent.Kind, diffs, to)
		}
	}
	return seenAnyChanges
}

func generateDiff(oldContent *manifest.MappingResult, newContent *manifest.MappingResult) []difflib.DiffRecord {
	const sep = "\n"
	return difflib.Diff(strings.Split(oldContent.Content, sep), strings.Split(newContent.Content, sep))
}

func printDiff(suppressedKinds []string, kind string, diffs []difflib.DiffRecord, to io.Writer) {

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

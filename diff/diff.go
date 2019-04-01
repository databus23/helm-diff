package diff

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strings"

	"github.com/aryann/difflib"
	"github.com/mgutz/ansi"

	"github.com/databus23/helm-diff/manifest"
)

func DiffManifests(oldIndex, newIndex map[string]*manifest.MappingResult, suppressedKinds []string, context int, to io.Writer) bool {
	seenAnyChanges := false
	emptyMapping := &manifest.MappingResult{}
	for key, oldContent := range oldIndex {
		if newContent, ok := newIndex[key]; ok {
			if oldContent.Content != newContent.Content {
				// modified
				fmt.Fprintf(to, ansi.Color("%s has changed:", "yellow")+"\n", key)
				diffs := diffMappingResults(oldContent, newContent)
				if len(diffs) > 0 {
					seenAnyChanges = true
				}
				printDiffRecords(suppressedKinds, oldContent.Kind, context, diffs, to)
			}
		} else {
			// removed
			fmt.Fprintf(to, ansi.Color("%s has been removed:", "yellow")+"\n", key)
			diffs := diffMappingResults(oldContent, emptyMapping)
			if len(diffs) > 0 {
				seenAnyChanges = true
			}
			printDiffRecords(suppressedKinds, oldContent.Kind, context, diffs, to)
		}
	}

	for key, newContent := range newIndex {
		if _, ok := oldIndex[key]; !ok {
			// added
			fmt.Fprintf(to, ansi.Color("%s has been added:", "yellow")+"\n", key)
			diffs := diffMappingResults(emptyMapping, newContent)
			if len(diffs) > 0 {
				seenAnyChanges = true
			}
			printDiffRecords(suppressedKinds, newContent.Kind, context, diffs, to)
		}
	}
	return seenAnyChanges
}

// DiffReleases reindex the content  based on the template names and pass it to DiffManifests
func DiffReleases(oldIndex, newIndex map[string]*manifest.MappingResult, suppressedKinds []string, context int, to io.Writer) bool {
	oldIndex = reIndexForRelease(oldIndex)
	newIndex = reIndexForRelease(newIndex)
	return DiffManifests(oldIndex, newIndex, suppressedKinds, context, to)
}

func diffMappingResults(oldContent *manifest.MappingResult, newContent *manifest.MappingResult) []difflib.DiffRecord {
	return diffStrings(oldContent.Content, newContent.Content)
}

func diffStrings(before, after string) []difflib.DiffRecord {
	const sep = "\n"
	return difflib.Diff(strings.Split(before, sep), strings.Split(after, sep))
}

func printDiffRecords(suppressedKinds []string, kind string, context int, diffs []difflib.DiffRecord, to io.Writer) {
	for _, ckind := range suppressedKinds {
		if ckind == kind {
			str := fmt.Sprintf("+ Changes suppressed on sensitive content of type %s\n", kind)
			fmt.Fprintf(to, ansi.Color(str, "yellow"))
			return
		}
	}

	if context >= 0 {
		distances := calculateDistances(diffs)
		omitting := false
		for i, diff := range diffs {
			if distances[i] > context {
				if !omitting {
					fmt.Fprintln(to, "...")
					omitting = true
				}
			} else {
				omitting = false
				printDiffRecord(diff, to)
			}
		}
	} else {
		for _, diff := range diffs {
			printDiffRecord(diff, to)
		}
	}
}

func printDiffRecord(diff difflib.DiffRecord, to io.Writer) {
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

// Calculate distance of every diff-line to the closest change
func calculateDistances(diffs []difflib.DiffRecord) map[int]int {
	distances := map[int]int{}

	// Iterate forwards through diffs, set 'distance' based on closest 'change' before this line
	change := -1
	for i, diff := range diffs {
		if diff.Delta != difflib.Common {
			change = i
		}
		distance := math.MaxInt32
		if change != -1 {
			distance = i - change
		}
		distances[i] = distance
	}

	// Iterate backwards through diffs, reduce 'distance' based on closest 'change' after this line
	change = -1
	for i := len(diffs) - 1; i >= 0; i-- {
		diff := diffs[i]
		if diff.Delta != difflib.Common {
			change = i
		}
		if change != -1 {
			distance := change - i
			if distance < distances[i] {
				distances[i] = distance
			}
		}
	}

	return distances
}

// reIndexForRelease based on template names
func reIndexForRelease(index map[string]*manifest.MappingResult) map[string]*manifest.MappingResult {

	// sort the index to iterate map in the same order
	var keys []string
	for key := range index {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// holds number of object in a single file
	count := make(map[string]int)

	newIndex := make(map[string]*manifest.MappingResult)

	for key := range keys {

		str := strings.Replace(strings.Split(index[keys[key]].Content, "\n")[0], "# Source: ", "", 1)

		if _, ok := newIndex[str]; ok {
			count[str]++
			str += fmt.Sprintf(" %d", count[str])
			newIndex[str] = index[keys[key]]
		} else {
			newIndex[str] = index[keys[key]]
			count[str]++
		}
	}
	return newIndex
}

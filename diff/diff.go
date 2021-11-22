package diff

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"

	"github.com/aryann/difflib"
	"github.com/mgutz/ansi"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/databus23/helm-diff/v3/manifest"
)

// Manifests diff on manifests
func Manifests(oldIndex, newIndex map[string]*manifest.MappingResult, suppressedKinds []string, showSecrets bool, context int, output string, stripTrailingCR bool, to io.Writer) bool {
	report.setupReportFormat(output)
	seenAnyChanges := false
	emptyMapping := &manifest.MappingResult{}
	for _, key := range sortedKeys(oldIndex) {
		oldContent := oldIndex[key]

		if newContent, ok := newIndex[key]; ok {
			if oldContent.Content != newContent.Content {
				// modified
				if !showSecrets {
					redactSecrets(oldContent, newContent)
				}

				diffs := diffMappingResults(oldContent, newContent, stripTrailingCR)
				if len(diffs) > 0 {
					seenAnyChanges = true
				}
				report.addEntry(key, suppressedKinds, oldContent.Kind, context, diffs, "MODIFY")
			}
		} else {
			// removed
			if !showSecrets {
				redactSecrets(oldContent, nil)

			}
			diffs := diffMappingResults(oldContent, emptyMapping, stripTrailingCR)
			if len(diffs) > 0 {
				seenAnyChanges = true
			}
			report.addEntry(key, suppressedKinds, oldContent.Kind, context, diffs, "REMOVE")
		}
	}

	for _, key := range sortedKeys(newIndex) {
		newContent := newIndex[key]

		if _, ok := oldIndex[key]; !ok {
			// added
			if !showSecrets {
				redactSecrets(nil, newContent)
			}
			diffs := diffMappingResults(emptyMapping, newContent, stripTrailingCR)
			if len(diffs) > 0 {
				seenAnyChanges = true
			}
			report.addEntry(key, suppressedKinds, newContent.Kind, context, diffs, "ADD")
		}
	}
	report.print(to)
	report.clean()
	return seenAnyChanges
}

func redactSecrets(old, new *manifest.MappingResult) {
	if (old != nil && old.Kind != "Secret") || (new != nil && new.Kind != "Secret") {
		return
	}
	serializer := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme,
		scheme.Scheme)
	var oldSecret, newSecret v1.Secret

	if old != nil {
		if err := yaml.NewYAMLToJSONDecoder(bytes.NewBufferString(old.Content)).Decode(&oldSecret); err != nil {
			old.Content = fmt.Sprintf("Error parsing old secret: %s", err)
		}
	}
	if new != nil {
		if err := yaml.NewYAMLToJSONDecoder(bytes.NewBufferString(new.Content)).Decode(&newSecret); err != nil {
			new.Content = fmt.Sprintf("Error parsing new secret: %s", err)
		}
	}
	if old != nil {
		oldSecret.StringData = make(map[string]string, len(oldSecret.Data))
		for k, v := range oldSecret.Data {
			if new != nil && bytes.Equal(v, newSecret.Data[k]) {
				oldSecret.StringData[k] = fmt.Sprintf("REDACTED # (%d bytes)", len(v))
			} else {
				oldSecret.StringData[k] = fmt.Sprintf("-------- # (%d bytes)", len(v))
			}
		}
	}
	if new != nil {
		newSecret.StringData = make(map[string]string, len(newSecret.Data))
		for k, v := range newSecret.Data {
			if old != nil && bytes.Equal(v, oldSecret.Data[k]) {
				newSecret.StringData[k] = fmt.Sprintf("REDACTED # (%d bytes)", len(v))
			} else {
				newSecret.StringData[k] = fmt.Sprintf("++++++++ # (%d bytes)", len(v))
			}
		}
	}
	// remove Data field now that we are using StringData for serialization
	var buf bytes.Buffer
	if old != nil {
		oldSecret.Data = nil
		if err := serializer.Encode(&oldSecret, &buf); err != nil {

		}
		old.Content = getComment(old.Content) + strings.Replace(strings.Replace(buf.String(), "stringData", "data", 1), "  creationTimestamp: null\n", "", 1)
		buf.Reset() //reuse buffer for new secret
	}
	if new != nil {
		newSecret.Data = nil
		if err := serializer.Encode(&newSecret, &buf); err != nil {

		}
		new.Content = getComment(new.Content) + strings.Replace(strings.Replace(buf.String(), "stringData", "data", 1), "  creationTimestamp: null\n", "", 1)
	}
}

// return the first line of a string if its a comment.
// This gives as the # Source: lines from the rendering
func getComment(s string) string {
	i := strings.Index(s, "\n")
	if i < 0 || !strings.HasPrefix(s, "#") {
		return ""
	}
	return s[:i+1]

}

// Releases reindex the content  based on the template names and pass it to Manifests
func Releases(oldIndex, newIndex map[string]*manifest.MappingResult, suppressedKinds []string, showSecrets bool, context int, output string, stripTrailingCR bool, to io.Writer) bool {
	oldIndex = reIndexForRelease(oldIndex)
	newIndex = reIndexForRelease(newIndex)
	return Manifests(oldIndex, newIndex, suppressedKinds, showSecrets, context, output, stripTrailingCR, to)
}

func diffMappingResults(oldContent *manifest.MappingResult, newContent *manifest.MappingResult, stripTrailingCR bool ) []difflib.DiffRecord {
	return diffStrings(oldContent.Content, newContent.Content, stripTrailingCR)
}

func diffStrings(before, after string, stripTrailingCR bool) []difflib.DiffRecord {
	return difflib.Diff(split(before, stripTrailingCR), split(after, stripTrailingCR))
}

func split(value string, stripTrailingCR bool) []string {
	const sep = "\n"
	split := strings.Split(value, sep)
	if !stripTrailingCR {
		return split
	}
	var stripped []string
	for _, s := range split {
		stripped = append(stripped, strings.TrimSuffix(s, "\r"))
	}
	return stripped
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

func sortedKeys(manifests map[string]*manifest.MappingResult) []string {
	var keys []string

	for key := range manifests {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}

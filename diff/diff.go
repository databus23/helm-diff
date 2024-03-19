package diff

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"regexp"
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

// Options are all the options to be passed to generate a diff
type Options struct {
	OutputFormat              string
	OutputContext             int
	StripTrailingCR           bool
	ShowSecrets               bool
	SuppressedKinds           []string
	FindRenames               float32
	SuppressedOutputLineRegex []string
}

// Manifests diff on manifests
func Manifests(oldIndex, newIndex map[string]*manifest.MappingResult, options *Options, to io.Writer) bool {
	report := Report{}
	report.setupReportFormat(options.OutputFormat)
	var possiblyRemoved []string

	for _, key := range sortedKeys(oldIndex) {
		oldContent := oldIndex[key]

		if newContent, ok := newIndex[key]; ok {
			// modified?
			doDiff(&report, key, oldContent, newContent, options)
		} else {
			possiblyRemoved = append(possiblyRemoved, key)
		}
	}

	var possiblyAdded []string
	for _, key := range sortedKeys(newIndex) {
		if _, ok := oldIndex[key]; !ok {
			possiblyAdded = append(possiblyAdded, key)
		}
	}

	removed, added := contentSearch(&report, possiblyRemoved, oldIndex, possiblyAdded, newIndex, options)

	for _, key := range removed {
		oldContent := oldIndex[key]
		if oldContent.ResourcePolicy != "keep" {
			doDiff(&report, key, oldContent, nil, options)
		}
	}

	for _, key := range added {
		newContent := newIndex[key]
		doDiff(&report, key, nil, newContent, options)
	}

	seenAnyChanges := len(report.entries) > 0

	report, err := doSuppress(report, options.SuppressedOutputLineRegex)
	if err != nil {
		panic(err)
	}

	report.print(to)
	report.clean()
	return seenAnyChanges
}

func doSuppress(report Report, suppressedOutputLineRegex []string) (Report, error) {
	if len(report.entries) == 0 || len(suppressedOutputLineRegex) == 0 {
		return report, nil
	}

	filteredReport := Report{}
	filteredReport.format = report.format
	filteredReport.entries = []ReportEntry{}

	var suppressOutputRegexes []*regexp.Regexp

	for _, suppressOutputRegex := range suppressedOutputLineRegex {
		regex, err := regexp.Compile(suppressOutputRegex)
		if err != nil {
			return Report{}, err
		}

		suppressOutputRegexes = append(suppressOutputRegexes, regex)
	}

	for _, entry := range report.entries {
		var diffs []difflib.DiffRecord

	DIFFS:
		for _, diff := range entry.diffs {
			for _, suppressOutputRegex := range suppressOutputRegexes {
				if suppressOutputRegex.MatchString(diff.Payload) {
					continue DIFFS
				}
			}

			diffs = append(diffs, diff)
		}

		containsDiff := false

		// Add entry to the report, if diffs are present.
		for _, diff := range diffs {
			if diff.Delta.String() != " " {
				containsDiff = true
				break
			}
		}

		if containsDiff {
			filteredReport.addEntry(entry.key, entry.suppressedKinds, entry.kind, entry.context, diffs, entry.changeType)
		} else {
			filteredReport.addEntry(entry.key, entry.suppressedKinds, entry.kind, entry.context, []difflib.DiffRecord{}, entry.changeType)
		}
	}

	return filteredReport, nil
}

func actualChanges(diff []difflib.DiffRecord) int {
	changes := 0
	for _, record := range diff {
		if record.Delta != difflib.Common {
			changes++
		}
	}
	return changes
}

func contentSearch(report *Report, possiblyRemoved []string, oldIndex map[string]*manifest.MappingResult, possiblyAdded []string, newIndex map[string]*manifest.MappingResult, options *Options) ([]string, []string) {
	if options.FindRenames <= 0 {
		return possiblyRemoved, possiblyAdded
	}

	var removed []string

	for _, removedKey := range possiblyRemoved {
		oldContent := oldIndex[removedKey]
		var smallestKey string
		var smallestFraction float32 = math.MaxFloat32
		for _, addedKey := range possiblyAdded {
			newContent := newIndex[addedKey]
			if oldContent.Kind != newContent.Kind {
				continue
			}

			if !options.ShowSecrets {
				redactSecrets(oldContent, newContent)
			}

			diff := diffMappingResults(oldContent, newContent, options.StripTrailingCR)
			delta := actualChanges(diff)
			if delta == 0 || len(diff) == 0 {
				continue // Should never happen, but better safe than sorry
			}
			fraction := float32(delta) / float32(len(diff))
			if fraction > 0 && fraction < smallestFraction {
				smallestKey = addedKey
				smallestFraction = fraction
			}
		}

		if smallestFraction < options.FindRenames {
			index := sort.SearchStrings(possiblyAdded, smallestKey)
			possiblyAdded = append(possiblyAdded[:index], possiblyAdded[index+1:]...)
			newContent := newIndex[smallestKey]
			doDiff(report, removedKey, oldContent, newContent, options)
		} else {
			removed = append(removed, removedKey)
		}
	}

	return removed, possiblyAdded
}

func doDiff(report *Report, key string, oldContent *manifest.MappingResult, newContent *manifest.MappingResult, options *Options) {
	if oldContent != nil && newContent != nil && oldContent.Content == newContent.Content {
		return
	}

	if !options.ShowSecrets {
		redactSecrets(oldContent, newContent)
	}

	if oldContent == nil {
		emptyMapping := &manifest.MappingResult{}
		diffs := diffMappingResults(emptyMapping, newContent, options.StripTrailingCR)
		report.addEntry(key, options.SuppressedKinds, newContent.Kind, options.OutputContext, diffs, "ADD")
	} else if newContent == nil {
		emptyMapping := &manifest.MappingResult{}
		diffs := diffMappingResults(oldContent, emptyMapping, options.StripTrailingCR)
		report.addEntry(key, options.SuppressedKinds, oldContent.Kind, options.OutputContext, diffs, "REMOVE")
	} else {
		diffs := diffMappingResults(oldContent, newContent, options.StripTrailingCR)
		if actualChanges(diffs) > 0 {
			report.addEntry(key, options.SuppressedKinds, oldContent.Kind, options.OutputContext, diffs, "MODIFY")
		}
	}
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
		//if we have a Secret containing `stringData`, apply the same
		//transformation that the apiserver would do with it (this protects
		//stringData keys from being overwritten down below)
		if len(oldSecret.StringData) > 0 && oldSecret.Data == nil {
			oldSecret.Data = make(map[string][]byte, len(oldSecret.StringData))
		}
		for k, v := range oldSecret.StringData {
			oldSecret.Data[k] = []byte(v)
		}
	}
	if new != nil {
		if err := yaml.NewYAMLToJSONDecoder(bytes.NewBufferString(new.Content)).Decode(&newSecret); err != nil {
			new.Content = fmt.Sprintf("Error parsing new secret: %s", err)
		}
		//same as above
		if len(newSecret.StringData) > 0 && newSecret.Data == nil {
			newSecret.Data = make(map[string][]byte, len(newSecret.StringData))
		}
		for k, v := range newSecret.StringData {
			newSecret.Data[k] = []byte(v)
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
			new.Content = fmt.Sprintf("Error encoding new secret: %s", err)
		}
		old.Content = getComment(old.Content) + strings.Replace(strings.Replace(buf.String(), "stringData", "data", 1), "  creationTimestamp: null\n", "", 1)
		buf.Reset() //reuse buffer for new secret
	}
	if new != nil {
		newSecret.Data = nil
		if err := serializer.Encode(&newSecret, &buf); err != nil {
			new.Content = fmt.Sprintf("Error encoding new secret: %s", err)
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
func Releases(oldIndex, newIndex map[string]*manifest.MappingResult, options *Options, to io.Writer) bool {
	oldIndex = reIndexForRelease(oldIndex)
	newIndex = reIndexForRelease(newIndex)
	return Manifests(oldIndex, newIndex, options, to)
}

func diffMappingResults(oldContent *manifest.MappingResult, newContent *manifest.MappingResult, stripTrailingCR bool) []difflib.DiffRecord {
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
			fmt.Fprint(to, ansi.Color(str, "yellow"))
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
		if text == "" {
			fmt.Fprintln(to)
		} else {
			fmt.Fprintf(to, "%s\n", "  "+text)
		}
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

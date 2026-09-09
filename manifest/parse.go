package manifest

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"regexp"
	"strings"

	jsoniter "github.com/json-iterator/go"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	hookAnnotation           = "helm.sh/hook"
	resourcePolicyAnnotation = "helm.sh/resource-policy"
)

var yamlSeparator = []byte("\n---\n")

// metadataLineRegex matches the top-level `metadata:` key of a manifest.
var metadataLineRegex = regexp.MustCompile(`^metadata:[ \t]*(#.*)?$`)

// stripEmptyMetadataKeys removes `labels:`/`annotations:` lines from content
// when their value is null or an empty mapping.
//
// Kubernetes treats a null or empty labels/annotations map exactly like an
// absent one, but charts frequently render the (empty) key anyway, e.g. via a
// conditional block:
//
//	metadata:
//	  name: example
//	  labels:
//
// A raw textual diff between such a manifest and one that omits the key
// reports a meaningless `- labels:` change. The removal is done line-based so
// the surrounding text keeps its original formatting; the parsed document is
// only consulted to confirm that the key really is null/empty (a `labels:`
// line followed by indented entries is left alone).
//
// See https://github.com/databus23/helm-diff/issues/1064
func stripEmptyMetadataKeys(content []byte) []byte {
	var doc map[interface{}]interface{}
	if err := yaml.Unmarshal(content, &doc); err != nil {
		return content
	}

	metadata, ok := doc["metadata"].(map[interface{}]interface{})
	if !ok {
		return content
	}

	strip := map[string]bool{}
	for _, key := range []string{"labels", "annotations"} {
		value, exists := metadata[key]
		if !exists {
			continue
		}
		if value == nil {
			strip[key] = true
			continue
		}
		if m, ok := value.(map[interface{}]interface{}); ok && len(m) == 0 {
			strip[key] = true
		}
	}
	if len(strip) == 0 {
		return content
	}

	lines := strings.Split(string(content), "\n")

	// Determine the indentation of metadata's direct children by looking at
	// the first non-blank line below the top-level `metadata:` key.
	childIndent := -1
	for i, line := range lines {
		if !metadataLineRegex.MatchString(line) {
			continue
		}
		for j := i + 1; j < len(lines); j++ {
			if strings.TrimSpace(lines[j]) == "" {
				continue
			}
			if indent := len(lines[j]) - len(strings.TrimLeft(lines[j], " \t")); indent > 0 {
				childIndent = indent
			}
			break
		}
		break
	}
	if childIndent <= 0 {
		return content
	}

	keyLine := func(key string) *regexp.Regexp {
		return regexp.MustCompile(fmt.Sprintf(`^ {%d}%s:[ \t]*(\{\}[ \t]*)?(#.*)?$`, childIndent, key))
	}
	regexes := make([]*regexp.Regexp, 0, len(strip))
	for key := range strip {
		regexes = append(regexes, keyLine(key))
	}

	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		drop := false
		for _, re := range regexes {
			if re.MatchString(line) {
				drop = true
				break
			}
		}
		if !drop {
			kept = append(kept, line)
		}
	}

	return []byte(strings.Join(kept, "\n"))
}

// MappingResult to store result of diff
type MappingResult struct {
	Name           string
	Kind           string
	Content        string
	ResourcePolicy string
}

type metadata struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string
	Metadata   struct {
		Namespace   string
		Name        string
		Annotations map[string]string
	}
}

func (m metadata) String() string {
	apiBase := m.APIVersion
	sp := strings.Split(apiBase, "/")
	if len(sp) > 1 {
		apiBase = strings.Join(sp[:len(sp)-1], "/")
	}
	name := m.Metadata.Name
	if a := m.Metadata.Annotations; a != nil {
		if baseName, ok := a["helm-diff/base-name"]; ok {
			name = baseName
		}
	}
	return fmt.Sprintf("%s, %s, %s (%s)", m.Metadata.Namespace, name, m.Kind, apiBase)
}

func scanYamlSpecs(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.Index(data, yamlSeparator); i >= 0 {
		// We have a full newline-terminated line.
		return i + len(yamlSeparator), data[0:i], nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
}

// Parse parses manifest bytes into MappingResult
func Parse(manifest []byte, defaultNamespace string, normalizeManifests bool, excludedHooks ...string) map[string]*MappingResult {
	scanner := bufio.NewScanner(io.MultiReader(strings.NewReader("\n"), bytes.NewReader(manifest)))
	scanner.Split(scanYamlSpecs)
	// Allow for tokens (specs) up to 10MiB in size
	scanner.Buffer(make([]byte, bufio.MaxScanTokenSize), 10485760)

	result := make(map[string]*MappingResult)

	for scanner.Scan() {
		content := bytes.TrimSpace(scanner.Bytes())
		if len(content) == 0 {
			continue
		}

		parsed, err := parseContent(content, defaultNamespace, normalizeManifests, excludedHooks...)
		if err != nil {
			log.Fatalf("%v", err)
		}

		for _, p := range parsed {
			name := p.Name

			if _, ok := result[name]; ok {
				log.Printf("Error: Found duplicate key %#v in manifest", name)
			} else {
				result[name] = p
			}
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading input: %s", err)
	}
	return result
}

func ParseObject(object runtime.Object, defaultNamespace string, excludedHooks ...string) (*MappingResult, string, error) {
	json, _ := jsoniter.ConfigCompatibleWithStandardLibrary.Marshal(object)
	var objectMap map[string]interface{}
	err := jsoniter.Unmarshal(json, &objectMap)
	if err != nil {
		return nil, "", fmt.Errorf("could not unmarshal byte sequence: %w", err)
	}

	metadata := objectMap["metadata"].(map[string]interface{})
	var oldRelease string
	if a := metadata["annotations"]; a != nil {
		annotations := a.(map[string]interface{})
		if releaseNs, ok := annotations["meta.helm.sh/release-namespace"].(string); ok {
			oldRelease += releaseNs + "/"
		}
		if releaseName, ok := annotations["meta.helm.sh/release-name"].(string); ok {
			oldRelease += releaseName
		}
	}

	// Clean namespace metadata as it exists in Kubernetes but not in Helm manifest
	purgedObj, _ := deleteStatusAndTidyMetadata(json)

	content, err := yaml.Marshal(purgedObj)
	if err != nil {
		return nil, "", err
	}

	result, err := parseContent(content, defaultNamespace, true, excludedHooks...)
	if err != nil {
		return nil, "", err
	}

	if len(result) != 1 {
		return nil, "", fmt.Errorf("failed to parse content of Kubernetes resource %s", metadata["name"])
	}

	result[0].Content = strings.TrimSuffix(result[0].Content, "\n")

	return result[0], oldRelease, nil
}

func parseContent(content []byte, defaultNamespace string, normalizeManifests bool, excludedHooks ...string) ([]*MappingResult, error) {
	var parsedMetadata metadata
	if err := yaml.Unmarshal(content, &parsedMetadata); err != nil {
		log.Fatalf("YAML unmarshal error: %s\nCan't unmarshal %s", err, content)
	}

	// Skip content without any metadata. It is probably a template that
	// only contains comments in the current state.
	if parsedMetadata.APIVersion == "" && parsedMetadata.Kind == "" {
		return nil, nil
	}

	if strings.HasSuffix(parsedMetadata.Kind, "List") {
		type ListV1 struct {
			Items []yaml.MapSlice `yaml:"items"`
		}

		var list ListV1

		if err := yaml.Unmarshal(content, &list); err != nil {
			log.Fatalf("YAML unmarshal error: %s\nCan't unmarshal %s", err, content)
		}

		var result []*MappingResult

		for _, item := range list.Items {
			subcontent, err := yaml.Marshal(item)
			if err != nil {
				log.Printf("YAML marshal error: %s\nCan't marshal %v", err, item)
			}

			subs, err := parseContent(subcontent, defaultNamespace, normalizeManifests, excludedHooks...)
			if err != nil {
				return nil, fmt.Errorf("Parsing YAML list item: %w", err)
			}

			result = append(result, subs...)
		}

		return result, nil
	}

	if normalizeManifests {
		var normalizeErr error
		content, normalizeErr = normalizeContent(content)
		if normalizeErr != nil {
			log.Fatalf("Error normalizing manifests: %v", normalizeErr)
		}
	}

	// Remove `labels:`/`annotations:` keys that are null or empty: they are
	// semantically identical to an absent key, yet a textual diff between a
	// manifest rendering the empty key and one omitting it would otherwise
	// report a meaningless change (#1064).
	content = stripEmptyMetadataKeys(content)

	if isHook(parsedMetadata, excludedHooks...) {
		return nil, nil
	}

	if parsedMetadata.Metadata.Namespace == "" {
		parsedMetadata.Metadata.Namespace = defaultNamespace
	}

	name := parsedMetadata.String()
	return []*MappingResult{
		{
			Name:           name,
			Kind:           parsedMetadata.Kind,
			Content:        string(content),
			ResourcePolicy: parsedMetadata.Metadata.Annotations[resourcePolicyAnnotation],
		},
	}, nil
}

func normalizeContent(content []byte) ([]byte, error) {
	// Unmarshal and marshal again content to normalize yaml structure
	// This avoids style differences to show up as diffs but it can
	// make the output different from the original template (since it is in normalized form)
	var object map[interface{}]interface{}
	if err := yaml.Unmarshal(content, &object); err != nil {
		return nil, err
	}
	normalizedContent, err := yaml.Marshal(object)
	if err != nil {
		return nil, err
	}
	return normalizedContent, nil
}

func isHook(metadata metadata, hooks ...string) bool {
	for _, hook := range hooks {
		if metadata.Metadata.Annotations[hookAnnotation] == hook {
			return true
		}
	}
	return false
}

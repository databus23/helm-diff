package manifest

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"strings"

	yaml "gopkg.in/yaml.v2"
	"k8s.io/helm/pkg/proto/hapi/release"
)

const (
	hookAnnotation = "helm.sh/hook"
)

var yamlSeperator = []byte("\n---\n")

// MappingResult to store result of diff
type MappingResult struct {
	Name    string
	Kind    string
	Content string
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

	return fmt.Sprintf("%s, %s, %s (%s)", m.Metadata.Namespace, m.Metadata.Name, m.Kind, apiBase)
}

func scanYamlSpecs(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.Index(data, yamlSeperator); i >= 0 {
		// We have a full newline-terminated line.
		return i + len(yamlSeperator), data[0:i], nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
}

func splitSpec(token string) (string, string) {
	if i := strings.Index(token, "\n"); i >= 0 {
		return token[0:i], token[i+1:]
	}
	return "", ""
}

// ParseRelease parses release objects into MappingResult
func ParseRelease(release *release.Release, includeTests bool) map[string]*MappingResult {
	manifest := release.Manifest
	for _, hook := range release.Hooks {
		if !includeTests && isTestHook(hook.Events) {
			continue
		}

		manifest += "\n---\n"
		manifest += fmt.Sprintf("# Source: %s\n", hook.Path)
		manifest += hook.Manifest
	}
	return Parse(manifest, release.Namespace)
}

// Parse parses manifest strings into MappingResult
func Parse(manifest string, defaultNamespace string, excludedHooks ...string) map[string]*MappingResult {
	// Ensure we have a newline in front of the yaml seperator
	scanner := bufio.NewScanner(strings.NewReader("\n" + manifest))
	scanner.Split(scanYamlSpecs)
	// Allow for tokens (specs) up to 10MiB in size
	scanner.Buffer(make([]byte, bufio.MaxScanTokenSize), 10485760)
	// Discard the first result, we only care about everything after the first separator
	scanner.Scan()

	result := make(map[string]*MappingResult)

	for scanner.Scan() {
		content := strings.TrimSpace(scanner.Text())
		if content == "" {
			continue
		}
		var parsedMetadata metadata
		if err := yaml.Unmarshal([]byte(content), &parsedMetadata); err != nil {
			log.Fatalf("YAML unmarshal error: %s\nCan't unmarshal %s", err, content)
		}

		// Skip content without any metadata. It is probably a template that
		// only contains comments in the current state.
		if parsedMetadata.APIVersion == "" && parsedMetadata.Kind == "" {
			continue
		}
		if isHook(parsedMetadata, excludedHooks...) {
			continue
		}

		if parsedMetadata.Metadata.Namespace == "" {
			parsedMetadata.Metadata.Namespace = defaultNamespace
		}
		name := parsedMetadata.String()
		if _, ok := result[name]; ok {
			log.Printf("Error: Found duplicate key %#v in manifest", name)
		} else {
			result[name] = &MappingResult{
				Name:    name,
				Kind:    parsedMetadata.Kind,
				Content: content,
			}
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading input: %s", err)
	}
	return result
}

func isHook(metadata metadata, hooks ...string) bool {
	for _, hook := range hooks {
		if metadata.Metadata.Annotations[hookAnnotation] == hook {
			return true
		}
	}
	return false
}

func isTestHook(hookEvents []release.Hook_Event) bool {
	for _, event := range hookEvents {
		if event == release.Hook_RELEASE_TEST_FAILURE || event == release.Hook_RELEASE_TEST_SUCCESS {
			return true
		}
	}

	return false
}

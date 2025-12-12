package diff

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	jsonpatch "gomodules.xyz/jsonpatch/v2"
	"sigs.k8s.io/yaml"

	"github.com/databus23/helm-diff/v3/manifest"
)

// StructuredEntry captures machine-readable diff information for a resource.
type StructuredEntry struct {
	APIVersion        string         `json:"apiVersion,omitempty"`
	Kind              string         `json:"kind,omitempty"`
	Namespace         string         `json:"namespace,omitempty"`
	Name              string         `json:"name,omitempty"`
	ChangeType        string         `json:"changeType,omitempty"`
	ResourceStatus    ResourceStatus `json:"resourceStatus"`
	Changes           []FieldChange  `json:"changes,omitempty"`
	ChangesSuppressed bool           `json:"changesSuppressed,omitempty"`
}

// ResourceStatus indicates whether manifests existed before or after the diff.
type ResourceStatus struct {
	OldExists bool `json:"oldExists"`
	NewExists bool `json:"newExists"`
}

// FieldChange stores a JSON-Pointer path and the change that occurred.
type FieldChange struct {
	Path     string      `json:"path,omitempty"`
	Field    string      `json:"field,omitempty"`
	Change   string      `json:"change"`
	OldValue interface{} `json:"oldValue,omitempty"`
	NewValue interface{} `json:"newValue,omitempty"`
}

func buildStructuredEntry(key, changeType, kind string, suppressedKinds []string, oldContent, newContent *manifest.MappingResult) (*StructuredEntry, error) {
	entry := &StructuredEntry{
		ChangeType: changeType,
		ResourceStatus: ResourceStatus{
			OldExists: manifestExists(oldContent),
			NewExists: manifestExists(newContent),
		},
	}

	isSuppressed := containsKind(suppressedKinds, kind)
	entry.ChangesSuppressed = isSuppressed

	oldJSON, oldObj, err := manifestToJSON(oldContent)
	if err != nil {
		return nil, fmt.Errorf("convert old manifest: %w", err)
	}
	newJSON, newObj, err := manifestToJSON(newContent)
	if err != nil {
		return nil, fmt.Errorf("convert new manifest: %w", err)
	}

	entry.populateMetadata(key, oldObj, newObj)

	if isSuppressed {
		return entry, nil
	}

	if changeType == "MODIFY" && oldJSON != nil && newJSON != nil {
		changes, err := calculateFieldChanges(oldJSON, newJSON)
		if err != nil {
			return nil, err
		}
		entry.Changes = changes
	}

	return entry, nil
}

func manifestExists(m *manifest.MappingResult) bool {
	return m != nil && strings.TrimSpace(m.Content) != ""
}

func manifestToJSON(m *manifest.MappingResult) ([]byte, map[string]interface{}, error) {
	if m == nil || strings.TrimSpace(m.Content) == "" {
		return nil, nil, nil
	}
	jsonBytes, err := yaml.YAMLToJSON([]byte(m.Content))
	if err != nil {
		return nil, nil, err
	}

	if len(jsonBytes) == 0 || string(jsonBytes) == "null" {
		return jsonBytes, nil, nil
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &obj); err != nil {
		return nil, nil, err
	}

	return jsonBytes, obj, nil
}

func (e *StructuredEntry) populateMetadata(key string, objects ...map[string]interface{}) {
	for _, obj := range objects {
		if obj == nil {
			continue
		}
		if e.APIVersion == "" {
			if v, ok := obj["apiVersion"].(string); ok {
				e.APIVersion = v
			}
		}
		if e.Kind == "" {
			if v, ok := obj["kind"].(string); ok {
				e.Kind = v
			}
		}
		if meta, ok := obj["metadata"].(map[string]interface{}); ok {
			if e.Name == "" {
				if v, ok := meta["name"].(string); ok {
					e.Name = v
				}
			}
			if e.Namespace == "" {
				if v, ok := meta["namespace"].(string); ok {
					e.Namespace = v
				}
			}
		}
	}

	if e.Kind == "" || e.Name == "" || e.Namespace == "" || e.APIVersion == "" {
		templateData := ReportTemplateSpec{}
		if err := templateData.loadFromKey(key); err == nil {
			if e.Kind == "" {
				e.Kind = templateData.Kind
			}
			if e.Name == "" {
				e.Name = templateData.Name
			}
			if e.Namespace == "" {
				e.Namespace = templateData.Namespace
			}
			if e.APIVersion == "" {
				e.APIVersion = templateData.API
			}
		}
	}
}

func calculateFieldChanges(oldJSON, newJSON []byte) ([]FieldChange, error) {
	patch, err := jsonpatch.CreatePatch(oldJSON, newJSON)
	if err != nil {
		return nil, err
	}
	if len(patch) == 0 {
		return nil, nil
	}

	var oldDoc interface{}
	if err := json.Unmarshal(oldJSON, &oldDoc); err != nil {
		return nil, err
	}

	changes := make([]FieldChange, 0, len(patch))
	for _, operation := range patch {
		tokens := pointerTokens(operation.Path)
		path, field := splitPointer(tokens)

		change := FieldChange{
			Path:   path,
			Field:  field,
			Change: operation.Operation,
		}

		if (operation.Operation == "remove" || operation.Operation == "replace") && operation.Path != "" {
			if value, err := resolveJSONPointer(oldDoc, operation.Path); err == nil {
				change.OldValue = value
			}
		}

		if (operation.Operation == "add" || operation.Operation == "replace") && operation.Value != nil {
			change.NewValue = operation.Value
		}

		changes = append(changes, change)
	}

	return changes, nil
}

func resolveJSONPointer(doc interface{}, pointer string) (interface{}, error) {
	if pointer == "" {
		return doc, nil
	}

	rawTokens := strings.Split(pointer, "/")[1:]
	tokens := make([]string, 0, len(rawTokens))
	for _, rawToken := range rawTokens {
		tokens = append(tokens, decodePointerToken(rawToken))
	}

	current := doc

	for _, rawToken := range tokens {
		token := rawToken
		switch typed := current.(type) {
		case map[string]interface{}:
			current = typed[token]
		case []interface{}:
			if token == "-" {
				return nil, fmt.Errorf("pointer '-' not addressable")
			}
			index, err := strconv.Atoi(token)
			if err != nil || index < 0 || index >= len(typed) {
				return nil, fmt.Errorf("invalid array index %s", token)
			}
			current = typed[index]
		default:
			return nil, fmt.Errorf("unable to navigate pointer through %T", current)
		}
	}

	return current, nil
}

func containsKind(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}

func pointerTokens(pointer string) []string {
	if pointer == "" {
		return nil
	}
	rawTokens := strings.Split(pointer, "/")[1:]
	tokens := make([]string, 0, len(rawTokens))
	for _, token := range rawTokens {
		tokens = append(tokens, decodePointerToken(token))
	}
	return tokens
}

func decodePointerToken(token string) string {
	token = strings.ReplaceAll(token, "~1", "/")
	token = strings.ReplaceAll(token, "~0", "~")
	return token
}

func splitPointer(tokens []string) (string, string) {
	if len(tokens) == 0 {
		return "", ""
	}
	parent := formatPath(tokens[:len(tokens)-1])
	field := tokens[len(tokens)-1]
	return parent, field
}

func formatPath(tokens []string) string {
	if len(tokens) == 0 {
		return ""
	}
	segments := []string{}
	for _, token := range tokens {
		if token == "" {
			continue
		}
		if isArrayIndex(token) {
			if len(segments) == 0 {
				segments = append(segments, "["+token+"]")
			} else {
				segments[len(segments)-1] = segments[len(segments)-1] + "[" + token + "]"
			}
			continue
		}
		segments = append(segments, token)
	}
	return strings.Join(segments, ".")
}

func isArrayIndex(token string) bool {
	if token == "" {
		return false
	}
	for _, r := range token {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

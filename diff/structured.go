package diff

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"
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
	patchBytes, err := jsonpatch.CreateMergePatch(oldJSON, newJSON)
	if err != nil {
		return nil, err
	}
	trimmed := bytes.TrimSpace(patchBytes)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("{}")) {
		return nil, nil
	}

	var patch interface{}
	if err := json.Unmarshal(patchBytes, &patch); err != nil {
		return nil, err
	}

	var oldDoc interface{}
	if len(oldJSON) > 0 {
		if err := json.Unmarshal(oldJSON, &oldDoc); err != nil {
			return nil, err
		}
	}

	var newDoc interface{}
	if len(newJSON) > 0 {
		if err := json.Unmarshal(newJSON, &newDoc); err != nil {
			return nil, err
		}
	}

	var changes []FieldChange
	if err := walkPatch(&changes, nil, patch, oldDoc, newDoc); err != nil {
		return nil, err
	}
	return changes, nil
}

func walkPatch(changes *[]FieldChange, tokens []string, patchNode, oldNode, newNode interface{}) error {
	switch typed := patchNode.(type) {
	case map[string]interface{}:
		if len(typed) == 0 {
			return nil
		}
		oldMap, _ := oldNode.(map[string]interface{})
		newMap, _ := newNode.(map[string]interface{})
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		for _, key := range keys {
			var oldChild interface{}
			var newChild interface{}
			if oldMap != nil {
				oldChild = oldMap[key]
			}
			if newMap != nil {
				newChild = newMap[key]
			}
			if err := walkPatch(changes, append(tokens, key), typed[key], oldChild, newChild); err != nil {
				return err
			}
		}
	case []interface{}:
		return diffArrayNodes(changes, tokens, oldNode, newNode)
	default:
		path, field := splitTokens(tokens)
		change := FieldChange{
			Path:  path,
			Field: field,
		}

		if patchNode == nil {
			change.Change = "remove"
			change.OldValue = oldNode
		} else {
			if oldNode == nil {
				change.Change = "add"
				change.NewValue = newNode
			} else {
				if reflect.DeepEqual(oldNode, newNode) {
					return nil
				}
				change.Change = "replace"
				change.OldValue = oldNode
				change.NewValue = newNode
			}
		}
		*changes = append(*changes, change)
	}
	return nil
}

func diffArrayNodes(changes *[]FieldChange, tokens []string, oldNode, newNode interface{}) error {
	oldArr, _ := oldNode.([]interface{})
	newArr, _ := newNode.([]interface{})

	maxLen := len(oldArr)
	if len(newArr) > maxLen {
		maxLen = len(newArr)
	}

	for i := 0; i < maxLen; i++ {
		next := append(tokens, strconv.Itoa(i))
		var oldVal interface{}
		var newVal interface{}
		if i < len(oldArr) {
			oldVal = oldArr[i]
		}
		if i < len(newArr) {
			newVal = newArr[i]
		}

		switch {
		case oldVal != nil && newVal != nil:
			if reflect.DeepEqual(oldVal, newVal) {
				continue
			}
			subPatch, err := createNodePatch(oldVal, newVal)
			if err != nil {
				return err
			}
			if subPatch == nil {
				path, field := splitTokens(next)
				*changes = append(*changes, FieldChange{
					Path:     path,
					Field:    field,
					Change:   "replace",
					OldValue: oldVal,
					NewValue: newVal,
				})
				continue
			}
			if err := walkPatch(changes, next, subPatch, oldVal, newVal); err != nil {
				return err
			}
		case oldVal != nil:
			path, field := splitTokens(next)
			*changes = append(*changes, FieldChange{
				Path:     path,
				Field:    field,
				Change:   "remove",
				OldValue: oldVal,
			})
		case newVal != nil:
			path, field := splitTokens(next)
			*changes = append(*changes, FieldChange{
				Path:     path,
				Field:    field,
				Change:   "add",
				NewValue: newVal,
			})
		}
	}

	return nil
}

func createNodePatch(oldNode, newNode interface{}) (interface{}, error) {
	oldJSON, err := json.Marshal(oldNode)
	if err != nil {
		return nil, err
	}
	newJSON, err := json.Marshal(newNode)
	if err != nil {
		return nil, err
	}
	patchBytes, err := jsonpatch.CreateMergePatch(oldJSON, newJSON)
	if err != nil {
		return nil, err
	}
	trimmed := bytes.TrimSpace(patchBytes)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("{}")) {
		return nil, nil
	}
	var patch interface{}
	if err := json.Unmarshal(patchBytes, &patch); err != nil {
		return nil, err
	}
	return patch, nil
}

func splitTokens(tokens []string) (string, string) {
	if len(tokens) == 0 {
		return "", ""
	}
	return formatPath(tokens[:len(tokens)-1]), tokens[len(tokens)-1]
}

func containsKind(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
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

package manifest

import (
	"fmt"

	jsoniter "github.com/json-iterator/go"
)

func deleteStatusAndTidyMetadata(obj []byte) (map[string]any, error) {
	var objectMap map[string]any
	err := jsoniter.Unmarshal(obj, &objectMap)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal byte sequence: %w", err)
	}

	if objectMap == nil {
		return nil, nil
	}

	delete(objectMap, "status")

	metadata, ok := objectMap["metadata"].(map[string]any)
	if !ok {
		return objectMap, nil
	}

	delete(metadata, "managedFields")
	delete(metadata, "generation")
	delete(metadata, "creationTimestamp")
	delete(metadata, "resourceVersion")
	delete(metadata, "uid")

	// See the below for the goal of this metadata tidy logic.
	// https://github.com/databus23/helm-diff/issues/326#issuecomment-1008253274
	pruneNestedMap(metadata, "annotations",
		"meta.helm.sh/release-name",
		"meta.helm.sh/release-namespace",
		"deployment.kubernetes.io/revision",
	)

	pruneNestedMap(metadata, "labels",
		"app.kubernetes.io/managed-by",
		"helm.toolkit.fluxcd.io/name",
		"helm.toolkit.fluxcd.io/namespace",
	)

	return objectMap, nil
}

// pruneNestedMap removes the given fields from the nested map found at key in
// target. If the nested map ends up empty afterwards, key itself is removed
// from target. A null-valued key (e.g. a chart template rendering "labels:"
// with nothing under it) is removed as well, so it does not show up as a
// confusing "- labels:" diff entry. See
// https://github.com/databus23/helm-diff/issues/1064
func pruneNestedMap(target map[string]interface{}, key string, fields ...string) {
	sub, ok := target[key].(map[string]interface{})
	if !ok {
		// The key is either absent or explicitly null. A null value (JSON
		// "labels": null) would otherwise survive as an empty "labels:" entry
		// in the rendered YAML and produce a meaningless diff.
		if target[key] == nil {
			delete(target, key)
		}
		return
	}

	for _, field := range fields {
		delete(sub, field)
	}

	if len(sub) == 0 {
		delete(target, key)
	}
}

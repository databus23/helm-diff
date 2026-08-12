package manifest

import (
	"fmt"

	jsoniter "github.com/json-iterator/go"
)

func deleteStatusAndTidyMetadata(obj []byte) (map[string]interface{}, error) {
	var objectMap map[string]interface{}
	err := jsoniter.Unmarshal(obj, &objectMap)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal byte sequence: %w", err)
	}

	delete(objectMap, "status")

	metadata := objectMap["metadata"].(map[string]interface{})

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
	pruneNestedMap(metadata, "labels", "app.kubernetes.io/managed-by")

	return objectMap, nil
}

// pruneNestedMap removes the given fields from the nested map found at key in
// target. If the nested map ends up empty afterwards, key itself is removed
// from target.
func pruneNestedMap(target map[string]interface{}, key string, fields ...string) {
	sub, ok := target[key].(map[string]interface{})
	if !ok {
		return
	}

	for _, field := range fields {
		delete(sub, field)
	}

	if len(sub) == 0 {
		delete(target, key)
	}
}

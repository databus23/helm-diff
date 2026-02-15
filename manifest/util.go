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

	if a := metadata["annotations"]; a != nil {
		annotations := a.(map[string]any)
		delete(annotations, "meta.helm.sh/release-name")
		delete(annotations, "meta.helm.sh/release-namespace")
		delete(annotations, "deployment.kubernetes.io/revision")

		if len(annotations) == 0 {
			delete(metadata, "annotations")
		}
	}

	return objectMap, nil
}

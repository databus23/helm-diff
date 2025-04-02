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
	if a := metadata["annotations"]; a != nil {
		annotations := a.(map[string]interface{})
		delete(annotations, "meta.helm.sh/release-name")
		delete(annotations, "meta.helm.sh/release-namespace")
		delete(annotations, "deployment.kubernetes.io/revision")

		if len(annotations) == 0 {
			delete(metadata, "annotations")
		}
	}

	return objectMap, nil
}

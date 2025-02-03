package manifest

import (
	"bytes"
	"encoding/json"
	"fmt"

	jsonpatch "github.com/evanphx/json-patch/v5"
	jsoniter "github.com/json-iterator/go"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/kube"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/yaml"
)

const (
	Helm2TestSuccessHook = "test-success"
	Helm3TestHook        = "test"
)

func Generate(actionConfig *action.Configuration, originalManifest, targetManifest []byte) ([]byte, []byte, error) {
	var err error
	original, err := actionConfig.KubeClient.Build(bytes.NewBuffer(originalManifest), false)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to build kubernetes objects from original release manifest: %w", err)
	}
	target, err := actionConfig.KubeClient.Build(bytes.NewBuffer(targetManifest), false)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to build kubernetes objects from new release manifest: %w", err)
	}
	releaseManifest, installManifest := make([]byte, 0), make([]byte, 0)
	// to be deleted
	targetResources := make(map[string]bool)
	for _, r := range target {
		targetResources[objectKey(r)] = true
	}
	for _, r := range original {
		if !targetResources[objectKey(r)] {
			out, _ := yaml.Marshal(r.Object)
			releaseManifest = append(releaseManifest, yamlSeparator...)
			releaseManifest = append(releaseManifest, out...)
		}
	}

	existingResources := make(map[string]bool)
	for _, r := range original {
		existingResources[objectKey(r)] = true
	}

	var toBeCreated kube.ResourceList
	for _, r := range target {
		if !existingResources[objectKey(r)] {
			toBeCreated = append(toBeCreated, r)
		}
	}

	toBeUpdated, err := existingResourceConflict(toBeCreated)
	if err != nil {
		return nil, nil, fmt.Errorf("rendered manifests contain a resource that already exists. Unable to continue with update: %w", err)
	}

	_ = toBeUpdated.Visit(func(r *resource.Info, err error) error {
		if err != nil {
			return err
		}
		original.Append(r)
		return nil
	})

	err = target.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}
		kind := info.Mapping.GroupVersionKind.Kind

		// Fetch the current object for the three-way merge
		helper := resource.NewHelper(info.Client, info.Mapping)
		currentObj, err := helper.Get(info.Namespace, info.Name)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("could not get information about the resource: %w", err)
			}
			// to be created
			out, _ := yaml.Marshal(info.Object)
			installManifest = append(installManifest, yamlSeparator...)
			installManifest = append(installManifest, out...)
			return nil
		}
		// to be updated
		out, _ := jsoniter.ConfigCompatibleWithStandardLibrary.Marshal(currentObj)
		pruneObj, err := deleteStatusAndTidyMetadata(out)
		if err != nil {
			return fmt.Errorf("prune current obj %q with kind %s: %w", info.Name, kind, err)
		}
		pruneOut, err := yaml.Marshal(pruneObj)
		if err != nil {
			return fmt.Errorf("prune current out %q with kind %s: %w", info.Name, kind, err)
		}
		releaseManifest = append(releaseManifest, yamlSeparator...)
		releaseManifest = append(releaseManifest, pruneOut...)

		originalInfo := original.Get(info)
		if originalInfo == nil {
			return fmt.Errorf("could not find %q", info.Name)
		}

		patch, patchType, err := createPatch(originalInfo.Object, currentObj, info)
		if err != nil {
			return err
		}

		helper.ServerDryRun = true
		targetObj, err := helper.Patch(info.Namespace, info.Name, patchType, patch, nil)
		if err != nil {
			return fmt.Errorf("cannot patch %q with kind %s: %w", info.Name, kind, err)
		}
		out, _ = jsoniter.ConfigCompatibleWithStandardLibrary.Marshal(targetObj)
		pruneObj, err = deleteStatusAndTidyMetadata(out)
		if err != nil {
			return fmt.Errorf("prune current obj %q with kind %s: %w", info.Name, kind, err)
		}
		pruneOut, err = yaml.Marshal(pruneObj)
		if err != nil {
			return fmt.Errorf("prune current out %q with kind %s: %w", info.Name, kind, err)
		}
		installManifest = append(installManifest, yamlSeparator...)
		installManifest = append(installManifest, pruneOut...)
		return nil
	})

	return releaseManifest, installManifest, err
}

func createPatch(originalObj, currentObj runtime.Object, target *resource.Info) ([]byte, types.PatchType, error) {
	oldData, err := json.Marshal(originalObj)
	if err != nil {
		return nil, types.StrategicMergePatchType, fmt.Errorf("serializing current configuration: %w", err)
	}
	newData, err := json.Marshal(target.Object)
	if err != nil {
		return nil, types.StrategicMergePatchType, fmt.Errorf("serializing target configuration: %w", err)
	}

	// Even if currentObj is nil (because it was not found), it will marshal just fine
	currentData, err := json.Marshal(currentObj)
	if err != nil {
		return nil, types.StrategicMergePatchType, fmt.Errorf("serializing live configuration: %w", err)
	}
	// kind := target.Mapping.GroupVersionKind.Kind
	// if kind == "Deployment" {
	// 	curr, _ := yaml.Marshal(currentObj)
	// 	fmt.Println(string(curr))
	// }

	// Get a versioned object
	versionedObject := kube.AsVersioned(target)

	// Unstructured objects, such as CRDs, may not have an not registered error
	// returned from ConvertToVersion. Anything that's unstructured should
	// use the jsonpatch.CreateMergePatch. Strategic Merge Patch is not supported
	// on objects like CRDs.
	_, isUnstructured := versionedObject.(runtime.Unstructured)

	// On newer K8s versions, CRDs aren't unstructured but has this dedicated type
	_, isCRD := versionedObject.(*apiextv1.CustomResourceDefinition)

	if isUnstructured || isCRD {
		// fall back to generic JSON merge patch
		patch, err := jsonpatch.CreateMergePatch(oldData, newData)
		return patch, types.MergePatchType, err
	}

	patchMeta, err := strategicpatch.NewPatchMetaFromStruct(versionedObject)
	if err != nil {
		return nil, types.StrategicMergePatchType, fmt.Errorf("unable to create patch metadata from object: %w", err)
	}

	patch, err := strategicpatch.CreateThreeWayMergePatch(oldData, newData, currentData, patchMeta, true)
	return patch, types.StrategicMergePatchType, err
}

func objectKey(r *resource.Info) string {
	gvk := r.Object.GetObjectKind().GroupVersionKind()
	return fmt.Sprintf("%s/%s/%s/%s", gvk.GroupVersion().String(), gvk.Kind, r.Namespace, r.Name)
}

func existingResourceConflict(resources kube.ResourceList) (kube.ResourceList, error) {
	var requireUpdate kube.ResourceList

	err := resources.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}

		helper := resource.NewHelper(info.Client, info.Mapping)
		_, err = helper.Get(info.Namespace, info.Name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("could not get information about the resource: %w", err)
		}

		requireUpdate.Append(info)
		return nil
	})

	return requireUpdate, err
}

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

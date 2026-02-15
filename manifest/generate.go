package manifest

import (
	"bytes"
	"encoding/json"
	"fmt"

	jsonpatch "github.com/evanphx/json-patch/v5"
	jsoniter "github.com/json-iterator/go"
	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/kube"
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
		// For unstructured objects (CRDs, CRs), we need to perform a three-way merge
		// to detect manual changes made in the cluster.
		//
		// The approach is:
		// 1. Create a patch from old -> new (chart changes)
		// 2. Apply this patch to current (live state with manual changes)
		// 3. Create a patch from current -> merged result
		// 4. If chart changed (old != new), return step 3's patch
		// 5. If chart unchanged (old == new), build desired state by applying new onto current
		//    (preserving live-only fields), then diff current -> desired to detect drift

		// Clean metadata fields that shouldn't be compared (they're server-managed)
		// This prevents "resourceVersion: Invalid value: 0" errors when dry-running patches
		cleanedOldData, err := cleanMetadataForPatch(oldData)
		if err != nil {
			return nil, types.MergePatchType, fmt.Errorf("cleaning old metadata: %w", err)
		}
		cleanedNewData, err := cleanMetadataForPatch(newData)
		if err != nil {
			return nil, types.MergePatchType, fmt.Errorf("cleaning new metadata: %w", err)
		}
		cleanedCurrentData, err := cleanMetadataForPatch(currentData)
		if err != nil {
			return nil, types.MergePatchType, fmt.Errorf("cleaning current metadata: %w", err)
		}

		// Step 1: Create patch from old -> new (what the chart wants to change)
		chartChanges, err := jsonpatch.CreateMergePatch(cleanedOldData, cleanedNewData)
		if err != nil {
			return nil, types.MergePatchType, fmt.Errorf("creating chart changes patch: %w", err)
		}

		// Check if chart actually changed anything
		chartChanged := !isPatchEmpty(chartChanges)

		if chartChanged {
			// Step 2: Apply chart changes to current (merge chart changes with live state)
			mergedData, err := jsonpatch.MergePatch(cleanedCurrentData, chartChanges)
			if err != nil {
				return nil, types.MergePatchType, fmt.Errorf("applying chart changes to current: %w", err)
			}

			// Step 3: Create patch from current -> merged (what to apply to current)
			// This patch, when applied to current, will produce the merged result
			patch, err := jsonpatch.CreateMergePatch(cleanedCurrentData, mergedData)
			if err != nil {
				return nil, types.MergePatchType, fmt.Errorf("creating patch from current to merged: %w", err)
			}
			return patch, types.MergePatchType, nil
		}

		// Chart didn't change (old == new), but we need to detect if current diverges
		// from the chart state on chart-owned fields.
		// Build desired state by applying new onto current (preserves live-only additions),
		// then diff current -> desired to detect drift on chart-owned fields.
		desiredData, err := jsonpatch.MergePatch(cleanedCurrentData, cleanedNewData)
		if err != nil {
			return nil, types.MergePatchType, fmt.Errorf("building desired state: %w", err)
		}
		patch, err := jsonpatch.CreateMergePatch(cleanedCurrentData, desiredData)
		if err != nil {
			return nil, types.MergePatchType, fmt.Errorf("creating patch from current to desired: %w", err)
		}
		return patch, types.MergePatchType, nil
	}

	patchMeta, err := strategicpatch.NewPatchMetaFromStruct(versionedObject)
	if err != nil {
		return nil, types.StrategicMergePatchType, fmt.Errorf("unable to create patch metadata from object: %w", err)
	}

	patch, err := strategicpatch.CreateThreeWayMergePatch(oldData, newData, currentData, patchMeta, true)
	return patch, types.StrategicMergePatchType, err
}

func isPatchEmpty(patch []byte) bool {
	return len(patch) == 0 || string(patch) == "{}" || string(patch) == "null"
}

func cleanMetadataForPatch(data []byte) ([]byte, error) {
	objMap, err := deleteStatusAndTidyMetadata(data)
	if err != nil {
		return nil, err
	}
	if objMap == nil {
		return []byte("null"), nil
	}
	return json.Marshal(objMap)
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

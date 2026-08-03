package manifest

import (
	"testing"

	"github.com/stretchr/testify/require"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/resource"
)

// TestCreatePatchForUnstructured tests the three-way merge implementation for unstructured objects (CRDs, CRs).
// This tests the fix for issue #917 where manual changes to CRs were not being detected.
func TestCreatePatchForUnstructured(t *testing.T) {
	tests := []struct {
		name           string
		original       runtime.Object
		current        runtime.Object
		target         *resource.Info
		expectedChange bool
		description    string
	}{
		{
			name: "CR with manual annotation in current (chart doesn't change anything)",
			original: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "keda.sh/v1alpha1",
					"kind":       "ScaledObject",
					"metadata": map[string]interface{}{
						"name":      "test-so",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"maxReplicaCount": 10,
					},
				},
			},
			current: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "keda.sh/v1alpha1",
					"kind":       "ScaledObject",
					"metadata": map[string]interface{}{
						"name":      "test-so",
						"namespace": "default",
						"annotations": map[string]interface{}{
							"manual-change": "true",
						},
					},
					"spec": map[string]interface{}{
						"maxReplicaCount": 10,
					},
				},
			},
			target: &resource.Info{
				Mapping: &meta.RESTMapping{
					GroupVersionKind: schema.GroupVersionKind{
						Group:   "keda.sh",
						Version: "v1alpha1",
						Kind:    "ScaledObject",
					},
				},
				Object: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "keda.sh/v1alpha1",
						"kind":       "ScaledObject",
						"metadata": map[string]interface{}{
							"name":      "test-so",
							"namespace": "default",
						},
						"spec": map[string]interface{}{
							"maxReplicaCount": 10,
						},
					},
				},
			},
			expectedChange: false,
			description:    "Manual annotation not present in chart should be preserved; no effective change expected from three-way merge",
		},
		{
			name: "CR with no manual changes",
			original: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "keda.sh/v1alpha1",
					"kind":       "ScaledObject",
					"metadata": map[string]interface{}{
						"name":      "test-so",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"maxReplicaCount": 10,
					},
				},
			},
			current: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "keda.sh/v1alpha1",
					"kind":       "ScaledObject",
					"metadata": map[string]interface{}{
						"name":      "test-so",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"maxReplicaCount": 10,
					},
				},
			},
			target: &resource.Info{
				Mapping: &meta.RESTMapping{
					GroupVersionKind: schema.GroupVersionKind{
						Group:   "keda.sh",
						Version: "v1alpha1",
						Kind:    "ScaledObject",
					},
				},
				Object: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "keda.sh/v1alpha1",
						"kind":       "ScaledObject",
						"metadata": map[string]interface{}{
							"name":      "test-so",
							"namespace": "default",
						},
						"spec": map[string]interface{}{
							"maxReplicaCount": 10,
						},
					},
				},
			},
			expectedChange: false,
			description:    "No changes should result in empty patch",
		},
		{
			name: "CR with chart change and manual change on different fields",
			original: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "keda.sh/v1alpha1",
					"kind":       "ScaledObject",
					"metadata": map[string]interface{}{
						"name":      "test-so",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"maxReplicaCount": 10,
						"minReplicaCount": 1,
					},
				},
			},
			current: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "keda.sh/v1alpha1",
					"kind":       "ScaledObject",
					"metadata": map[string]interface{}{
						"name":      "test-so",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"maxReplicaCount": 30,
						"minReplicaCount": 2,
					},
				},
			},
			target: &resource.Info{
				Mapping: &meta.RESTMapping{
					GroupVersionKind: schema.GroupVersionKind{
						Group:   "keda.sh",
						Version: "v1alpha1",
						Kind:    "ScaledObject",
					},
				},
				Object: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "keda.sh/v1alpha1",
						"kind":       "ScaledObject",
						"metadata": map[string]interface{}{
							"name":      "test-so",
							"namespace": "default",
						},
						"spec": map[string]interface{}{
							"maxReplicaCount": 20,
							"minReplicaCount": 1,
						},
					},
				},
			},
			expectedChange: true,
			description:    "Chart changes maxReplicaCount, manual changes minReplicaCount - should merge",
		},
		{
			name: "CR with field unchanged in chart but modified in live state (issue #917)",
			original: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "keda.sh/v1alpha1",
					"kind":       "ScaledObject",
					"metadata": map[string]interface{}{
						"name":      "test-so",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"maxReplicaCount": 10,
					},
				},
			},
			current: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "keda.sh/v1alpha1",
					"kind":       "ScaledObject",
					"metadata": map[string]interface{}{
						"name":      "test-so",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"maxReplicaCount": 30,
					},
				},
			},
			target: &resource.Info{
				Mapping: &meta.RESTMapping{
					GroupVersionKind: schema.GroupVersionKind{
						Group:   "keda.sh",
						Version: "v1alpha1",
						Kind:    "ScaledObject",
					},
				},
				Object: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "keda.sh/v1alpha1",
						"kind":       "ScaledObject",
						"metadata": map[string]interface{}{
							"name":      "test-so",
							"namespace": "default",
						},
						"spec": map[string]interface{}{
							"maxReplicaCount": 10,
						},
					},
				},
			},
			expectedChange: true,
			description:    "Field maxReplicaCount is 10 in both original and target, but 30 in current - should detect drift",
		},
		{
			name: "CR with chart overriding manual change",
			original: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "keda.sh/v1alpha1",
					"kind":       "ScaledObject",
					"metadata": map[string]interface{}{
						"name":      "test-so",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"maxReplicaCount": 10,
					},
				},
			},
			current: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "keda.sh/v1alpha1",
					"kind":       "ScaledObject",
					"metadata": map[string]interface{}{
						"name":      "test-so",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"maxReplicaCount": 30,
					},
				},
			},
			target: &resource.Info{
				Mapping: &meta.RESTMapping{
					GroupVersionKind: schema.GroupVersionKind{
						Group:   "keda.sh",
						Version: "v1alpha1",
						Kind:    "ScaledObject",
					},
				},
				Object: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "keda.sh/v1alpha1",
						"kind":       "ScaledObject",
						"metadata": map[string]interface{}{
							"name":      "test-so",
							"namespace": "default",
						},
						"spec": map[string]interface{}{
							"maxReplicaCount": 20,
						},
					},
				},
			},
			expectedChange: true,
			description:    "Chart explicitly changes maxReplicaCount from 10 to 20, overriding manual value of 30",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patch, patchType, err := createPatch(tt.original, tt.current, tt.target)
			require.NoError(t, err, "createPatch should not return an error")
			require.Equal(t, types.MergePatchType, patchType, "patch type should be MergePatchType for unstructured objects")

			t.Logf("Patch result: %s", string(patch))

			if tt.expectedChange {
				require.NotEmpty(t, patch, tt.description+": expected patch to detect changes, got empty patch")
				require.NotEqual(t, []byte("{}"), patch, tt.description+": expected patch to detect changes, got empty patch")
				require.NotEqual(t, []byte("null"), patch, tt.description+": expected patch to detect changes, got null patch")
			} else {
				// No changes expected: patch must be empty or effectively empty ("{}" or "null")
				if len(patch) == 0 || string(patch) == "{}" || string(patch) == "null" {
					return
				}
				require.Failf(t, tt.description+": expected no changes, got unexpected patch", "unexpected patch: %s", string(patch))
			}
		})
	}
}

// TestCreatePatchForCRD tests the three-way merge implementation for CRD objects.
// This ensures the isCRD branch in createPatch is properly covered.
func TestCreatePatchForCRD(t *testing.T) {
	originalCRD := &apiextv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "crds.example.com",
		},
		Spec: apiextv1.CustomResourceDefinitionSpec{
			Group: "example.com",
			Names: apiextv1.CustomResourceDefinitionNames{
				Plural:   "crds",
				Singular: "crd",
				Kind:     "Crd",
			},
			Versions: []apiextv1.CustomResourceDefinitionVersion{
				{
					Name:    "v1",
					Served:  true,
					Storage: true,
				},
			},
		},
	}

	currentCRD := &apiextv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "crds.example.com",
		},
		Spec: apiextv1.CustomResourceDefinitionSpec{
			Group: "example.com",
			Names: apiextv1.CustomResourceDefinitionNames{
				Plural:   "crds",
				Singular: "crd",
				Kind:     "Crd",
			},
			Versions: []apiextv1.CustomResourceDefinitionVersion{
				{
					Name:    "v1",
					Served:  true,
					Storage: true,
				},
			},
		},
	}

	targetCRD := &apiextv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "crds.example.com",
		},
		Spec: apiextv1.CustomResourceDefinitionSpec{
			Group: "example.com",
			Names: apiextv1.CustomResourceDefinitionNames{
				Plural:   "crds",
				Singular: "crd",
				Kind:     "Crd",
			},
			Versions: []apiextv1.CustomResourceDefinitionVersion{
				{
					Name:    "v1",
					Served:  true,
					Storage: true,
				},
			},
		},
	}

	target := &resource.Info{
		Mapping: &meta.RESTMapping{
			GroupVersionKind: schema.GroupVersionKind{
				Group:   "apiextensions.k8s.io",
				Version: "v1",
				Kind:    "CustomResourceDefinition",
			},
		},
		Object: targetCRD,
	}

	patch, patchType, err := createPatch(originalCRD, currentCRD, target)
	require.NoError(t, err, "createPatch should not return an error for CRD")
	require.Equal(t, types.MergePatchType, patchType, "patch type should be MergePatchType for CRD objects")

	t.Logf("CRD Patch result: %s", string(patch))

	require.Equal(t, "{}", string(patch), "CRD with no changes should result in empty patch")
}

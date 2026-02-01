package manifest

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
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
			expectedChange: true,
			description:    "Manual annotation that is not in chart will cause diff to remove it (JSON merge patch semantics)",
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

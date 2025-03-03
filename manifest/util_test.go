package manifest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_deleteStatusAndTidyMetadata(t *testing.T) {
	tests := []struct {
		name    string
		obj     []byte
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name:    "not valid json",
			obj:     []byte("notvalid"),
			want:    nil,
			wantErr: true,
		},
		{
			name: "valid json",
			obj: []byte(`
{
    "apiVersion": "apps/v1",
    "kind": "Deployment",
    "metadata": {
        "annotations": {
            "deployment.kubernetes.io/revision": "1",
			"meta.helm.sh/release-name": "test-release",
			"meta.helm.sh/release-namespace": "test-ns",
			"other-annot": "value"
        },
        "creationTimestamp": "2025-03-03T10:07:50Z",
        "generation": 1,
        "name": "nginx-deployment",
        "namespace": "test-ns",
        "resourceVersion": "33648",
        "uid": "7a8d3b74-6452-46f4-a31f-4fdacbe828ac"
    },
    "spec": {
        "template": {
            "spec": {
                "containers": [
                    {
                        "image": "nginx:1.14.2",
                        "imagePullPolicy": "IfNotPresent",
                        "name": "nginx"
                    }
                ]
            }
        }
    },
    "status": {
        "availableReplicas": 2
    }
}
`),
			want: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"other-annot": "value",
					},
					"name":      "nginx-deployment",
					"namespace": "test-ns",
				},
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"containers": []interface{}{
								map[string]interface{}{
									"image":           "nginx:1.14.2",
									"imagePullPolicy": "IfNotPresent",
									"name":            "nginx",
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := deleteStatusAndTidyMetadata(tt.obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("deleteStatusAndTidyMetadata() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			require.EqualValuesf(t, tt.want, got, "deleteStatusAndTidyMetadata() = %v, want %v", got, tt.want)
		})
	}
}

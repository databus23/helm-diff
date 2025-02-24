package manifest_test

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"os"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	. "github.com/databus23/helm-diff/v3/manifest"
)

func foundObjects(result map[string]*MappingResult) []string {
	objs := make([]string, 0, len(result))
	for k := range result {
		objs = append(objs, k)
	}
	sort.Strings(objs)
	return objs
}

func TestPod(t *testing.T) {
	spec, err := os.ReadFile("testdata/pod.yaml")
	require.NoError(t, err)

	require.Equal(t,
		[]string{"default, nginx, Pod (v1)"},
		foundObjects(Parse(string(spec), "default", false)),
	)
}

func TestPodNamespace(t *testing.T) {
	spec, err := os.ReadFile("testdata/pod_namespace.yaml")
	require.NoError(t, err)

	require.Equal(t,
		[]string{"batcave, nginx, Pod (v1)"},
		foundObjects(Parse(string(spec), "default", false)),
	)
}

func TestPodHook(t *testing.T) {
	spec, err := os.ReadFile("testdata/pod_hook.yaml")
	require.NoError(t, err)

	require.Equal(t,
		[]string{"default, nginx, Pod (v1)"},
		foundObjects(Parse(string(spec), "default", false)),
	)

	require.Equal(t,
		[]string{"default, nginx, Pod (v1)"},
		foundObjects(Parse(string(spec), "default", false, "test-success")),
	)

	require.Equal(t,
		[]string{},
		foundObjects(Parse(string(spec), "default", false, "test")),
	)
}

func TestDeployV1(t *testing.T) {
	spec, err := os.ReadFile("testdata/deploy_v1.yaml")
	require.NoError(t, err)

	require.Equal(t,
		[]string{"default, nginx, Deployment (apps)"},
		foundObjects(Parse(string(spec), "default", false)),
	)
}

func TestDeployV1Beta1(t *testing.T) {
	spec, err := os.ReadFile("testdata/deploy_v1beta1.yaml")
	require.NoError(t, err)

	require.Equal(t,
		[]string{"default, nginx, Deployment (apps)"},
		foundObjects(Parse(string(spec), "default", false)),
	)
}

func TestList(t *testing.T) {
	spec, err := os.ReadFile("testdata/list.yaml")
	require.NoError(t, err)

	require.Equal(t,
		[]string{
			"default, prometheus-operator-example, PrometheusRule (monitoring.coreos.com)",
			"default, prometheus-operator-example2, PrometheusRule (monitoring.coreos.com)",
		},
		foundObjects(Parse(string(spec), "default", false)),
	)
}

func TestConfigMapList(t *testing.T) {
	spec, err := os.ReadFile("testdata/configmaplist_v1.yaml")
	require.NoError(t, err)

	require.Equal(t,
		[]string{
			"default, configmap-2-1, ConfigMap (v1)",
			"default, configmap-2-2, ConfigMap (v1)",
		},
		foundObjects(Parse(string(spec), "default", false)),
	)
}

func TestSecretList(t *testing.T) {
	spec, err := os.ReadFile("testdata/secretlist_v1.yaml")
	require.NoError(t, err)

	require.Equal(t,
		[]string{
			"default, my-secret-1, Secret (v1)",
			"default, my-secret-2, Secret (v1)",
		},
		foundObjects(Parse(string(spec), "default", false)),
	)
}

func TestEmpty(t *testing.T) {
	spec, err := os.ReadFile("testdata/empty.yaml")
	require.NoError(t, err)

	require.Equal(t,
		[]string{},
		foundObjects(Parse(string(spec), "default", false)),
	)
}

func TestBaseNameAnnotation(t *testing.T) {
	spec, err := os.ReadFile("testdata/secret_immutable.yaml")
	require.NoError(t, err)

	require.Equal(t,
		[]string{"default, bat-secret, Secret (v1)"},
		foundObjects(Parse(string(spec), "default", false)),
	)
}

func TestParseObject(t *testing.T) {
	for _, tt := range []struct {
		name        string
		filename    string
		releaseName string
		kind        string
		oldRelease  string
	}{
		{
			name:        "no release info",
			filename:    "testdata/pod_no_release_annotations.yaml",
			releaseName: "testNS, nginx, Pod (v1)",
			kind:        "Pod",
			oldRelease:  "",
		},
		{
			name:        "get old release info",
			filename:    "testdata/pod_release_annotations.yaml",
			releaseName: "testNS, nginx, Pod (v1)",
			kind:        "Pod",
			oldRelease:  "oldNS/oldReleaseName",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := os.ReadFile(tt.filename)
			require.NoError(t, err)

			obj, _, err := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(spec, nil, nil)
			require.NoError(t, err)

			release, oldRelease, err := ParseObject(obj, "testNS")
			require.NoError(t, err)

			require.Equal(t, tt.releaseName, release.Name)
			require.Equal(t, tt.kind, release.Kind)
			require.Equal(t, tt.oldRelease, oldRelease)
		})
	}
}

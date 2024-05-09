package manifest_test

import (
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

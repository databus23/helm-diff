package manifest_test

import (
	"io/ioutil"
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
	spec, err := ioutil.ReadFile("testdata/pod.yaml")
	require.NoError(t, err)

	require.Equal(t,
		[]string{"default, nginx, Pod (v1)"},
		foundObjects(Parse(string(spec), "default", false)),
	)
}

func TestPodNamespace(t *testing.T) {
	spec, err := ioutil.ReadFile("testdata/pod_namespace.yaml")
	require.NoError(t, err)

	require.Equal(t,
		[]string{"batcave, nginx, Pod (v1)"},
		foundObjects(Parse(string(spec), "default", false)),
	)
}

func TestPodHook(t *testing.T) {
	spec, err := ioutil.ReadFile("testdata/pod_hook.yaml")
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
	spec, err := ioutil.ReadFile("testdata/deploy_v1.yaml")
	require.NoError(t, err)

	require.Equal(t,
		[]string{"default, nginx, Deployment (apps)"},
		foundObjects(Parse(string(spec), "default", false)),
	)
}

func TestDeployV1Beta1(t *testing.T) {
	spec, err := ioutil.ReadFile("testdata/deploy_v1beta1.yaml")
	require.NoError(t, err)

	require.Equal(t,
		[]string{"default, nginx, Deployment (apps)"},
		foundObjects(Parse(string(spec), "default", false)),
	)
}

func TestList(t *testing.T) {
	spec, err := ioutil.ReadFile("testdata/list.yaml")
	require.NoError(t, err)

	require.Equal(t,
		[]string{
			"default, prometheus-operator-example, PrometheusRule (monitoring.coreos.com)",
			"default, prometheus-operator-example2, PrometheusRule (monitoring.coreos.com)",
		},
		foundObjects(Parse(string(spec), "default", false)),
	)
}

func TestEmpty(t *testing.T) {
	spec, err := ioutil.ReadFile("testdata/empty.yaml")
	require.NoError(t, err)

	require.Equal(t,
		[]string{},
		foundObjects(Parse(string(spec), "default", false)),
	)
}

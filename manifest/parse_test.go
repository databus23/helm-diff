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
		foundObjects(Parse(string(spec), "default")),
	)
}

func TestPodNamespace(t *testing.T) {
	spec, err := ioutil.ReadFile("testdata/pod_namespace.yaml")
	require.NoError(t, err)

	require.Equal(t,
		[]string{"batcave, nginx, Pod (v1)"},
		foundObjects(Parse(string(spec), "default")),
	)
}

func TestPodHook(t *testing.T) {
	spec, err := ioutil.ReadFile("testdata/pod_hook.yaml")
	require.NoError(t, err)

	require.Equal(t,
		[]string{"default, nginx, Pod (v1)"},
		foundObjects(Parse(string(spec), "default")),
	)

	require.Equal(t,
		[]string{"default, nginx, Pod (v1)"},
		foundObjects(Parse(string(spec), "default", "test-success")),
	)

	require.Equal(t,
		[]string{},
		foundObjects(Parse(string(spec), "default", "test")),
	)
}

func TestDeployV1(t *testing.T) {
	spec, err := ioutil.ReadFile("testdata/deploy_v1.yaml")
	require.NoError(t, err)

	require.Equal(t,
		[]string{"default, nginx, Deployment (apps)"},
		foundObjects(Parse(string(spec), "default")),
	)
}

func TestDeployV1Beta1(t *testing.T) {
	spec, err := ioutil.ReadFile("testdata/deploy_v1beta1.yaml")
	require.NoError(t, err)

	require.Equal(t,
		[]string{"default, nginx, Deployment (apps)"},
		foundObjects(Parse(string(spec), "default")),
	)
}

func TestEmpty(t *testing.T) {
	spec, err := ioutil.ReadFile("testdata/empty.yaml")
	require.NoError(t, err)

	require.Equal(t,
		[]string{},
		foundObjects(Parse(string(spec), "default")),
	)
}

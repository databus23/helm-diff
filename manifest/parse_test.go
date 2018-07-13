package manifest_test

import (
	"io/ioutil"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	. "github.com/databus23/helm-diff/manifest"
)

func foundObjects(result map[string]*MappingResult) []string {
	objs := make([]string, 0, len(result))
	for k, _ := range result {
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

func TestDeployV1(t *testing.T) {
	spec, err := ioutil.ReadFile("testdata/deploy_v1.yaml")
	require.NoError(t, err)

	require.Equal(t,
		[]string{"default, nginx, Deployment (apps/v1)"},
		foundObjects(Parse(string(spec), "default")),
	)
}

func TestDeployV1Beta1(t *testing.T) {
	spec, err := ioutil.ReadFile("testdata/deploy_v1beta1.yaml")
	require.NoError(t, err)

	require.Equal(t,
		[]string{"default, nginx, Deployment (apps/v1beta1)"},
		foundObjects(Parse(string(spec), "default")),
	)
}

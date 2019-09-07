package manifest_test

import (
	"io/ioutil"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	rspb "helm.sh/helm/pkg/release"
	rel "k8s.io/helm/pkg/proto/hapi/release"

	. "github.com/databus23/helm-diff/manifest"
)

func foundObjects(result map[string]*MappingResult) []string {
	objs := make([]string, 0, len(result))
	for k := range result {
		objs = append(objs, k)
	}
	sort.Strings(objs)
	return objs
}

func releaseV2(manifest string, namespace string) ReleaseResponse {
	return ReleaseResponse{Release: &rel.Release{Manifest: manifest, Namespace: namespace}}
}

func releaseV3(manifest string, namespace string) ReleaseResponse {
	return ReleaseResponse{ReleaseV3: &rspb.Release{Manifest: manifest, Namespace: namespace}}
}

func TestPod(t *testing.T) {
	spec, err := ioutil.ReadFile("testdata/pod.yaml")
	require.NoError(t, err)

	require.Equal(t,
		[]string{"default, nginx, Pod (v1)"},
		foundObjects(Parse(releaseV2(string(spec), "default"))),
	)
	require.Equal(t,
		[]string{"default, nginx, Pod (v1)"},
		foundObjects(Parse(releaseV3(string(spec), "default"))),
	)
}

func TestPodNamespace(t *testing.T) {
	spec, err := ioutil.ReadFile("testdata/pod_namespace.yaml")
	require.NoError(t, err)

	require.Equal(t,
		[]string{"batcave, nginx, Pod (v1)"},
		foundObjects(Parse(releaseV2(string(spec), "default"))),
	)
	require.Equal(t,
		[]string{"batcave, nginx, Pod (v1)"},
		foundObjects(Parse(releaseV3(string(spec), "default"))),
	)
}

func TestDeployV1(t *testing.T) {
	spec, err := ioutil.ReadFile("testdata/deploy_v1.yaml")
	require.NoError(t, err)

	require.Equal(t,
		[]string{"default, nginx, Deployment (apps)"},
		foundObjects(Parse(releaseV2(string(spec), "default"))),
	)
	require.Equal(t,
		[]string{"default, nginx, Deployment (apps)"},
		foundObjects(Parse(releaseV3(string(spec), "default"))),
	)
}

func TestDeployV1Beta1(t *testing.T) {
	spec, err := ioutil.ReadFile("testdata/deploy_v1beta1.yaml")
	require.NoError(t, err)

	require.Equal(t,
		[]string{"default, nginx, Deployment (apps)"},
		foundObjects(Parse(releaseV2(string(spec), "default"))),
	)
	require.Equal(t,
		[]string{"default, nginx, Deployment (apps)"},
		foundObjects(Parse(releaseV3(string(spec), "default"))),
	)
}

func TestEmpty(t *testing.T) {
	spec, err := ioutil.ReadFile("testdata/empty.yaml")
	require.NoError(t, err)

	require.Equal(t,
		[]string{},
		foundObjects(Parse(releaseV2(string(spec), "default"))),
	)
	require.Equal(t,
		[]string{},
		foundObjects(Parse(releaseV3(string(spec), "default"))),
	)
}

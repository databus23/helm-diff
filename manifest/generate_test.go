package manifest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/yaml"
)

func infoFor(t *testing.T, obj runtime.Object, gvk schema.GroupVersionKind) *resource.Info {
	t.Helper()
	return &resource.Info{
		Object:    obj,
		Namespace: "default",
		Name:      "nginx",
		Mapping:   &meta.RESTMapping{GroupVersionKind: gvk},
	}
}

func deployment(replicas int32, image string, labels map[string]string) *appsv1.Deployment {
	return &appsv1.Deployment{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{Name: "nginx", Namespace: "default", Labels: labels},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "nginx", Image: image}},
				},
			},
		},
	}
}

// TestCreatePatchApplyLocally_Strategic asserts that applying the three-way
// merge patch locally yields the same object the API server would have returned
// from the dry-run patch: the change from the chart is applied, while the field
// only present in the cluster is preserved.
func TestCreatePatchApplyLocally_Strategic(t *testing.T) {
	gvk := appsv1.SchemeGroupVersion.WithKind("Deployment")

	original := deployment(1, "nginx:1.0", nil)
	target := deployment(2, "nginx:2.0", nil)

	// The live object carries a field nobody in the release manifests knows
	// about, e.g. set by a mutating webhook or another controller.
	live := deployment(1, "nginx:1.0", nil)
	live.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{{Name: "INJECTED", Value: "yes"}}

	patch, err := createPatch(original, live, infoFor(t, target, gvk))
	require.NoError(t, err)
	require.Equal(t, types.StrategicMergePatchType, patch.patchType)
	require.NotNil(t, patch.patchMeta)

	liveData, err := yaml.Marshal(live)
	require.NoError(t, err)
	liveJSON, err := yaml.YAMLToJSON(liveData)
	require.NoError(t, err)

	merged, err := patch.apply(liveJSON)
	require.NoError(t, err)

	var got appsv1.Deployment
	require.NoError(t, yaml.Unmarshal(merged, &got))

	assert.Equal(t, int32(2), *got.Spec.Replicas)
	assert.Equal(t, "nginx:2.0", got.Spec.Template.Spec.Containers[0].Image)
	assert.Equal(t,
		[]corev1.EnvVar{{Name: "INJECTED", Value: "yes"}},
		got.Spec.Template.Spec.Containers[0].Env,
		"a field only present in the cluster must survive the local merge")
}

// TestCreatePatchApplyLocally_StrategicRemoval asserts that a field dropped from
// the chart is removed by the local merge, which is what distinguishes the
// three-way merge from a plain two-way merge.
func TestCreatePatchApplyLocally_StrategicRemoval(t *testing.T) {
	gvk := appsv1.SchemeGroupVersion.WithKind("Deployment")

	original := deployment(1, "nginx:1.0", map[string]string{"keep": "me", "drop": "me"})
	target := deployment(1, "nginx:1.0", map[string]string{"keep": "me"})
	live := deployment(1, "nginx:1.0", map[string]string{"keep": "me", "drop": "me"})

	patch, err := createPatch(original, live, infoFor(t, target, gvk))
	require.NoError(t, err)

	liveJSON, err := yaml.Marshal(live)
	require.NoError(t, err)
	liveJSON, err = yaml.YAMLToJSON(liveJSON)
	require.NoError(t, err)

	merged, err := patch.apply(liveJSON)
	require.NoError(t, err)

	var got appsv1.Deployment
	require.NoError(t, yaml.Unmarshal(merged, &got))

	assert.Equal(t, map[string]string{"keep": "me"}, got.ObjectMeta.Labels)
}

// TestCreatePatchApplyLocally_Unstructured covers custom resources, which use a
// plain JSON merge patch instead of a strategic merge patch.
func TestCreatePatchApplyLocally_Unstructured(t *testing.T) {
	gvk := schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Widget"}

	newWidget := func(size string) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "example.com/v1",
			"kind":       "Widget",
			"metadata":   map[string]interface{}{"name": "nginx", "namespace": "default"},
			"spec":       map[string]interface{}{"size": size},
		}}
	}

	original := newWidget("small")
	target := newWidget("large")

	live := newWidget("small")
	require.NoError(t, unstructured.SetNestedField(live.Object, "set-by-the-cluster", "spec", "extra"))

	patch, err := createPatch(original, live, infoFor(t, target, gvk))
	require.NoError(t, err)
	require.Equal(t, types.MergePatchType, patch.patchType)

	liveJSON, err := live.MarshalJSON()
	require.NoError(t, err)

	merged, err := patch.apply(liveJSON)
	require.NoError(t, err)

	var got unstructured.Unstructured
	require.NoError(t, got.UnmarshalJSON(merged))

	size, _, _ := unstructured.NestedString(got.Object, "spec", "size")
	assert.Equal(t, "large", size)
	extra, _, _ := unstructured.NestedString(got.Object, "spec", "extra")
	assert.Equal(t, "set-by-the-cluster", extra, "a field only present in the cluster must survive the local merge")
}

func TestIsPatchNotAllowed(t *testing.T) {
	gr := schema.GroupResource{Group: "apps", Resource: "deployments"}

	assert.True(t, isPatchNotAllowed(apierrors.NewForbidden(gr, "nginx", assert.AnError)))
	assert.True(t, isPatchNotAllowed(apierrors.NewMethodNotSupported(gr, "patch")))
	assert.False(t, isPatchNotAllowed(apierrors.NewNotFound(gr, "nginx")))
	assert.False(t, isPatchNotAllowed(apierrors.NewInternalError(assert.AnError)))
	assert.False(t, isPatchNotAllowed(assert.AnError))
}

func TestResourcePatchApplyUnsupportedType(t *testing.T) {
	p := &resourcePatch{data: []byte("{}"), patchType: types.JSONPatchType}
	_, err := p.apply([]byte("{}"))
	assert.ErrorContains(t, err, "unsupported patch type")
}

// The cases below reproduce the differences that were reported between the
// client-side merge and the server-side dry-run.
//
// The manifests are decoded into unstructured objects because that is what
// helm's kube.Client.Build hands to Generate: they keep whatever the chart
// spelled out, including the zero values a typed object would omit. The live
// object is written the way the API server returns it, with the defaults filled
// in and the zero values gone.

func fromYAML(t *testing.T, manifest string) *unstructured.Unstructured {
	t.Helper()
	obj := &unstructured.Unstructured{}
	require.NoError(t, yaml.Unmarshal([]byte(manifest), &obj.Object))
	return obj
}

// applyLocally is the client-side path of applyPatch: build the patch from the
// three inputs and merge it into the live object without the API server.
func applyLocally(t *testing.T, original, live, target string) map[string]interface{} {
	t.Helper()

	targetObj := fromYAML(t, target)
	liveObj := fromYAML(t, live)
	gvk := targetObj.GroupVersionKind()

	patch, err := createPatch(fromYAML(t, original), liveObj, infoFor(t, targetObj, gvk))
	require.NoError(t, err)

	liveJSON, err := liveObj.MarshalJSON()
	require.NoError(t, err)

	merged, err := patch.apply(liveJSON)
	require.NoError(t, err)

	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(merged, &got))
	return got
}

func nested(t *testing.T, obj map[string]interface{}, fields ...string) (interface{}, bool) {
	t.Helper()
	value, found, err := unstructured.NestedFieldNoCopy(obj, fields...)
	require.NoError(t, err)
	return value, found
}

// The rolling update strategy is defaulted by the API server and pruned by the
// $retainKeys directive the patch carries for spec.strategy.
func TestLocalMerge_KeepsDefaultedRollingUpdate(t *testing.T) {
	chart := `
apiVersion: apps/v1
kind: Deployment
metadata: {name: nginx, namespace: default}
spec:
  strategy: {type: RollingUpdate}
  selector: {matchLabels: {app: nginx}}
  template:
    metadata: {labels: {app: nginx}}
    spec:
      containers: [{name: nginx, image: "%s"}]
`
	live := `
apiVersion: apps/v1
kind: Deployment
metadata: {name: nginx, namespace: default}
spec:
  replicas: 1
  strategy:
    type: RollingUpdate
    rollingUpdate: {maxSurge: 25%, maxUnavailable: 25%}
  selector: {matchLabels: {app: nginx}}
  template:
    metadata: {labels: {app: nginx}}
    spec:
      containers: [{name: nginx, image: "nginx:1.0"}]
`
	got := applyLocally(t, fmt.Sprintf(chart, "nginx:1.0"), live, fmt.Sprintf(chart, "nginx:2.0"))

	rollingUpdate, found := nested(t, got, "spec", "strategy", "rollingUpdate")
	require.True(t, found, "the defaulted rolling update strategy must not be dropped")
	assert.Equal(t, map[string]interface{}{"maxSurge": "25%", "maxUnavailable": "25%"}, rollingUpdate)

	replicas, found := nested(t, got, "spec", "replicas")
	require.True(t, found, "the defaulted replica count must not be dropped")
	assert.EqualValues(t, 1, replicas)

	containers, _ := nested(t, got, "spec", "template", "spec", "containers")
	assert.Equal(t, "nginx:2.0", containers.([]interface{})[0].(map[string]interface{})["image"],
		"the actual change must still be applied")
}

// Values the chart spells out but the Go type omits must not show up as
// additions: the API server drops them when it answers the patch.
func TestLocalMerge_DropsExplicitZeroValues(t *testing.T) {
	chart := `
apiVersion: apps/v1
kind: StatefulSet
metadata: {name: meilisearch, namespace: default}
spec:
  serviceName: meilisearch
  selector: {matchLabels: {app: meilisearch}}
  template:
    metadata: {labels: {app: meilisearch}}
    spec:
      hostIPC: false
      hostNetwork: false
      securityContext: {supplementalGroups: [], sysctls: []}
      containers:
        - name: meilisearch
          image: "%s"
          livenessProbe: {initialDelaySeconds: 0, exec: {command: [ok]}}
          readinessProbe: {initialDelaySeconds: 0, exec: {command: [ok]}}
`
	live := `
apiVersion: apps/v1
kind: StatefulSet
metadata: {name: meilisearch, namespace: default}
spec:
  serviceName: meilisearch
  selector: {matchLabels: {app: meilisearch}}
  template:
    metadata: {labels: {app: meilisearch}}
    spec:
      securityContext: {}
      containers:
        - name: meilisearch
          image: getmeili/meilisearch:v1.0
          livenessProbe: {exec: {command: [ok]}}
          readinessProbe: {exec: {command: [ok]}}
`
	got := applyLocally(t, fmt.Sprintf(chart, "getmeili/meilisearch:v1.0"), live, fmt.Sprintf(chart, "getmeili/meilisearch:v1.1"))

	podSpec, found := nested(t, got, "spec", "template", "spec")
	require.True(t, found)
	pod := podSpec.(map[string]interface{})

	assert.NotContains(t, pod, "hostIPC", "hostIPC: false must be normalized away")
	assert.NotContains(t, pod, "hostNetwork", "hostNetwork: false must be normalized away")
	assert.Equal(t, map[string]interface{}{}, pod["securityContext"],
		"supplementalGroups: [] and sysctls: [] must be normalized away")

	container := pod["containers"].([]interface{})[0].(map[string]interface{})
	assert.Equal(t, "getmeili/meilisearch:v1.1", container["image"], "the actual change must still be applied")
	for _, probe := range []string{"livenessProbe", "readinessProbe"} {
		assert.NotContains(t, container[probe], "initialDelaySeconds",
			"initialDelaySeconds: 0 must be normalized away in the %s", probe)
	}
}

// NetworkPolicy ports are an atomic list, so the patch replaces the whole list
// and drops the protocol the API server defaulted in.
func TestLocalMerge_KeepsDefaultedFieldsInAtomicLists(t *testing.T) {
	chart := `
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata: {name: mongodb, namespace: default}
spec:
  podSelector: {matchLabels: {app: "%s"}}
  ingress:
    - ports: [{port: 27017}]
`
	live := `
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata: {name: mongodb, namespace: default}
spec:
  podSelector: {matchLabels: {app: mongo}}
  policyTypes: [Ingress]
  ingress:
    - ports: [{port: 27017, protocol: TCP}]
`
	got := applyLocally(t, fmt.Sprintf(chart, "mongo"), live, fmt.Sprintf(chart, "mongodb"))

	ingress, found := nested(t, got, "spec", "ingress")
	require.True(t, found)
	port := ingress.([]interface{})[0].(map[string]interface{})["ports"].([]interface{})[0].(map[string]interface{})
	assert.Equal(t, "TCP", port["protocol"], "the defaulted protocol must not be dropped")
	assert.EqualValues(t, 27017, port["port"])

	app, _ := nested(t, got, "spec", "podSelector", "matchLabels", "app")
	assert.Equal(t, "mongodb", app, "the actual change must still be applied")
}

// Volume claim templates are an atomic list too, and the API server populates
// them with a type, a status and a volume mode.
func TestLocalMerge_KeepsServerPopulatedVolumeClaimTemplates(t *testing.T) {
	chart := `
apiVersion: apps/v1
kind: StatefulSet
metadata: {name: mongodb, namespace: default}
spec:
  serviceName: mongodb
  selector: {matchLabels: {app: mongodb}}
  template:
    metadata: {labels: {app: mongodb}}
    spec:
      containers: [{name: mongodb, image: "%s"}]
  volumeClaimTemplates:
    - metadata: {name: data}
      spec:
        accessModes: [ReadWriteOnce]
        resources: {requests: {storage: 8Gi}}
`
	live := `
apiVersion: apps/v1
kind: StatefulSet
metadata: {name: mongodb, namespace: default}
spec:
  serviceName: mongodb
  selector: {matchLabels: {app: mongodb}}
  template:
    metadata: {labels: {app: mongodb}}
    spec:
      containers: [{name: mongodb, image: "mongo:6.0"}]
  volumeClaimTemplates:
    - apiVersion: v1
      kind: PersistentVolumeClaim
      metadata: {name: data}
      spec:
        accessModes: [ReadWriteOnce]
        resources: {requests: {storage: 8Gi}}
        volumeMode: Filesystem
      status: {phase: Pending}
`
	got := applyLocally(t, fmt.Sprintf(chart, "mongo:6.0"), live, fmt.Sprintf(chart, "mongo:7.0"))

	templates, found := nested(t, got, "spec", "volumeClaimTemplates")
	require.True(t, found)
	claim := templates.([]interface{})[0].(map[string]interface{})

	assert.Equal(t, "v1", claim["apiVersion"], "the server-populated apiVersion must not be dropped")
	assert.Equal(t, "PersistentVolumeClaim", claim["kind"], "the server-populated kind must not be dropped")
	assert.Equal(t, map[string]interface{}{"phase": "Pending"}, claim["status"], "the status must not be dropped")
	assert.Equal(t, "Filesystem", claim["spec"].(map[string]interface{})["volumeMode"],
		"the defaulted volumeMode must not be dropped")
}

// A field the chart itself stops setting is a real change and must stay
// visible, even where the API server would default it back.
func TestLocalMerge_ReportsFieldsTheChartRemoves(t *testing.T) {
	deploy := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  namespace: default
  labels: {%s}
spec:
  selector: {matchLabels: {app: nginx}}
  template:
    metadata: {labels: {app: nginx}}
    spec:
      containers: [{name: nginx, image: "nginx:1.0"}]
`
	withLabel := fmt.Sprintf(deploy, "drop: me")
	got := applyLocally(t, withLabel, withLabel, fmt.Sprintf(deploy, ""))

	labels, found := nested(t, got, "metadata", "labels")
	assert.False(t, found, "a label the chart no longer sets must be reported as removed, got %v", labels)
}

// Drift must survive the restoration pass: a value the cluster and the chart
// disagree about is not a defaulted field.
func TestLocalMerge_KeepsReportingDrift(t *testing.T) {
	chart := `
apiVersion: apps/v1
kind: Deployment
metadata: {name: nginx, namespace: default}
spec:
  replicas: 1
  selector: {matchLabels: {app: nginx}}
  template:
    metadata: {labels: {app: nginx}}
    spec:
      containers: [{name: nginx, image: nginx:1.0}]
`
	// Someone scaled and re-imaged the deployment by hand.
	live := `
apiVersion: apps/v1
kind: Deployment
metadata: {name: nginx, namespace: default}
spec:
  replicas: 5
  selector: {matchLabels: {app: nginx}}
  template:
    metadata: {labels: {app: nginx}}
    spec:
      containers: [{name: nginx, image: nginx:9.9}]
`
	got := applyLocally(t, chart, live, chart)

	replicas, _ := nested(t, got, "spec", "replicas")
	assert.EqualValues(t, 1, replicas, "drifted replicas must be reset to the chart value")
	containers, _ := nested(t, got, "spec", "template", "spec", "containers")
	assert.Equal(t, "nginx:1.0", containers.([]interface{})[0].(map[string]interface{})["image"],
		"a drifted image must be reset to the chart value")
}

// A chart that leaves a value unset renders the field as an explicit `null`
// (`replicas:` with nothing after it). The key is present in both manifests, so
// the patch carries it as a change rather than a deletion and wipes the value
// the API server had defaulted into the cluster - even though the chart itself
// did not change at all.
func TestLocalMerge_KeepsDefaultsUnderExplicitNulls(t *testing.T) {
	chart := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: librechat-librechat-rag-api
  namespace: librechat
  labels: {app.kubernetes.io/instance: librechat}
spec:
  replicas:
  selector:
    matchLabels: {app.kubernetes.io/name: rag}
  template:
    metadata:
      annotations:
      labels: {app.kubernetes.io/name: rag}
    spec:
      securityContext: {}
      containers:
        - name: rag
          image: "ghcr.io/danny-avila/librechat-rag-api-dev-lite:%s"
          imagePullPolicy: IfNotPresent
          ports: [{name: http, containerPort: 8000, protocol: TCP}]
          livenessProbe:
            null
          readinessProbe:
            null
          resources: {}
      volumes:
`
	live := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: librechat-librechat-rag-api
  namespace: librechat
  labels: {app.kubernetes.io/instance: librechat}
spec:
  replicas: 1
  revisionHistoryLimit: 10
  progressDeadlineSeconds: 600
  selector:
    matchLabels: {app.kubernetes.io/name: rag}
  strategy:
    type: RollingUpdate
    rollingUpdate: {maxSurge: 25%, maxUnavailable: 25%}
  template:
    metadata:
      labels: {app.kubernetes.io/name: rag}
    spec:
      securityContext: {}
      dnsPolicy: ClusterFirst
      restartPolicy: Always
      containers:
        - name: rag
          image: "ghcr.io/danny-avila/librechat-rag-api-dev-lite:latest"
          imagePullPolicy: IfNotPresent
          ports: [{name: http, containerPort: 8000, protocol: TCP}]
          resources: {}
          terminationMessagePath: /dev/termination-log
`
	// The chart is byte-for-byte the same on both sides.
	unchanged := fmt.Sprintf(chart, "latest")
	got := applyLocally(t, unchanged, live, unchanged)

	replicas, found := nested(t, got, "spec", "replicas")
	require.True(t, found, "an unset `replicas:` must not wipe the replica count the API server defaulted in")
	assert.EqualValues(t, 1, replicas)

	rollingUpdate, found := nested(t, got, "spec", "strategy", "rollingUpdate")
	require.True(t, found, "the defaulted rolling update strategy must survive too")
	assert.Equal(t, map[string]interface{}{"maxSurge": "25%", "maxUnavailable": "25%"}, rollingUpdate)

	for _, field := range []string{"revisionHistoryLimit", "progressDeadlineSeconds"} {
		_, found = nested(t, got, "spec", field)
		assert.True(t, found, "the defaulted %s must survive too", field)
	}
}

// The same explicit `null`, but this time the chart really does change what it
// asks for. The removal is then a change and has to stay visible.
func TestLocalMerge_ReportsExplicitNullThatTheChartIntroduces(t *testing.T) {
	deploy := `
apiVersion: apps/v1
kind: Deployment
metadata: {name: rag, namespace: default}
spec:
  replicas: %s
  selector: {matchLabels: {app: rag}}
  template:
    metadata: {labels: {app: rag}}
    spec:
      containers: [{name: rag, image: rag:1.0}]
`
	live := `
apiVersion: apps/v1
kind: Deployment
metadata: {name: rag, namespace: default}
spec:
  replicas: 3
  selector: {matchLabels: {app: rag}}
  template:
    metadata: {labels: {app: rag}}
    spec:
      containers: [{name: rag, image: rag:1.0}]
`
	// The chart used to pin three replicas and now leaves the field unset.
	got := applyLocally(t, fmt.Sprintf(deploy, "3"), live, fmt.Sprintf(deploy, ""))

	_, found := nested(t, got, "spec", "replicas")
	assert.False(t, found, "a replica count the chart stops pinning is a real change and must be reported")
}

// A chart that renders an empty collection literally (`rules: []`, the branch
// grafana takes when no sidecar is enabled) disagrees with the cluster about how
// to write "nothing": the API server stores objects as protobuf, which cannot
// tell an empty list from an absent one, and answers with `rules: null`. The two
// are the same object and must not be reported as a change.
func TestLocalMerge_TreatsEmptyCollectionsAsEqual(t *testing.T) {
	role := func(rules string) string {
		return fmt.Sprintf(`
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata: {name: grafana, namespace: grafana}
rules: %s
`, rules)
	}

	t.Run("chart writes [], cluster answers null", func(t *testing.T) {
		got := applyLocally(t, role("[]"), role("null"), role("[]"))
		rules, _ := nested(t, got, "rules")
		assert.Nil(t, rules, "an empty list must not be reported as a change against a null")
	})

	t.Run("chart writes null, cluster answers []", func(t *testing.T) {
		got := applyLocally(t, role("null"), role("[]"), role("null"))
		rules, _ := nested(t, got, "rules")
		assert.Equal(t, []interface{}{}, rules, "a null must not be reported as a change against an empty list")
	})

	// Emptying a collection that actually had entries is a real change.
	t.Run("chart empties a populated list", func(t *testing.T) {
		populated := `
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata: {name: grafana, namespace: grafana}
rules:
  - apiGroups: [""]
    resources: [configmaps]
    verbs: [get]
`
		got := applyLocally(t, populated, populated, role("[]"))
		rules, _ := nested(t, got, "rules")
		assert.Empty(t, rules, "emptying a populated list is a real change and must be reported")
	})
}

// Once the API server has refused one dry-run patch it will refuse the rest, so
// the whole run switches to the local merge instead of collecting a denied
// request, and an audit log entry, per resource.
func TestClientSideFallbackAppliesToTheWholeRun(t *testing.T) {
	forbidden := apierrors.NewForbidden(
		schema.GroupResource{Group: "apps", Resource: "deployments"}, "nginx", assert.AnError)

	info := infoFor(t, deployment(1, "nginx:1.0", nil), appsv1.SchemeGroupVersion.WithKind("Deployment"))

	var warnings bytes.Buffer
	fallback := &clientSideFallback{mode: ThreeWayMergeAuto, warn: &warnings}
	require.Equal(t, ThreeWayMergeAuto, fallback.currentMode())

	assert.True(t, fallback.patchDenied(info, forbidden))
	assert.Equal(t, ThreeWayMergeClient, fallback.currentMode(),
		"the rest of the run must not retry a patch the API server already refused")

	firstWarning := warnings.String()
	assert.Contains(t, firstWarning, "Falling back to computing the three-way merge locally")

	assert.True(t, fallback.patchDenied(info, forbidden))
	assert.Equal(t, firstWarning, warnings.String(), "the fallback must be reported once per run")
	assert.Equal(t, ThreeWayMergeClient, fallback.currentMode())
}

// The explicit modes are never overridden by the fallback.
func TestClientSideFallbackLeavesExplicitModesAlone(t *testing.T) {
	var warnings bytes.Buffer
	fallback := &clientSideFallback{mode: ThreeWayMergeClient, warn: &warnings}

	assert.True(t, fallback.patchDenied(infoFor(t, deployment(1, "nginx:1.0", nil), appsv1.SchemeGroupVersion.WithKind("Deployment")), assert.AnError))
	assert.Equal(t, ThreeWayMergeClient, fallback.currentMode())
	assert.Empty(t, warnings.String(), "a run that never asks the server has nothing to report")
}

// Both manifests asking for the same value is not the same as neither asking for
// anything. normalize drops an explicit `hostNetwork: false` through omitempty,
// and restoring the live value there would undo the very correction the upgrade
// makes.
func TestLocalMerge_KeepsReportingDriftOnChartOwnedZeroValues(t *testing.T) {
	pod := func(hostNetwork string) string {
		return fmt.Sprintf(`
apiVersion: apps/v1
kind: Deployment
metadata: {name: nginx, namespace: default}
spec:
  selector: {matchLabels: {app: nginx}}
  template:
    metadata: {labels: {app: nginx}}
    spec:
      hostNetwork: %s
      containers: [{name: nginx, image: nginx:1.0}]
`, hostNetwork)
	}

	// The chart pins false on both sides; someone flipped the cluster to true.
	got := applyLocally(t, pod("false"), pod("true"), pod("false"))

	value, found := nested(t, got, "spec", "template", "spec", "hostNetwork")
	assert.False(t, found && value == true,
		"a chart-owned value must not be restored from the cluster, got hostNetwork=%v", value)
}

// A custom resource is stored as JSON, which keeps null, [] and {} apart, and a
// JSON merge patch already leaves untouched live fields alone. Neither the
// empty-value equivalence nor the restoration pass may run over one.
func TestLocalMerge_LeavesCustomResourcesToTheMergePatch(t *testing.T) {
	widget := func(items string) string {
		return fmt.Sprintf(`
apiVersion: example.com/v1
kind: Widget
metadata: {name: w, namespace: default}
spec:
  items: %s
  size: large
`, items)
	}

	t.Run("null to empty list is a change the chart asked for", func(t *testing.T) {
		got := applyLocally(t, widget("null"), widget("null"), widget("[]"))
		items, _ := nested(t, got, "spec", "items")
		assert.Equal(t, []interface{}{}, items, "the chart changed null to [], which must survive")
	})

	t.Run("empty list to null is a change the chart asked for", func(t *testing.T) {
		got := applyLocally(t, widget("[]"), widget("[]"), widget("null"))
		items, found := nested(t, got, "spec", "items")
		assert.Nil(t, items, "the chart changed [] to null, which must survive (found=%v)", found)
	})

	t.Run("fields the chart never mentions are left alone by the merge patch", func(t *testing.T) {
		live := `
apiVersion: example.com/v1
kind: Widget
metadata: {name: w, namespace: default}
spec:
  items: null
  size: large
  status: {observed: 3}
`
		got := applyLocally(t, widget("null"), live, widget("null"))
		observed, found := nested(t, got, "spec", "status", "observed")
		require.True(t, found, "a JSON merge patch must not prune a field it never mentions")
		assert.EqualValues(t, 3, observed)
	})
}

// A Forbidden that RBAC did not produce - an admission webhook or a quota
// controller refusing an authorized request - is the upgrade's own outcome and
// must not be worked around locally.
func TestClientSideFallbackDistinguishesAdmissionFromPermissions(t *testing.T) {
	info := infoFor(t, deployment(1, "nginx:1.0", nil), appsv1.SchemeGroupVersion.WithKind("Deployment"))
	info.Mapping.Resource = appsv1.SchemeGroupVersion.WithResource("deployments")

	denial := apierrors.NewForbidden(
		schema.GroupResource{Group: "apps", Resource: "deployments"}, "nginx", assert.AnError)

	t.Run("patching is allowed, so the refusal came from elsewhere", func(t *testing.T) {
		var warnings bytes.Buffer
		fallback := &clientSideFallback{
			mode:     ThreeWayMergeAuto,
			warn:     &warnings,
			canPatch: func(*resource.Info) (bool, error) { return true, nil },
		}

		assert.False(t, fallback.patchDenied(info, denial), "the caller must surface an admission refusal")
		assert.Equal(t, ThreeWayMergeAuto, fallback.currentMode(), "the run must not degrade")
		assert.Empty(t, warnings.String())
	})

	t.Run("patching is not allowed, so it is a permission problem", func(t *testing.T) {
		var warnings bytes.Buffer
		fallback := &clientSideFallback{
			mode:     ThreeWayMergeAuto,
			warn:     &warnings,
			canPatch: func(*resource.Info) (bool, error) { return false, nil },
		}

		assert.True(t, fallback.patchDenied(info, denial))
		assert.Equal(t, ThreeWayMergeClient, fallback.currentMode())
		assert.Contains(t, warnings.String(), "Falling back")
	})

	t.Run("the check itself fails, so the refusal is taken at face value", func(t *testing.T) {
		var warnings bytes.Buffer
		fallback := &clientSideFallback{
			mode:     ThreeWayMergeAuto,
			warn:     &warnings,
			canPatch: func(*resource.Info) (bool, error) { return false, assert.AnError },
		}

		assert.True(t, fallback.patchDenied(info, denial))
		assert.Equal(t, ThreeWayMergeClient, fallback.currentMode())
	})
}

// A 405 says the API server takes no patch for this resource whoever is asking,
// so being entitled to send one changes nothing: the run still has to fall back.
// Only a 403 is ambiguous enough to be worth an authorization review.
func TestClientSideFallbackAlwaysFallsBackOnMethodNotAllowed(t *testing.T) {
	info := infoFor(t, deployment(1, "nginx:1.0", nil), appsv1.SchemeGroupVersion.WithKind("Deployment"))
	notSupported := apierrors.NewMethodNotSupported(
		schema.GroupResource{Group: "apps", Resource: "deployments"}, "patch")

	reviewed := false
	var warnings bytes.Buffer
	fallback := &clientSideFallback{
		mode: ThreeWayMergeAuto,
		warn: &warnings,
		canPatch: func(*resource.Info) (bool, error) {
			reviewed = true
			return true, nil
		},
	}

	assert.True(t, fallback.patchDenied(info, notSupported),
		"a 405 must fall back even where the credentials may patch")
	assert.Equal(t, ThreeWayMergeClient, fallback.currentMode())
	assert.False(t, reviewed, "a 405 is unambiguous, so it is not worth an authorization review")
}

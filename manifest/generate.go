package manifest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"

	jsonpatch "github.com/evanphx/json-patch/v5"
	jsoniter "github.com/json-iterator/go"
	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/kube"
	authorizationv1 "k8s.io/api/authorization/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/yaml"
)

const (
	Helm2TestSuccessHook = "test-success"
	Helm3TestHook        = "test"
)

// ThreeWayMergeMode selects how the three-way merge patch is turned into the
// object that the diff is computed against.
type ThreeWayMergeMode string

const (
	// ThreeWayMergeAuto sends the patch to the API server as a dry-run and
	// falls back to merging locally when the server refuses the request because
	// the user has no permission to patch the resource.
	ThreeWayMergeAuto ThreeWayMergeMode = "auto"
	// ThreeWayMergeServer always sends the patch to the API server as a dry-run
	// and fails when that is not permitted.
	ThreeWayMergeServer ThreeWayMergeMode = "server"
	// ThreeWayMergeClient always merges locally and never sends a patch to the
	// API server, so that only read permissions are required.
	ThreeWayMergeClient ThreeWayMergeMode = "client"
)

// ValidThreeWayMergeModes lists every accepted ThreeWayMergeMode value.
var ValidThreeWayMergeModes = []string{
	string(ThreeWayMergeAuto),
	string(ThreeWayMergeServer),
	string(ThreeWayMergeClient),
}

type generateOptions struct {
	mergeMode ThreeWayMergeMode
}

// GenerateOption customizes the behavior of Generate.
type GenerateOption func(*generateOptions)

// WithThreeWayMergeMode selects how the three-way merge patch is applied.
// It defaults to ThreeWayMergeAuto.
func WithThreeWayMergeMode(mode ThreeWayMergeMode) GenerateOption {
	return func(o *generateOptions) {
		o.mergeMode = mode
	}
}

func Generate(actionConfig *action.Configuration, originalManifest, targetManifest []byte, opts ...GenerateOption) ([]byte, []byte, error) {
	options := generateOptions{mergeMode: ThreeWayMergeAuto}
	for _, opt := range opts {
		opt(&options)
	}

	var err error
	original, err := actionConfig.KubeClient.Build(bytes.NewBuffer(originalManifest), false)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to build kubernetes objects from original release manifest: %w", err)
	}
	target, err := actionConfig.KubeClient.Build(bytes.NewBuffer(targetManifest), false)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to build kubernetes objects from new release manifest: %w", err)
	}
	releaseManifest, installManifest := make([]byte, 0), make([]byte, 0)
	// to be deleted
	targetResources := make(map[string]bool)
	for _, r := range target {
		targetResources[objectKey(r)] = true
	}
	for _, r := range original {
		if !targetResources[objectKey(r)] {
			out, _ := yaml.Marshal(r.Object)
			releaseManifest = append(releaseManifest, yamlSeparator...)
			releaseManifest = append(releaseManifest, out...)
		}
	}

	existingResources := make(map[string]bool)
	for _, r := range original {
		existingResources[objectKey(r)] = true
	}

	var toBeCreated kube.ResourceList
	for _, r := range target {
		if !existingResources[objectKey(r)] {
			toBeCreated = append(toBeCreated, r)
		}
	}

	toBeUpdated, err := existingResourceConflict(toBeCreated)
	if err != nil {
		return nil, nil, fmt.Errorf("rendered manifests contain a resource that already exists. Unable to continue with update: %w", err)
	}

	_ = toBeUpdated.Visit(func(r *resource.Info, err error) error {
		if err != nil {
			return err
		}
		original.Append(r)
		return nil
	})

	fallback := &clientSideFallback{
		mode:     options.mergeMode,
		warn:     os.Stderr,
		canPatch: patchPermissionCheck(actionConfig),
	}

	err = target.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}
		kind := info.Mapping.GroupVersionKind.Kind

		// Fetch the current object for the three-way merge
		helper := resource.NewHelper(info.Client, info.Mapping)
		currentObj, err := helper.Get(info.Namespace, info.Name)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("could not get information about the resource: %w", err)
			}
			// to be created
			out, _ := yaml.Marshal(info.Object)
			installManifest = append(installManifest, yamlSeparator...)
			installManifest = append(installManifest, out...)
			return nil
		}
		// to be updated
		out, _ := jsoniter.ConfigCompatibleWithStandardLibrary.Marshal(currentObj)
		pruneObj, err := deleteStatusAndTidyMetadata(out)
		if err != nil {
			return fmt.Errorf("prune current obj %q with kind %s: %w", info.Name, kind, err)
		}
		pruneOut, err := yaml.Marshal(pruneObj)
		if err != nil {
			return fmt.Errorf("prune current out %q with kind %s: %w", info.Name, kind, err)
		}
		releaseManifest = append(releaseManifest, yamlSeparator...)
		releaseManifest = append(releaseManifest, pruneOut...)

		originalInfo := original.Get(info)
		if originalInfo == nil {
			return fmt.Errorf("could not find %q", info.Name)
		}

		patch, err := createPatch(originalInfo.Object, currentObj, info)
		if err != nil {
			return err
		}

		// `out` still holds the live object, which is what the patch applies to.
		merged, err := applyPatch(helper, info, patch, out, fallback.currentMode(), fallback.patchDenied)
		if err != nil {
			return err
		}

		pruneObj, err = deleteStatusAndTidyMetadata(merged)
		if err != nil {
			return fmt.Errorf("prune current obj %q with kind %s: %w", info.Name, kind, err)
		}
		pruneOut, err = yaml.Marshal(pruneObj)
		if err != nil {
			return fmt.Errorf("prune current out %q with kind %s: %w", info.Name, kind, err)
		}
		installManifest = append(installManifest, yamlSeparator...)
		installManifest = append(installManifest, pruneOut...)
		return nil
	})

	return releaseManifest, installManifest, err
}

// clientSideFallback decides, for the whole run, whether a patch still goes to
// the API server.
//
// A refused dry-run says something about the credentials rather than about the
// resource, so once the server has turned one patch down the remaining
// resources are merged locally as well. Retrying each of them would only collect
// one denied request - and one audit log entry - per object in the release.
type clientSideFallback struct {
	mode ThreeWayMergeMode
	warn io.Writer
	// canPatch reports whether the credentials may patch a resource at all. It
	// is nil when the question cannot be put, and a refusal is then taken at
	// face value.
	canPatch func(*resource.Info) (bool, error)
}

// currentMode is how the next resource should be merged.
func (f *clientSideFallback) currentMode() ThreeWayMergeMode {
	return f.mode
}

// patchDenied records that the API server refused to dry-run a patch. It reports
// whether the run may carry on with the local merge; false means the refusal was
// not about permissions, so the caller has to surface it instead.
func (f *clientSideFallback) patchDenied(info *resource.Info, cause error) bool {
	if f.mode == ThreeWayMergeClient {
		return true
	}

	// A 403 need not come from RBAC. An admission webhook or a quota controller
	// can turn down a request the credentials are entitled to make, and that
	// refusal is precisely what the upgrade would run into as well - working
	// around it locally would hide the outcome the diff exists to predict. Ask
	// whether patching is permitted at all before reading a refusal as a
	// permission problem.
	if f.canPatch != nil {
		if allowed, err := f.canPatch(info); err == nil && allowed {
			return false
		}
	}

	f.mode = ThreeWayMergeClient
	if _, err := fmt.Fprintf(f.warn, "Not allowed to dry-run the patch against the cluster (%v).\n"+
		"Falling back to computing the three-way merge locally for the rest of this run. The diff\n"+
		"may deviate from the actual upgrade result because server-side defaulting and mutating\n"+
		"webhooks are not applied.\n", cause); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed writing fallback warning: %v\n", err)
	}

	return true
}

// patchPermissionCheck asks the API server whether the credentials may patch a
// resource. It returns nil when the question cannot be put - helm hands over a
// *kube.Client in practice, but the interface does not promise one - and a
// refusal is then assumed to be about permissions, which is how the fallback
// behaved before the check existed.
func patchPermissionCheck(actionConfig *action.Configuration) func(*resource.Info) (bool, error) {
	client, ok := actionConfig.KubeClient.(*kube.Client)
	if !ok || client.Factory == nil {
		return nil
	}

	clientset, err := client.Factory.KubernetesClientSet()
	if err != nil {
		return nil
	}

	return func(info *resource.Info) (bool, error) {
		review := &authorizationv1.SelfSubjectAccessReview{
			Spec: authorizationv1.SelfSubjectAccessReviewSpec{
				ResourceAttributes: &authorizationv1.ResourceAttributes{
					Namespace: info.Namespace,
					Name:      info.Name,
					Verb:      "patch",
					Group:     info.Mapping.Resource.Group,
					Resource:  info.Mapping.Resource.Resource,
				},
			},
		}

		result, err := clientset.AuthorizationV1().SelfSubjectAccessReviews().
			Create(context.Background(), review, metav1.CreateOptions{})
		if err != nil {
			return false, err
		}

		return result.Status.Allowed, nil
	}
}

// resourcePatch is the patch computed for a single resource, together with
// everything that is needed to apply it locally instead of on the API server.
type resourcePatch struct {
	data      []byte
	patchType types.PatchType
	// patchMeta describes how the individual fields of the object have to be
	// merged. It is only set for strategic merge patches.
	patchMeta strategicpatch.LookupPatchMeta
	// versionedObject is the target object in its versioned type. It is nil for
	// unstructured resources, which have no Go type to normalize against.
	versionedObject runtime.Object
	// originalData and modifiedData are the manifests of the old and the new
	// release, the two inputs that tell which fields a chart actually asks for.
	originalData []byte
	modifiedData []byte
}

// apply merges the patch into the live object without contacting the API
// server. Unlike the server-side dry-run this needs no permission to patch, but
// it also skips defaulting, validation and mutating webhooks, so the result is
// post-processed to stay as close to the server's answer as possible.
func (p *resourcePatch) apply(liveData []byte) ([]byte, error) {
	var merged []byte
	var err error

	switch p.patchType {
	case types.MergePatchType:
		merged, err = jsonpatch.MergePatch(liveData, p.data)
	case types.StrategicMergePatchType:
		merged, err = strategicpatch.StrategicMergePatchUsingLookupPatchMeta(liveData, p.data, p.patchMeta)
	default:
		return nil, fmt.Errorf("unsupported patch type %q", p.patchType)
	}
	if err != nil {
		return nil, err
	}

	if merged, err = p.normalize(merged); err != nil {
		return nil, err
	}

	// A JSON merge patch carries only what the chart changed between the two
	// release manifests, so it never prunes a field the cluster populated and
	// there is nothing to put back. Custom resources are also stored as JSON
	// rather than protobuf, which keeps `null`, `[]` and `{}` apart, so the
	// restoration pass would misread a chart changing one into another as noise
	// and undo it.
	if p.patchType == types.MergePatchType {
		return merged, nil
	}

	return p.restoreServerPopulatedFields(merged, liveData)
}

// normalize round-trips the merged object through its Go type, the way the API
// server does before it answers a patch request. That drops the empty values
// the manifests spell out explicitly but the type omits, such as
// `initialDelaySeconds: 0`, `hostNetwork: false` or `sysctls: []`, which would
// otherwise show up as additions that the upgrade does not actually make.
func (p *resourcePatch) normalize(merged []byte) ([]byte, error) {
	if p.versionedObject == nil {
		return merged, nil
	}

	objType := reflect.TypeOf(p.versionedObject)
	if objType.Kind() != reflect.Pointer {
		return merged, nil
	}
	typed, ok := reflect.New(objType.Elem()).Interface().(runtime.Object)
	if !ok {
		return merged, nil
	}
	if err := json.Unmarshal(merged, typed); err != nil {
		return nil, fmt.Errorf("decoding the merged object: %w", err)
	}
	out, err := json.Marshal(typed)
	if err != nil {
		return nil, fmt.Errorf("encoding the merged object: %w", err)
	}
	return out, nil
}

// restoreServerPopulatedFields copies back the fields that only the API server
// knows how to fill in.
//
// The merged object loses defaulted values in two ways. A three-way merge patch
// replaces `retainKeys` structs and atomic lists as a whole, which drops what
// the API server had defaulted into them - the rolling update strategy of a
// Deployment, the `protocol: TCP` of a NetworkPolicy port, the `volumeMode` of a
// volume claim template. And a manifest that renders a field as an explicit
// `null`, the way a chart writes `replicas:` for a value it leaves unset, turns
// into a `null` in the patch: the key is in both manifests, so it counts as a
// change rather than a deletion and deletes the live value. Either way the API
// server defaults the field straight back, but client-go does not ship the
// defaulting functions, so the local merge has to recover the value differently.
//
// A field is copied back from the live object when the old and the new release
// manifest agree about it - neither mentions it, or both give it the same value,
// `null` included. Nothing then asked for the live value to go, so its
// disappearance is an artifact of the patch rather than a change to report. Once
// the two manifests disagree the deletion is honored, because that is a change
// the chart really makes.
//
// Only fields missing from the merged object are restored, never values that it
// already carries, so drift between the cluster and the chart is still
// reported.
func (p *resourcePatch) restoreServerPopulatedFields(merged, liveData []byte) ([]byte, error) {
	var mergedObj, liveObj, originalObj, modifiedObj interface{}

	for _, in := range []struct {
		data []byte
		out  *interface{}
	}{
		{merged, &mergedObj},
		{liveData, &liveObj},
		{p.originalData, &originalObj},
		{p.modifiedData, &modifiedObj},
	} {
		if len(in.data) == 0 {
			continue
		}
		if err := json.Unmarshal(in.data, in.out); err != nil {
			return nil, fmt.Errorf("decoding the object to restore defaulted fields: %w", err)
		}
	}

	restored := restoreMissing(mergedObj, liveObj, originalObj, modifiedObj)

	out, err := json.Marshal(restored)
	if err != nil {
		return nil, fmt.Errorf("encoding the object with the restored defaulted fields: %w", err)
	}
	return out, nil
}

// isEmpty reports whether v carries nothing - a null, an empty list or an empty
// map. The three are interchangeable in a stored Kubernetes object, so telling
// them apart in a diff only ever produces noise.
func isEmpty(v interface{}) bool {
	switch v := v.(type) {
	case nil:
		return true
	case []interface{}:
		return len(v) == 0
	case map[string]interface{}:
		return len(v) == 0
	}
	return false
}

// restoreMissing walks merged and live in parallel and copies over the parts of
// live that merged lost without either release manifest asking for it. It
// returns the updated merged value and never overwrites a value merged already
// has, so a field the chart does specify keeps whatever the patch made of it.
func restoreMissing(merged, live, original, modified interface{}) interface{} {
	switch live := live.(type) {
	case map[string]interface{}:
		mergedMap, ok := merged.(map[string]interface{})
		if !ok {
			return merged
		}
		originalMap, _ := original.(map[string]interface{})
		modifiedMap, _ := modified.(map[string]interface{})

		for key, liveValue := range live {
			mergedValue, inMerged := mergedMap[key]
			if inMerged && isEmpty(mergedValue) && isEmpty(liveValue) {
				// Two empty values that differ only in how they are written are
				// not a change: the API server stores objects as protobuf,
				// which cannot tell an empty list from an absent one, so a
				// chart's `rules: []` comes back from the cluster as
				// `rules: null`. Keep whichever the cluster reports.
				mergedMap[key] = liveValue
				continue
			}
			if !inMerged {
				// Only where neither manifest carries a value for the key: an
				// absent key and a `null` both read as nil here, which is what
				// lets an unset `replicas:` count as "the chart says nothing".
				// Agreeing on an actual value is not enough - two manifests that
				// both say `hostNetwork: false` are asking for false, and
				// copying a drifted `true` back would hide the correction the
				// upgrade makes.
				if originalMap[key] == nil && modifiedMap[key] == nil {
					mergedMap[key] = liveValue
				}
				continue
			}
			mergedMap[key] = restoreMissing(mergedValue, liveValue, originalMap[key], modifiedMap[key])
		}
		return mergedMap

	case []interface{}:
		mergedList, ok := merged.([]interface{})
		if !ok || len(mergedList) != len(live) {
			return merged
		}
		// Elements are paired by position, which is only sound as long as the
		// chart itself left the list alone. Once the old and the new manifest
		// disagree about it, the positions may mean different things and the
		// list is left as the patch produced it.
		originalList, _ := original.([]interface{})
		modifiedList, _ := modified.([]interface{})
		if len(originalList) != len(mergedList) || !reflect.DeepEqual(originalList, modifiedList) {
			return mergedList
		}

		for i := range mergedList {
			mergedList[i] = restoreMissing(mergedList[i], live[i], originalList[i], modifiedList[i])
		}
		return mergedList

	default:
		return merged
	}
}

// applyPatch computes the patched object, either by letting the API server
// dry-run the patch or by merging locally, depending on mode. warn is called at
// most once, when mode is ThreeWayMergeAuto and the API server refused the
// dry-run.
func applyPatch(helper *resource.Helper, info *resource.Info, patch *resourcePatch, liveData []byte, mode ThreeWayMergeMode, patchDenied func(*resource.Info, error) bool) ([]byte, error) {
	kind := info.Mapping.GroupVersionKind.Kind

	if mode != ThreeWayMergeClient {
		helper.ServerDryRun = true
		targetObj, err := helper.Patch(info.Namespace, info.Name, patch.patchType, patch.data, nil)
		switch {
		case err == nil:
			out, err := jsoniter.ConfigCompatibleWithStandardLibrary.Marshal(targetObj)
			if err != nil {
				return nil, fmt.Errorf("serializing patched %q with kind %s: %w", info.Name, kind, err)
			}
			return out, nil
		case mode == ThreeWayMergeAuto && isPatchNotAllowed(err):
			if !patchDenied(info, err) {
				return nil, fmt.Errorf("cannot patch %q with kind %s: %w", info.Name, kind, err)
			}
		default:
			return nil, fmt.Errorf("cannot patch %q with kind %s: %w", info.Name, kind, err)
		}
	}

	out, err := patch.apply(liveData)
	if err != nil {
		return nil, fmt.Errorf("cannot merge %q with kind %s: %w", info.Name, kind, err)
	}
	return out, nil
}

// isPatchNotAllowed reports whether the API server rejected the patch because
// the caller is not allowed to perform it, rather than because the patch itself
// is bad.
func isPatchNotAllowed(err error) bool {
	return apierrors.IsForbidden(err) || apierrors.IsMethodNotSupported(err)
}

func createPatch(originalObj, currentObj runtime.Object, target *resource.Info) (*resourcePatch, error) {
	oldData, err := json.Marshal(originalObj)
	if err != nil {
		return nil, fmt.Errorf("serializing original configuration: %w", err)
	}
	newData, err := json.Marshal(target.Object)
	if err != nil {
		return nil, fmt.Errorf("serializing target configuration: %w", err)
	}

	// Even if currentObj is nil (because it was not found), it will marshal just fine
	currentData, err := json.Marshal(currentObj)
	if err != nil {
		return nil, fmt.Errorf("serializing live configuration: %w", err)
	}

	// Get a versioned object
	versionedObject := kube.AsVersioned(target)

	// Unstructured objects, such as CRDs, may not return a "not registered"
	// error from ConvertToVersion. Anything that is unstructured should use
	// jsonpatch.CreateMergePatch, because a strategic merge patch is not
	// supported on objects like CRDs.
	_, isUnstructured := versionedObject.(runtime.Unstructured)

	// On newer Kubernetes versions CRDs are not unstructured but have this
	// dedicated type.
	_, isCRD := versionedObject.(*apiextv1.CustomResourceDefinition)

	patch := &resourcePatch{originalData: oldData, modifiedData: newData}
	if !isUnstructured {
		patch.versionedObject = versionedObject
	}

	if isUnstructured || isCRD {
		// fall back to generic JSON merge patch
		patch.data, err = jsonpatch.CreateMergePatch(oldData, newData)
		if err != nil {
			return nil, err
		}
		patch.patchType = types.MergePatchType
		return patch, nil
	}

	patchMeta, err := strategicpatch.NewPatchMetaFromStruct(versionedObject)
	if err != nil {
		return nil, fmt.Errorf("unable to create patch metadata from object: %w", err)
	}

	patch.data, err = strategicpatch.CreateThreeWayMergePatch(oldData, newData, currentData, patchMeta, true)
	if err != nil {
		return nil, err
	}
	patch.patchType = types.StrategicMergePatchType
	patch.patchMeta = patchMeta
	return patch, nil
}

func objectKey(r *resource.Info) string {
	gvk := r.Object.GetObjectKind().GroupVersionKind()
	return fmt.Sprintf("%s/%s/%s/%s", gvk.GroupVersion().String(), gvk.Kind, r.Namespace, r.Name)
}

func existingResourceConflict(resources kube.ResourceList) (kube.ResourceList, error) {
	var requireUpdate kube.ResourceList

	err := resources.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}

		helper := resource.NewHelper(info.Client, info.Mapping)
		_, err = helper.Get(info.Namespace, info.Name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("could not get information about the resource: %w", err)
		}

		requireUpdate.Append(info)
		return nil
	})

	return requireUpdate, err
}

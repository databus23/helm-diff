package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	jsoniterator "github.com/json-iterator/go"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/kube"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/yaml"

	"github.com/spf13/cobra"
	"k8s.io/helm/pkg/helm"

	"github.com/databus23/helm-diff/v3/diff"
	"github.com/databus23/helm-diff/v3/manifest"
)

type diffCmd struct {
	release                  string
	chart                    string
	chartVersion             string
	chartRepo                string
	client                   helm.Interface
	detailedExitCode         bool
	devel                    bool
	disableValidation        bool
	disableOpenAPIValidation bool
	dryRun                   bool
	namespace                string // namespace to assume the release to be installed into. Defaults to the current kube config namespace.
	valueFiles               valueFiles
	values                   []string
	stringValues             []string
	fileValues               []string
	reuseValues              bool
	resetValues              bool
	allowUnreleased          bool
	noHooks                  bool
	includeTests             bool
	postRenderer             string
	postRendererArgs         []string
	install                  bool
	normalizeManifests       bool
	threeWayMerge            bool
	extraAPIs                []string
	kubeVersion              string
	useUpgradeDryRun         bool
	diff.Options
}

func (d *diffCmd) isAllowUnreleased() bool {
	// helm update --install is effectively the same as helm-diff's --allow-unreleased option,
	// support both so that helm diff plugin can be applied on the same command
	// https://github.com/databus23/helm-diff/issues/108
	return d.allowUnreleased || d.install
}

const globalUsage = `Show a diff explaining what a helm upgrade would change.

This fetches the currently deployed version of a release
and compares it to a chart plus values.
This can be used visualize what changes a helm upgrade will
perform.
`

var envSettings = cli.New()
var yamlSeperator = []byte("\n---\n")

func newChartCommand() *cobra.Command {
	diff := diffCmd{
		namespace: os.Getenv("HELM_NAMESPACE"),
	}

	cmd := &cobra.Command{
		Use:   "upgrade [flags] [RELEASE] [CHART]",
		Short: "Show a diff explaining what a helm upgrade would change.",
		Long:  globalUsage,
		Example: strings.Join([]string{
			"  helm diff upgrade my-release stable/postgresql --values values.yaml",
			"",
			"  # Set HELM_DIFF_IGNORE_UNKNOWN_FLAGS=true to ignore unknown flags",
			"  # It's useful when you're using `helm-diff` in a `helm upgrade` wrapper.",
			"  # See https://github.com/databus23/helm-diff/issues/278 for more information.",
			"  HELM_DIFF_IGNORE_UNKNOWN_FLAGS=true helm diff upgrade my-release stable/postgres --wait",
			"",
			"  # Set HELM_DIFF_USE_UPGRADE_DRY_RUN=true to",
			"  # use `helm upgrade --dry-run` instead of `helm template` to render manifests from the chart.",
			"  # See https://github.com/databus23/helm-diff/issues/253 for more information.",
			"  HELM_DIFF_USE_UPGRADE_DRY_RUN=true helm diff upgrade my-release datadog/datadog",
			"",
			"  # Set HELM_DIFF_THREE_WAY_MERGE=true to",
			"  # enable the three-way-merge on diff.",
			"  # This is equivalent to specifying the --three-way-merge flag.",
			"  # Read the flag usage below for more information on --three-way-merge.",
			"  HELM_DIFF_THREE_WAY_MERGE=true helm diff upgrade my-release datadog/datadog",
			"",
			"  # Set HELM_DIFF_NORMALIZE_MANIFESTS=true to",
			"  # normalize the yaml file content when using helm diff.",
			"  # This is equivalent to specifying the --normalize-manifests flag.",
			"  # Read the flag usage below for more information on --normalize-manifests.",
			"  HELM_DIFF_NORMALIZE_MANIFESTS=true helm diff upgrade my-release datadog/datadog",
			"",
			"# Set HELM_DIFF_OUTPUT_CONTEXT=n to configure the output context to n lines.",
			"# This is equivalent to specifying the --context flag.",
			"# Read the flag usage below for more information on --context.",
			"HELM_DIFF_OUTPUT_CONTEXT=5 helm diff upgrade my-release datadog/datadog",
		}, "\n"),
		Args: func(cmd *cobra.Command, args []string) error {
			return checkArgsLength(len(args), "release name", "chart path")
		},
		PreRun: func(*cobra.Command, []string) {
			expandTLSPaths()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Suppress the command usage on error. See #77 for more info
			cmd.SilenceUsage = true

			// See https://github.com/databus23/helm-diff/issues/253
			diff.useUpgradeDryRun = os.Getenv("HELM_DIFF_USE_UPGRADE_DRY_RUN") == "true"

			if !diff.threeWayMerge && !cmd.Flags().Changed("three-way-merge") {
				enabled := os.Getenv("HELM_DIFF_THREE_WAY_MERGE") == "true"
				diff.threeWayMerge = enabled

				if enabled {
					fmt.Println("Enabled three way merge via the envvar")
				}
			}

			if !diff.normalizeManifests && !cmd.Flags().Changed("normalize-manifests") {
				enabled := os.Getenv("HELM_DIFF_NORMALIZE_MANIFESTS") == "true"
				diff.normalizeManifests = enabled

				if enabled {
					fmt.Println("Enabled normalize manifests via the envvar")
				}
			}

			if diff.OutputContext == -1 && !cmd.Flags().Changed("context") {
				contextEnvVar := os.Getenv("HELM_DIFF_OUTPUT_CONTEXT")
				if contextEnvVar != "" {
					context, err := strconv.Atoi(contextEnvVar)
					if err == nil {
						diff.OutputContext = context
					}
				}
			}

			ProcessDiffOptions(cmd.Flags(), &diff.Options)

			diff.release = args[0]
			diff.chart = args[1]
			if isHelm3() {
				return diff.runHelm3()
			}
			if diff.client == nil {
				diff.client = createHelmClient()
			}
			return diff.run()
		},
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: os.Getenv("HELM_DIFF_IGNORE_UNKNOWN_FLAGS") == "true",
		},
	}

	f := cmd.Flags()
	var kubeconfig string
	f.StringVar(&kubeconfig, "kubeconfig", "", "This flag is ignored, to allow passing of this top level flag to helm")
	f.BoolVar(&diff.threeWayMerge, "three-way-merge", false, "use three-way-merge to compute patch and generate diff output")
	// f.StringVar(&diff.kubeContext, "kube-context", "", "name of the kubeconfig context to use")
	f.StringVar(&diff.chartVersion, "version", "", "specify the exact chart version to use. If this is not specified, the latest version is used")
	f.StringVar(&diff.chartRepo, "repo", "", "specify the chart repository url to locate the requested chart")
	f.BoolVar(&diff.detailedExitCode, "detailed-exitcode", false, "return a non-zero exit code when there are changes")
	// See the below links for more context on when to use this flag
	// - https://github.com/helm/helm/blob/d9ffe37d371c9d06448c55c852c800051830e49a/cmd/helm/template.go#L184
	// - https://github.com/databus23/helm-diff/issues/318
	f.StringArrayVarP(&diff.extraAPIs, "api-versions", "a", []string{}, "Kubernetes api versions used for Capabilities.APIVersions")
	// Support for kube-version was re-enabled and ported from helm2 to helm3 on https://github.com/helm/helm/pull/9040
	f.StringVar(&diff.kubeVersion, "kube-version", "", "Kubernetes version used for Capabilities.KubeVersion")
	f.VarP(&diff.valueFiles, "values", "f", "specify values in a YAML file (can specify multiple)")
	f.StringArrayVar(&diff.values, "set", []string{}, "set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.StringArrayVar(&diff.stringValues, "set-string", []string{}, "set STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.StringArrayVar(&diff.fileValues, "set-file", []string{}, "set values from respective files specified via the command line (can specify multiple or separate values with commas: key1=path1,key2=path2)")
	f.BoolVar(&diff.reuseValues, "reuse-values", false, "reuse the last release's values and merge in any new values. If '--reset-values' is specified, this is ignored")
	f.BoolVar(&diff.resetValues, "reset-values", false, "reset the values to the ones built into the chart and merge in any new values")
	f.BoolVar(&diff.allowUnreleased, "allow-unreleased", false, "enables diffing of releases that are not yet deployed via Helm")
	f.BoolVar(&diff.install, "install", false, "enables diffing of releases that are not yet deployed via Helm (equivalent to --allow-unreleased, added to match \"helm upgrade --install\" command")
	f.BoolVar(&diff.noHooks, "no-hooks", false, "disable diffing of hooks")
	f.BoolVar(&diff.includeTests, "include-tests", false, "enable the diffing of the helm test hooks")
	f.BoolVar(&diff.devel, "devel", false, "use development versions, too. Equivalent to version '>0.0.0-0'. If --version is set, this is ignored.")
	f.BoolVar(&diff.disableValidation, "disable-validation", false, "disables rendered templates validation against the Kubernetes cluster you are currently pointing to. This is the same validation performed on an install")
	f.BoolVar(&diff.disableOpenAPIValidation, "disable-openapi-validation", false, "disables rendered templates validation against the Kubernetes OpenAPI Schema")
	f.BoolVar(&diff.dryRun, "dry-run", false, "disables cluster access and show diff as if it was install. Implies --install, --reset-values, and --disable-validation")
	f.StringVar(&diff.postRenderer, "post-renderer", "", "the path to an executable to be used for post rendering. If it exists in $PATH, the binary will be used, otherwise it will try to look for the executable at the given path")
	f.StringArrayVar(&diff.postRendererArgs, "post-renderer-args", []string{}, "an argument to the post-renderer (can specify multiple)")
	f.BoolVar(&diff.normalizeManifests, "normalize-manifests", false, "normalize manifests before running diff to exclude style differences from the output")

	AddDiffOptions(f, &diff.Options)

	if !isHelm3() {
		f.StringVar(&diff.namespace, "namespace", "default", "namespace to assume the release to be installed into")
	}

	if !isHelm3() {
		addCommonCmdOptions(f)
	}

	return cmd

}

func (d *diffCmd) runHelm3() error {

	if err := compatibleHelm3Version(); err != nil {
		return err
	}

	var releaseManifest []byte

	var err error

	if !d.dryRun {
		releaseManifest, err = getRelease(d.release, d.namespace)
	}

	var newInstall bool
	if err != nil && strings.Contains(err.Error(), "release: not found") {
		if d.isAllowUnreleased() {
			fmt.Printf("********************\n\n\tRelease was not present in Helm.  Diff will show entire contents as new.\n\n********************\n")
			newInstall = true
			err = nil
		} else {
			fmt.Printf("********************\n\n\tRelease was not present in Helm.  Include the `--allow-unreleased` to perform diff without exiting in error.\n\n********************\n")
			return err
		}
	}
	if err != nil {
		return fmt.Errorf("Failed to get release %s in namespace %s: %s", d.release, d.namespace, err)
	}

	installManifest, err := d.template(!newInstall)
	if err != nil {
		return fmt.Errorf("Failed to render chart: %s", err)
	}

	if d.threeWayMerge {
		actionConfig := new(action.Configuration)
		if err := actionConfig.Init(envSettings.RESTClientGetter(), envSettings.Namespace(), os.Getenv("HELM_DRIVER"), log.Printf); err != nil {
			log.Fatalf("%+v", err)
		}
		if err := actionConfig.KubeClient.IsReachable(); err != nil {
			return err
		}
		original, err := actionConfig.KubeClient.Build(bytes.NewBuffer(releaseManifest), false)
		if err != nil {
			return errors.Wrap(err, "unable to build kubernetes objects from original release manifest")
		}
		target, err := actionConfig.KubeClient.Build(bytes.NewBuffer(installManifest), false)
		if err != nil {
			return errors.Wrap(err, "unable to build kubernetes objects from new release manifest")
		}
		releaseManifest, installManifest, err = genManifest(original, target)
		if err != nil {
			return errors.Wrap(err, "unable to generate manifests")
		}
	}

	currentSpecs := make(map[string]*manifest.MappingResult)
	if !newInstall && !d.dryRun {
		if !d.noHooks && !d.threeWayMerge {
			hooks, err := getHooks(d.release, d.namespace)
			if err != nil {
				return err
			}
			releaseManifest = append(releaseManifest, hooks...)
		}
		if d.includeTests {
			currentSpecs = manifest.Parse(string(releaseManifest), d.namespace, d.normalizeManifests)
		} else {
			currentSpecs = manifest.Parse(string(releaseManifest), d.namespace, d.normalizeManifests, helm3TestHook, helm2TestSuccessHook)
		}
	}
	var newSpecs map[string]*manifest.MappingResult
	if d.includeTests {
		newSpecs = manifest.Parse(string(installManifest), d.namespace, d.normalizeManifests)
	} else {
		newSpecs = manifest.Parse(string(installManifest), d.namespace, d.normalizeManifests, helm3TestHook, helm2TestSuccessHook)
	}
	seenAnyChanges := diff.Manifests(currentSpecs, newSpecs, &d.Options, os.Stdout)

	if d.detailedExitCode && seenAnyChanges {
		return Error{
			error: errors.New("identified at least one change, exiting with non-zero exit code (detailed-exitcode parameter enabled)"),
			Code:  2,
		}
	}

	return nil
}

func genManifest(original, target kube.ResourceList) ([]byte, []byte, error) {
	var err error
	releaseManifest, installManifest := make([]byte, 0), make([]byte, 0)

	// to be deleted
	targetResources := make(map[string]bool)
	for _, r := range target {
		targetResources[objectKey(r)] = true
	}
	for _, r := range original {
		if !targetResources[objectKey(r)] {
			out, _ := yaml.Marshal(r.Object)
			releaseManifest = append(releaseManifest, yamlSeperator...)
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
		return nil, nil, errors.Wrap(err, "rendered manifests contain a resource that already exists. Unable to continue with update")
	}

	_ = toBeUpdated.Visit(func(r *resource.Info, err error) error {
		if err != nil {
			return err
		}
		original.Append(r)
		return nil
	})

	err = target.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}
		kind := info.Mapping.GroupVersionKind.Kind

		// Fetch the current object for the three way merge
		helper := resource.NewHelper(info.Client, info.Mapping)
		currentObj, err := helper.Get(info.Namespace, info.Name)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return errors.Wrap(err, "could not get information about the resource")
			}
			// to be created
			out, _ := yaml.Marshal(info.Object)
			installManifest = append(installManifest, yamlSeperator...)
			installManifest = append(installManifest, out...)
			return nil
		}
		// to be updated
		out, _ := jsoniterator.ConfigCompatibleWithStandardLibrary.Marshal(currentObj)
		pruneObj, err := deleteStatusAndTidyMetadata(out)
		if err != nil {
			return errors.Wrapf(err, "prune current obj %q with kind %s", info.Name, kind)
		}
		pruneOut, err := yaml.Marshal(pruneObj)
		if err != nil {
			return errors.Wrapf(err, "prune current out %q with kind %s", info.Name, kind)
		}
		releaseManifest = append(releaseManifest, yamlSeperator...)
		releaseManifest = append(releaseManifest, pruneOut...)

		originalInfo := original.Get(info)
		if originalInfo == nil {
			return fmt.Errorf("could not find %q", info.Name)
		}

		patch, patchType, err := createPatch(originalInfo.Object, currentObj, info)
		if err != nil {
			return err
		}

		helper.ServerDryRun = true
		targetObj, err := helper.Patch(info.Namespace, info.Name, patchType, patch, nil)
		if err != nil {
			return errors.Wrapf(err, "cannot patch %q with kind %s", info.Name, kind)
		}
		out, _ = jsoniterator.ConfigCompatibleWithStandardLibrary.Marshal(targetObj)
		pruneObj, err = deleteStatusAndTidyMetadata(out)
		if err != nil {
			return errors.Wrapf(err, "prune current obj %q with kind %s", info.Name, kind)
		}
		pruneOut, err = yaml.Marshal(pruneObj)
		if err != nil {
			return errors.Wrapf(err, "prune current out %q with kind %s", info.Name, kind)
		}
		installManifest = append(installManifest, yamlSeperator...)
		installManifest = append(installManifest, pruneOut...)
		return nil
	})

	return releaseManifest, installManifest, err
}

func (d *diffCmd) run() error {
	if d.chartVersion == "" && d.devel {
		d.chartVersion = ">0.0.0-0"
	}

	chartPath, err := locateChartPath(d.chart, d.chartVersion, false, "")
	if err != nil {
		return err
	}

	if err := d.valueFiles.Valid(); err != nil {
		return err
	}

	rawVals, err := d.vals()
	if err != nil {
		return err
	}

	releaseResponse, err := d.client.ReleaseContent(d.release)

	var newInstall bool
	if err != nil && strings.Contains(err.Error(), fmt.Sprintf("release: %q not found", d.release)) {
		if d.isAllowUnreleased() {
			fmt.Printf("********************\n\n\tRelease was not present in Helm.  Diff will show entire contents as new.\n\n********************\n")
			newInstall = true
			err = nil
		} else {
			fmt.Printf("********************\n\n\tRelease was not present in Helm.  Include the `--allow-unreleased` to perform diff without exiting in error.\n\n********************\n")
		}

	}

	if err != nil {
		return prettyError(err)
	}

	var currentSpecs, newSpecs map[string]*manifest.MappingResult
	if newInstall {
		installResponse, err := d.client.InstallRelease(
			chartPath,
			d.namespace,
			helm.ReleaseName(d.release),
			helm.ValueOverrides(rawVals),
			helm.InstallDryRun(true),
		)
		if err != nil {
			return prettyError(err)
		}

		currentSpecs = make(map[string]*manifest.MappingResult)
		newSpecs = manifest.Parse(installResponse.Release.Manifest, installResponse.Release.Namespace, d.normalizeManifests)
	} else {
		upgradeResponse, err := d.client.UpdateRelease(
			d.release,
			chartPath,
			helm.UpdateValueOverrides(rawVals),
			helm.ReuseValues(d.reuseValues),
			helm.ResetValues(d.resetValues),
			helm.UpgradeDryRun(true),
		)
		if err != nil {
			return prettyError(err)
		}

		if d.noHooks {
			currentSpecs = manifest.Parse(releaseResponse.Release.Manifest, releaseResponse.Release.Namespace, d.normalizeManifests)
			newSpecs = manifest.Parse(upgradeResponse.Release.Manifest, upgradeResponse.Release.Namespace, d.normalizeManifests)
		} else {
			currentSpecs = manifest.ParseRelease(releaseResponse.Release, d.includeTests, d.normalizeManifests)
			newSpecs = manifest.ParseRelease(upgradeResponse.Release, d.includeTests, d.normalizeManifests)
		}
	}

	seenAnyChanges := diff.Manifests(currentSpecs, newSpecs, &d.Options, os.Stdout)

	if d.detailedExitCode && seenAnyChanges {
		return Error{
			error: errors.New("identified at least one change, exiting with non-zero exit code (detailed-exitcode parameter enabled)"),
			Code:  2,
		}
	}

	return nil
}

func createPatch(originalObj, currentObj runtime.Object, target *resource.Info) ([]byte, types.PatchType, error) {
	oldData, err := json.Marshal(originalObj)
	if err != nil {
		return nil, types.StrategicMergePatchType, errors.Wrap(err, "serializing current configuration")
	}
	newData, err := json.Marshal(target.Object)
	if err != nil {
		return nil, types.StrategicMergePatchType, errors.Wrap(err, "serializing target configuration")
	}

	// Even if currentObj is nil (because it was not found), it will marshal just fine
	currentData, err := json.Marshal(currentObj)
	if err != nil {
		return nil, types.StrategicMergePatchType, errors.Wrap(err, "serializing live configuration")
	}
	// kind := target.Mapping.GroupVersionKind.Kind
	// if kind == "Deployment" {
	// 	curr, _ := yaml.Marshal(currentObj)
	// 	fmt.Println(string(curr))
	// }

	// Get a versioned object
	versionedObject := kube.AsVersioned(target)

	// Unstructured objects, such as CRDs, may not have an not registered error
	// returned from ConvertToVersion. Anything that's unstructured should
	// use the jsonpatch.CreateMergePatch. Strategic Merge Patch is not supported
	// on objects like CRDs.
	_, isUnstructured := versionedObject.(runtime.Unstructured)

	// On newer K8s versions, CRDs aren't unstructured but has this dedicated type
	_, isCRD := versionedObject.(*apiextv1.CustomResourceDefinition)

	if isUnstructured || isCRD {
		// fall back to generic JSON merge patch
		patch, err := jsonpatch.CreateMergePatch(oldData, newData)
		return patch, types.MergePatchType, err
	}

	patchMeta, err := strategicpatch.NewPatchMetaFromStruct(versionedObject)
	if err != nil {
		return nil, types.StrategicMergePatchType, errors.Wrap(err, "unable to create patch metadata from object")
	}

	patch, err := strategicpatch.CreateThreeWayMergePatch(oldData, newData, currentData, patchMeta, true)
	return patch, types.StrategicMergePatchType, err
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
			return errors.Wrap(err, "could not get information about the resource")
		}

		requireUpdate.Append(info)
		return nil
	})

	return requireUpdate, err
}

func deleteStatusAndTidyMetadata(obj []byte) (map[string]interface{}, error) {
	var objectMap map[string]interface{}
	err := jsoniterator.Unmarshal(obj, &objectMap)
	if err != nil {
		return nil, errors.Wrap(err, "could not unmarshal byte sequence")
	}

	delete(objectMap, "status")

	metadata := objectMap["metadata"].(map[string]interface{})

	delete(metadata, "managedFields")
	delete(metadata, "generation")

	// See the below for the goal of this metadata tidy logic.
	// https://github.com/databus23/helm-diff/issues/326#issuecomment-1008253274
	if a := metadata["annotations"]; a != nil {
		annotations := a.(map[string]interface{})
		delete(annotations, "meta.helm.sh/release-name")
		delete(annotations, "meta.helm.sh/release-namespace")
		delete(annotations, "deployment.kubernetes.io/revision")

		if len(annotations) == 0 {
			delete(metadata, "annotations")
		}
	}

	return objectMap, nil
}

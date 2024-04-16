package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"
	jsoniterator "github.com/json-iterator/go"
	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/kube"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/yaml"

	"github.com/databus23/helm-diff/v3/diff"
	"github.com/databus23/helm-diff/v3/manifest"
)

var (
	validDryRunValues = []string{"server", "client", "true", "false"}
)

const (
	dryRunNoOptDefVal = "client"
)

type diffCmd struct {
	release                  string
	chart                    string
	chartVersion             string
	chartRepo                string
	detailedExitCode         bool
	devel                    bool
	disableValidation        bool
	disableOpenAPIValidation bool
	enableDNS                bool
	namespace                string // namespace to assume the release to be installed into. Defaults to the current kube config namespace.
	valueFiles               valueFiles
	values                   []string
	stringValues             []string
	stringLiteralValues      []string
	jsonValues               []string
	fileValues               []string
	reuseValues              bool
	resetValues              bool
	allowUnreleased          bool
	noHooks                  bool
	includeTests             bool
	postRenderer             string
	postRendererArgs         []string
	insecureSkipTLSVerify    bool
	install                  bool
	normalizeManifests       bool
	threeWayMerge            bool
	extraAPIs                []string
	kubeVersion              string
	useUpgradeDryRun         bool
	diff.Options

	// dryRunMode can take the following values:
	// - "none": no dry run is performed
	// - "client": dry run is performed without remote cluster access
	// - "server": dry run is performed with remote cluster access
	// - "true": same as "client"
	// - "false": same as "none"
	dryRunMode string
}

func (d *diffCmd) isAllowUnreleased() bool {
	// helm update --install is effectively the same as helm-diff's --allow-unreleased option,
	// support both so that helm diff plugin can be applied on the same command
	// https://github.com/databus23/helm-diff/issues/108
	return d.allowUnreleased || d.install
}

// clusterAccessAllowed returns true if the diff command is allowed to access the cluster at some degree.
//
// helm-diff basically have 2 modes of operation:
// 1. without cluster access at all when --dry-run=true or --dry-run=client is specified.
// 2. with cluster access when --dry-run is unspecified, false, or server.
//
// clusterAccessAllowed returns true when the mode is either 2 or 3.
//
// If false, helm-diff should not access the cluster at all.
// More concretely:
// - It shouldn't pass --validate to helm-template because it requires cluster access.
// - It shouldn't get the current release manifest using helm-get-manifest because it requires cluster access.
// - It shouldn't get the current release hooks using helm-get-hooks because it requires cluster access.
// - It shouldn't get the current release values using helm-get-values because it requires cluster access.
//
// See also https://github.com/helm/helm/pull/9426#discussion_r1181397259
func (d *diffCmd) clusterAccessAllowed() bool {
	return d.dryRunMode == "none" || d.dryRunMode == "false" || d.dryRunMode == "server"
}

const globalUsage = `Show a diff explaining what a helm upgrade would change.

This fetches the currently deployed version of a release
and compares it to a chart plus values.
This can be used to visualize what changes a helm upgrade will
perform.
`

var envSettings = cli.New()
var yamlSeperator = []byte("\n---\n")

func newChartCommand() *cobra.Command {
	diff := diffCmd{
		namespace: os.Getenv("HELM_NAMESPACE"),
	}
	unknownFlags := os.Getenv("HELM_DIFF_IGNORE_UNKNOWN_FLAGS") == "true"

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
			"  # helm-diff disallows the use of the `lookup` function by default.",
			"  # To enable it, you must set HELM_DIFF_USE_INSECURE_SERVER_SIDE_DRY_RUN=true to",
			"  # use `helm template --dry-run=server` or",
			"  # `helm upgrade --dry-run=server` (in case you also set `HELM_DIFF_USE_UPGRADE_DRY_RUN`)",
			"  # See https://github.com/databus23/helm-diff/pull/458",
			"  # for more information.",
			"  HELM_DIFF_USE_INSECURE_SERVER_SIDE_DRY_RUN=true helm diff upgrade my-release datadog/datadog",
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
		RunE: func(cmd *cobra.Command, args []string) error {
			if diff.dryRunMode == "" {
				diff.dryRunMode = "none"
			} else if !slices.Contains(validDryRunValues, diff.dryRunMode) {
				return fmt.Errorf("flag %q must take a bool value or either %q or %q, but got %q", "dry-run", "client", "server", diff.dryRunMode)
			}

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
			return diff.runHelm3()
		},
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: unknownFlags,
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
	f.StringArrayVar(&diff.stringLiteralValues, "set-literal", []string{}, "set STRING literal values on the command line")
	f.StringArrayVar(&diff.jsonValues, "set-json", []string{}, "set JSON values on the command line (can specify multiple or separate values with commas: key1=jsonval1,key2=jsonval2)")
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
	f.StringVar(&diff.dryRunMode, "dry-run", "", "--dry-run, --dry-run=client, or --dry-run=true disables cluster access and show diff as if it was install. Implies --install, --reset-values, and --disable-validation."+
		" --dry-run=server enables the cluster access with helm-get and the lookup template function.")
	f.Lookup("dry-run").NoOptDefVal = dryRunNoOptDefVal
	f.BoolVar(&diff.enableDNS, "enable-dns", false, "enable DNS lookups when rendering templates")
	f.StringVar(&diff.postRenderer, "post-renderer", "", "the path to an executable to be used for post rendering. If it exists in $PATH, the binary will be used, otherwise it will try to look for the executable at the given path")
	f.StringArrayVar(&diff.postRendererArgs, "post-renderer-args", []string{}, "an argument to the post-renderer (can specify multiple)")
	f.BoolVar(&diff.insecureSkipTLSVerify, "insecure-skip-tls-verify", false, "skip tls certificate checks for the chart download")
	f.BoolVar(&diff.normalizeManifests, "normalize-manifests", false, "normalize manifests before running diff to exclude style differences from the output")

	AddDiffOptions(f, &diff.Options)

	return cmd
}

func (d *diffCmd) runHelm3() error {
	if err := compatibleHelm3Version(); err != nil {
		return err
	}

	var releaseManifest []byte

	var err error

	if d.clusterAccessAllowed() {
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
			return fmt.Errorf("unable to build kubernetes objects from original release manifest: %w", err)
		}
		target, err := actionConfig.KubeClient.Build(bytes.NewBuffer(installManifest), false)
		if err != nil {
			return fmt.Errorf("unable to build kubernetes objects from new release manifest: %w", err)
		}
		releaseManifest, installManifest, err = genManifest(original, target)
		if err != nil {
			return fmt.Errorf("unable to generate manifests: %w", err)
		}
	}

	currentSpecs := make(map[string]*manifest.MappingResult)
	if !newInstall && d.clusterAccessAllowed() {
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
		return nil, nil, fmt.Errorf("rendered manifests contain a resource that already exists. Unable to continue with update: %w", err)
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
				return fmt.Errorf("could not get information about the resource: %w", err)
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
			return fmt.Errorf("prune current obj %q with kind %s: %w", info.Name, kind, err)
		}
		pruneOut, err := yaml.Marshal(pruneObj)
		if err != nil {
			return fmt.Errorf("prune current out %q with kind %s: %w", info.Name, kind, err)
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
			return fmt.Errorf("cannot patch %q with kind %s: %w", info.Name, kind, err)
		}
		out, _ = jsoniterator.ConfigCompatibleWithStandardLibrary.Marshal(targetObj)
		pruneObj, err = deleteStatusAndTidyMetadata(out)
		if err != nil {
			return fmt.Errorf("prune current obj %q with kind %s: %w", info.Name, kind, err)
		}
		pruneOut, err = yaml.Marshal(pruneObj)
		if err != nil {
			return fmt.Errorf("prune current out %q with kind %s: %w", info.Name, kind, err)
		}
		installManifest = append(installManifest, yamlSeperator...)
		installManifest = append(installManifest, pruneOut...)
		return nil
	})

	return releaseManifest, installManifest, err
}

func createPatch(originalObj, currentObj runtime.Object, target *resource.Info) ([]byte, types.PatchType, error) {
	oldData, err := json.Marshal(originalObj)
	if err != nil {
		return nil, types.StrategicMergePatchType, fmt.Errorf("serializing current configuration: %w", err)
	}
	newData, err := json.Marshal(target.Object)
	if err != nil {
		return nil, types.StrategicMergePatchType, fmt.Errorf("serializing target configuration: %w", err)
	}

	// Even if currentObj is nil (because it was not found), it will marshal just fine
	currentData, err := json.Marshal(currentObj)
	if err != nil {
		return nil, types.StrategicMergePatchType, fmt.Errorf("serializing live configuration: %w", err)
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
		return nil, types.StrategicMergePatchType, fmt.Errorf("unable to create patch metadata from object: %w", err)
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
			return fmt.Errorf("could not get information about the resource: %w", err)
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
		return nil, fmt.Errorf("could not unmarshal byte sequence: %w", err)
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

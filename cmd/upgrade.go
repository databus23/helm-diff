package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/helm/pkg/helm"

	"github.com/databus23/helm-diff/v3/diff"
	"github.com/databus23/helm-diff/v3/manifest"
)

type diffCmd struct {
	release                  string
	chart                    string
	chartVersion             string
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
	suppressedKinds          []string
	outputContext            int
	showSecrets              bool
	postRenderer             string
	output                   string
	install                  bool
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

func newChartCommand() *cobra.Command {
	diff := diffCmd{
		namespace: os.Getenv("HELM_NAMESPACE"),
	}

	cmd := &cobra.Command{
		Use:     "upgrade [flags] [RELEASE] [CHART]",
		Short:   "Show a diff explaining what a helm upgrade would change.",
		Long:    globalUsage,
		Example: "  helm diff upgrade my-release stable/postgresql --values values.yaml",
		Args: func(cmd *cobra.Command, args []string) error {
			return checkArgsLength(len(args), "release name", "chart path")
		},
		PreRun: func(*cobra.Command, []string) {
			expandTLSPaths()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Suppress the command usage on error. See #77 for more info
			cmd.SilenceUsage = true

			if q, _ := cmd.Flags().GetBool("suppress-secrets"); q {
				diff.suppressedKinds = append(diff.suppressedKinds, "Secret")
			}

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
			UnknownFlags: true,
		},
	}

	f := cmd.Flags()
	var kubeconfig string
	f.StringVar(&kubeconfig, "kubeconfig", "", "This flag is ignored, to allow passing of this top level flag to helm")
	f.StringVar(&diff.chartVersion, "version", "", "specify the exact chart version to use. If this is not specified, the latest version is used")
	f.BoolVar(&diff.detailedExitCode, "detailed-exitcode", false, "return a non-zero exit code when there are changes")
	f.BoolP("suppress-secrets", "q", false, "suppress secrets in the output")
	f.BoolVar(&diff.showSecrets, "show-secrets", false, "do not redact secret values in the output")
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
	f.StringArrayVar(&diff.suppressedKinds, "suppress", []string{}, "allows suppression of the values listed in the diff output")
	f.IntVarP(&diff.outputContext, "context", "C", -1, "output NUM lines of context around changes")
	f.BoolVar(&diff.disableValidation, "disable-validation", false, "disables rendered templates validation against the Kubernetes cluster you are currently pointing to. This is the same validation performed on an install")
	f.BoolVar(&diff.disableOpenAPIValidation, "disable-openapi-validation", false, "disables rendered templates validation against the Kubernetes OpenAPI Schema")
	f.BoolVar(&diff.dryRun, "dry-run", false, "disables cluster access and show diff as if it was install. Implies --install, --reset-values, and --disable-validation")
	f.StringVar(&diff.postRenderer, "post-renderer", "", "the path to an executable to be used for post rendering. If it exists in $PATH, the binary will be used, otherwise it will try to look for the executable at the given path")
	f.StringVar(&diff.output, "output", "diff", "Possible values: diff, simple, json, template. When set to \"template\", use the env var HELM_DIFF_TPL to specify the template.")
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

	currentSpecs := make(map[string]*manifest.MappingResult)
	if !newInstall && !d.dryRun {
		if !d.noHooks {
			hooks, err := getHooks(d.release, d.namespace)
			if err != nil {
				return err
			}
			releaseManifest = append(releaseManifest, hooks...)
		}
		if d.includeTests {
			currentSpecs = manifest.Parse(string(releaseManifest), d.namespace)
		} else {
			currentSpecs = manifest.Parse(string(releaseManifest), d.namespace, helm3TestHook, helm2TestSuccessHook)
		}
	}
	var newSpecs map[string]*manifest.MappingResult
	if d.includeTests {
		newSpecs = manifest.Parse(string(installManifest), d.namespace)
	} else {
		newSpecs = manifest.Parse(string(installManifest), d.namespace, helm3TestHook, helm2TestSuccessHook)
	}
	seenAnyChanges := diff.Manifests(currentSpecs, newSpecs, d.suppressedKinds, d.showSecrets, d.outputContext, d.output, os.Stdout)

	if d.detailedExitCode && seenAnyChanges {
		return Error{
			error: errors.New("identified at least one change, exiting with non-zero exit code (detailed-exitcode parameter enabled)"),
			Code:  2,
		}
	}

	return nil
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
		newSpecs = manifest.Parse(installResponse.Release.Manifest, installResponse.Release.Namespace)
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
			currentSpecs = manifest.Parse(releaseResponse.Release.Manifest, releaseResponse.Release.Namespace)
			newSpecs = manifest.Parse(upgradeResponse.Release.Manifest, upgradeResponse.Release.Namespace)
		} else {
			currentSpecs = manifest.ParseRelease(releaseResponse.Release, d.includeTests)
			newSpecs = manifest.ParseRelease(upgradeResponse.Release, d.includeTests)
		}
	}

	seenAnyChanges := diff.Manifests(currentSpecs, newSpecs, d.suppressedKinds, d.showSecrets, d.outputContext, d.output, os.Stdout)

	if d.detailedExitCode && seenAnyChanges {
		return Error{
			error: errors.New("identified at least one change, exiting with non-zero exit code (detailed-exitcode parameter enabled)"),
			Code:  2,
		}
	}

	return nil
}

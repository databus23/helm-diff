package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/databus23/helm-diff/v3/diff"
	"github.com/databus23/helm-diff/v3/manifest"
)

type local struct {
	chart1             string
	chart2             string
	release            string
	namespace          string
	detailedExitCode   bool
	includeTests       bool
	includeCRDs        bool
	normalizeManifests bool
	enableDNS          bool
	valueFiles         valueFiles
	values             []string
	stringValues       []string
	stringLiteralValues []string
	jsonValues         []string
	fileValues         []string
	postRenderer       string
	postRendererArgs   []string
	extraAPIs          []string
	kubeVersion        string
	diff.Options
}

const localCmdLongUsage = `
This command compares the manifests of two local chart directories.

It renders both charts using 'helm template' and shows the differences
between the resulting manifests.

This is useful for:
 - Comparing different versions of a chart
 - Previewing changes before committing
 - Validating chart modifications
`

func localCmd() *cobra.Command {
	diff := local{
		release: "release",
	}

	localCmd := &cobra.Command{
		Use:   "local [flags] CHART1 CHART2",
		Short: "Shows diff between two local chart directories",
		Long:  localCmdLongUsage,
		Example: strings.Join([]string{
			"  helm diff local ./chart-v1 ./chart-v2",
			"  helm diff local ./chart-v1 ./chart-v2 -f values.yaml",
			"  helm diff local /path/to/chart-a /path/to/chart-b --set replicas=3",
		}, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Suppress the command usage on error. See #77 for more info
			cmd.SilenceUsage = true

			if v, _ := cmd.Flags().GetBool("version"); v {
				fmt.Println(Version)
				return nil
			}

			if err := checkArgsLength(len(args), "chart1 path", "chart2 path"); err != nil {
				return err
			}

			ProcessDiffOptions(cmd.Flags(), &diff.Options)

			diff.chart1 = args[0]
			diff.chart2 = args[1]

			if diff.namespace == "" {
				diff.namespace = os.Getenv("HELM_NAMESPACE")
			}

			return diff.run()
		},
	}

	localCmd.Flags().StringVar(&diff.release, "release", "release", "release name to use for template rendering")
	localCmd.Flags().StringVar(&diff.namespace, "namespace", "", "namespace to use for template rendering")
	localCmd.Flags().BoolVar(&diff.detailedExitCode, "detailed-exitcode", false, "return a non-zero exit code when there are changes")
	localCmd.Flags().BoolVar(&diff.includeTests, "include-tests", false, "enable the diffing of the helm test hooks")
	localCmd.Flags().BoolVar(&diff.includeCRDs, "include-crds", false, "include CRDs in the diffing")
	localCmd.Flags().BoolVar(&diff.normalizeManifests, "normalize-manifests", false, "normalize manifests before running diff to exclude style differences from the output")
	localCmd.Flags().BoolVar(&diff.enableDNS, "enable-dns", false, "enable DNS lookups when rendering templates")
	localCmd.Flags().VarP(&diff.valueFiles, "values", "f", "specify values in a YAML file (can specify multiple)")
	localCmd.Flags().StringArrayVar(&diff.values, "set", []string{}, "set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	localCmd.Flags().StringArrayVar(&diff.stringValues, "set-string", []string{}, "set STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	localCmd.Flags().StringArrayVar(&diff.stringLiteralValues, "set-literal", []string{}, "set STRING literal values on the command line")
	localCmd.Flags().StringArrayVar(&diff.jsonValues, "set-json", []string{}, "set JSON values on the command line (can specify multiple or separate values with commas: key1=jsonval1,key2=jsonval2)")
	localCmd.Flags().StringArrayVar(&diff.fileValues, "set-file", []string{}, "set values from respective files specified via the command line (can specify multiple or separate values with commas: key1=path1,key2=path2)")
	localCmd.Flags().StringVar(&diff.postRenderer, "post-renderer", "", "the path to an executable to be used for post rendering. If it exists in $PATH, the binary will be used, otherwise it will try to look for the executable at the given path")
	localCmd.Flags().StringArrayVar(&diff.postRendererArgs, "post-renderer-args", []string{}, "an argument to the post-renderer (can specify multiple)")
	localCmd.Flags().StringArrayVarP(&diff.extraAPIs, "api-versions", "a", []string{}, "Kubernetes api versions used for Capabilities.APIVersions")
	localCmd.Flags().StringVar(&diff.kubeVersion, "kube-version", "", "Kubernetes version used for Capabilities.KubeVersion")

	AddDiffOptions(localCmd.Flags(), &diff.Options)

	localCmd.SuggestionsMinimumDistance = 1

	return localCmd
}

func (l *local) run() error {
	manifest1, err := l.renderChart(l.chart1)
	if err != nil {
		return fmt.Errorf("Failed to render chart %s: %w", l.chart1, err)
	}

	manifest2, err := l.renderChart(l.chart2)
	if err != nil {
		return fmt.Errorf("Failed to render chart %s: %w", l.chart2, err)
	}

	excludes := []string{manifest.Helm3TestHook, manifest.Helm2TestSuccessHook}
	if l.includeTests {
		excludes = []string{}
	}

	specs1 := manifest.Parse(string(manifest1), l.namespace, l.normalizeManifests, excludes...)
	specs2 := manifest.Parse(string(manifest2), l.namespace, l.normalizeManifests, excludes...)

	seenAnyChanges := diff.Manifests(specs1, specs2, &l.Options, os.Stdout)

	if l.detailedExitCode && seenAnyChanges {
		return Error{
			error: errors.New("identified at least one change, exiting with non-zero exit code (detailed-exitcode parameter enabled)"),
			Code:  2,
		}
	}

	return nil
}

func (l *local) renderChart(chartPath string) ([]byte, error) {
	flags := []string{}

	if l.includeCRDs {
		flags = append(flags, "--include-crds")
	}

	if l.namespace != "" {
		flags = append(flags, "--namespace", l.namespace)
	}

	if l.postRenderer != "" {
		flags = append(flags, "--post-renderer", l.postRenderer)
	}

	for _, arg := range l.postRendererArgs {
		flags = append(flags, "--post-renderer-args", arg)
	}

	for _, valueFile := range l.valueFiles {
		if strings.TrimSpace(valueFile) == "-" {
			bytes, err := io.ReadAll(os.Stdin)
			if err != nil {
				return nil, err
			}

			tmpfile, err := os.CreateTemp("", "helm-diff-stdin-values")
			if err != nil {
				return nil, err
			}
			defer func() {
				_ = os.Remove(tmpfile.Name())
			}()

			if _, err := tmpfile.Write(bytes); err != nil {
				_ = tmpfile.Close()
				return nil, err
			}

			if err := tmpfile.Close(); err != nil {
				return nil, err
			}

			flags = append(flags, "--values", tmpfile.Name())
		} else {
			flags = append(flags, "--values", valueFile)
		}
	}

	for _, value := range l.values {
		flags = append(flags, "--set", value)
	}

	for _, stringValue := range l.stringValues {
		flags = append(flags, "--set-string", stringValue)
	}

	for _, stringLiteralValue := range l.stringLiteralValues {
		flags = append(flags, "--set-literal", stringLiteralValue)
	}

	for _, jsonValue := range l.jsonValues {
		flags = append(flags, "--set-json", jsonValue)
	}

	for _, fileValue := range l.fileValues {
		flags = append(flags, "--set-file", fileValue)
	}

	if l.enableDNS {
		flags = append(flags, "--enable-dns")
	}

	for _, a := range l.extraAPIs {
		flags = append(flags, "--api-versions", a)
	}

	if l.kubeVersion != "" {
		flags = append(flags, "--kube-version", l.kubeVersion)
	}

	args := []string{"template", l.release, chartPath}
	args = append(args, flags...)

	cmd := exec.Command(os.Getenv("HELM_BIN"), args...)
	return outputWithRichError(cmd)
}

package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/helm/pkg/helm"

	"github.com/databus23/helm-diff/v3/diff"
	"github.com/databus23/helm-diff/v3/manifest"
)

type release struct {
	client           helm.Interface
	detailedExitCode bool
	suppressedKinds  []string
	releases         []string
	outputContext    int
	includeTests     bool
	showSecrets      bool
	output           string
	stripTrailingCR  bool
}

const releaseCmdLongUsage = `
This command compares the manifests details of a different releases created from the same chart

It can be used to compare the manifests of

 - release1 with release2
	$ helm diff release [flags] release1 release2
   Example:
	$ helm diff release my-prod my-stage
`

func releaseCmd() *cobra.Command {
	diff := release{}
	releaseCmd := &cobra.Command{
		Use:   "release [flags] RELEASE release1 [release2]",
		Short: "Shows diff between release's manifests",
		Long:  releaseCmdLongUsage,
		PreRun: func(*cobra.Command, []string) {
			expandTLSPaths()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Suppress the command usage on error. See #77 for more info
			cmd.SilenceUsage = true

			if v, _ := cmd.Flags().GetBool("version"); v {
				fmt.Println(Version)
				return nil
			}

			switch {
			case len(args) < 2:
				return errors.New("Too few arguments to Command \"release\".\nMinimum 2 arguments required: release name-1, release name-2")
			}

			if q, _ := cmd.Flags().GetBool("suppress-secrets"); q {
				diff.suppressedKinds = append(diff.suppressedKinds, "Secret")
			}

			diff.releases = args[0:]
			if isHelm3() {
				return diff.differentiateHelm3()
			}
			if diff.client == nil {
				diff.client = createHelmClient()
			}
			return diff.differentiate()
		},
	}

	releaseCmd.Flags().BoolP("suppress-secrets", "q", false, "suppress secrets in the output")
	releaseCmd.Flags().BoolVar(&diff.showSecrets, "show-secrets", false, "do not redact secret values in the output")
	releaseCmd.Flags().BoolVar(&diff.detailedExitCode, "detailed-exitcode", false, "return a non-zero exit code when there are changes")
	releaseCmd.Flags().StringArrayVar(&diff.suppressedKinds, "suppress", []string{}, "allows suppression of the values listed in the diff output")
	releaseCmd.Flags().IntVarP(&diff.outputContext, "context", "C", -1, "output NUM lines of context around changes")
	releaseCmd.Flags().BoolVar(&diff.includeTests, "include-tests", false, "enable the diffing of the helm test hooks")
	releaseCmd.Flags().StringVar(&diff.output, "output", "diff", "Possible values: diff, simple, template. When set to \"template\", use the env var HELM_DIFF_TPL to specify the template.")
	releaseCmd.Flags().BoolVar(&diff.stripTrailingCR, "strip-trailing-cr", false, "strip trailing carriage return on input")

	releaseCmd.SuggestionsMinimumDistance = 1

	if !isHelm3() {
		addCommonCmdOptions(releaseCmd.Flags())
	}

	return releaseCmd
}

func (d *release) differentiateHelm3() error {
	namespace := os.Getenv("HELM_NAMESPACE")
	excludes := []string{helm3TestHook, helm2TestSuccessHook}
	if d.includeTests {
		excludes = []string{}
	}
	releaseResponse1, err := getRelease(d.releases[0], namespace)
	if err != nil {
		return err
	}
	releaseChart1, err := getChart(d.releases[0], namespace)
	if err != nil {
		return err
	}

	releaseResponse2, err := getRelease(d.releases[1], namespace)
	if err != nil {
		return err
	}
	releaseChart2, err := getChart(d.releases[1], namespace)
	if err != nil {
		return err
	}

	if releaseChart1 == releaseChart2 {
		seenAnyChanges := diff.Releases(
			manifest.Parse(string(releaseResponse1), namespace, excludes...),
			manifest.Parse(string(releaseResponse2), namespace, excludes...),
			d.suppressedKinds,
			d.showSecrets,
			d.outputContext,
			d.output,
			d.stripTrailingCR,
			os.Stdout)

		if d.detailedExitCode && seenAnyChanges {
			return Error{
				error: errors.New("identified at least one change, exiting with non-zero exit code (detailed-exitcode parameter enabled)"),
				Code:  2,
			}
		}
	} else {
		fmt.Printf("Error : Incomparable Releases \n Unable to compare releases from two different charts \"%s\", \"%s\". \n try helm diff release --help to know more \n", releaseChart1, releaseChart2)
	}
	return nil
}

func (d *release) differentiate() error {

	releaseResponse1, err := d.client.ReleaseContent(d.releases[0])
	if err != nil {
		return prettyError(err)
	}

	releaseResponse2, err := d.client.ReleaseContent(d.releases[1])
	if err != nil {
		return prettyError(err)
	}

	if releaseResponse1.Release.Chart.Metadata.Name == releaseResponse2.Release.Chart.Metadata.Name {
		seenAnyChanges := diff.Releases(
			manifest.ParseRelease(releaseResponse1.Release, d.includeTests),
			manifest.ParseRelease(releaseResponse2.Release, d.includeTests),
			d.suppressedKinds,
			d.showSecrets,
			d.outputContext,
			d.output,
			d.stripTrailingCR,
			os.Stdout)

		if d.detailedExitCode && seenAnyChanges {
			return Error{
				error: errors.New("identified at least one change, exiting with non-zero exit code (detailed-exitcode parameter enabled)"),
				Code:  2,
			}
		}
	} else {
		fmt.Printf("Error : Incomparable Releases \n Unable to compare releases from two different charts \"%s\", \"%s\". \n try helm diff release --help to know more \n", releaseResponse1.Release.Chart.Metadata.Name, releaseResponse2.Release.Chart.Metadata.Name)
	}
	return nil
}

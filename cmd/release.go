package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/databus23/helm-diff/diff"
	"github.com/databus23/helm-diff/manifest"
)

type release struct {
	clientHolder
	detailedExitCode bool
	suppressedKinds  []string
	releases         []string
	outputContext    int
	includeTests     bool
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
			diff.init()
			return diff.differentiate()
		},
	}

	releaseCmd.Flags().BoolP("suppress-secrets", "q", false, "suppress secrets in the output")
	releaseCmd.Flags().StringArrayVar(&diff.suppressedKinds, "suppress", []string{}, "allows suppression of the values listed in the diff output")
	releaseCmd.Flags().IntVarP(&diff.outputContext, "context", "C", -1, "output NUM lines of context around changes")
	releaseCmd.Flags().BoolVar(&diff.includeTests, "include-tests", false, "enable the diffing of the helm test hooks")
	releaseCmd.SuggestionsMinimumDistance = 1

	addCommonCmdOptions(releaseCmd.Flags())

	return releaseCmd
}

func (d *release) differentiate() error {

	releaseResponse1, err := d.deployedRelease(d.releases[0])
	if err != nil {
		return prettyError(err)
	}

	releaseResponse2, err := d.deployedRelease(d.releases[1])
	if err != nil {
		return prettyError(err)
	}

	if releaseResponse1.ChartName() == releaseResponse2.ChartName() {
		seenAnyChanges := diff.Releases(
			manifest.ParseRelease(releaseResponse1, d.includeTests),
			manifest.ParseRelease(releaseResponse2, d.includeTests),
			d.suppressedKinds,
			d.outputContext,
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

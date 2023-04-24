package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/databus23/helm-diff/v3/diff"
	"github.com/databus23/helm-diff/v3/manifest"
)

type release struct {
	detailedExitCode   bool
	releases           []string
	includeTests       bool
	normalizeManifests bool
	diff.Options
}

const releaseCmdLongUsage = `
This command compares the manifests details of a different releases created from the same chart.
The release name may be specified using namespace/release syntax.

It can be used to compare the manifests of

 - release1 with release2
	$ helm diff release [flags] release1 release2
   Example:
	$ helm diff release my-prod my-stage
	$ helm diff release prod/my-prod stage/my-stage
`

func releaseCmd() *cobra.Command {
	diff := release{}
	releaseCmd := &cobra.Command{
		Use:   "release [flags] RELEASE release1 [release2]",
		Short: "Shows diff between release's manifests",
		Long:  releaseCmdLongUsage,
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

			ProcessDiffOptions(cmd.Flags(), &diff.Options)

			diff.releases = args[0:]
			return diff.differentiateHelm3()
		},
	}

	releaseCmd.Flags().BoolVar(&diff.detailedExitCode, "detailed-exitcode", false, "return a non-zero exit code when there are changes")
	releaseCmd.Flags().BoolVar(&diff.includeTests, "include-tests", false, "enable the diffing of the helm test hooks")
	releaseCmd.Flags().BoolVar(&diff.normalizeManifests, "normalize-manifests", false, "normalize manifests before running diff to exclude style differences from the output")
	AddDiffOptions(releaseCmd.Flags(), &diff.Options)

	releaseCmd.SuggestionsMinimumDistance = 1

	return releaseCmd
}

func (d *release) differentiateHelm3() error {
	excludes := []string{helm3TestHook, helm2TestSuccessHook}
	if d.includeTests {
		excludes = []string{}
	}

	namespace1 := os.Getenv("HELM_NAMESPACE")
	release1 := d.releases[0]
	if strings.Contains(release1, "/") {
		namespace1 = strings.Split(release1, "/")[0]
		release1 = strings.Split(release1, "/")[1]
	}
	releaseResponse1, err := getRelease(release1, namespace1)
	if err != nil {
		return err
	}
	releaseChart1, err := getChart(release1, namespace1)
	if err != nil {
		return err
	}

	namespace2 := os.Getenv("HELM_NAMESPACE")
	release2 := d.releases[1]
	if strings.Contains(release2, "/") {
		namespace2 = strings.Split(release2, "/")[0]
		release2 = strings.Split(release2, "/")[1]
	}
	releaseResponse2, err := getRelease(release2, namespace2)
	if err != nil {
		return err
	}
	releaseChart2, err := getChart(release2, namespace2)
	if err != nil {
		return err
	}

	if releaseChart1 == releaseChart2 {
		seenAnyChanges := diff.Releases(
			manifest.Parse(string(releaseResponse1), namespace1, d.normalizeManifests, excludes...),
			manifest.Parse(string(releaseResponse2), namespace2, d.normalizeManifests, excludes...),
			&d.Options,
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

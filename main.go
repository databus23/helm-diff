package main

import (
	"errors"
	"fmt"
	"github.com/mgutz/ansi"
	"github.com/spf13/cobra"
	"k8s.io/helm/pkg/helm"
	"os"
	"strconv"

	"github.com/databus23/helm-diff/manifest"
)

const globalUsage = `

The Helm Diff Plugin

* Shows a diff explaing what a helm upgrade would change:
    This fetches the currently deployed version of a release
  and compares it to a local chart plus values. This can be 
  used visualize what changes a helm upgrade will perform.

* Shows a diff explaing what had changed between two revisions:
    This fetches previously deployed versions of a release
  and compares them. This can be used visualize what changes 
  were made during revision change.
`

// Version identifier populated via the CI/CD process.
var Version = "HEAD"

type diffCmd struct {
	release         string
	chart           string
	client          helm.Interface
	valueFiles      valueFiles
	values          []string
	reuseValues     bool
	suppressedKinds []string
	revisions       []string
}

func main() {
	diff := diffCmd{}

	const upgradeCmdLongUsage = `
This command compares the manifests details of a named release 
with values generated form charts.

It forecasts/visualizes changes, that a helm upgrade could perform.
`
	upgradeCmd := &cobra.Command{
		Use:   "upgrade [flags] [RELEASE] [CHART]",
		Short: "visualize changes, that a helm upgrade could perform",
		Long:  upgradeCmdLongUsage,
		RunE: func(cmd *cobra.Command, args []string) error {
			if v, _ := cmd.Flags().GetBool("version"); v {
				fmt.Println(Version)
				return nil
			}

			if err := checkArgsLength(len(args), "release name", "chart path"); err != nil {
				return err
			}

			if q, _ := cmd.Flags().GetBool("suppress-secrets"); q {
				diff.suppressedKinds = append(diff.suppressedKinds, "Secret")
			}

			diff.release = args[0]
			diff.chart = args[1]

			if diff.client == nil {
				diff.client = helm.NewClient(helm.Host(os.Getenv("TILLER_HOST")))
			}

			return diff.forecast()
		},
	}

	upgradeCmd.SuggestionsMinimumDistance = 1
	f := upgradeCmd.Flags()
	f.VarP(&diff.valueFiles, "values", "f", "specify values in a YAML file (can specify multiple)")
	f.StringArrayVar(&diff.values, "set", []string{}, "set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.BoolP("suppress-secrets", "q", false, "suppress secrets in the output")
	f.BoolVar(&diff.reuseValues, "reuse-values", false, "reuse the last release's values and merge in any new values")
	f.StringArrayVar(&diff.suppressedKinds, "suppress", []string{}, "allows suppression of the values listed in the diff output")

	const revisionCmdLongUsage = `
This command compares the manifests details of a named release.

It can be used to compare the manifests of 
 
 - lastest REVISION with specified REVISION
	$ helm diff revision [flags] [RELEASE] REVISION

 - REVISION1 with REVISION2
	$ helm diff revision [flags] [RELEASE] REVISION1 REVISION1
`

	revisionCmd := &cobra.Command{
		Use:   "revision [flags] [RELEASE] REVISION1 REVISION2",
		Short: "Shows diff between revision's manifests",
		Long:  revisionCmdLongUsage,
		RunE: func(cmd *cobra.Command, args []string) error {
			if v, _ := cmd.Flags().GetBool("version"); v {
				fmt.Println(Version)
				return nil
			}

			switch {
			case len(args) < 2:
				return errors.New("Too few arguments to Command \"revision\".\nMinimum 2 arguments required: release name, revision")
			case len(args) > 3:
				return errors.New("Too many arguments to Command \"revision\".\nMaximum 3 arguments allowed: release name, revision1, revision2")
			}

			if q, _ := cmd.Flags().GetBool("suppress-secrets"); q {
				diff.suppressedKinds = append(diff.suppressedKinds, "Secret")
			}

			if nc, _ := cmd.Flags().GetBool("no-color"); nc {
				ansi.DisableColors(true)
			}

			diff.release = args[0]
			diff.revisions = args[1:]
			if diff.client == nil {
				diff.client = helm.NewClient(helm.Host(os.Getenv("TILLER_HOST")))
			}
			return diff.differentiate()
		},
	}

	revisionCmd.SuggestionsMinimumDistance = 1

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "print the version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(Version)
			return nil
		},
	}

	rootCmd := &cobra.Command{
		Use:   "diff",
		Short: "Show differences between release revisions",
		Long:  globalUsage,
	}

	rootCmd.AddCommand(
		upgradeCmd,
		revisionCmd,
		versionCmd,
	)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(0)
	}
}

func (d *diffCmd) forecast() error {
	chartPath, err := locateChartPath(d.chart, "", false, "")
	if err != nil {
		return err
	}

	rawVals, err := d.vals()
	if err != nil {
		return err
	}

	releaseResponse, err := d.client.ReleaseContent(d.release)

	if err != nil {
		return prettyError(err)
	}

	upgradeResponse, err := d.client.UpdateRelease(
		d.release,
		chartPath,
		helm.UpdateValueOverrides(rawVals),
		helm.ReuseValues(d.reuseValues),
		helm.UpgradeDryRun(true),
	)
	if err != nil {
		return prettyError(err)
	}

	currentSpecs := manifest.Parse(releaseResponse.Release.Manifest)
	newSpecs := manifest.Parse(upgradeResponse.Release.Manifest)

	diffManifests(currentSpecs, newSpecs, d.suppressedKinds, os.Stdout)

	return nil
}

func (d *diffCmd) differentiate() error {

	switch len(d.revisions) {
	case 1:
		releaseResponse, err := d.client.ReleaseContent(d.release)

		if err != nil {
			return prettyError(err)
		}

		revision, _ := strconv.Atoi(d.revisions[0])
		revisionResponse, err := d.client.ReleaseContent(d.release, helm.ContentReleaseVersion(int32(revision)))
		if err != nil {
			return prettyError(err)
		}

		diffManifests(manifest.Parse(revisionResponse.Release.Manifest), manifest.Parse(releaseResponse.Release.Manifest), d.suppressedKinds, os.Stdout)

	case 2:
		revision1, _ := strconv.Atoi(d.revisions[0])
		revision2, _ := strconv.Atoi(d.revisions[1])
		if revision1 > revision2 {
			revision1, revision2 = revision2, revision1
		}

		revisionResponse1, err := d.client.ReleaseContent(d.release, helm.ContentReleaseVersion(int32(revision1)))
		if err != nil {
			return prettyError(err)
		}

		revisionResponse2, err := d.client.ReleaseContent(d.release, helm.ContentReleaseVersion(int32(revision2)))
		if err != nil {
			return prettyError(err)
		}

		diffManifests(manifest.Parse(revisionResponse1.Release.Manifest), manifest.Parse(revisionResponse2.Release.Manifest), d.suppressedKinds, os.Stdout)

	default:
		return errors.New("Invalid Arguments")
	}

	return nil
}

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/helm/pkg/helm"

	"github.com/databus23/helm-diff/manifest"
)

const globalUsage = `
Show a diff explaing what a helm upgrade would change.

This fetches the currently deployed version of a release
and compares it to a local chart plus values.
This can be used visualize what changes a helm upgrade will
perform.
`

var Version string = "HEAD"

type diffCmd struct {
	release         string
	chart           string
	client          helm.Interface
	valueFiles      valueFiles
	values          []string
	suppressedKinds []string
}

func main() {
	diff := diffCmd{}

	cmd := &cobra.Command{
		Use:   "diff [flags] [RELEASE] [CHART]",
		Short: "Show manifest differences",
		Long:  globalUsage,
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
			return diff.run()
		},
	}

	f := cmd.Flags()
	f.BoolP("version", "v", false, "show version")
	f.BoolP("suppress-secrets", "q", false,  "suppress secrets in the output")
	f.VarP(&diff.valueFiles, "values", "f", "specify values in a YAML file (can specify multiple)")
	f.StringArrayVar(&diff.values, "set", []string{}, "set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.StringArrayVar(&diff.suppressedKinds, "suppress", []string{}, "suppress secrets in the output")

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func (d *diffCmd) run() error {
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

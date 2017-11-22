package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/databus23/helm-diff/manifest"
	"github.com/spf13/cobra"
	"k8s.io/helm/pkg/helm"
)

const globalUsage = `
Show a diff explaing what a helm upgrade would change.

This fetches the currently deployed version of a release
and compares it to a local chart plus values.
This can be used visualize what changes a helm upgrade will
perform.
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
	f.BoolP("suppress-secrets", "q", false, "suppress secrets in the output")
	f.VarP(&diff.valueFiles, "values", "f", "specify values in a YAML file (can specify multiple)")
	f.StringArrayVar(&diff.values, "set", []string{}, "set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.BoolVar(&diff.reuseValues, "reuse-values", false, "reuse the last release's values and merge in any new values")
	f.StringArrayVar(&diff.suppressedKinds, "suppress", []string{}, "allows suppression of the values listed in the diff output")

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

	var newInstall bool
	if err != nil && strings.Contains(err.Error(), fmt.Sprintf("release: %q not found", d.release)) {
		fmt.Println("Release was not present in Helm.  Diff will show entire contents as new.")
		newInstall = true
		err = nil
	}

	if err != nil {
		return prettyError(err)
	}

	var currentSpecs, newSpecs map[string]*manifest.MappingResult
	if newInstall {
		installResponse, err := d.client.InstallRelease(
			chartPath,
			"default",
			helm.ReleaseName(d.release),
			helm.ValueOverrides(rawVals),
			helm.InstallDryRun(true),
		)
		if err != nil {
			return prettyError(err)
		}

		currentSpecs = make(map[string]*manifest.MappingResult)
		newSpecs = manifest.Parse(installResponse.Release.Manifest)
	} else {
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

		currentSpecs = manifest.Parse(releaseResponse.Release.Manifest)
		newSpecs = manifest.Parse(upgradeResponse.Release.Manifest)
	}

	diffManifests(currentSpecs, newSpecs, d.suppressedKinds, os.Stdout)

	return nil
}

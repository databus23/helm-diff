package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/databus23/helm-diff/diff"
	"github.com/databus23/helm-diff/manifest"
	"github.com/spf13/cobra"
	"k8s.io/helm/pkg/helm"
)

type rollback struct {
	release         string
	client          helm.Interface
	suppressedKinds []string
	revisions       []string
	outputContext   int
}

const rollbackCmdLongUsage = `
This command compares the laset manifests details of a named release 
with specific revision values to rollback.

It forecasts/visualizes changes, that a helm rollback could perform.
`

func rollbackCmd() *cobra.Command {
	diff := rollback{}
	rollbackCmd := &cobra.Command{
		Use:     "rollback [flags] [RELEASE] [REVISION]",
		Short:   "Show a diff explaining what a helm rollback could perform",
		Long:    rollbackCmdLongUsage,
		Example: "  helm diff rollback my-release 2",
		RunE: func(cmd *cobra.Command, args []string) error {
			if v, _ := cmd.Flags().GetBool("version"); v {
				fmt.Println(Version)
				return nil
			}

			if err := checkArgsLength(len(args), "release name", "revision number"); err != nil {
				return err
			}

			if q, _ := cmd.Flags().GetBool("suppress-secrets"); q {
				diff.suppressedKinds = append(diff.suppressedKinds, "Secret")
			}

			diff.release = args[0]
			diff.revisions = args[1:]

			if diff.client == nil {
				diff.client = helm.NewClient(helm.Host(os.Getenv("TILLER_HOST")), helm.ConnectTimeout(int64(30)))
			}

			return diff.backcast()
		},
	}

	rollbackCmd.Flags().BoolP("suppress-secrets", "q", false, "suppress secrets in the output")
	rollbackCmd.Flags().StringArrayVar(&diff.suppressedKinds, "suppress", []string{}, "allows suppression of the values listed in the diff output")
	rollbackCmd.Flags().IntVarP(&diff.outputContext, "context", "C", -1, "output NUM lines of context around changes")
	rollbackCmd.SuggestionsMinimumDistance = 1
	return rollbackCmd
}

func (d *rollback) backcast() error {

	// get manifest of the latest release
	releaseResponse, err := d.client.ReleaseContent(d.release)

	if err != nil {
		return prettyError(err)
	}

	// get manifest of the release to rollback
	revision, _ := strconv.Atoi(d.revisions[0])
	revisionResponse, err := d.client.ReleaseContent(d.release, helm.ContentReleaseVersion(int32(revision)))
	if err != nil {
		return prettyError(err)
	}

	// create a diff between the current manifest and the version of the manifest that a user is intended to rollback
	diff.DiffManifests(manifest.Parse(releaseResponse.Release.Manifest), manifest.Parse(revisionResponse.Release.Manifest), d.suppressedKinds, d.outputContext, os.Stdout)

	return nil
}

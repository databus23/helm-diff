package cmd

import (
	"fmt"
	"os"
	"strconv"

	"errors"
	"github.com/databus23/helm-diff/diff"
	"github.com/databus23/helm-diff/manifest"
	"github.com/spf13/cobra"
	"k8s.io/helm/pkg/helm"
)

type rollback struct {
	release          string
	client           helm.Interface
	detailedExitCode bool
	suppressedKinds  []string
	revisions        []string
	outputContext    int
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
		PersistentPreRun: func(*cobra.Command, []string) {
			expandTLSPaths()
		},
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
				diff.client = createHelmClient()
			}

			return diff.backcast()
		},
	}

	rollbackCmd.Flags().BoolP("suppress-secrets", "q", false, "suppress secrets in the output")
	rollbackCmd.Flags().StringArrayVar(&diff.suppressedKinds, "suppress", []string{}, "allows suppression of the values listed in the diff output")
	rollbackCmd.Flags().IntVarP(&diff.outputContext, "context", "C", -1, "output NUM lines of context around changes")
	rollbackCmd.SuggestionsMinimumDistance = 1

	addCommonCmdOptions(rollbackCmd.Flags())

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
	seenAnyChanges := diff.DiffManifests(
		manifest.ParseRelease(releaseResponse.Release),
		manifest.ParseRelease(revisionResponse.Release),
		d.suppressedKinds,
		d.outputContext,
		os.Stdout)

	if d.detailedExitCode && seenAnyChanges {
		return Error{
			error: errors.New("identified at least one change, exiting with non-zero exit code (detailed-exitcode parameter enabled)"),
			Code:  2,
		}
	}

	return nil
}

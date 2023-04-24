package cmd

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/databus23/helm-diff/v3/diff"
	"github.com/databus23/helm-diff/v3/manifest"
)

type rollback struct {
	release            string
	detailedExitCode   bool
	revisions          []string
	includeTests       bool
	normalizeManifests bool
	diff.Options
}

const rollbackCmdLongUsage = `
This command compares the latest manifest details of a named release
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
			// Suppress the command usage on error. See #77 for more info
			cmd.SilenceUsage = true

			if v, _ := cmd.Flags().GetBool("version"); v {
				fmt.Println(Version)
				return nil
			}

			if err := checkArgsLength(len(args), "release name", "revision number"); err != nil {
				return err
			}

			ProcessDiffOptions(cmd.Flags(), &diff.Options)

			diff.release = args[0]
			diff.revisions = args[1:]

			return diff.backcastHelm3()
		},
	}

	rollbackCmd.Flags().BoolVar(&diff.detailedExitCode, "detailed-exitcode", false, "return a non-zero exit code when there are changes")
	rollbackCmd.Flags().BoolVar(&diff.includeTests, "include-tests", false, "enable the diffing of the helm test hooks")
	rollbackCmd.Flags().BoolVar(&diff.normalizeManifests, "normalize-manifests", false, "normalize manifests before running diff to exclude style differences from the output")
	AddDiffOptions(rollbackCmd.Flags(), &diff.Options)

	rollbackCmd.SuggestionsMinimumDistance = 1

	return rollbackCmd
}

func (d *rollback) backcastHelm3() error {
	namespace := os.Getenv("HELM_NAMESPACE")
	excludes := []string{helm3TestHook, helm2TestSuccessHook}
	if d.includeTests {
		excludes = []string{}
	}
	// get manifest of the latest release
	releaseResponse, err := getRelease(d.release, namespace)

	if err != nil {
		return err
	}

	// get manifest of the release to rollback
	revision, _ := strconv.Atoi(d.revisions[0])
	revisionResponse, err := getRevision(d.release, revision, namespace)
	if err != nil {
		return err
	}

	// create a diff between the current manifest and the version of the manifest that a user is intended to rollback
	seenAnyChanges := diff.Manifests(
		manifest.Parse(string(releaseResponse), namespace, d.normalizeManifests, excludes...),
		manifest.Parse(string(revisionResponse), namespace, d.normalizeManifests, excludes...),
		&d.Options,
		os.Stdout)

	if d.detailedExitCode && seenAnyChanges {
		return Error{
			error: errors.New("identified at least one change, exiting with non-zero exit code (detailed-exitcode parameter enabled)"),
			Code:  2,
		}
	}

	return nil
}

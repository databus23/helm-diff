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

type revision struct {
	release            string
	detailedExitCode   bool
	revisions          []string
	includeTests       bool
	normalizeManifests bool
	diff.Options
}

const revisionCmdLongUsage = `
This command compares the manifests details of a named release.

It can be used to compare the manifests of

 - latest REVISION with specified REVISION
	$ helm diff revision [flags] RELEASE REVISION1
   Example:
	$ helm diff revision my-release 2

 - REVISION1 with REVISION2
	$ helm diff revision [flags] RELEASE REVISION1 REVISION2
   Example:
	$ helm diff revision my-release 2 3
`

func revisionCmd() *cobra.Command {
	diff := revision{}
	revisionCmd := &cobra.Command{
		Use:   "revision [flags] RELEASE REVISION1 [REVISION2]",
		Short: "Shows diff between revision's manifests",
		Long:  revisionCmdLongUsage,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Suppress the command usage on error. See #77 for more info
			cmd.SilenceUsage = true

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

			ProcessDiffOptions(cmd.Flags(), &diff.Options)

			diff.release = args[0]
			diff.revisions = args[1:]
			return diff.differentiateHelm3()
		},
	}

	revisionCmd.Flags().BoolVar(&diff.detailedExitCode, "detailed-exitcode", false, "return a non-zero exit code when there are changes")
	revisionCmd.Flags().BoolVar(&diff.includeTests, "include-tests", false, "enable the diffing of the helm test hooks")
	revisionCmd.Flags().BoolVar(&diff.normalizeManifests, "normalize-manifests", false, "normalize manifests before running diff to exclude style differences from the output")
	AddDiffOptions(revisionCmd.Flags(), &diff.Options)

	revisionCmd.SuggestionsMinimumDistance = 1

	return revisionCmd
}

func (d *revision) differentiateHelm3() error {
	namespace := os.Getenv("HELM_NAMESPACE")
	excludes := []string{helm3TestHook, helm2TestSuccessHook}
	if d.includeTests {
		excludes = []string{}
	}
	switch len(d.revisions) {
	case 1:
		releaseResponse, err := getRelease(d.release, namespace)

		if err != nil {
			return err
		}

		revision, _ := strconv.Atoi(d.revisions[0])
		revisionResponse, err := getRevision(d.release, revision, namespace)
		if err != nil {
			return err
		}

		diff.Manifests(
			manifest.Parse(string(revisionResponse), namespace, d.normalizeManifests, excludes...),
			manifest.Parse(string(releaseResponse), namespace, d.normalizeManifests, excludes...),
			&d.Options,
			os.Stdout)

	case 2:
		revision1, _ := strconv.Atoi(d.revisions[0])
		revision2, _ := strconv.Atoi(d.revisions[1])
		if revision1 > revision2 {
			revision1, revision2 = revision2, revision1
		}

		revisionResponse1, err := getRevision(d.release, revision1, namespace)
		if err != nil {
			return err
		}

		revisionResponse2, err := getRevision(d.release, revision2, namespace)
		if err != nil {
			return err
		}

		seenAnyChanges := diff.Manifests(
			manifest.Parse(string(revisionResponse1), namespace, d.normalizeManifests, excludes...),
			manifest.Parse(string(revisionResponse2), namespace, d.normalizeManifests, excludes...),
			&d.Options,
			os.Stdout)

		if d.detailedExitCode && seenAnyChanges {
			return Error{
				error: errors.New("identified at least one change, exiting with non-zero exit code (detailed-exitcode parameter enabled)"),
				Code:  2,
			}
		}

	default:
		return errors.New("Invalid Arguments")
	}

	return nil
}

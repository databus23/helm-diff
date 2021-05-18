package cmd

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"k8s.io/helm/pkg/helm"

	"github.com/databus23/helm-diff/v3/diff"
	"github.com/databus23/helm-diff/v3/manifest"
)

type revision struct {
	release          string
	client           helm.Interface
	detailedExitCode bool
	suppressedKinds  []string
	revisions        []string
	outputContext    int
	includeTests     bool
	showSecrets      bool
	output           string
	stripTrailingCR  bool
}

const revisionCmdLongUsage = `
This command compares the manifests details of a named release.

It can be used to compare the manifests of

 - lastest REVISION with specified REVISION
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
				return errors.New("Too few arguments to Command \"revision\".\nMinimum 2 arguments required: release name, revision")
			case len(args) > 3:
				return errors.New("Too many arguments to Command \"revision\".\nMaximum 3 arguments allowed: release name, revision1, revision2")
			}

			if q, _ := cmd.Flags().GetBool("suppress-secrets"); q {
				diff.suppressedKinds = append(diff.suppressedKinds, "Secret")
			}

			diff.release = args[0]
			diff.revisions = args[1:]
			if isHelm3() {
				return diff.differentiateHelm3()
			}
			if diff.client == nil {
				diff.client = createHelmClient()
			}
			return diff.differentiate()
		},
	}

	revisionCmd.Flags().BoolP("suppress-secrets", "q", false, "suppress secrets in the output")
	revisionCmd.Flags().BoolVar(&diff.showSecrets, "show-secrets", false, "do not redact secret values in the output")
	revisionCmd.Flags().BoolVar(&diff.detailedExitCode, "detailed-exitcode", false, "return a non-zero exit code when there are changes")
	revisionCmd.Flags().StringArrayVar(&diff.suppressedKinds, "suppress", []string{}, "allows suppression of the values listed in the diff output")
	revisionCmd.Flags().IntVarP(&diff.outputContext, "context", "C", -1, "output NUM lines of context around changes")
	revisionCmd.Flags().BoolVar(&diff.includeTests, "include-tests", false, "enable the diffing of the helm test hooks")
	revisionCmd.Flags().StringVar(&diff.output, "output", "diff", "Possible values: diff, simple, template. When set to \"template\", use the env var HELM_DIFF_TPL to specify the template.")
	revisionCmd.Flags().BoolVar(&diff.stripTrailingCR, "strip-trailing-cr", false, "strip trailing carriage return on input")

	revisionCmd.SuggestionsMinimumDistance = 1

	if !isHelm3() {
		addCommonCmdOptions(revisionCmd.Flags())
	}

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
			manifest.Parse(string(revisionResponse), namespace, excludes...),
			manifest.Parse(string(releaseResponse), namespace, excludes...),
			d.suppressedKinds,
			d.showSecrets,
			d.outputContext,
			d.output,
			d.stripTrailingCR,
			os.Stdout)

	case 2:
		revision1, _ := strconv.Atoi(d.revisions[0])
		revision2, _ := strconv.Atoi(d.revisions[1])
		if revision1 > revision2 {
			revision1, revision2 = revision2, revision1
		}

		revisionResponse1, err := getRevision(d.release, revision1, namespace)
		if err != nil {
			return prettyError(err)
		}

		revisionResponse2, err := getRevision(d.release, revision2, namespace)
		if err != nil {
			return prettyError(err)
		}

		seenAnyChanges := diff.Manifests(
			manifest.Parse(string(revisionResponse1), namespace, excludes...),
			manifest.Parse(string(revisionResponse2), namespace, excludes...),
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

	default:
		return errors.New("Invalid Arguments")
	}

	return nil
}

func (d *revision) differentiate() error {

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

		diff.Manifests(
			manifest.ParseRelease(revisionResponse.Release, d.includeTests),
			manifest.ParseRelease(releaseResponse.Release, d.includeTests),
			d.suppressedKinds,
			d.showSecrets,
			d.outputContext,
			d.output,
			d.stripTrailingCR,
			os.Stdout)

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

		seenAnyChanges := diff.Manifests(
			manifest.ParseRelease(revisionResponse1.Release, d.includeTests),
			manifest.ParseRelease(revisionResponse2.Release, d.includeTests),
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

	default:
		return errors.New("Invalid Arguments")
	}

	return nil
}

package cmd

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/databus23/helm-diff/diff"
	"github.com/databus23/helm-diff/manifest"
	"github.com/spf13/cobra"
	"k8s.io/helm/pkg/helm"
)

type revision struct {
	release         string
	client          helm.Interface
	suppressedKinds []string
	revisions       []string
	outputContext   int
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

			diff.release = args[0]
			diff.revisions = args[1:]
			if diff.client == nil {
				diff.client = helm.NewClient(helm.Host(os.Getenv("TILLER_HOST")), helm.ConnectTimeout(int64(30)))
			}
			return diff.differentiate()
		},
	}

	revisionCmd.Flags().BoolP("suppress-secrets", "q", false, "suppress secrets in the output")
	revisionCmd.Flags().StringArrayVar(&diff.suppressedKinds, "suppress", []string{}, "allows suppression of the values listed in the diff output")
	revisionCmd.Flags().IntVarP(&diff.outputContext, "context", "C", -1, "output NUM lines of context around changes")
	revisionCmd.SuggestionsMinimumDistance = 1
	return revisionCmd
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

		diff.DiffManifests(
			manifest.ParseRelease(revisionResponse.Release),
			manifest.ParseRelease(releaseResponse.Release),
			d.suppressedKinds,
			d.outputContext,
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

		diff.DiffManifests(
			manifest.ParseRelease(revisionResponse1.Release),
			manifest.ParseRelease(revisionResponse2.Release),
			d.suppressedKinds,
			d.outputContext,
			os.Stdout)

	default:
		return errors.New("Invalid Arguments")
	}

	return nil
}

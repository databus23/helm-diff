package cmd

import (
	"github.com/mgutz/ansi"
	"github.com/spf13/cobra"
)

const rootCmdLongUsage = `
The Helm Diff Plugin

* Shows a diff explaining what a helm upgrade would change:
    This fetches the currently deployed version of a release
  and compares it to a local chart plus values. This can be 
  used visualize what changes a helm upgrade will perform.

* Shows a diff explaining what had changed between two revisions:
    This fetches previously deployed versions of a release
  and compares them. This can be used visualize what changes 
  were made during revision change.

* Shows a diff explaining what a helm rollback would change:
    This fetches the currently deployed version of a release
  and compares it to adeployed versions of a release, that you 
  want to rollback. This can be used visualize what changes a 
  helm rollback will perform.
`

func New() *cobra.Command {

	chartCommand := newChartCommand()

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Show manifest differences",
		Long:  rootCmdLongUsage,
		//Alias root command to chart subcommand
		Args: chartCommand.Args,
		// parse the flags and check for actions like suppress-secrets, no-colors
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if nc, _ := cmd.Flags().GetBool("no-color"); nc {
				ansi.DisableColors(true)
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println(`Command "helm diff" is deprecated, use "helm diff upgrade" instead`)
			return chartCommand.RunE(cmd, args)
		},
	}

	// add no-color as global flag
	cmd.PersistentFlags().Bool("no-color", false, "remove colors from the output")
	// add flagset from chartCommand
	cmd.Flags().AddFlagSet(chartCommand.Flags())
	cmd.AddCommand(newVersionCmd(), chartCommand)
	// add subcommands
	cmd.AddCommand(
		revisionCmd(),
		rollbackCmd(),
		releaseCmd(),
	)
	cmd.SetHelpCommand(&cobra.Command{}) // Disable the help command
	return cmd
}

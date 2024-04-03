package cmd

import (
	"os"
	"strconv"
	"strings"

	"github.com/gonvenience/bunt"
	"github.com/mgutz/ansi"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const rootCmdLongUsage = `
The Helm Diff Plugin

* Shows a diff explaining what a helm upgrade would change:
    This fetches the currently deployed version of a release
  and compares it to a local chart plus values. This can be
  used to visualize what changes a helm upgrade will perform.

* Shows a diff explaining what had changed between two revisions:
    This fetches previously deployed versions of a release
  and compares them. This can be used to visualize what changes
  were made during revision change.

* Shows a diff explaining what a helm rollback would change:
    This fetches the currently deployed version of a release
  and compares it to the previously deployed version of the release, that you
  want to rollback. This can be used to visualize what changes a
  helm rollback will perform.
`

// New creates a new cobra client
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
			var fc *bool

			if cmd.Flags().Changed("color") {
				v, _ := cmd.Flags().GetBool("color")
				fc = &v
			} else {
				v, err := strconv.ParseBool(os.Getenv("HELM_DIFF_COLOR"))
				if err == nil {
					fc = &v
				}
			}

			if !cmd.Flags().Changed("output") {
				v, set := os.LookupEnv("HELM_DIFF_OUTPUT")
				if set && strings.TrimSpace(v) != "" {
					_ = cmd.Flags().Set("output", v)
				}
			}

			// Dyff relies on bunt, default to color=on
			bunt.SetColorSettings(bunt.ON, bunt.ON)
			nc, _ := cmd.Flags().GetBool("no-color")

			if nc || (fc != nil && !*fc) {
				ansi.DisableColors(true)
				bunt.SetColorSettings(bunt.OFF, bunt.OFF)
			} else if !cmd.Flags().Changed("no-color") && fc == nil {
				term := term.IsTerminal(int(os.Stdout.Fd()))
				// https://github.com/databus23/helm-diff/issues/281
				dumb := os.Getenv("TERM") == "dumb"
				ansi.DisableColors(!term || dumb)
				bunt.SetColorSettings(bunt.OFF, bunt.OFF)
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println(`Command "helm diff" is deprecated, use "helm diff upgrade" instead`)
			return chartCommand.RunE(cmd, args)
		},
	}

	// add no-color as global flag
	cmd.PersistentFlags().Bool("no-color", false, "remove colors from the output. If both --no-color and --color are unspecified, coloring enabled only when the stdout is a term and TERM is not \"dumb\"")
	cmd.PersistentFlags().Bool("color", false, "color output. You can control the value for this flag via HELM_DIFF_COLOR=[true|false]. If both --no-color and --color are unspecified, coloring enabled only when the stdout is a term and TERM is not \"dumb\"")
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

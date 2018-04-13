package cmd

import "github.com/spf13/cobra"

func New() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Show manifest differences",
	}

	chartCommand := newChartCommand()
	cmd.AddCommand(newVersionCmd(), chartCommand)
	//Alias root command to chart subcommand
	cmd.Args = chartCommand.Args
	cmd.Flags().AddFlagSet(chartCommand.Flags())
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		cmd.Println(`Command "helm diff" is deprecated, use "helm diff upgrade" instead`)
		return chartCommand.RunE(cmd, args)
	}
	cmd.SetHelpCommand(&cobra.Command{}) // Disable the help command

	return cmd

}

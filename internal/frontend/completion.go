package frontend

import (
	"fmt"

	"github.com/spf13/cobra"
)

const completionNoDescFlag = "no-descriptions"

func init() {
	rootCmd.AddCommand(newCompletionCmd())
}

func newCompletionCmd() *cobra.Command {
	completionCmd := &cobra.Command{
		Use:               "completion",
		Short:             "Generate the autocompletion script for the specified shell",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
	}

	completionCmd.AddCommand(
		newCompletionSubcommand("bash", "bash", func(cmd *cobra.Command, noDesc bool) error {
			return cmd.Root().GenBashCompletionV2(cmd.OutOrStdout(), !noDesc)
		}),
		newCompletionSubcommand("zsh", "zsh", func(cmd *cobra.Command, noDesc bool) error {
			if noDesc {
				return cmd.Root().GenZshCompletionNoDesc(cmd.OutOrStdout())
			}
			return cmd.Root().GenZshCompletion(cmd.OutOrStdout())
		}),
		newCompletionSubcommand("fish", "fish", func(cmd *cobra.Command, noDesc bool) error {
			return cmd.Root().GenFishCompletion(cmd.OutOrStdout(), !noDesc)
		}),
		newCompletionSubcommand("powershell", "powershell", func(cmd *cobra.Command, noDesc bool) error {
			if noDesc {
				return cmd.Root().GenPowerShellCompletion(cmd.OutOrStdout())
			}
			return cmd.Root().GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
		}),
	)

	return completionCmd
}

func newCompletionSubcommand(use, shell string, run func(cmd *cobra.Command, noDesc bool) error) *cobra.Command {
	var noDesc bool
	cmd := &cobra.Command{
		Use:   use,
		Short: fmt.Sprintf("Generate the autocompletion script for %s", shell),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd, noDesc)
		},
	}
	cmd.Flags().BoolVarP(&noDesc, completionNoDescFlag, "n", false, "disable completion descriptions")
	return cmd
}

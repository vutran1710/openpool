package cli

import (
	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "dating",
		Short: "A terminal-native dating platform",
		Long:  "Dating CLI — find meaningful connections from your terminal.",
		RunE: func(cmd *cobra.Command, args []string) error {
			printHeader()
			return cmd.Help()
		},
	}

	root.AddCommand(
		newAuthCmd(),
		newFetchCmd(),
		newViewCmd(),
		newLikeCmd(),
		newMatchesCmd(),
		newChatCmd(),
		newCommitCmd(),
		newStatusCmd(),
		newProfileCmd(),
	)

	return root
}

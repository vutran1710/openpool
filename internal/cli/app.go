package cli

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/cli/tui"
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "dating",
		Short: "A terminal-native dating platform",
		Long:  "Dating CLI — find meaningful connections from your terminal.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if isInteractive() {
				cfg, _ := config.Load()
				user := ""
				pool := ""
				if cfg != nil && cfg.IsRegistered() {
					user = cfg.User.PublicID
				}
				if cfg != nil && cfg.ActivePool() != nil {
					pool = cfg.ActivePool().Name
				}
				tui.RunOrFallback(user, pool)
				return nil
			}
			printHeader()
			return cmd.Help()
		},
	}

	root.AddCommand(
		newAuthCmd(),
		newPoolCmd(),
		newFetchCmd(),
		newViewCmd(),
		newLikeCmd(),
		newInboxCmd(),
		newAcceptCmd(),
		newMatchesCmd(),
		newChatCmd(),
		newCommitCmd(),
		newProfileCmd(),
	)

	return root
}

func isInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

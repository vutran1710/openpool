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
				if cfg == nil {
					cfg = &config.Config{}
				}

				needsOnboarding := !cfg.HasToken()
				registry := cfg.ActiveRegistry

				userName := cfg.User.DisplayName
				userHash := cfg.User.IDHash
				pool := ""
				poolStatuses := make(map[string]string)
				poolIssues := make(map[string]int)
				if cfg.ActivePool() != nil {
					pool = cfg.ActivePool().Name
				}
				for _, p := range cfg.Pools {
					status := p.Status
					if status == "" {
						status = "active"
					}
					poolStatuses[p.Name] = status
					if p.PendingIssue > 0 {
						poolIssues[p.Name] = p.PendingIssue
					}
				}
				tui.RunOrFallback(userName, userHash, pool, registry, poolStatuses, poolIssues, needsOnboarding)
				return nil
			}
			printHeader()
			return cmd.Help()
		},
	}

	root.AddCommand(
		newAuthCmd(),
		newRegistryCmd(),
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
		newDiscoverCmd(),
		newResetCmd(),
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

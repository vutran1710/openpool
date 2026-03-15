package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
)

func newMatchesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "matches",
		Short: "List your matches",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			pool, err := requirePool(cfg)
			if err != nil {
				return nil
			}

			client := poolClient(pool)
			matchDirs, err := client.ListMatches(ctx)
			if err != nil {
				return fmt.Errorf("fetching matches: %w", err)
			}

			if len(matchDirs) == 0 {
				printDim("  No matches yet. Try: dating fetch")
				return nil
			}

			fmt.Println()
			fmt.Printf("  %s\n\n", bold.Render("Your Matches"))
			for _, m := range matchDirs {
				fmt.Printf("  %s  %s\n", brand.Render("♥"), m)
			}
			fmt.Println()
			printDim("  Start a conversation: dating chat <public_id>")
			fmt.Println()
			return nil
		},
	}
}

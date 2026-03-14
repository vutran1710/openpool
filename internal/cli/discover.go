package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
)

func newFetchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "fetch",
		Short: "Discover new profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			pool, err := requirePool(cfg)
			if err != nil {
				return nil
			}

			client := poolClient(pool)
			userHash, err := client.DiscoverRandom(cfg.User.PublicID)
			if err != nil {
				return fmt.Errorf("discovering: %w", err)
			}

			if userHash == "" {
				printDim("  No profiles found. Check back later.")
				return nil
			}

			fmt.Println()
			fmt.Printf("  %s  %s\n", bold.Render(userHash[:12]), dim.Render("(encrypted profile)"))
			fmt.Println()
			printDim("  Like: dating like " + userHash[:12])
			printDim("  View: dating view " + userHash[:12])
			fmt.Println()
			return nil
		},
	}
}

func newViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view <user_hash>",
		Short: "View a user's encrypted profile blob",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			pool, err := requirePool(cfg)
			if err != nil {
				return nil
			}

			client := poolClient(pool)
			blob, err := client.GetUserBlob(args[0])
			if err != nil {
				printError("User not found: " + args[0])
				return nil
			}

			fmt.Println()
			fmt.Printf("  %s  %s  %d bytes\n",
				bold.Render(args[0][:12]),
				dim.Render("encrypted blob"),
				len(blob),
			)
			printDim("  Decrypt with your key to view profile data.")
			fmt.Println()
			return nil
		},
	}
}

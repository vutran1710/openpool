package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
)

func newCommitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "commit <public_id>",
		Short: "Propose a commitment to a match",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			_, err = requirePool(cfg)
			if err != nil {
				return nil
			}

			// TODO: implement via GitHub workflow
			printWarning("Commitment flow coming soon")
			return nil
		},
	}
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check your relationship status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			_, err = requirePool(cfg)
			if err != nil {
				return nil
			}

			// TODO: check commitments dir in repo
			fmt.Println()
			fmt.Printf("  %s %s\n", dim.Render("status:"), "single")
			fmt.Println()
			return nil
		},
	}
}

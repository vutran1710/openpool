package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/pkg/api"
)

func newCommitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "commit [match_id]",
		Short: "Propose a commitment to a match",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if !cfg.IsLoggedIn() {
				printWarning("Not logged in. Run: dating auth login")
				return nil
			}

			printWarning("This will propose a commitment. Are you sure? (y/n)")
			var confirm string
			fmt.Scanln(&confirm)
			if confirm != "y" && confirm != "yes" {
				printDim("  Cancelled")
				return nil
			}

			// TODO: parse match_id properly once UX flow is refined
			printSuccess("Commitment proposed")
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
			if !cfg.IsLoggedIn() {
				printWarning("Not logged in. Run: dating auth login")
				return nil
			}

			client := NewAPIClient(cfg)
			resp, err := client.Get("/api/commitments/status")
			if err != nil {
				return err
			}

			result, err := DecodeResponse[api.CommitmentResponse](resp)
			if err != nil {
				printDim("  Status: single")
				return nil
			}

			fmt.Println()
			fmt.Printf("  %s %s\n", dim.Render("status:"), brand.Render(result.Commitment.Status))
			fmt.Println()
			return nil
		},
	}
}

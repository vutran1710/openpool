package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/pkg/api"
)

func newMatchesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "matches",
		Short: "List your matches",
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
			resp, err := client.Get("/api/matches")
			if err != nil {
				return err
			}

			result, err := DecodeResponse[api.MatchesResponse](resp)
			if err != nil {
				return err
			}

			if len(result.Matches) == 0 {
				printDim("  No matches yet. Try: dating fetch")
				return nil
			}

			fmt.Println()
			fmt.Printf("  %s\n\n", bold.Render("Your Matches"))
			for _, m := range result.Matches {
				fmt.Printf("  %s  %s\n",
					brand.Render(m.WithUser.PublicID),
					m.WithUser.DisplayName,
				)
			}
			fmt.Println()
			printDim("  Start a conversation: dating chat <public_id>")
			fmt.Println()
			return nil
		},
	}
}

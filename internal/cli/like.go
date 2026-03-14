package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/pkg/api"
)

func newLikeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "like [public_id]",
		Short: "Express interest in someone",
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

			client := NewAPIClient(cfg)
			resp, err := client.Post("/api/likes", api.LikeRequest{
				TargetPublicID: args[0],
			})
			if err != nil {
				return err
			}

			result, err := DecodeResponse[api.LikeResponse](resp)
			if err != nil {
				if resp.StatusCode == 409 {
					printWarning("You already liked " + args[0])
					return nil
				}
				return err
			}

			if result.Matched {
				fmt.Println()
				printBrand("  It's a match!")
				printSuccess(fmt.Sprintf("You and %s matched. Start chatting: dating chat %s", args[0], args[0]))
			} else {
				printSuccess(fmt.Sprintf("Liked %s", args[0]))
			}
			return nil
		},
	}
}

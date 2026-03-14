package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/crypto"
)

func newLikeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "like <public_id>",
		Short: "Express interest in someone",
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

			_, priv, err := crypto.LoadKeyPair(config.KeysDir())
			if err != nil {
				return fmt.Errorf("loading keys: %w", err)
			}

			payload, _ := json.Marshal(map[string]string{
				"action":   "like",
				"liker_id": cfg.User.PublicID,
				"liked_id": args[0],
			})
			signature := crypto.Sign(priv, payload)

			client := poolClient(pool)
			if err := client.CreateLikePR(cfg.User.PublicID, args[0], signature); err != nil {
				return fmt.Errorf("sending like: %w", err)
			}

			printSuccess(fmt.Sprintf("Interest sent to %s", args[0]))
			return nil
		},
	}
}

func newInboxCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "inbox",
		Short: "View incoming interests",
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
			prs, err := client.ListIncomingLikes(cfg.User.PublicID)
			if err != nil {
				return fmt.Errorf("fetching inbox: %w", err)
			}

			if len(prs) == 0 {
				printDim("  No incoming interests yet.")
				return nil
			}

			fmt.Println()
			fmt.Printf("  %s\n\n", bold.Render("Incoming Interests"))
			for _, pr := range prs {
				fmt.Printf("  %s  PR #%d  %s\n",
					brand.Render("♥"),
					pr.Number,
					pr.Title,
				)
			}
			fmt.Println()
			printDim("  Accept with: dating accept <pr_number>")
			fmt.Println()
			return nil
		},
	}
}

func newAcceptCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "accept <pr_number>",
		Short: "Accept an incoming interest",
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

			_, priv, err := crypto.LoadKeyPair(config.KeysDir())
			if err != nil {
				return fmt.Errorf("loading keys: %w", err)
			}

			payload, _ := json.Marshal(map[string]string{
				"action":    "accept",
				"public_id": cfg.User.PublicID,
				"pr_number": args[0],
			})
			signature := crypto.Sign(priv, payload)

			var prNumber int
			fmt.Sscanf(args[0], "%d", &prNumber)

			client := poolClient(pool)
			if err := client.AcceptLike(prNumber, cfg.User.PublicID, signature); err != nil {
				return fmt.Errorf("accepting: %w", err)
			}

			printBrand("  It's a match!")
			printSuccess("Match created. Start chatting: dating chat <public_id>")
			return nil
		},
	}
}

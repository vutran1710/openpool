package cli

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/crypto"
)

func newLikeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "like <public_id> [message]",
		Short: "Express interest in someone",
		Args:  cobra.MinimumNArgs(1),
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

			_, priv, err := crypto.LoadKeyPair(config.KeysDir())
			if err != nil {
				return fmt.Errorf("loading keys: %w", err)
			}

			// Encrypt initial message to operator
			message := "Hey! I'd like to connect."
			if len(args) > 1 {
				message = strings.Join(args[1:], " ")
			}

			opKeyHex := pool.OperatorPubKey
			opPub, err := hex.DecodeString(opKeyHex)
			if err != nil {
				return fmt.Errorf("invalid operator key: %w", err)
			}

			encMsg, err := crypto.Encrypt(opPub, []byte(message))
			if err != nil {
				return fmt.Errorf("encrypting message: %w", err)
			}
			encMsgHex := hex.EncodeToString(encMsg)

			payload, err := json.Marshal(map[string]string{
				"action":   "like",
				"liker_id": cfg.User.PublicID,
				"liked_id": args[0],
			})
			if err != nil {
				return fmt.Errorf("marshaling payload: %w", err)
			}
			signature := crypto.Sign(priv, payload)

			client := poolClient(pool)
			issueNum, err := client.CreateLikeIssue(ctx, cfg.User.PublicID, args[0], encMsgHex, signature)
			if err != nil {
				return fmt.Errorf("sending like: %w", err)
			}

			printSuccess(fmt.Sprintf("Interest sent to %s (Issue #%d)", args[0], issueNum))
			return nil
		},
	}
}

func newInboxCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "inbox",
		Short: "View incoming interests",
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
			prs, err := client.ListIncomingLikes(ctx, cfg.User.PublicID)
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
			ctx := cmd.Context()

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			pool, err := requirePool(cfg)
			if err != nil {
				return nil
			}

			prNumber, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid PR number %q: %w", args[0], err)
			}

			client := poolClient(pool)
			if err := client.AcceptLike(ctx, prNumber); err != nil {
				return fmt.Errorf("accepting: %w", err)
			}

			printBrand("  It's a match!")
			printSuccess("Match created. Start chatting: dating chat <public_id>")
			return nil
		},
	}
}

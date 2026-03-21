package cli

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
)

func newLikeCmd() *cobra.Command {
	var greeting string

	cmd := &cobra.Command{
		Use:   "like <match_hash>",
		Short: "Express interest in someone from discovery",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			targetMatchHash := args[0]

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			pool := cfg.ActivePool()
			if pool == nil {
				printError("No active pool. Run: dating pool join <name>")
				return nil
			}
			if pool.MatchHash == "" || pool.BinHash == "" {
				printError("Not fully registered. Complete registration first.")
				return nil
			}

			// Resolve GitHub token
			ghToken, err := resolveGitHubTokenNonInteractive()
			if err != nil {
				printError("GitHub authentication required.")
				printDim("  Run: gh auth login")
				return nil
			}

			operatorPubBytes, err := hex.DecodeString(pool.OperatorPubKey)
			if err != nil {
				return fmt.Errorf("invalid operator key: %w", err)
			}

			if greeting == "" {
				greeting = "Hey! I'd like to connect."
			}

			client := poolClientWithToken(pool, ghToken)
			issueNumber, err := client.CreateInterestIssue(
				ctx,
				pool.BinHash,
				pool.MatchHash,
				targetMatchHash,
				greeting,
				ed25519.PublicKey(operatorPubBytes),
			)
			if err != nil {
				if strings.Contains(err.Error(), "422") {
					printDim("  Interest already sent to this person.")
					return nil
				}
				return fmt.Errorf("sending interest: %w", err)
			}

			printSuccess(fmt.Sprintf("Interest sent! (Issue #%d)", issueNumber))
			printDim("  If they like you back, you'll match automatically.")
			return nil
		},
	}

	cmd.Flags().StringVar(&greeting, "greeting", "", "greeting message (optional)")
	return cmd
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

			pool := cfg.ActivePool()
			if pool == nil {
				printError("No active pool.")
				return nil
			}
			if pool.MatchHash == "" {
				printError("Not fully registered.")
				return nil
			}

			ghToken, err := resolveGitHubTokenNonInteractive()
			if err != nil {
				printError("GitHub authentication required.")
				printDim("  Run: gh auth login")
				return nil
			}

			client := poolClientWithToken(pool, ghToken)
			issues, err := client.ListInterestsForMeIssues(ctx, pool.MatchHash)
			if err != nil {
				return fmt.Errorf("fetching inbox: %w", err)
			}

			if len(issues) == 0 {
				printDim("  No incoming interests yet.")
				return nil
			}

			fmt.Println()
			fmt.Printf("  %s  %d incoming interests\n\n",
				brand.Render("♥"),
				len(issues),
			)
			printDim("  Browse Discover to find your match — mutual likes create a match automatically.")
			fmt.Println()
			return nil
		},
	}
}

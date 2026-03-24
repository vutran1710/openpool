package cli

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
)

func newUnmatchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unmatch <match_hash>",
		Short: "Dissolve a match with someone",
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
				printError("No active pool.")
				return nil
			}
			if pool.MatchHash == "" {
				printError("Not registered.")
				return nil
			}

			ghToken, err := resolveGitHubTokenNonInteractive()
			if err != nil {
				printError("GitHub authentication required.")
				return nil
			}

			operatorPubBytes, err := hex.DecodeString(pool.OperatorPubKey)
			if err != nil {
				return fmt.Errorf("invalid operator key: %w", err)
			}

			client := poolClientWithToken(pool, ghToken)
			issueNumber, err := client.SubmitUnmatchIssue(
				ctx,
				pool.MatchHash,
				targetMatchHash,
				ed25519.PublicKey(operatorPubBytes),
			)
			if err != nil {
				if strings.Contains(err.Error(), "422") {
					printDim("  Unmatch already requested.")
					return nil
				}
				return fmt.Errorf("submitting unmatch: %w", err)
			}

			printSuccess(fmt.Sprintf("Unmatch submitted (Issue #%d)", issueNumber))
			printDim("  The match will be dissolved once the pool processes this request.")
			return nil
		},
	}
}

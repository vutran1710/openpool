package cli

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/crypto"
	gh "github.com/vutran1710/dating-dev/internal/github"
)

// Match represents a discovered match from a closed interest issue.
type Match struct {
	BinHash     string // counterpart's bin_hash for relay connection
	Greeting    string // counterpart's greeting message
	IssueNumber int    // the interest issue number
}

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
				printError("GitHub auth required. Run: gh auth login")
				return nil
			}

			_, priv, err := crypto.LoadKeyPair(config.KeysDir())
			if err != nil {
				printError("Keys not found.")
				return nil
			}

			operatorPubBytes, err := hex.DecodeString(pool.OperatorPubKey)
			if err != nil {
				return fmt.Errorf("invalid operator pubkey: %w", err)
			}
			operatorPub := ed25519.PublicKey(operatorPubBytes)

			client := poolClientWithToken(pool, ghToken)

			// Find closed issues with label=interest that have match comments
			issues, err := client.Client().ListIssues(ctx, "closed", "interest")
			if err != nil {
				return fmt.Errorf("fetching issues: %w", err)
			}

			var matches []Match
			for _, iss := range issues {
				signedBlob, err := gh.FindOperatorReplyInIssue(ctx, client.Client(), iss.Number, "match", operatorPub)
				if err != nil {
					continue
				}
				plaintext, err := crypto.DecryptSignedBlob(signedBlob, priv)
				if err != nil {
					continue
				}
				var data struct {
					MatchedBinHash string `json:"matched_bin_hash"`
					Greeting       string `json:"greeting"`
				}
				if err := json.Unmarshal(plaintext, &data); err != nil || data.MatchedBinHash == "" {
					continue
				}
				matches = append(matches, Match{
					BinHash:    data.MatchedBinHash,
					Greeting:   data.Greeting,
					IssueNumber: iss.Number,
				})
			}

			if len(matches) == 0 {
				printDim("  No matches yet. Keep discovering!")
				return nil
			}

			fmt.Println()
			fmt.Printf("  %s  %d matches\n\n", brand.Render("♥"), len(matches))
			for _, m := range matches {
				greeting := m.Greeting
				if len(greeting) > 50 {
					greeting = greeting[:47] + "..."
				}
				fmt.Printf("  %s  %s\n", bold.Render(m.BinHash), dim.Render("\""+greeting+"\""))
			}
			fmt.Println()
			printDim("  Chat: dating chat <bin_hash>")
			fmt.Println()
			return nil
		},
	}
}


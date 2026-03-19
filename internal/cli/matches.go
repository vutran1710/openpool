package cli

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/crypto"
)

// Match represents a discovered match from a closed interest PR.
type Match struct {
	BinHash  string // counterpart's bin_hash for relay connection
	Greeting string // counterpart's greeting message
	PRNumber int    // the interest PR number
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

			client := poolClientWithToken(pool, ghToken)

			// Find closed PRs with label=interest that have match comments
			prs, err := client.Client().ListPullRequests(ctx, "closed")
			if err != nil {
				return fmt.Errorf("fetching PRs: %w", err)
			}

			var matches []Match
			for _, pr := range prs {
				hasLabel := false
				for _, l := range pr.Labels {
					if l.Name == "interest" {
						hasLabel = true
						break
					}
				}
				if !hasLabel {
					continue
				}

				// Check for encrypted comment from Action
				comments, err := client.Client().ListIssueComments(ctx, pr.Number)
				if err != nil {
					continue
				}

				for _, c := range comments {
					if c.User.Login != "github-actions[bot]" {
						continue
					}
					m, err := decryptMatchComment(c.Body, priv)
					if err != nil {
						continue
					}
					m.PRNumber = pr.Number
					matches = append(matches, *m)
				}
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

func decryptMatchComment(body string, priv ed25519.PrivateKey) (*Match, error) {
	blobBytes, err := base64.StdEncoding.DecodeString(body)
	if err != nil {
		return nil, err
	}
	plaintext, err := crypto.Decrypt(priv, blobBytes)
	if err != nil {
		return nil, err
	}
	var data struct {
		MatchedBinHash string `json:"matched_bin_hash"`
		Greeting       string `json:"greeting"`
	}
	if err := json.Unmarshal(plaintext, &data); err != nil {
		return nil, err
	}
	if data.MatchedBinHash == "" {
		return nil, fmt.Errorf("missing matched_bin_hash")
	}
	return &Match{
		BinHash:  data.MatchedBinHash,
		Greeting: data.Greeting,
	}, nil
}

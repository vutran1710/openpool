package cli

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vutran1710/openpool/internal/cli/config"
	"github.com/vutran1710/openpool/internal/crypto"
)

func newFetchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "fetch",
		Short: "Discover new profiles via relay",
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

			if pool.RelayURL == "" {
				printError("This pool has no relay server configured.")
				return nil
			}

			pub, priv, err := crypto.LoadKeyPair(config.KeysDir())
			if err != nil {
				return fmt.Errorf("loading keys: %w", err)
			}

			// Sign the discovery request to prove identity
			message := []byte("discover:" + cfg.User.IDHash)
			signature := crypto.Sign(priv, message)

			reqBody, err := json.Marshal(map[string]string{
				"user_hash": cfg.User.IDHash,
				"pool_repo": pool.Repo,
				"pub_key":   hex.EncodeToString(pub),
				"signature": signature,
			})
			if err != nil {
				return fmt.Errorf("marshaling request: %w", err)
			}

			// Call relay discover endpoint
			relayURL := strings.TrimSuffix(pool.RelayURL, "/")
			// Convert ws(s):// to http(s)://
			relayURL = strings.Replace(relayURL, "wss://", "https://", 1)
			relayURL = strings.Replace(relayURL, "ws://", "http://", 1)

			req, err := http.NewRequestWithContext(ctx, "POST", relayURL+"/discover", bytes.NewReader(reqBody))
			if err != nil {
				return fmt.Errorf("creating request: %w", err)
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := ghHTTPClient.Do(req)
			if err != nil {
				return fmt.Errorf("contacting relay: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != 200 {
				body, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("relay error (%d): %s", resp.StatusCode, string(body))
			}

			var result struct {
				UserHash         string `json:"user_hash"`
				EncryptedProfile string `json:"encrypted_profile"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				return fmt.Errorf("parsing relay response: %w", err)
			}

			if result.UserHash == "" {
				printDim("  No profiles found. Check back later.")
				return nil
			}

			// Decrypt the profile re-encrypted for us
			encryptedBytes, err := hex.DecodeString(result.EncryptedProfile)
			if err != nil {
				return fmt.Errorf("decoding profile: %w", err)
			}

			plaintext, err := crypto.Decrypt(priv, encryptedBytes)
			if err != nil {
				return fmt.Errorf("decrypting profile: %w", err)
			}

			var profile map[string]any
			if err := json.Unmarshal(plaintext, &profile); err != nil {
				return fmt.Errorf("parsing profile: %w", err)
			}

			fmt.Println()
			fmt.Printf("  %s\n", bold.Render(crypto.ShortHash(result.UserHash)))
			if name, ok := profile["display_name"].(string); ok && name != "" {
				fmt.Printf("  %s %s\n", dim.Render("Name:"), name)
			}
			if bio, ok := profile["bio"].(string); ok && bio != "" {
				fmt.Printf("  %s %s\n", dim.Render("Bio:"), bio)
			}
			if city, ok := profile["city"].(string); ok && city != "" {
				fmt.Printf("  %s %s\n", dim.Render("City:"), city)
			}
			if interests, ok := profile["interests"].([]any); ok && len(interests) > 0 {
				strs := make([]string, len(interests))
				for i, v := range interests {
					strs[i] = fmt.Sprint(v)
				}
				fmt.Printf("  %s %s\n", dim.Render("Interests:"), strings.Join(strs, ", "))
			}
			fmt.Println()
			printDim("  Like: op like " + crypto.ShortHash(result.UserHash))
			fmt.Println()
			return nil
		},
	}
}

func newViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view <user_hash>",
		Short: "View a user's encrypted profile blob",
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

			client := poolClient(pool)
			blob, err := client.GetUserBlob(ctx, args[0])
			if err != nil {
				printError("User not found: " + args[0])
				return nil
			}

			fmt.Println()
			fmt.Printf("  %s  %s  %d bytes\n",
				bold.Render(crypto.ShortHash(args[0])),
				dim.Render("encrypted blob"),
				len(blob),
			)
			printDim("  Profile is encrypted to operator. Use `op fetch` for decrypted profiles.")
			fmt.Println()
			return nil
		},
	}
}

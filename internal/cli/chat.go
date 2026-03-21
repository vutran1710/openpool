package cli

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/chat"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	relayclient "github.com/vutran1710/dating-dev/internal/cli/relay"
	"github.com/vutran1710/dating-dev/internal/crypto"
)

func newChatCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "chat <match_hash>",
		Short: "Chat with a match via relay",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			pool := cfg.ActivePool()
			if pool == nil {
				printError("No active pool.")
				return nil
			}
			if pool.RelayURL == "" {
				printError("No relay URL configured for this pool.")
				return nil
			}
			if pool.MatchHash == "" {
				printError("Not registered. Complete registration first.")
				return nil
			}

			pub, priv, err := crypto.LoadKeyPair(config.KeysDir())
			if err != nil {
				return fmt.Errorf("loading keys: %w", err)
			}

			targetMatchHash := args[0]

			idHash := pool.IDHash
			if idHash == "" {
				// Fallback: compute from config fields
				idHash = string(crypto.UserHash(pool.Repo, cfg.User.Provider, cfg.User.ProviderUserID))
			}

			// Connect to relay
			client := relayclient.NewClient(relayclient.Config{
				RelayURL:  pool.RelayURL,
				PoolURL:   pool.Repo,
				IDHash:    idHash,
				MatchHash: pool.MatchHash,
				Pub:       pub,
				Priv:      priv,
			})

			// Load peer pubkey from conversations DB
			dbPath := filepath.Join(config.Dir(), "conversations.db")
			convoDB, dbErr := chat.OpenConversationDB(dbPath)
			if dbErr == nil {
				defer convoDB.Close()
				peerPub, keyErr := convoDB.GetPeerKey(targetMatchHash)
				if keyErr == nil && len(peerPub) == ed25519.PublicKeySize {
					client.SetPeerKey(targetMatchHash, ed25519.PublicKey(peerPub))
				}
			}
			// Fallback: PEER_PUB env var (for testing/scripting)
			if peerPubHex := os.Getenv("PEER_PUB"); peerPubHex != "" {
				peerPubBytes, err := hex.DecodeString(peerPubHex)
				if err == nil && len(peerPubBytes) == ed25519.PublicKeySize {
					client.SetPeerKey(targetMatchHash, ed25519.PublicKey(peerPubBytes))
				}
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			fmt.Println("  Connecting to relay...")
			if err := client.Connect(ctx); err != nil {
				return fmt.Errorf("connecting to relay: %w", err)
			}
			defer client.Close()

			printSuccess("Connected!")
			fmt.Println()
			printDim(fmt.Sprintf("  Chatting with %s", targetMatchHash[:12]+"..."))
			printDim("  Type a message and press Enter. /exit to leave.")
			fmt.Println()

			// Handle incoming messages
			client.OnMessage(func(senderMatchHash string, plaintext []byte) {
				timestamp := time.Now().Format("15:04")
				sender := senderMatchHash
				if len(sender) > 12 {
					sender = sender[:12] + "..."
				}
				fmt.Printf("\r  %s %s: %s\n", dim.Render("["+timestamp+"]"), brand.Render(sender), string(plaintext))
				fmt.Printf("%s ", dim.Render("you>"))
			})

			client.OnControl(func(msg string) {
				fmt.Printf("\r  %s %s\n", dim.Render("[relay]"), msg)
				fmt.Printf("%s ", dim.Render("you>"))
			})

			// Chat loop
			scanner := bufio.NewScanner(os.Stdin)
			for {
				fmt.Printf("%s ", dim.Render("you>"))
				if !scanner.Scan() {
					break
				}

				input := scanner.Text()
				if input == "/exit" {
					printDim("  Left chat.")
					return nil
				}
				if input == "" {
					continue
				}

				if err := client.SendMessage(targetMatchHash, input); err != nil {
					fmt.Printf("  %s\n", dim.Render("(send failed: "+err.Error()+")"))
					continue
				}

				timestamp := time.Now().Format("15:04")
				fmt.Printf("  %s %s: %s\n", dim.Render("["+timestamp+"]"), brand.Render("you"), input)
			}
			return nil
		},
	}
}

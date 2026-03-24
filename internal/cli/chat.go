package cli

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/vutran1710/openpool/internal/cli/chat"
	"github.com/vutran1710/openpool/internal/cli/config"
	relayclient "github.com/vutran1710/openpool/internal/cli/relay"
	"github.com/vutran1710/openpool/internal/crypto"
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
				idHash = string(crypto.UserHash(pool.Repo, cfg.User.Provider, cfg.User.ProviderUserID))
			}

			// Open conversations DB
			dbPath := filepath.Join(config.Dir(), "conversations.db")
			convoDB, err := chat.OpenConversationDB(dbPath)
			if err != nil {
				return fmt.Errorf("opening conversations db: %w", err)
			}
			defer convoDB.Close()

			// Create relay client
			relay := relayclient.NewClient(relayclient.Config{
				RelayURL:  pool.RelayURL,
				PoolURL:   pool.Repo,
				IDHash:    idHash,
				MatchHash: pool.MatchHash,
				Pub:       pub,
				Priv:      priv,
			})

			// Create ChatClient (relay + DB)
			chatClient := chat.NewChatClient(relay, convoDB)
			chatClient.LoadPeerKeys()

			// Fallback: PEER_PUB env var (for testing/scripting)
			if peerPubHex := os.Getenv("PEER_PUB"); peerPubHex != "" {
				peerPubBytes, err := hex.DecodeString(peerPubHex)
				if err == nil && len(peerPubBytes) == ed25519.PublicKeySize {
					chatClient.SetPeerKey(targetMatchHash, ed25519.PublicKey(peerPubBytes))
				}
			}

			// Connect
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			fmt.Println("  Connecting to relay...")
			if err := relay.Connect(ctx); err != nil {
				return fmt.Errorf("connecting to relay: %w", err)
			}
			defer relay.Close()

			// Mark read on entry
			chatClient.MarkRead(targetMatchHash)

			printSuccess("Connected!")
			fmt.Println()

			// Show chat history
			history, _ := chatClient.History(targetMatchHash)
			for _, m := range history {
				ts := m.CreatedAt.Format("15:04")
				if m.IsMe {
					fmt.Printf("  %s %s: %s\n", dim.Render("["+ts+"]"), brand.Render("you"), m.Body)
				} else {
					sender := targetMatchHash
					if len(sender) > 12 {
						sender = sender[:12] + "..."
					}
					fmt.Printf("  %s %s: %s\n", dim.Render("["+ts+"]"), brand.Render(sender), m.Body)
				}
			}

			printDim(fmt.Sprintf("  Chatting with %s", targetMatchHash[:12]+"..."))
			printDim("  Type a message and press Enter. /exit to leave.")
			fmt.Println()

			// Handle incoming via ChatClient (persists to DB + prints)
			chatClient.OnMsg = func(senderMatchHash string) {
				// Load latest from DB
				msgs, _ := chatClient.History(senderMatchHash)
				if len(msgs) > 0 {
					latest := msgs[len(msgs)-1]
					if !latest.IsMe {
						timestamp := latest.CreatedAt.Format("15:04")
						sender := senderMatchHash
						if len(sender) > 12 {
							sender = sender[:12] + "..."
						}
						fmt.Printf("\r  %s %s: %s\n", dim.Render("["+timestamp+"]"), brand.Render(sender), latest.Body)
						fmt.Printf("%s ", dim.Render("you>"))
					}
				}
			}

			relay.OnControl(func(msg string) {
				fmt.Printf("\r  %s %s\n", dim.Render("[relay]"), msg)
				fmt.Printf("%s ", dim.Render("you>"))
			})

			// Chat loop — send via ChatClient (persists to DB)
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

				if err := chatClient.Send(targetMatchHash, input); err != nil {
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

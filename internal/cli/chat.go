package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	relayclient "github.com/vutran1710/dating-dev/internal/cli/relay"
	"github.com/vutran1710/dating-dev/internal/crypto"
	"github.com/vutran1710/dating-dev/internal/protocol"
)

func newChatCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "chat <bin_hash>",
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
			if pool.BinHash == "" || pool.MatchHash == "" {
				printError("Not registered. Complete registration first.")
				return nil
			}

			pub, priv, err := crypto.LoadKeyPair(config.KeysDir())
			if err != nil {
				return fmt.Errorf("loading keys: %w", err)
			}

			targetBinHash := args[0]

			// Connect to relay
			client := relayclient.NewClient(relayclient.Config{
				RelayURL:  pool.RelayURL,
				PoolURL:   pool.Repo,
				BinHash:   pool.BinHash,
				MatchHash: pool.MatchHash,
				Pub:       pub,
				Priv:      priv,
			})

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			fmt.Println("  Connecting to relay...")
			if err := client.Connect(ctx); err != nil {
				return fmt.Errorf("connecting to relay: %w", err)
			}
			defer client.Close()

			printSuccess("Connected!")
			fmt.Println()
			printDim(fmt.Sprintf("  Chatting with %s", targetBinHash[:12]+"..."))
			printDim("  Type a message and press Enter. /exit to leave.")
			fmt.Println()

			// Handle incoming messages
			client.OnMessage(func(msg protocol.Message) {
				timestamp := time.Now().Format("15:04")
				sender := msg.SourceHash[:12] + "..."
				fmt.Printf("\r  %s %s: %s\n", dim.Render("["+timestamp+"]"), brand.Render(sender), msg.Body)
				fmt.Printf("%s ", dim.Render("you>"))
			})

			client.OnError(func(e protocol.Error) {
				fmt.Printf("\r  %s %s\n", dim.Render("[error]"), e.Message)
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

				if err := client.SendMessage(targetBinHash, input); err != nil {
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

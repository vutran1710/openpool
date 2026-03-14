package cli

import (
	"bufio"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/telegram"
)

func newChatCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "chat <public_id>",
		Short: "Chat with a match",
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

			if pool.BotToken == "" {
				printError("This pool has no chat configured.")
				return nil
			}

			bot := telegram.NewBot(pool.BotToken)
			targetID := args[0]

			// TODO: resolve chat ID from match metadata in the repo
			// For now, use a deterministic chat ID placeholder
			_ = bot
			_ = targetID

			fmt.Println()
			fmt.Printf("  %s\n", bold.Render("dating:"+targetID+">"))
			printDim("  Type a message and press Enter. /exit to leave.")
			fmt.Println()

			return runChatLoop(cfg.User.PublicID, targetID)
		},
	}
}

func runChatLoop(selfID, targetID string) error {
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Printf("%s ", dim.Render("dating:"+targetID+">"))
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

		timestamp := time.Now().Format("15:04")
		fmt.Printf("  %s %s: %s\n", dim.Render("["+timestamp+"]"), brand.Render("you"), input)
	}
	return nil
}

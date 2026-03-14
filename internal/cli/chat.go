package cli

import (
	"bufio"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
)

func newChatCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "chat <user_hash>",
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

			if pool.RelayURL == "" {
				printError("This pool has no relay server configured.")
				return nil
			}

			targetHash := args[0]

			fmt.Println()
			fmt.Printf("  %s\n", bold.Render("dating:"+targetHash[:8]+">"))
			printDim("  Type a message and press Enter. /exit to leave.")
			fmt.Println()

			return runChatLoop(targetHash)
		},
	}
}

func runChatLoop(targetHash string) error {
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Printf("%s ", dim.Render("dating:"+targetHash[:8]+">"))
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

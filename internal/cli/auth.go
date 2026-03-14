package cli

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/crypto"
)

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication commands",
	}
	cmd.AddCommand(newRegisterCmd(), newWhoamiCmd())
	return cmd
}

func newRegisterCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "register",
		Short: "Create a new identity",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			if cfg.IsRegistered() {
				printWarning("Already registered as " + cfg.User.DisplayName + " (" + cfg.User.PublicID + ")")
				return nil
			}

			printHeader()
			fmt.Println("  Let's create your identity.")
			fmt.Println()

			reader := bufio.NewReader(os.Stdin)
			name := prompt(reader, "  Display name: ")
			if name == "" {
				printError("Display name is required")
				return nil
			}

			fmt.Println()
			fmt.Println("  Generating your keys...")
			pub, _, err := crypto.GenerateKeyPair(config.KeysDir())
			if err != nil {
				return fmt.Errorf("generating keys: %w", err)
			}

			publicID := crypto.PublicIDFromKey(pub)

			cfg.User.PublicID = publicID
			cfg.User.DisplayName = name
			if err := cfg.Save(); err != nil {
				return err
			}

			printSuccess("Identity created")
			fmt.Printf("  %s  %s\n", bold.Render(name), dim.Render("("+publicID+")"))
			fmt.Printf("  %s  %s\n", dim.Render("public key:"), hex.EncodeToString(pub)[:16]+"...")
			fmt.Printf("  %s  %s\n", dim.Render("keys stored:"), config.KeysDir())
			fmt.Println()
			printDim("  Next: join a pool with  dating pool join <url>")
			fmt.Println()
			return nil
		},
	}
}

func newWhoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show current identity",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if !cfg.IsRegistered() {
				printWarning("Not registered. Run: dating auth register")
				return nil
			}

			pub, _, err := crypto.LoadKeyPair(config.KeysDir())
			if err != nil {
				return err
			}

			fmt.Println()
			fmt.Printf("  %s  %s\n", bold.Render(cfg.User.DisplayName), dim.Render("("+cfg.User.PublicID+")"))
			fmt.Printf("  %s  %s\n", dim.Render("public key:"), hex.EncodeToString(pub)[:16]+"...")

			if cfg.Active != "" {
				fmt.Printf("  %s  %s\n", dim.Render("active pool:"), cfg.Active)
			}
			pool := cfg.ActivePool()
			if pool != nil && pool.BotToken != "" {
				fmt.Printf("  %s  %s\n", dim.Render("chat:"), "telegram (via pool)")
			}
			fmt.Println()
			return nil
		},
	}
}

func prompt(reader *bufio.Reader, label string) string {
	fmt.Print(label)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

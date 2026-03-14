package cli

import (
	"encoding/hex"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/crypto"
)

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Identity commands",
	}
	cmd.AddCommand(newWhoamiCmd())
	return cmd
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
				printWarning("Not registered. Run: dating pool join <name>")
				return nil
			}

			pub, _, err := crypto.LoadKeyPair(config.KeysDir())
			if err != nil {
				return err
			}

			fmt.Println()
			fmt.Printf("  %s  %s\n", bold.Render(cfg.User.DisplayName), dim.Render("("+cfg.User.PublicID+")"))
			fmt.Printf("  %s  %s\n", dim.Render("provider:"), cfg.User.Provider)
			fmt.Printf("  %s  %s\n", dim.Render("public key:"), hex.EncodeToString(pub)[:16]+"...")

			if cfg.Active != "" {
				fmt.Printf("  %s  %s\n", dim.Render("active pool:"), cfg.Active)
			}
			pool := cfg.ActivePool()
			if pool != nil && pool.RelayURL != "" {
				fmt.Printf("  %s  %s\n", dim.Render("relay:"), pool.RelayURL)
			}
			fmt.Println()
			return nil
		},
	}
}

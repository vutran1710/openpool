package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
)

func newResetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reset",
		Short: "Reset all configuration and data",
		Long:  "Archives current data to ~/.dating/archive/ and clears all config, keys, and pool data.",
		RunE: func(cmd *cobra.Command, args []string) error {
			datingDir := config.Dir()

			// Check if there's anything to reset
			if _, err := os.Stat(datingDir); os.IsNotExist(err) {
				printDim("  Nothing to reset — no data found at " + datingDir)
				return nil
			}

			// Show what will be affected
			fmt.Println()
			printWarning("This will reset all dating.dev data:")
			fmt.Println()

			settingPath := config.Path()
			if _, err := os.Stat(settingPath); err == nil {
				cfg, _ := config.Load()
				if cfg != nil {
					if cfg.ActiveRegistry != "" {
						fmt.Printf("    Registry:  %s\n", cfg.ActiveRegistry)
					}
					if len(cfg.Pools) > 0 {
						fmt.Printf("    Pools:     %d joined\n", len(cfg.Pools))
					}
					if cfg.User.DisplayName != "" {
						fmt.Printf("    Identity:  %s (%s)\n", cfg.User.DisplayName, cfg.User.Provider)
					}
				}
			}

			keysDir := config.KeysDir()
			if _, err := os.Stat(keysDir); err == nil {
				fmt.Printf("    Keys:      %s\n", keysDir)
			}

			fmt.Println()
			printDim("  Your data will be archived before deletion.")
			fmt.Println()

			// Confirm
			reader := bufio.NewReader(os.Stdin)
			input := prompt(reader, "  Type \"reset\" to confirm: ")
			if strings.TrimSpace(input) != "reset" {
				printDim("  Cancelled.")
				return nil
			}

			// Archive
			timestamp := time.Now().Format("20060102-150405")
			archiveDir := filepath.Join(datingDir, "archive", timestamp)

			fmt.Println()
			if err := withSpinnerNoResult("Archiving current data", func() error {
				return archiveData(datingDir, archiveDir)
			}); err != nil {
				return err
			}

			// Clear config, keys, and cloned repos
			if err := withSpinnerNoResult("Clearing configuration", func() error {
				os.Remove(config.Path())
				os.RemoveAll(config.KeysDir())
				os.RemoveAll(filepath.Join(datingDir, "pools"))
				os.RemoveAll(filepath.Join(datingDir, "repos"))
				return nil
			}); err != nil {
				return err
			}

			fmt.Println()
			printSuccess("Reset complete")
			printDim(fmt.Sprintf("  Archive saved to: %s", archiveDir))
			printDim("  Run `dating` to start fresh.")
			fmt.Println()
			return nil
		},
	}
}

// archiveData copies setting.toml and keys/ into the archive directory.
func archiveData(datingDir, archiveDir string) error {
	if err := os.MkdirAll(archiveDir, 0700); err != nil {
		return fmt.Errorf("creating archive dir: %w", err)
	}

	// Archive setting.toml
	settingPath := filepath.Join(datingDir, "setting.toml")
	if data, err := os.ReadFile(settingPath); err == nil {
		os.WriteFile(filepath.Join(archiveDir, "setting.toml"), data, 0600)
	}

	// Archive keys/
	keysDir := filepath.Join(datingDir, "keys")
	if entries, err := os.ReadDir(keysDir); err == nil {
		archiveKeys := filepath.Join(archiveDir, "keys")
		os.MkdirAll(archiveKeys, 0700)
		for _, e := range entries {
			src := filepath.Join(keysDir, e.Name())
			dst := filepath.Join(archiveKeys, e.Name())
			if data, err := os.ReadFile(src); err == nil {
				os.WriteFile(dst, data, 0600)
			}
		}
	}

	// Archive pool operator keys if any
	poolsDir := filepath.Join(datingDir, "pools")
	if entries, err := os.ReadDir(poolsDir); err == nil {
		archivePools := filepath.Join(archiveDir, "pools")
		os.MkdirAll(archivePools, 0700)
		for _, e := range entries {
			if e.IsDir() {
				poolDir := filepath.Join(poolsDir, e.Name())
				archivePoolDir := filepath.Join(archivePools, e.Name())
				os.MkdirAll(archivePoolDir, 0700)
				if files, err := os.ReadDir(poolDir); err == nil {
					for _, f := range files {
						src := filepath.Join(poolDir, f.Name())
						dst := filepath.Join(archivePoolDir, f.Name())
						if data, err := os.ReadFile(src); err == nil {
							os.WriteFile(dst, data, 0600)
						}
					}
				}
			}
		}
	}

	return nil
}

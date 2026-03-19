package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/cli/suggestions"
	"github.com/vutran1710/dating-dev/internal/gitrepo"
)

func newPoolSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync <pool>",
		Short: "Sync pool repo and update local suggestions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			poolName := args[0]

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			pool := findPool(cfg, poolName)
			if pool == nil {
				printError("Pool not found: " + poolName)
				return nil
			}

			fmt.Println("  Syncing pool repo...")
			repo, err := gitrepo.Clone(gitrepo.EnsureGitURL(pool.Repo))
			if err != nil {
				return fmt.Errorf("cloning pool repo: %w", err)
			}
			if _, err := repo.Sync(); err != nil {
				return fmt.Errorf("syncing pool repo: %w", err)
			}

			packPath := filepath.Join(config.Dir(), "pools", poolName, "suggestions.pack")

			pack, err := suggestions.Load(packPath)
			if err != nil {
				return fmt.Errorf("loading suggestions: %w", err)
			}

			// Prefer index.pack (cron-built), fall back to index/ directory (.rec files)
			indexPackPath := filepath.Join(repo.LocalDir, "index.pack")
			if _, err := os.Stat(indexPackPath); err == nil {
				loaded, err := pack.SyncFromIndexPack(indexPackPath)
				if err != nil {
					return fmt.Errorf("loading index.pack: %w", err)
				}
				if err := pack.Save(packPath); err != nil {
					return fmt.Errorf("saving suggestions: %w", err)
				}
				printSuccess(fmt.Sprintf("Synced %d profiles from index.pack", loaded))
			} else {
				indexDir := filepath.Join(repo.LocalDir, "index")
				added, err := pack.SyncFromRecDir(indexDir)
				if err != nil {
					return fmt.Errorf("syncing vectors: %w", err)
				}
				if err := pack.Save(packPath); err != nil {
					return fmt.Errorf("saving suggestions: %w", err)
				}
				printSuccess(fmt.Sprintf("Synced %d new vectors (total %d)", added, len(pack.Records)))
			}
			return nil
		},
	}
}

func findPool(cfg *config.Config, name string) *config.PoolConfig {
	for i := range cfg.Pools {
		if cfg.Pools[i].Name == name {
			return &cfg.Pools[i]
		}
	}
	return nil
}

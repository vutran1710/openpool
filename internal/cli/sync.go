package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	gh "github.com/vutran1710/dating-dev/internal/github"
	"github.com/vutran1710/dating-dev/internal/gitrepo"
	"github.com/vutran1710/dating-dev/internal/pooldb"
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

			poolDBPath := filepath.Join(config.Dir(), "pool.db")
			pdb, err := pooldb.Open(poolDBPath)
			if err != nil {
				return fmt.Errorf("opening pool db: %w", err)
			}
			defer pdb.Close()

			// Download index.db from release asset
			indexPath := filepath.Join(config.Dir(), "pools", poolName, "index.db")
			if dlErr := gh.DownloadReleaseAsset(pool.Repo, "index-latest", "index.db", indexPath); dlErr == nil {
				synced, err := pdb.SyncFromIndex(indexPath)
				if err != nil {
					return fmt.Errorf("syncing from index.db: %w", err)
				}
				printSuccess(fmt.Sprintf("Synced %d profiles from index.db", synced))
			} else {
				printError("Failed to download index.db: " + dlErr.Error())
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

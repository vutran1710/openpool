package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/cli/suggestions"
	"github.com/vutran1710/dating-dev/internal/gitrepo"
)

func newPoolSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync <pool>",
		Short: "Sync pool repo and update local suggestions DB",
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

			indexDir := filepath.Join(repo.LocalDir, "index")
			dbPath := filepath.Join(config.Dir(), "pools", poolName, "suggestions.db")

			db, err := suggestions.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening suggestions DB: %w", err)
			}
			defer db.Close()

			added, err := db.SyncFromDir(indexDir)
			if err != nil {
				return fmt.Errorf("syncing vectors: %w", err)
			}

			records, _ := db.LoadAll()
			printSuccess(fmt.Sprintf("Synced %d new vectors (total %d)", added, len(records)))
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

package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/cli/suggestions"
	"github.com/vutran1710/dating-dev/internal/gitrepo"
)

func newDiscoverCmd() *cobra.Command {
	var (
		limit   int
		doSync  bool
	)

	cmd := &cobra.Command{
		Use:   "discover <pool>",
		Short: "Show profile suggestions for a pool",
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

			if pool.MatchHash == "" {
				printError("No match_hash for this pool. Complete registration first.")
				printDim("  Run: dating pool join " + poolName)
				return nil
			}

			// Sync if requested
			if doSync {
				fmt.Println("  Syncing...")
				repo, err := gitrepo.Clone(gitrepo.EnsureGitURL(pool.Repo))
				if err != nil {
					return fmt.Errorf("cloning: %w", err)
				}
				if _, err := repo.Sync(); err != nil {
					return fmt.Errorf("syncing: %w", err)
				}

				indexDir := filepath.Join(repo.LocalDir, "index")
				dbPath := filepath.Join(config.Dir(), "pools", poolName, "suggestions.db")
				db, err := suggestions.Open(dbPath)
				if err != nil {
					return fmt.Errorf("opening DB: %w", err)
				}
				added, _ := db.SyncFromDir(indexDir)
				db.Close()
				if added > 0 {
					printDim(fmt.Sprintf("  Synced %d new vectors", added))
				}
			}

			// Open DB
			dbPath := filepath.Join(config.Dir(), "pools", poolName, "suggestions.db")
			db, err := suggestions.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening suggestions DB: %w", err)
			}
			defer db.Close()

			records, err := db.LoadAll()
			if err != nil {
				return fmt.Errorf("loading vectors: %w", err)
			}

			if len(records) == 0 {
				printDim("  No vectors in suggestions DB. Run: dating pool sync " + poolName)
				return nil
			}

			// Find own vector
			var myVec []float32
			for _, r := range records {
				if r.MatchHash == pool.MatchHash {
					myVec = r.Vector
					break
				}
			}
			if myVec == nil {
				printError("Your vector not found in suggestions DB. Run: dating pool sync " + poolName)
				return nil
			}

			// Rank
			ranked := suggestions.RankSuggestions(myVec, pool.MatchHash, records, limit)

			if len(ranked) == 0 {
				printDim("  No suggestions found.")
				return nil
			}

			fmt.Println()
			fmt.Printf("  %s (%d users)\n\n", bold.Render("Suggestions for "+poolName), len(records))
			for i, s := range ranked {
				fmt.Printf("  %2d. %s  score: %.2f\n", i+1, s.MatchHash, s.Score)
			}
			fmt.Println()
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "max suggestions to show")
	cmd.Flags().BoolVar(&doSync, "sync", false, "sync pool repo before discovering")
	return cmd
}

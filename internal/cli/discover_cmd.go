package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/cli/suggestions"
	gh "github.com/vutran1710/dating-dev/internal/github"
	"github.com/vutran1710/dating-dev/internal/gitrepo"
)

func newDiscoverCmd() *cobra.Command {
	var (
		limit  int
		doSync bool
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

			packPath := filepath.Join(config.Dir(), "pools", poolName, "suggestions.pack")

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
				pack, err := suggestions.Load(packPath)
				if err != nil {
					return fmt.Errorf("loading: %w", err)
				}
				added, _ := pack.SyncFromRecDir(indexDir)
				if added > 0 {
					pack.Save(packPath)
					printDim(fmt.Sprintf("  Synced %d new vectors", added))
				}
			}

			// Load pack
			pack, err := suggestions.Load(packPath)
			if err != nil {
				return fmt.Errorf("loading suggestions: %w", err)
			}

			if len(pack.Records) == 0 {
				printDim("  No vectors. Run: dating pool sync " + poolName)
				return nil
			}

			// Find own record
			me := pack.Find(pool.MatchHash)
			if me == nil {
				printError("Your vector not found. Run: dating pool sync " + poolName)
				return nil
			}

			// Load pool schema for filter-based ranking
			var schema *gh.PoolSchema
			repo, repoErr := gitrepo.Clone(gitrepo.EnsureGitURL(pool.Repo))
			if repoErr == nil {
				manifest, mErr := loadManifest(repo.LocalDir)
				if mErr == nil && manifest.Schema != nil {
					schema = manifest.Schema
				}
			}

			ranked := suggestions.RankSuggestions(schema, *me, pack.Records, limit)

			if len(ranked) == 0 {
				printDim("  No suggestions found.")
				return nil
			}

			fmt.Println()
			fmt.Printf("  %s (%d users)\n\n", bold.Render("Suggestions for "+poolName), len(pack.Records))
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

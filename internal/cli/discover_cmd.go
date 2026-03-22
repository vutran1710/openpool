package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/cli/suggestions"
	gh "github.com/vutran1710/dating-dev/internal/github"
	"github.com/vutran1710/dating-dev/internal/gitrepo"
	"github.com/vutran1710/dating-dev/internal/pooldb"
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

			poolDBPath := filepath.Join(config.Dir(), "pool.db")
			pdb, err := pooldb.Open(poolDBPath)
			if err != nil {
				return fmt.Errorf("opening pool db: %w", err)
			}
			defer pdb.Close()

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

				indexPath := filepath.Join(config.Dir(), "pools", poolName, "index.db")
				if dlErr := gh.DownloadReleaseAsset(pool.Repo, "index-latest", "index.db", indexPath); dlErr == nil {
					synced, _ := pdb.SyncFromIndex(indexPath)
					printDim(fmt.Sprintf("  Synced %d profiles", synced))
				}
			}

			// Load profiles from PoolDB
			profiles, err := pdb.ListProfiles()
			if err != nil {
				return fmt.Errorf("listing profiles: %w", err)
			}

			if len(profiles) == 0 {
				printDim("  No profiles. Run: dating pool sync " + poolName)
				return nil
			}

			records := make([]suggestions.Record, len(profiles))
			for i, p := range profiles {
				records[i] = suggestions.ProfileToRecord(p)
			}

			// Find own record
			var me *suggestions.Record
			for i := range records {
				if records[i].MatchHash == pool.MatchHash {
					me = &records[i]
					break
				}
			}
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

			seen, _ := pdb.GetSeen()
			ranked := suggestions.RankSuggestions(schema, *me, records, seen, limit)

			// Mark shown as seen
			for _, s := range ranked {
				pdb.MarkSeen(s.MatchHash, "view")
			}

			if len(ranked) == 0 {
				printDim("  No suggestions found.")
				return nil
			}

			total := len(records) - 1 // exclude self
			filtered := total - len(ranked)

			fmt.Println()
			fmt.Printf("  %s (%d users, %d filtered out)\n\n", bold.Render("Suggestions for "+poolName), total, filtered)
			for i, s := range ranked {
				fmt.Printf("  %2d. %s  score: %.2f\n", i+1, s.MatchHash, s.Score)
			}
			fmt.Println()
			printDim("  Like someone: dating like <match_hash>")
			fmt.Println()
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "max suggestions to show")
	cmd.Flags().BoolVar(&doSync, "sync", false, "sync pool repo before discovering")
	return cmd
}

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/vutran1710/openpool/internal/cli/config"
	"github.com/vutran1710/openpool/internal/gitclient"
	poolschema "github.com/vutran1710/openpool/internal/schema"
)

func newProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage your profile",
	}
	cmd.AddCommand(newProfileCreateCmd(), newProfileEditCmd(), newProfileShowCmd())
	return cmd
}

func newProfileCreateCmd() *cobra.Command {
	var profilePath string
	var poolRepo string

	cmd := &cobra.Command{
		Use:   "create <pool>",
		Short: "Create a local profile for a pool",
		Long: `Creates a profile.json at ~/.openpool/pools/<pool>/profile.json.

Profile values sourced from (in priority order):
  1. --profile flag: copy from the given JSON file
  2. Existing pool profile: duplicate from another pool's profile.json
  3. Base template: scaffold with user info from config, dating fields empty

Edit the generated file, then run: op pool join <pool>`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			poolName := args[0]

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			outPath := config.PoolProfilePath(poolName)

			// Source 1: explicit --profile flag
			if profilePath != "" {
				data, err := os.ReadFile(profilePath)
				if err != nil {
					return fmt.Errorf("reading profile: %w", err)
				}
				// Validate it's valid JSON
				var check map[string]any
				if err := json.Unmarshal(data, &check); err != nil {
					return fmt.Errorf("invalid profile JSON: %w", err)
				}
				if err := writeProfileFile(outPath, data); err != nil {
					return err
				}
				printSuccess(fmt.Sprintf("Profile created from %s", profilePath))
				printProfileHint(poolName, outPath)
				return nil
			}

			// Source 2: duplicate from existing pool profile
			for _, p := range cfg.Pools {
				if p.Name == poolName {
					continue
				}
				existing := config.PoolProfilePath(p.Name)
				data, err := os.ReadFile(existing)
				if err != nil {
					continue
				}
				if err := writeProfileFile(outPath, data); err != nil {
					return err
				}
				printSuccess(fmt.Sprintf("Profile created (duplicated from pool \"%s\")", p.Name))
				printProfileHint(poolName, outPath)
				return nil
			}

			// Source 3: scaffold from pool schema (pool.yaml)
			repoURL := poolRepo
			if repoURL == "" {
				pool := findPool(cfg, poolName)
				if pool != nil {
					repoURL = pool.Repo
				}
			}
			profile := map[string]any{
				"display_name": cfg.User.DisplayName,
			}
			if repoURL != "" {
				repo, err := gitclient.Clone(gitclient.EnsureGitURL(repoURL))
				if err == nil {
					repo.Sync()
					schemaPath := filepath.Join(repo.LocalDir, "pool.yaml")
					if s, sErr := poolschema.Load(schemaPath); sErr == nil {
						for name, attr := range s.Profile {
							switch attr.Type {
							case "enum", "text":
								profile[name] = ""
							case "multi":
								profile[name] = []string{}
							case "range":
								val := 0
								if attr.Min != nil {
									val = *attr.Min
								}
								profile[name] = val
							}
						}
					}
				}
			}

			data, err := json.MarshalIndent(profile, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling template: %w", err)
			}
			if err := writeProfileFile(outPath, data); err != nil {
				return err
			}

			printSuccess("Profile template created")
			printProfileHint(poolName, outPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&profilePath, "profile", "", "path to existing profile JSON to copy")
	cmd.Flags().StringVar(&poolRepo, "pool-repo", "", "pool repo (owner/name) to fetch schema from")
	return cmd
}

func newProfileEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit <pool>",
		Short: "Show where to edit your profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			poolName := args[0]
			path := config.PoolProfilePath(poolName)

			if _, err := os.Stat(path); os.IsNotExist(err) {
				printError(fmt.Sprintf("No profile found for pool \"%s\"", poolName))
				printDim(fmt.Sprintf("  Create one first: op profile create %s", poolName))
				return nil
			}

			fmt.Println()
			printDim(fmt.Sprintf("  Your profile is at: %s", path))
			printDim("  Edit it with your preferred editor, then run:")
			fmt.Printf("    op pool join %s\n", poolName)
			fmt.Println()
			return nil
		},
	}
}

func newProfileShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <pool>",
		Short: "Display your local profile for a pool",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			poolName := args[0]
			path := config.PoolProfilePath(poolName)

			data, err := os.ReadFile(path)
			if err != nil {
				if os.IsNotExist(err) {
					printDim(fmt.Sprintf("  No profile found for pool \"%s\".", poolName))
					printDim(fmt.Sprintf("  Create one: op profile create %s", poolName))
					return nil
				}
				return fmt.Errorf("reading profile: %w", err)
			}

			var profile map[string]any
			if err := json.Unmarshal(data, &profile); err != nil {
				return fmt.Errorf("parsing profile: %w", err)
			}

			fmt.Println()
			for k, v := range profile {
				fmt.Printf("  %s: %v\n", k, v)
			}
			fmt.Println()
			return nil
		},
	}
}

func writeProfileFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

func printProfileHint(poolName, path string) {
	fmt.Println()
	printDim(fmt.Sprintf("  Profile saved to: %s", path))
	printDim("  Edit it with your preferred editor, then run:")
	fmt.Printf("    op pool join %s\n", poolName)
	fmt.Println()
}

// Ensure imports are used
var _ = gitclient.Clone

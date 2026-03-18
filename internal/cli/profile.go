package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/cli/tui/components"
	gh "github.com/vutran1710/dating-dev/internal/github"
	"github.com/vutran1710/dating-dev/internal/gitrepo"
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

	cmd := &cobra.Command{
		Use:   "create <pool>",
		Short: "Create a local profile for a pool",
		Long: `Creates a profile.json at ~/.dating/pools/<pool>/profile.json.

Profile values sourced from (in priority order):
  1. --profile flag: copy from the given JSON file
  2. Existing pool profile: duplicate from another pool's profile.json
  3. Base template: scaffold with user info from config, dating fields empty

Edit the generated file, then run: dating pool join <pool>`,
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

			// Source 3: scaffold from pool schema
			pool := findPool(cfg, poolName)
			var schema *gh.PoolSchema
			if pool != nil && pool.Repo != "" {
				repo, err := gitrepo.Clone(gitrepo.EnsureGitURL(pool.Repo))
				if err == nil {
					repo.Sync()
					manifest, err := loadManifest(repo.LocalDir)
					if err == nil && manifest.Schema != nil {
						schema = manifest.Schema
					}
				}
			}

			profile := scaffoldProfile(cfg, schema)
			data, err := json.MarshalIndent(profile, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling template: %w", err)
			}
			if err := writeProfileFile(outPath, data); err != nil {
				return err
			}

			// Write schema reference file for editor help
			if schema != nil {
				schemaRef := buildSchemaRef(schema)
				schemaData, _ := json.MarshalIndent(schemaRef, "", "  ")
				writeProfileFile(config.PoolProfilePath(poolName)+".schema.json", schemaData)
			}

			printSuccess("Profile template created")
			printProfileHint(poolName, outPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&profilePath, "profile", "", "path to existing profile JSON to copy")
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
				printDim(fmt.Sprintf("  Create one first: dating profile create %s", poolName))
				return nil
			}

			fmt.Println()
			printDim(fmt.Sprintf("  Your profile is at: %s", path))
			printDim("  Edit it with your preferred editor, then run:")
			fmt.Printf("    dating pool join %s\n", poolName)
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
					printDim(fmt.Sprintf("  Create one: dating profile create %s", poolName))
					return nil
				}
				return fmt.Errorf("reading profile: %w", err)
			}

			var profile gh.DatingProfile
			if err := json.Unmarshal(data, &profile); err != nil {
				return fmt.Errorf("parsing profile: %w", err)
			}

			fmt.Println()
			fmt.Println(components.RenderProfile(profile, 60, components.ProfileNormal))
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
	fmt.Printf("    dating pool join %s\n", poolName)
	fmt.Println()
}

// loadManifest reads pool.json from a local repo directory.
func loadManifest(repoDir string) (*gh.PoolManifest, error) {
	data, err := os.ReadFile(filepath.Join(repoDir, "pool.json"))
	if err != nil {
		return nil, err
	}
	var manifest gh.PoolManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

// scaffoldProfile creates a profile template from the pool schema.
// If no schema, falls back to basic fields.
func scaffoldProfile(cfg *config.Config, schema *gh.PoolSchema) map[string]any {
	profile := map[string]any{
		"display_name": cfg.User.DisplayName,
	}

	if schema == nil {
		// Fallback: basic fields
		profile["bio"] = ""
		profile["interests"] = []string{}
		profile["intent"] = []string{}
		return profile
	}

	// Scaffold from schema fields
	for _, field := range schema.Fields {
		switch field.Type {
		case "enum":
			profile[field.Name] = ""
		case "multi":
			profile[field.Name] = []string{}
		case "range":
			profile[field.Name] = 0
		}
	}
	return profile
}

// buildSchemaRef creates a reference file showing valid values for each field.
func buildSchemaRef(schema *gh.PoolSchema) map[string]any {
	ref := make(map[string]any)
	for _, field := range schema.Fields {
		entry := map[string]any{"type": field.Type}
		if len(field.Values) > 0 {
			entry["values"] = field.Values
		}
		if field.Match != "" {
			entry["match"] = field.Match
		}
		if field.Min != nil {
			entry["min"] = *field.Min
		}
		if field.Max != nil {
			entry["max"] = *field.Max
		}
		ref[field.Name] = entry
	}
	return ref
}

// Ensure gitrepo import is used
var _ = gitrepo.Clone

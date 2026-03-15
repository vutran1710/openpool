package cli

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
)

// parseRegistryInput normalizes registry input to a git-cloneable URL.
// Accepts:
//   - owner/repo                          → https://github.com/owner/repo.git (GitHub shorthand)
//   - https://github.com/owner/repo      → https://github.com/owner/repo.git
//   - https://gitlab.com/owner/repo.git  → as-is
//   - git@github.com:owner/repo.git      → as-is
func parseRegistryInput(input string) (string, error) {
	input = strings.TrimSpace(input)

	// Already a full URL or SSH — return as-is
	if strings.HasPrefix(input, "https://") || strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "git@") {
		return input, nil
	}

	// GitHub shorthand: owner/repo
	parts := strings.Split(input, "/")
	if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
		return "https://github.com/" + input + ".git", nil
	}

	return "", fmt.Errorf("invalid registry format: expected owner/repo or full git URL, got %q", input)
}

// validateRegistry checks that the git repo exists and is accessible.
func validateRegistry(repoURL string) error {
	cmd := exec.Command("git", "ls-remote", "--exit-code", repoURL)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cannot access registry: %s", repoURL)
	}
	return nil
}

func newRegistryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "registry",
		Short: "Manage pool registries",
	}

	cmd.AddCommand(
		newRegistryAddCmd(),
		newRegistryRemoveCmd(),
		newRegistryListCmd(),
		newRegistrySwitchCmd(),
	)
	return cmd
}

func newRegistryAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <owner/repo>",
		Short: "Add a pool registry",
		Long: `Add a pool registry by GitHub repo (e.g. vutran1710/official-dating-registry).

Discover available registries at https://dating.dev/pools`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := parseRegistryInput(args[0])
			if err != nil {
				printError(err.Error())
				return nil
			}

			fmt.Printf("  Validating registry: %s ...\n", repo)
			if err := validateRegistry(repo); err != nil {
				printError(err.Error())
				return nil
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			cfg.AddRegistry(repo)
			if cfg.ActiveRegistry == "" {
				cfg.ActiveRegistry = repo
			}
			if err := cfg.Save(); err != nil {
				return err
			}

			printSuccess("Added registry: " + repo)
			if cfg.ActiveRegistry == repo {
				printDim("  Set as active registry")
			}
			return nil
		},
	}
}

func newRegistryRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <owner/repo>",
		Short: "Remove a pool registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			if !cfg.RemoveRegistry(args[0]) {
				printWarning("Registry not found: " + args[0])
				return nil
			}

			if err := cfg.Save(); err != nil {
				return err
			}
			printSuccess("Removed registry: " + args[0])
			return nil
		},
	}
}

func newRegistryListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured registries",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			if len(cfg.Registries) == 0 {
				printDim("  No registries configured.")
				printDim("  Add one with: dating registry add <owner/repo>")
				fmt.Println()
				printDim("  Discover registries at: https://dating.dev/pools")
				return nil
			}

			fmt.Println()
			for _, r := range cfg.Registries {
				marker := "  "
				if r == cfg.ActiveRegistry {
					marker = brand.Render("* ")
				}
				fmt.Printf("  %s%s\n", marker, r)
			}
			fmt.Println()
			return nil
		},
	}
}

func newRegistrySwitchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "switch <owner/repo>",
		Short: "Set active registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			found := false
			for _, r := range cfg.Registries {
				if r == args[0] {
					found = true
					break
				}
			}
			if !found {
				printWarning("Registry not found: " + args[0])
				printDim("  Add it first: dating registry add " + args[0])
				return nil
			}

			cfg.ActiveRegistry = args[0]
			if err := cfg.Save(); err != nil {
				return err
			}
			printSuccess("Active registry: " + args[0])
			return nil
		},
	}
}

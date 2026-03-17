package cli

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/crypto"
	gh "github.com/vutran1710/dating-dev/internal/github"
	"github.com/vutran1710/dating-dev/internal/gitrepo"
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
			return runRegistryAdd(args[0])
		},
	}
}

func runRegistryAdd(input string) error {
	ctx := context.Background()
	reader := bufio.NewReader(os.Stdin)

	// Step 1: Parse & clone registry
	fmt.Println("\n  Step 1: Clone registry")

	repoURL, err := parseRegistryInput(input)
	if err != nil {
		printError(err.Error())
		return nil
	}

	regRepo, err := withSpinner("Validating & cloning "+repoURL, func() (*gitrepo.Repo, error) {
		return gitrepo.CloneRegistry(repoURL)
	})
	if err != nil {
		return nil
	}

	// Step 2: Read pools from local clone
	fmt.Println("\n  Step 2: Fetch pools")

	reg := gh.NewLocalRegistry(regRepo)
	pools, err := withSpinner("Reading pool list", func() ([]gh.PoolEntry, error) {
		return reg.ListPools()
	})
	if err != nil {
		return nil
	}

	if len(pools) == 0 {
		printDim("  No pools found in this registry")
	} else {
		fmt.Printf("  Found %d pool(s):\n", len(pools))
		for _, p := range pools {
			desc := p.Description
			if desc == "" {
				desc = "no description"
			}
			fmt.Printf("    %s  %s\n", bold.Render(p.Name), dim.Render(desc))
			fmt.Printf("      %s\n", dim.Render(p.Repo))
		}
	}

	// Step 3: Resolve identity & keypair
	fmt.Println("\n  Step 3: Identity & keys")

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	var pub ed25519.PublicKey
	var identity *GitHubIdentity

	// Check for existing keypair
	existingPub, _, loadErr := crypto.LoadKeyPair(config.KeysDir())
	if loadErr == nil {
		pub = existingPub
		printSuccess(fmt.Sprintf("Local keypair found: %s", hex.EncodeToString(pub[:4])))
	} else {
		printDim("  No local keypair found")
		choice := prompt(reader, "  Generate a new keypair? (y/n): ")
		if choice == "y" || choice == "Y" {
			newPub, _, err := crypto.GenerateKeyPair(config.KeysDir())
			if err != nil {
				return fmt.Errorf("generating keys: %w", err)
			}
			pub = newPub
			printSuccess(fmt.Sprintf("Keypair generated: %s", hex.EncodeToString(pub[:4])))
		} else {
			printDim("  Skipping key generation")
		}
	}

	// Get GitHub identity for checking registration
	if len(pools) > 0 && pub != nil {
		fmt.Println("\n  Step 4: Check pool registrations")

		ghToken, err := resolveGitHubToken(func(label string) string {
			return prompt(reader, label)
		})
		if err != nil {
			printDim("  Skipping registration check (no GitHub token)")
		} else {
			identity, _ = withSpinner("Fetching GitHub identity", func() (*GitHubIdentity, error) {
				return fetchGitHubIdentity(ctx, ghToken)
			})
			if identity == nil {
				printDim("  Skipping registration check")
			}
		}
	}

	// Check registration status for each pool (clone pool repos)
	if identity != nil && len(pools) > 0 {
		fmt.Println()
		for _, p := range pools {
			userHash := crypto.UserHash(p.Repo, "github", identity.UserID)

			// Clone pool repo to check registration
			poolURL := gitrepo.EnsureGitURL(p.Repo)
			poolRepo, err := withSpinner(fmt.Sprintf("Cloning %s", p.Name), func() (*gitrepo.Repo, error) {
				return gitrepo.Clone(poolURL)
			})
			if err != nil {
				continue
			}

			pool := gh.NewLocalPool(poolRepo)
			registered := pool.IsUserRegistered(ctx, userHash.String())

			if registered {
				printSuccess(fmt.Sprintf("  %s — registered ✓", p.Name))
				cfg.AddPool(config.PoolConfig{
					Name:           p.Name,
					Repo:           p.Repo,
					OperatorPubKey: p.OperatorPubKey,
					RelayURL:       p.RelayURL,
					Status:         "active",
				})
				if cfg.Active == "" {
					cfg.Active = p.Name
				}
			} else {
				printDim(fmt.Sprintf("  %s — not registered  → dating pool join %s", p.Name, p.Name))
			}
		}

		// Save identity to config
		cfg.User.Provider = "github"
		cfg.User.ProviderUserID = identity.UserID
		cfg.User.DisplayName = identity.DisplayName
		if pub != nil {
			cfg.User.PublicID = crypto.UserHash(pools[0].Repo, "github", identity.UserID).String()
		}
	}

	// Save registry to config
	fmt.Println()

	cfg.AddRegistry(repoURL)
	if cfg.ActiveRegistry == "" {
		cfg.ActiveRegistry = repoURL
	}
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	printSuccess("Registry added: " + repoURL)
	if cfg.ActiveRegistry == repoURL {
		printDim("  Set as active registry")
	}
	fmt.Println()
	return nil
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

package cli

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/crypto"
	gh "github.com/vutran1710/dating-dev/internal/github"
)

func parseCSV(s string) []string {
	var result []string
	for _, item := range strings.Split(s, ",") {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// requireRegistry returns the active registry, prompting the user to add one if none configured.
func requireRegistry(cfg *config.Config) (string, error) {
	if cfg != nil && cfg.ActiveRegistry != "" {
		return cfg.ActiveRegistry, nil
	}

	fmt.Println("  No registry configured.")
	printDim("  Discover registries at: https://dating.dev/pools")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	input := prompt(reader, "  Enter registry (owner/repo or git URL): ")
	if input == "" {
		return "", fmt.Errorf("no registry provided")
	}

	if err := runRegistryAdd(input); err != nil {
		return "", err
	}

	// Reload config after registration
	newCfg, err := config.Load()
	if err != nil {
		return "", err
	}
	*cfg = *newCfg

	if cfg.ActiveRegistry == "" {
		return "", fmt.Errorf("registry setup incomplete")
	}
	return cfg.ActiveRegistry, nil
}

func newPoolCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pool",
		Short: "Manage dating pools",
	}

	cmd.AddCommand(
		newPoolCreateCmd(),
		newPoolJoinCmd(),
		newPoolLeaveCmd(),
		newPoolListCmd(),
		newPoolSwitchCmd(),
		newPoolBrowseCmd(),
	)
	return cmd
}

func newPoolCreateCmd() *cobra.Command {
	var (
		repo        string
		ghToken     string
		description string
		relayURL    string
		regToken    string
	)

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create and register a new pool",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			name := args[0]

			if repo == "" || ghToken == "" {
				printError("Required: --repo and --gh-token")
				return nil
			}
			if regToken == "" {
				printError("Required: --registry-token (PAT for the registry repo)")
				return nil
			}

			fmt.Println("  Generating operator key pair...")
			pub, _, err := crypto.GenerateKeyPair(filepath.Join(config.Dir(), "pools", name))
			if err != nil {
				return fmt.Errorf("generating operator keys: %w", err)
			}
			operatorPubHex := hex.EncodeToString(pub)

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			registryRepo, err := requireRegistry(cfg)
			if err != nil {
				printError(err.Error())
				return nil
			}
			reg := gh.NewRegistry(registryRepo, regToken)

			templateBody, err := fillPRTemplate(ctx, reg.Client(), "register-pool")
			if err != nil {
				return err
			}

			entry := gh.PoolEntry{
				Name:           name,
				Repo:           repo,
				Description:    description,
				OperatorPubKey: operatorPubHex,
				RelayURL:       relayURL,
				CreatedAt:      time.Now().UTC().Format(time.RFC3339),
			}
			tokens := gh.PoolTokens{
				GHToken: ghToken,
			}

			prNumber, err := reg.RegisterPool(ctx, entry, tokens, templateBody)
			if err != nil {
				return fmt.Errorf("registering pool: %w", err)
			}

			printSuccess(fmt.Sprintf("Pool \"%s\" PR #%d created — pending registry maintainer approval", name, prNumber))
			printDim(fmt.Sprintf("  Operator keys saved to: %s", filepath.Join(config.Dir(), "pools", name)))
			printDim(fmt.Sprintf("  Operator public key: %s...", operatorPubHex[:16]))
			if relayURL != "" {
				printDim(fmt.Sprintf("  Relay: %s", relayURL))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&repo, "repo", "", "GitHub repo for the pool (owner/name)")
	cmd.Flags().StringVar(&ghToken, "gh-token", "", "Fine-grained PAT for the pool repo")
	cmd.Flags().StringVar(&description, "desc", "", "Pool description")
	cmd.Flags().StringVar(&relayURL, "relay-url", "", "WebSocket relay server URL")
	cmd.Flags().StringVar(&regToken, "registry-token", "", "PAT for the registry repo")
	return cmd
}

func newPoolBrowseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "browse",
		Short: "Browse available pools from the active registry",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			registryRepo, err := requireRegistry(cfg)
			if err != nil {
				printError(err.Error())
				return nil
			}
			reg, err := gh.CloneRegistry(registryRepo)
			if err != nil {
				return fmt.Errorf("loading registry: %w", err)
			}
			pools, err := reg.ListPools()
			if err != nil {
				return fmt.Errorf("browsing pools: %w", err)
			}

			if len(pools) == 0 {
				printDim("  No pools registered yet.")
				return nil
			}

			fmt.Println()
			fmt.Printf("  %s\n\n", bold.Render("Available Pools"))
			for _, p := range pools {
				desc := p.Description
				if desc == "" {
					desc = "no description"
				}
				fmt.Printf("  %s  %s\n", brand.Render(p.Name), dim.Render(desc))
				fmt.Printf("     %s %s\n", dim.Render("repo:"), p.Repo)
			}
			fmt.Println()
			printDim("  Join with: dating pool join <name>")
			fmt.Println()
			return nil
		},
	}

	return cmd
}

func newPoolJoinCmd() *cobra.Command {
	var (
		profileFlag string
		noWait      bool
	)

	cmd := &cobra.Command{
		Use:   "join <name>",
		Short: "Submit registration to a pool",
		Long: `Reads your local profile, encrypts it, and submits a registration issue.
Then polls for the Action's response to receive your relay hashes.

Prerequisites:
  - Registry configured (dating registry add <repo>)
  - Keys generated (dating auth)
  - GitHub authenticated (gh auth login)
  - Profile created (dating profile create <pool>)`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			name := args[0]

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			// Prerequisite: registry
			if cfg.ActiveRegistry == "" {
				printError("No registry configured.")
				printDim("  Run: dating registry add <repo>")
				return nil
			}

			reg, err := gh.CloneRegistry(cfg.ActiveRegistry)
			if err != nil {
				return fmt.Errorf("loading registry: %w", err)
			}
			if !reg.IsPoolRegistered(name) {
				printError("Pool not found or not yet approved: " + name)
				return nil
			}

			entry, err := reg.GetPoolEntry(name)
			if err != nil {
				printError("Pool not found: " + name)
				return nil
			}

			// Prerequisite: GitHub auth (no interactive fallback)
			ghToken, err := resolveGitHubTokenNonInteractive()
			if err != nil {
				printError("GitHub authentication required.")
				printDim("  Run: gh auth login")
				return nil
			}

			identity, err := fetchGitHubIdentity(ctx, ghToken)
			if err != nil {
				return fmt.Errorf("fetching GitHub identity: %w", err)
			}

			// Prerequisite: keys
			pub, priv, err := crypto.LoadKeyPair(config.KeysDir())
			if err != nil {
				printError("Keys not found.")
				printDim("  Run: dating auth")
				return nil
			}

			// Resolve profile
			profilePath := profileFlag
			if profilePath == "" {
				profilePath = config.PoolProfilePath(name)
			}
			plaintext, err := os.ReadFile(profilePath)
			if err != nil {
				if os.IsNotExist(err) {
					printError(fmt.Sprintf("Profile not found at: %s", profilePath))
					printDim(fmt.Sprintf("  Create one: dating profile create %s", name))
					return nil
				}
				return fmt.Errorf("reading profile: %w", err)
			}
			// Validate JSON
			var check map[string]any
			if err := json.Unmarshal(plaintext, &check); err != nil {
				return fmt.Errorf("invalid profile JSON at %s: %w", profilePath, err)
			}

			// Encrypt profile to operator's pubkey
			operatorPubBytes, err := hex.DecodeString(entry.OperatorPubKey)
			if err != nil {
				return fmt.Errorf("decoding operator public key: %w", err)
			}
			bin, err := crypto.PackUserBin(pub, operatorPubBytes, plaintext)
			if err != nil {
				return fmt.Errorf("packing profile: %w", err)
			}

			// Compute identity proof + signature
			userHash := crypto.UserHash(entry.Repo, "github", identity.UserID)
			identityProof, err := crypto.EncryptIdentityProof(
				entry.OperatorPubKey,
				"github",
				identity.UserID,
			)
			if err != nil {
				return fmt.Errorf("encrypting identity proof: %w", err)
			}

			payload, err := json.Marshal(map[string]string{
				"action":    "register",
				"user_hash": userHash.String(),
			})
			if err != nil {
				return fmt.Errorf("marshaling payload: %w", err)
			}
			signature := crypto.Sign(priv, payload)

			// Submit registration issue
			poolGH := gh.NewPool(entry.Repo, identity.Token)
			pubKeyHex := hex.EncodeToString(pub)
			issueNumber, err := poolGH.RegisterUserViaIssue(ctx, userHash.String(), bin, pubKeyHex, signature, identityProof)
			if err != nil {
				return fmt.Errorf("submitting registration: %w", err)
			}

			cfg.User.IDHash = userHash.String()
			cfg.User.DisplayName = identity.DisplayName
			cfg.User.Provider = "github"
			cfg.User.ProviderUserID = identity.UserID

			fmt.Println()
			printSuccess(fmt.Sprintf("Registration issue #%d created", issueNumber))
			printDim(fmt.Sprintf("  https://github.com/%s/issues/%d", entry.Repo, issueNumber))

			if noWait {
				pool := config.PoolConfig{
					Name:           entry.Name,
					Repo:           entry.Repo,
					OperatorPubKey: entry.OperatorPubKey,
					RelayURL:       entry.RelayURL,
					Status:         gh.PoolStatusPending,
					PendingIssue:   issueNumber,
				}
				cfg.AddPool(pool)
				if cfg.Active == "" {
					cfg.Active = pool.Name
				}
				if err := cfg.Save(); err != nil {
					return err
				}
				printDim("  Skipping poll (--no-wait). Run: dating pool list — it will auto-check for hashes.")
				return nil
			}

			fmt.Println("  Waiting for GitHub Action to process registration...")

			pollCtx, pollCancel := context.WithTimeout(ctx, 5*time.Minute)
			defer pollCancel()

			binHash, matchHash, err := poolGH.PollRegistrationResult(pollCtx, issueNumber, priv)
			if err != nil {
				printWarning("Could not retrieve hashes: " + err.Error())
				printDim("  Run pool join again to retry polling.")

				pool := config.PoolConfig{
					Name:           entry.Name,
					Repo:           entry.Repo,
					OperatorPubKey: entry.OperatorPubKey,
					RelayURL:       entry.RelayURL,
					Status:         gh.PoolStatusPending,
					PendingIssue:   issueNumber,
				}
				cfg.AddPool(pool)
				if cfg.Active == "" {
					cfg.Active = pool.Name
				}
				return cfg.Save()
			}

			pool := config.PoolConfig{
				Name:           entry.Name,
				Repo:           entry.Repo,
				OperatorPubKey: entry.OperatorPubKey,
				RelayURL:       entry.RelayURL,
				Status:         gh.PoolStatusActive,
				BinHash:        binHash,
				MatchHash:      matchHash,
			}
			cfg.AddPool(pool)
			if cfg.Active == "" {
				cfg.Active = pool.Name
			}
			if err := cfg.Save(); err != nil {
				return err
			}

			printSuccess("Registration complete! Hashes received and saved.")
			printDim(fmt.Sprintf("  bin_hash:   %s", binHash))
			printDim(fmt.Sprintf("  match_hash: %s", matchHash))
			fmt.Println()
			return nil
		},
	}

	cmd.Flags().StringVar(&profileFlag, "profile", "", "path to profile JSON (default: ~/.dating/pools/<pool>/profile.json)")
	cmd.Flags().BoolVar(&noWait, "no-wait", false, "submit issue without polling for hashes")
	return cmd
}

func newPoolLeaveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "leave <name>",
		Short: "Leave a dating pool",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			if !cfg.RemovePool(args[0]) {
				printWarning("Pool not found: " + args[0])
				return nil
			}

			if err := cfg.Save(); err != nil {
				return err
			}
			printSuccess("Left \"" + args[0] + "\"")
			return nil
		},
	}
}

func newPoolListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List joined pools",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			if len(cfg.Pools) == 0 {
				printDim("  No pools joined. Try: dating pool browse")
				return nil
			}

			// Try to load keys for hash polling (best-effort)
			_, priv, keyErr := crypto.LoadKeyPair(config.KeysDir())

			updated := false
			fmt.Println()
			for i, p := range cfg.Pools {
				if p.Status == gh.PoolStatusPending {
					// Try polling for encrypted hashes if we have a pending issue
					if p.PendingIssue > 0 && p.BinHash == "" && keyErr == nil {
						ghToken, tokenErr := resolveGitHubTokenNonInteractive()
						if tokenErr == nil {
							poolGH := gh.NewPool(p.Repo, ghToken)
							pollCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
							binHash, matchHash, pollErr := poolGH.PollRegistrationResult(pollCtx, p.PendingIssue, priv)
							cancel()
							if pollErr == nil {
								cfg.Pools[i].BinHash = binHash
								cfg.Pools[i].MatchHash = matchHash
								cfg.Pools[i].Status = gh.PoolStatusActive
								updated = true
								p = cfg.Pools[i]
								printSuccess(fmt.Sprintf("  %s: hashes received!", p.Name))
							}
						}
					}

					// Fallback: check if .bin file exists (old flow)
					if p.Status == gh.PoolStatusPending {
						pool := gh.NewPool(p.Repo, "")
						if pool.IsUserRegistered(ctx, cfg.User.IDHash) {
							cfg.Pools[i].Status = gh.PoolStatusActive
							updated = true
							p = cfg.Pools[i]
						}
					}
				}

				marker := "  "
				if p.Name == cfg.Active {
					marker = brand.Render("* ")
				}
				status := dim.Render("  [" + poolDisplayStatus(p.Status) + "]")
				chat := ""
				if p.RelayURL != "" {
					chat = dim.Render("  [relay]")
				}
				fmt.Printf("  %s%s%s%s\n", marker, p.Name, status, chat)
			}
			fmt.Println()

			if updated {
				if err := cfg.Save(); err != nil {
					return fmt.Errorf("saving config: %w", err)
				}
			}
			return nil
		},
	}
}

func poolDisplayStatus(status string) string {
	if status == "" || status == gh.PoolStatusActive {
		return "active"
	}
	return status
}

func newPoolSwitchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "switch <name>",
		Short: "Set active pool",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			found := false
			for _, p := range cfg.Pools {
				if p.Name == args[0] {
					found = true
					break
				}
			}
			if !found {
				printWarning("Pool not found: " + args[0])
				return nil
			}

			cfg.Active = args[0]
			if err := cfg.Save(); err != nil {
				return err
			}
			printSuccess("Active pool: " + args[0])
			return nil
		},
	}
}

package cli

import (
	"encoding/hex"
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/crypto"
	gh "github.com/vutran1710/dating-dev/internal/github"
)

const defaultRegistry = "vutran1710/dating-pool-registry"

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
		registry    string
		regToken    string
	)

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create and register a new pool",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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

			reg := gh.NewRegistry(registry, regToken)

			templateBody, err := fillPRTemplate(reg.Client(), "register-pool")
			if err != nil {
				return err
			}

			entry := gh.PoolEntry{
				Name:           name,
				Repo:           repo,
				Description:    description,
				OperatorPubKey: operatorPubHex,
				CreatedAt:      time.Now().UTC().Format(time.RFC3339),
			}
			tokens := gh.PoolTokens{
				GHToken: ghToken,
			}

			prNumber, err := reg.RegisterPool(entry, tokens, templateBody)
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
	cmd.Flags().StringVar(&registry, "registry", defaultRegistry, "Registry repo")
	cmd.Flags().StringVar(&regToken, "registry-token", "", "PAT for the registry repo")
	return cmd
}

func newPoolBrowseCmd() *cobra.Command {
	var registry string

	cmd := &cobra.Command{
		Use:   "browse",
		Short: "Browse available pools from a registry",
		RunE: func(cmd *cobra.Command, args []string) error {
			reg := gh.NewPublicRegistry(registry)
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

	cmd.Flags().StringVar(&registry, "registry", defaultRegistry, "Registry repo to browse")
	return cmd
}

func newPoolJoinCmd() *cobra.Command {
	var registry string

	cmd := &cobra.Command{
		Use:   "join <name>",
		Short: "Join a pool from the registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if !cfg.IsRegistered() {
				printWarning("Register first: dating auth register")
				return nil
			}

			name := args[0]
			reg := gh.NewPublicRegistry(registry)

			if !reg.IsPoolRegistered(name) {
				printError("Pool not found or not yet approved: " + name)
				return nil
			}

			entry, err := reg.GetPoolEntry(name)
			if err != nil {
				printError("Pool not found: " + name)
				return nil
			}

			tokens, err := reg.GetPoolTokens(name)
			if err != nil {
				return fmt.Errorf("fetching pool tokens: %w", err)
			}

			poolClient := gh.NewClient(entry.Repo, tokens.GHToken)
			templateBody, err := fillPRTemplate(poolClient, "join")
			if err != nil {
				return err
			}
			_ = templateBody

			pool := config.PoolConfig{
				Name:           entry.Name,
				Repo:           entry.Repo,
				Token:          tokens.GHToken,
				OperatorPubKey: entry.OperatorPubKey,
				Status:         gh.PoolStatusPending,
			}

			cfg.AddPool(pool)
			if cfg.Active == "" {
				cfg.Active = pool.Name
			}
			if err := cfg.Save(); err != nil {
				return err
			}

			printSuccess(fmt.Sprintf("Joined \"%s\" — status: pending (awaiting pool operator approval)", pool.Name))
			if cfg.Active == pool.Name {
				printDim("  Set as active pool")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&registry, "registry", defaultRegistry, "Registry repo")
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
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			if len(cfg.Pools) == 0 {
				printDim("  No pools joined. Try: dating pool browse")
				return nil
			}

			updated := false
			fmt.Println()
			for i, p := range cfg.Pools {
				if p.Status == gh.PoolStatusPending {
					pool := gh.NewPool(p.Repo, p.Token)
					if pool.IsUserRegistered(cfg.User.PublicID) {
						cfg.Pools[i].Status = gh.PoolStatusActive
						updated = true
						p.Status = gh.PoolStatusActive
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
				cfg.Save()
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

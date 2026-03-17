package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/crypto"
)

func newCommitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "commit",
		Short: "Relationship proposals",
	}

	cmd.AddCommand(
		newProposeCmd(),
		newProposalsCmd(),
		newAcceptProposeCmd(),
		newStatusCmd(),
	)
	return cmd
}

func newProposeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "propose <public_id>",
		Short: "Propose a relationship to a match",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			pool, err := requirePool(cfg)
			if err != nil {
				return nil
			}

			_, priv, err := crypto.LoadKeyPair(config.KeysDir())
			if err != nil {
				return fmt.Errorf("loading keys: %w", err)
			}

			payload, err := json.Marshal(map[string]string{
				"action":   "propose",
				"proposer": cfg.User.IDHash,
				"target":   args[0],
			})
			if err != nil {
				return fmt.Errorf("marshaling payload: %w", err)
			}
			signature := crypto.Sign(priv, payload)

			client := poolClient(pool)
			prNumber, err := client.CreateProposePR(ctx, cfg.User.IDHash, args[0], signature)
			if err != nil {
				return fmt.Errorf("proposing: %w", err)
			}

			printBrand("  Proposal sent!")
			printSuccess(fmt.Sprintf("PR #%d created — waiting for %s to accept", prNumber, args[0]))
			return nil
		},
	}
}

func newProposalsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "proposals",
		Short: "View incoming proposals",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			pool, err := requirePool(cfg)
			if err != nil {
				return nil
			}

			client := poolClient(pool)
			prs, err := client.ListIncomingProposals(ctx, cfg.User.IDHash)
			if err != nil {
				return fmt.Errorf("fetching proposals: %w", err)
			}

			if len(prs) == 0 {
				printDim("  No incoming proposals.")
				return nil
			}

			fmt.Println()
			fmt.Printf("  %s\n\n", bold.Render("Incoming Proposals"))
			for _, pr := range prs {
				fmt.Printf("  %s  PR #%d  %s\n",
					brand.Render("♥"),
					pr.Number,
					pr.Title,
				)
			}
			fmt.Println()
			printDim("  Accept with: dating commit accept <pr_number>")
			fmt.Println()
			return nil
		},
	}
}

func newAcceptProposeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "accept <pr_number>",
		Short: "Accept a relationship proposal",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			pool, err := requirePool(cfg)
			if err != nil {
				return nil
			}

			prNumber, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid PR number %q: %w", args[0], err)
			}

			client := poolClient(pool)
			if err := client.AcceptPropose(ctx, prNumber); err != nil {
				return fmt.Errorf("accepting proposal: %w", err)
			}

			printBrand("  Committed! ♥")
			printSuccess("Relationship created")
			return nil
		},
	}
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check relationship status",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			pool, err := requirePool(cfg)
			if err != nil {
				return nil
			}

			client := poolClient(pool)
			rels, err := client.ListRelationships(ctx)
			if err != nil {
				return fmt.Errorf("checking status: %w", err)
			}

			if len(rels) == 0 {
				fmt.Println()
				fmt.Printf("  %s %s\n", dim.Render("status:"), "single")
				fmt.Println()
				return nil
			}

			fmt.Println()
			fmt.Printf("  %s\n\n", bold.Render("Relationships"))
			for _, r := range rels {
				fmt.Printf("  %s  %s\n", brand.Render("♥"), r)
			}
			fmt.Println()
			return nil
		},
	}
}

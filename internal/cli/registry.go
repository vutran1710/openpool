package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
)

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
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			repo := args[0]
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
				printDim("  No registries configured. Try: dating registry add <owner/repo>")
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

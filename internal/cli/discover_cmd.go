package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newDiscoverCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "discover",
		Short: "Discover profiles (migrating to chain encryption engine)",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Discovery is being migrated to the new chain encryption engine.")
			fmt.Println("Use the TUI for now: dating → Discover")
			return nil
		},
	}
}

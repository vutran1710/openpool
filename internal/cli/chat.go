package cli

import (
	"github.com/spf13/cobra"
)

func newChatCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "chat [public_id]",
		Short: "Chat with a match",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: implement with Supabase Realtime
			printWarning("Chat coming soon — will use Supabase Realtime")
			return nil
		},
	}
}

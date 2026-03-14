package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
)

func newProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage your profile",
	}

	cmd.AddCommand(newProfileEditCmd())
	cmd.AddCommand(newProfileSyncCmd())
	return cmd
}

func newProfileEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Edit your profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if !cfg.IsLoggedIn() {
				printWarning("Not logged in. Run: dating auth login")
				return nil
			}

			reader := bufio.NewReader(os.Stdin)
			fmt.Println()
			fmt.Println(bold.Render("  Edit your profile"))
			fmt.Println()

			bio := prompt(reader, "  Bio: ")
			city := prompt(reader, "  City: ")
			interests := prompt(reader, "  Interests (comma-separated): ")
			lookingFor := prompt(reader, "  Looking for (dating/friends/networking/any): ")

			interestList := []string{}
			for _, i := range strings.Split(interests, ",") {
				trimmed := strings.TrimSpace(i)
				if trimmed != "" {
					interestList = append(interestList, trimmed)
				}
			}

			client := NewAPIClient(cfg)
			_, err = client.Put("/api/profile", map[string]any{
				"bio":         bio,
				"city":        city,
				"interests":   interestList,
				"looking_for": lookingFor,
			})
			if err != nil {
				return err
			}

			printSuccess("Profile updated")
			return nil
		},
	}
}

func newProfileSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Sync profile to GitHub",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: implement GitHub sync
			printWarning("GitHub sync coming soon")
			return nil
		},
	}
}

func prompt(reader *bufio.Reader, label string) string {
	fmt.Print(label)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

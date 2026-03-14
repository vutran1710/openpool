package cli

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/crypto"
	"github.com/vutran1710/dating-dev/internal/github"
)

func newProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage your profile",
	}
	cmd.AddCommand(newProfileEditCmd(), newProfileShowCmd())
	return cmd
}

func newProfileEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Edit and publish your profile to the active pool",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			pool, err := requirePool(cfg)
			if err != nil {
				return nil
			}

			pub, priv, err := crypto.LoadKeyPair(config.KeysDir())
			if err != nil {
				return fmt.Errorf("loading keys: %w", err)
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

			profile := github.UserProfile{
				PublicID:    cfg.User.PublicID,
				DisplayName: cfg.User.DisplayName,
				Bio:         bio,
				City:        city,
				Interests:   interestList,
				LookingFor:  lookingFor,
				PublicKey:   hex.EncodeToString(pub),
				Status:      "open",
				JoinedAt:    time.Now().UTC().Format(time.RFC3339),
			}

			payload, _ := json.Marshal(profile)
			signature := crypto.Sign(priv, payload)

			client := poolClient(pool)
			if err := client.RegisterProfile(profile, signature); err != nil {
				return fmt.Errorf("publishing profile: %w", err)
			}

			printSuccess("Profile published to " + pool.Name)
			return nil
		},
	}
}

func newProfileShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show your current profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			pool, err := requirePool(cfg)
			if err != nil {
				return nil
			}

			client := poolClient(pool)
			profile, err := client.GetProfile(cfg.User.PublicID)
			if err != nil {
				printDim("  No profile found in this pool. Run: dating profile edit")
				return nil
			}

			renderProfile(*profile)
			return nil
		},
	}
}

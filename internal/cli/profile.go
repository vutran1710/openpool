package cli

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/crypto"
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

			name := prompt(reader, "  Display name: ")
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

			profileData := map[string]any{
				"display_name": name,
				"bio":          bio,
				"city":         city,
				"interests":    interestList,
				"looking_for":  lookingFor,
				"public_key":   hex.EncodeToString(pub),
				"status":       "open",
			}

			plaintext, _ := json.Marshal(profileData)
			bin, err := crypto.PackUserBin(pub, plaintext)
			if err != nil {
				return fmt.Errorf("packing profile: %w", err)
			}

			userHash := crypto.UserHash(pool.Secret, cfg.User.Provider, cfg.User.ProviderUserID)

			payload, _ := json.Marshal(map[string]string{
				"action":    "register",
				"user_hash": userHash,
			})
			signature := crypto.Sign(priv, payload)

			client := poolClient(pool)
			templateBody, err := fillPRTemplate(client.Client(), "join")
			if err != nil {
				return err
			}

			prNumber, err := client.RegisterUser(userHash, bin, signature, templateBody)
			if err != nil {
				return fmt.Errorf("publishing profile: %w", err)
			}

			cfg.User.PublicID = userHash[:12]
			cfg.Save()

			printSuccess(fmt.Sprintf("Profile PR #%d created — pending pool operator approval", prNumber))
			return nil
		},
	}
}

func newProfileShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show your current profile (decrypted locally)",
		RunE: func(cmd *cobra.Command, args []string) error {
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

			userHash := crypto.UserHash(pool.Secret, cfg.User.Provider, cfg.User.ProviderUserID)
			client := poolClient(pool)
			bin, err := client.GetUserBlob(userHash)
			if err != nil {
				printDim("  No profile found in this pool. Run: dating profile edit")
				return nil
			}

			plaintext, err := crypto.UnpackUserBin(priv, bin)
			if err != nil {
				return fmt.Errorf("decrypting profile: %w", err)
			}

			var profile map[string]any
			json.Unmarshal(plaintext, &profile)

			fmt.Println()
			for k, v := range profile {
				fmt.Printf("  %s  %v\n", dim.Render(k+":"), v)
			}
			fmt.Println()
			return nil
		},
	}
}

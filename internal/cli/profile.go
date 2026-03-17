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
	"github.com/vutran1710/dating-dev/internal/cli/tui/components"
	"github.com/vutran1710/dating-dev/internal/crypto"
	gh "github.com/vutran1710/dating-dev/internal/github"
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
			ctx := cmd.Context()

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

			plaintext, err := json.Marshal(profileData)
			if err != nil {
				return fmt.Errorf("marshaling profile: %w", err)
			}
			operatorPubBytes, err := hex.DecodeString(pool.OperatorPubKey)
			if err != nil {
				return fmt.Errorf("decoding operator public key: %w", err)
			}
			bin, err := crypto.PackUserBin(pub, operatorPubBytes, plaintext)
			if err != nil {
				return fmt.Errorf("packing profile: %w", err)
			}

			userHash := crypto.UserHash(pool.Repo, cfg.User.Provider, cfg.User.ProviderUserID)

			identityProof, err := crypto.EncryptIdentityProof(
				pool.OperatorPubKey,
				cfg.User.Provider,
				cfg.User.ProviderUserID,
			)
			if err != nil {
				return fmt.Errorf("encrypting identity proof: %w", err)
			}

			payload, err := json.Marshal(map[string]string{
				"action":    "register",
				"user_hash": userHash,
			})
			if err != nil {
				return fmt.Errorf("marshaling payload: %w", err)
			}
			signature := crypto.Sign(priv, payload)

			client := poolClient(pool)
			templateBody, err := fillPRTemplate(ctx, client.Client(), "join")
			if err != nil {
				return err
			}

			prNumber, err := client.RegisterUser(ctx, userHash, bin, signature, identityProof, templateBody)
			if err != nil {
				return fmt.Errorf("publishing profile: %w", err)
			}

			cfg.User.PublicID = userHash
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			printSuccess(fmt.Sprintf("Profile PR #%d created — pending pool operator approval", prNumber))
			return nil
		},
	}
}

func newProfileShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show your dating profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Try local profile first
			data, err := os.ReadFile(config.ProfilePath())
			if err != nil {
				printDim("  No profile found. Join a pool to create one.")
				return nil
			}

			var profile gh.DatingProfile
			if err := json.Unmarshal(data, &profile); err != nil {
				return fmt.Errorf("parsing profile: %w", err)
			}

			fmt.Println()
			fmt.Println(components.RenderProfile(profile, 60, components.ProfileNormal))
			fmt.Println()
			return nil
		},
	}
}

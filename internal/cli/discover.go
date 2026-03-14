package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/github"
)

func newFetchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "fetch",
		Short: "Discover new profiles",
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
			profile, err := client.DiscoverRandom(cfg.User.PublicID)
			if err != nil {
				return fmt.Errorf("discovering profiles: %w", err)
			}

			if profile == nil {
				printDim("  No profiles found. Check back later.")
				return nil
			}

			renderProfile(*profile)
			return nil
		},
	}
}

func newViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view <public_id>",
		Short: "View a user's profile",
		Args:  cobra.ExactArgs(1),
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
			profile, err := client.GetProfile(args[0])
			if err != nil {
				printError("Profile not found: " + args[0])
				return nil
			}

			renderProfile(*profile)
			return nil
		},
	}
}

func renderProfile(p github.UserProfile) {
	interests := strings.Join(p.Interests, ", ")
	if interests == "" {
		interests = "none listed"
	}

	lookingFor := p.LookingFor
	if lookingFor == "" {
		lookingFor = "not specified"
	}

	content := fmt.Sprintf(
		"%s  %s  %s\n\n%s\n\n%s %s\n%s %s",
		bold.Render(p.PublicID),
		dim.Render(p.City),
		dim.Render(p.Status),
		p.Bio,
		dim.Render("interests:"),
		interests,
		dim.Render("looking for:"),
		lookingFor,
	)

	fmt.Println()
	fmt.Println(profileBox.Render(content))
	fmt.Println()
}

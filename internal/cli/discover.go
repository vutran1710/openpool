package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/pkg/api"
	"github.com/vutran1710/dating-dev/pkg/models"
)

func newFetchCmd() *cobra.Command {
	var city, interest string

	cmd := &cobra.Command{
		Use:   "fetch",
		Short: "Discover new profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if !cfg.IsLoggedIn() {
				printWarning("Not logged in. Run: dating auth login")
				return nil
			}

			client := NewAPIClient(cfg)
			path := "/api/discover?limit=1"
			if city != "" {
				path += "&city=" + city
			}
			if interest != "" {
				path += "&interest=" + interest
			}

			resp, err := client.Get(path)
			if err != nil {
				return err
			}

			result, err := DecodeResponse[api.DiscoverResponse](resp)
			if err != nil {
				return err
			}

			if len(result.Profiles) == 0 {
				printDim("  No profiles found. Try different filters or check back later.")
				return nil
			}

			renderProfile(result.Profiles[0])
			return nil
		},
	}

	cmd.Flags().StringVar(&city, "city", "", "Filter by city")
	cmd.Flags().StringVar(&interest, "interest", "", "Filter by interest")
	return cmd
}

func newViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view [public_id]",
		Short: "View a user's profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if !cfg.IsLoggedIn() {
				printWarning("Not logged in. Run: dating auth login")
				return nil
			}

			client := NewAPIClient(cfg)
			resp, err := client.Get("/api/profiles/" + args[0])
			if err != nil {
				return err
			}

			profile, err := DecodeResponse[models.ProfileIndex](resp)
			if err != nil {
				return err
			}

			renderProfile(*profile)
			return nil
		},
	}
}

func renderProfile(p models.ProfileIndex) {
	interests := strings.Join(p.Interests, ", ")
	if interests == "" {
		interests = "none listed"
	}

	lookingFor := p.LookingFor
	if lookingFor == "" {
		lookingFor = "not specified"
	}

	content := fmt.Sprintf(
		"%s  %s\n\n%s\n\n%s %s\n%s %s\n%s %s",
		bold.Render(p.PublicID),
		dim.Render(p.City),
		p.Bio,
		dim.Render("interests:"),
		interests,
		dim.Render("looking for:"),
		lookingFor,
		dim.Render(""),
		dim.Render("[l]ike  [s]kip  [v]iew more"),
	)

	fmt.Println()
	fmt.Println(profileBox.Render(content))
	fmt.Println()
}

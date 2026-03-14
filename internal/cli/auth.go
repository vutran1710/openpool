package cli

import (
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/pkg/api"
)

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication commands",
	}

	cmd.AddCommand(
		newRegisterCmd(),
		newLoginCmd(),
		newLogoutCmd(),
		newWhoamiCmd(),
	)

	return cmd
}

func newRegisterCmd() *cobra.Command {
	var provider string

	cmd := &cobra.Command{
		Use:   "register",
		Short: "Create a new account",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			if cfg.IsLoggedIn() {
				printWarning("Already logged in as " + cfg.User.DisplayName)
				return nil
			}

			printHeader()
			fmt.Println("  Creating your account...")
			fmt.Println()
			return doOAuthFlow(cfg, provider)
		},
	}

	cmd.Flags().StringVarP(&provider, "provider", "p", "github", "Auth provider (github or google)")
	return cmd
}

func newLoginCmd() *cobra.Command {
	var provider string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Sign in to an existing account",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			if cfg.IsLoggedIn() {
				printWarning("Already logged in as " + cfg.User.DisplayName)
				return nil
			}

			printHeader()
			return doOAuthFlow(cfg, provider)
		},
	}

	cmd.Flags().StringVarP(&provider, "provider", "p", "github", "Auth provider (github or google)")
	return cmd
}

func newLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Sign out",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if err := cfg.Clear(); err != nil {
				return err
			}
			printSuccess("Logged out")
			return nil
		},
	}
}

func newWhoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show current user",
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
			resp, err := client.Get("/api/whoami")
			if err != nil {
				return err
			}

			whoami, err := DecodeResponse[api.WhoamiResponse](resp)
			if err != nil {
				return err
			}

			fmt.Println()
			fmt.Printf("  %s  %s\n", bold.Render(whoami.DisplayName), dim.Render("("+whoami.PublicID+")"))
			fmt.Printf("  %s  %s\n", dim.Render("provider:"), whoami.Provider)
			fmt.Printf("  %s  %s\n", dim.Render("status:"), whoami.Status)
			fmt.Println()
			return nil
		},
	}
}

func doOAuthFlow(cfg *config.Config, provider string) error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("starting local server: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	tokenCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token == "" {
			errCh <- fmt.Errorf("no token received")
			fmt.Fprintf(w, "<html><body><h1>Authentication failed</h1></body></html>")
			return
		}
		tokenCh <- token
		fmt.Fprintf(w, "<html><body><h1>Authenticated! You can close this tab.</h1></body></html>")
	})

	srv := &http.Server{Handler: mux}
	go srv.Serve(listener)
	defer srv.Close()

	authURL := fmt.Sprintf("%s/auth/%s?redirect_port=%d", cfg.Server.BackendURL, provider, port)
	fmt.Printf("  Opening browser for %s authentication...\n", provider)
	openBrowser(authURL)
	printDim(fmt.Sprintf("  If browser didn't open, visit: %s", authURL))

	select {
	case token := <-tokenCh:
		cfg.Auth.Token = token
		client := NewAPIClient(cfg)
		resp, err := client.Get("/api/whoami")
		if err != nil {
			return fmt.Errorf("fetching user info: %w", err)
		}

		whoami, err := DecodeResponse[api.WhoamiResponse](resp)
		if err != nil {
			return fmt.Errorf("decoding user info: %w", err)
		}

		cfg.User.PublicID = whoami.PublicID
		cfg.User.DisplayName = whoami.DisplayName
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		printSuccess(fmt.Sprintf("Logged in as %s (%s)", whoami.DisplayName, whoami.PublicID))
		return nil

	case err := <-errCh:
		return err

	case <-time.After(2 * time.Minute):
		return fmt.Errorf("login timed out after 2 minutes")
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	}
	cmd.Start()
}

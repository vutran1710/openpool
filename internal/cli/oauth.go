package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"
)

type OAuthResult struct {
	Provider       string
	ProviderUserID string
	DisplayName    string
	Email          string
}

func doOAuth(provider string) (*OAuthResult, error) {
	switch provider {
	case "github":
		return doGitHubDeviceFlow()
	case "google":
		return doGoogleLocalFlow()
	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
}

func doGitHubDeviceFlow() (*OAuthResult, error) {
	printDim("  Opening browser for GitHub authentication...")
	printDim("  If browser doesn't open, visit: https://github.com/login/device")

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("starting local server: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	resultCh := make(chan *OAuthResult, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("access_token")
		if token == "" {
			errCh <- fmt.Errorf("no token received")
			fmt.Fprintf(w, "<html><body><h1>Authentication failed</h1></body></html>")
			return
		}

		user, err := fetchGitHubUser(token)
		if err != nil {
			errCh <- err
			return
		}

		resultCh <- user
		fmt.Fprintf(w, "<html><body><h1>Authenticated! You can close this tab.</h1></body></html>")
	})

	srv := &http.Server{Handler: mux}
	go srv.Serve(listener)
	defer srv.Close()

	openBrowser(fmt.Sprintf("https://github.com/login/oauth/authorize?client_id=PLACEHOLDER&redirect_uri=http://127.0.0.1:%d/callback&scope=read:user", port))

	select {
	case result := <-resultCh:
		return result, nil
	case err := <-errCh:
		return nil, err
	case <-time.After(2 * time.Minute):
		return nil, fmt.Errorf("authentication timed out")
	}
}

func fetchGitHubUser(token string) (*OAuthResult, error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var data struct {
		ID    int    `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	json.Unmarshal(body, &data)

	name := data.Name
	if name == "" {
		name = data.Login
	}

	return &OAuthResult{
		Provider:       "github",
		ProviderUserID: fmt.Sprintf("%d", data.ID),
		DisplayName:    name,
		Email:          data.Email,
	}, nil
}

func doGoogleLocalFlow() (*OAuthResult, error) {
	printDim("  Opening browser for Google authentication...")

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("starting local server: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	resultCh := make(chan *OAuthResult, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("access_token")
		if token == "" {
			errCh <- fmt.Errorf("no token received")
			fmt.Fprintf(w, "<html><body><h1>Authentication failed</h1></body></html>")
			return
		}

		user, err := fetchGoogleUser(token)
		if err != nil {
			errCh <- err
			return
		}

		resultCh <- user
		fmt.Fprintf(w, "<html><body><h1>Authenticated! You can close this tab.</h1></body></html>")
	})

	srv := &http.Server{Handler: mux}
	go srv.Serve(listener)
	defer srv.Close()

	openBrowser(fmt.Sprintf("https://accounts.google.com/o/oauth2/v2/auth?client_id=PLACEHOLDER&redirect_uri=http://127.0.0.1:%d/callback&response_type=token&scope=openid+profile+email", port))

	select {
	case result := <-resultCh:
		return result, nil
	case err := <-errCh:
		return nil, err
	case <-time.After(2 * time.Minute):
		return nil, fmt.Errorf("authentication timed out")
	}
}

func fetchGoogleUser(token string) (*OAuthResult, error) {
	req, _ := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	json.NewDecoder(resp.Body).Decode(&data)

	return &OAuthResult{
		Provider:       "google",
		ProviderUserID: data.ID,
		DisplayName:    data.Name,
		Email:          data.Email,
	}, nil
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

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type OAuthResult struct {
	Provider       string
	ProviderUserID string
	DisplayName    string
	Email          string
}

type OAuthConfig struct {
	ClientID     string
	ClientSecret string
}

var (
	githubOAuth = OAuthConfig{
		ClientID:     getEnvOrDefault("GITHUB_CLIENT_ID", "PLACEHOLDER"),
		ClientSecret: getEnvOrDefault("GITHUB_CLIENT_SECRET", "PLACEHOLDER"),
	}
	googleOAuth = OAuthConfig{
		ClientID:     getEnvOrDefault("GOOGLE_CLIENT_ID", "PLACEHOLDER"),
		ClientSecret: getEnvOrDefault("GOOGLE_CLIENT_SECRET", "PLACEHOLDER"),
	}
)

func getEnvOrDefault(key, fallback string) string {
	if v := getEnv(key); v != "" {
		return v
	}
	return fallback
}

func getEnv(key string) string {
	// avoid importing os just for this; already imported via exec
	val, _ := exec.Command("sh", "-c", "echo $"+key).Output()
	return strings.TrimSpace(string(val))
}

func doOAuth(provider string) (*OAuthResult, error) {
	switch provider {
	case "github":
		return doGitHubOAuth()
	case "google":
		return doGoogleOAuth()
	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
}

func doGitHubOAuth() (*OAuthResult, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("starting local server: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	resultCh := make(chan *OAuthResult, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			errCh <- fmt.Errorf("no authorization code received")
			return
		}

		token, err := exchangeGitHubCode(code, port)
		if err != nil {
			http.Error(w, "authentication failed", http.StatusUnauthorized)
			errCh <- fmt.Errorf("code exchange failed: %w", err)
			return
		}

		user, err := fetchGitHubUser(token)
		if err != nil {
			http.Error(w, "failed to verify identity", http.StatusUnauthorized)
			errCh <- fmt.Errorf("identity verification failed: %w", err)
			return
		}

		resultCh <- user
		fmt.Fprintf(w, "<html><body><h1>Authenticated! You can close this tab.</h1></body></html>")
	})

	srv := &http.Server{Handler: mux}
	go srv.Serve(listener)
	defer srv.Close()

	authURL := fmt.Sprintf(
		"https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&scope=read:user",
		githubOAuth.ClientID,
		url.QueryEscape(fmt.Sprintf("http://127.0.0.1:%d/callback", port)),
	)

	printDim(fmt.Sprintf("  Sign in at:\n  %s", authURL))
	fmt.Println()
	openBrowser(authURL)
	printDim("  Waiting for authentication...")

	select {
	case result := <-resultCh:
		return result, nil
	case err := <-errCh:
		return nil, err
	case <-time.After(2 * time.Minute):
		return nil, fmt.Errorf("authentication timed out")
	}
}

func exchangeGitHubCode(code string, port int) (string, error) {
	data := url.Values{
		"client_id":     {githubOAuth.ClientID},
		"client_secret": {githubOAuth.ClientSecret},
		"code":          {code},
		"redirect_uri":  {fmt.Sprintf("http://127.0.0.1:%d/callback", port)},
	}

	req, _ := http.NewRequest("POST", "https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if result.Error != "" {
		return "", fmt.Errorf("%s: %s", result.Error, result.ErrorDesc)
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("no access token in response")
	}

	return result.AccessToken, nil
}

func fetchGitHubUser(token string) (*OAuthResult, error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var data struct {
		ID    int    `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

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

func doGoogleOAuth() (*OAuthResult, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("starting local server: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	resultCh := make(chan *OAuthResult, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			errCh <- fmt.Errorf("no authorization code received")
			return
		}

		token, err := exchangeGoogleCode(code, port)
		if err != nil {
			http.Error(w, "authentication failed", http.StatusUnauthorized)
			errCh <- fmt.Errorf("code exchange failed: %w", err)
			return
		}

		user, err := fetchGoogleUser(token)
		if err != nil {
			http.Error(w, "failed to verify identity", http.StatusUnauthorized)
			errCh <- fmt.Errorf("identity verification failed: %w", err)
			return
		}

		resultCh <- user
		fmt.Fprintf(w, "<html><body><h1>Authenticated! You can close this tab.</h1></body></html>")
	})

	srv := &http.Server{Handler: mux}
	go srv.Serve(listener)
	defer srv.Close()

	authURL := fmt.Sprintf(
		"https://accounts.google.com/o/oauth2/v2/auth?client_id=%s&redirect_uri=%s&response_type=code&scope=openid+profile+email",
		googleOAuth.ClientID,
		url.QueryEscape(fmt.Sprintf("http://127.0.0.1:%d/callback", port)),
	)

	printDim(fmt.Sprintf("  Sign in at:\n  %s", authURL))
	fmt.Println()
	openBrowser(authURL)
	printDim("  Waiting for authentication...")

	select {
	case result := <-resultCh:
		return result, nil
	case err := <-errCh:
		return nil, err
	case <-time.After(2 * time.Minute):
		return nil, fmt.Errorf("authentication timed out")
	}
}

func exchangeGoogleCode(code string, port int) (string, error) {
	data := url.Values{
		"client_id":     {googleOAuth.ClientID},
		"client_secret": {googleOAuth.ClientSecret},
		"code":          {code},
		"redirect_uri":  {fmt.Sprintf("http://127.0.0.1:%d/callback", port)},
		"grant_type":    {"authorization_code"},
	}

	resp, err := http.PostForm("https://oauth2.googleapis.com/token", data)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if result.Error != "" {
		return "", fmt.Errorf("%s: %s", result.Error, result.ErrorDesc)
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("no access token in response")
	}

	return result.AccessToken, nil
}

func fetchGoogleUser(token string) (*OAuthResult, error) {
	req, _ := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Google API returned %d", resp.StatusCode)
	}

	var data struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

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

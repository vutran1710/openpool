package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
)

type providerUserInfo struct {
	ProviderID  string
	DisplayName string
	Email       string
	AvatarURL   string
}

func fetchProviderUserInfo(ctx context.Context, provider string, cfg *oauth2.Config, token *oauth2.Token) (*providerUserInfo, error) {
	client := cfg.Client(ctx, token)

	switch provider {
	case "github":
		return fetchGitHubUser(client)
	case "google":
		return fetchGoogleUser(client)
	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
}

func fetchGitHubUser(client *http.Client) (*providerUserInfo, error) {
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data struct {
		ID        int    `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	displayName := data.Name
	if displayName == "" {
		displayName = data.Login
	}

	return &providerUserInfo{
		ProviderID:  fmt.Sprintf("%d", data.ID),
		DisplayName: displayName,
		Email:       data.Email,
		AvatarURL:   data.AvatarURL,
	}, nil
}

func fetchGoogleUser(client *http.Client) (*providerUserInfo, error) {
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Email   string `json:"email"`
		Picture string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	return &providerUserInfo{
		ProviderID:  data.ID,
		DisplayName: data.Name,
		Email:       data.Email,
		AvatarURL:   data.Picture,
	}, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

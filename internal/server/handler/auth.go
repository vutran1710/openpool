package handler

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	googleOAuth "golang.org/x/oauth2/google"

	"github.com/vutran1710/dating-dev/internal/server/middleware"
	"github.com/vutran1710/dating-dev/internal/server/store"
	"github.com/vutran1710/dating-dev/pkg/api"
)

type AuthHandler struct {
	store        *store.Store
	auth         *middleware.AuthMiddleware
	githubOAuth  *oauth2.Config
	googleOAuth  *oauth2.Config
	stateSecret  []byte
}

type AuthConfig struct {
	GithubClientID     string
	GithubClientSecret string
	GoogleClientID     string
	GoogleClientSecret string
	BackendURL         string
	StateSecret        string
}

func NewAuthHandler(s *store.Store, authMw *middleware.AuthMiddleware, cfg AuthConfig) *AuthHandler {
	return &AuthHandler{
		store:       s,
		auth:        authMw,
		stateSecret: []byte(cfg.StateSecret),
		githubOAuth: &oauth2.Config{
			ClientID:     cfg.GithubClientID,
			ClientSecret: cfg.GithubClientSecret,
			Endpoint:     github.Endpoint,
			RedirectURL:  cfg.BackendURL + "/auth/callback/github",
			Scopes:       []string{"user:email"},
		},
		googleOAuth: &oauth2.Config{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			Endpoint:     googleOAuth.Endpoint,
			RedirectURL:  cfg.BackendURL + "/auth/callback/google",
			Scopes:       []string{"openid", "profile", "email"},
		},
	}
}

func (h *AuthHandler) StartOAuth(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	redirectPort := r.URL.Query().Get("redirect_port")

	state := h.generateState(redirectPort)

	var authURL string
	switch provider {
	case "github":
		authURL = h.githubOAuth.AuthCodeURL(state)
	case "google":
		authURL = h.googleOAuth.AuthCodeURL(state)
	default:
		http.Error(w, `{"error":"unknown_provider"}`, http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

func (h *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	redirectPort, err := h.validateState(state)
	if err != nil {
		http.Error(w, `{"error":"invalid_state"}`, http.StatusBadRequest)
		return
	}

	var oauthCfg *oauth2.Config
	switch provider {
	case "github":
		oauthCfg = h.githubOAuth
	case "google":
		oauthCfg = h.googleOAuth
	default:
		http.Error(w, `{"error":"unknown_provider"}`, http.StatusBadRequest)
		return
	}

	token, err := oauthCfg.Exchange(r.Context(), code)
	if err != nil {
		http.Error(w, `{"error":"oauth_exchange_failed"}`, http.StatusBadGateway)
		return
	}

	userInfo, err := fetchProviderUserInfo(r.Context(), provider, oauthCfg, token)
	if err != nil {
		http.Error(w, `{"error":"failed_to_fetch_user"}`, http.StatusBadGateway)
		return
	}

	user, created, err := h.store.UpsertUser(
		r.Context(), provider, userInfo.ProviderID,
		userInfo.DisplayName, userInfo.Email, userInfo.AvatarURL,
	)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	if created {
		_ = h.store.CreateProfileIndex(r.Context(), user.ID, user.DisplayName, user.PublicID, user.AvatarURL)
	}

	jwt, err := h.auth.GenerateToken(user.ID, user.PublicID)
	if err != nil {
		http.Error(w, `{"error":"token_generation_failed"}`, http.StatusInternalServerError)
		return
	}

	redirectURL := fmt.Sprintf("http://127.0.0.1:%s/callback?token=%s", redirectPort, jwt)
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

func (h *AuthHandler) Whoami(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	user, err := h.store.GetUserByID(r.Context(), userID)
	if err != nil {
		http.Error(w, `{"error":"user_not_found"}`, http.StatusNotFound)
		return
	}

	resp := api.WhoamiResponse{
		PublicID:    user.PublicID,
		DisplayName: user.DisplayName,
		Email:       user.Email,
		AvatarURL:   user.AvatarURL,
		Provider:    user.AuthProvider,
		Status:      user.Status,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *AuthHandler) generateState(redirectPort string) string {
	nonce := make([]byte, 16)
	rand.Read(nonce)
	payload := hex.EncodeToString(nonce) + ":" + redirectPort
	mac := hmac.New(sha256.New, h.stateSecret)
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return payload + ":" + sig
}

func (h *AuthHandler) validateState(state string) (string, error) {
	parts := splitState(state)
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid state format")
	}
	payload := parts[0] + ":" + parts[1]
	mac := hmac.New(sha256.New, h.stateSecret)
	mac.Write([]byte(payload))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return "", fmt.Errorf("invalid state signature")
	}
	return parts[1], nil
}

func splitState(state string) []string {
	var parts []string
	var current string
	colonCount := 0
	for _, c := range state {
		if c == ':' {
			colonCount++
			if colonCount <= 2 {
				parts = append(parts, current)
				current = ""
				continue
			}
		}
		current += string(c)
	}
	parts = append(parts, current)
	return parts
}

package store

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/google/uuid"
	"github.com/vutran1710/dating-dev/pkg/models"
)

func (s *Store) UpsertUser(ctx context.Context, provider, providerID, displayName, email, avatarURL string) (*models.User, bool, error) {
	var user models.User
	var created bool

	publicID, err := generatePublicID()
	if err != nil {
		return nil, false, fmt.Errorf("generating public ID: %w", err)
	}
	storageKey := deriveStorageKey(provider, providerID)

	err = s.pool.QueryRow(ctx, `
		INSERT INTO users (public_id, storage_key, auth_provider, provider_id, display_name, email, avatar_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (auth_provider, provider_id) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			email = EXCLUDED.email,
			avatar_url = EXCLUDED.avatar_url,
			updated_at = now()
		RETURNING id, public_id, storage_key, auth_provider, provider_id, display_name, email, avatar_url, status, created_at, updated_at,
			(xmax = 0) AS created
	`, publicID, storageKey, provider, providerID, displayName, email, avatarURL,
	).Scan(
		&user.ID, &user.PublicID, &user.StorageKey, &user.AuthProvider,
		&user.ProviderID, &user.DisplayName, &user.Email, &user.AvatarURL,
		&user.Status, &user.CreatedAt, &user.UpdatedAt, &created,
	)
	if err != nil {
		return nil, false, fmt.Errorf("upserting user: %w", err)
	}

	return &user, created, nil
}

func (s *Store) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var user models.User
	err := s.pool.QueryRow(ctx, `
		SELECT id, public_id, storage_key, auth_provider, provider_id, display_name, email, avatar_url, status, created_at, updated_at
		FROM users WHERE id = $1
	`, id).Scan(
		&user.ID, &user.PublicID, &user.StorageKey, &user.AuthProvider,
		&user.ProviderID, &user.DisplayName, &user.Email, &user.AvatarURL,
		&user.Status, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("getting user by ID: %w", err)
	}
	return &user, nil
}

func (s *Store) GetUserByPublicID(ctx context.Context, publicID string) (*models.User, error) {
	var user models.User
	err := s.pool.QueryRow(ctx, `
		SELECT id, public_id, storage_key, auth_provider, provider_id, display_name, email, avatar_url, status, created_at, updated_at
		FROM users WHERE public_id = $1
	`, publicID).Scan(
		&user.ID, &user.PublicID, &user.StorageKey, &user.AuthProvider,
		&user.ProviderID, &user.DisplayName, &user.Email, &user.AvatarURL,
		&user.Status, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("getting user by public ID: %w", err)
	}
	return &user, nil
}

func generatePublicID() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b)[:5], nil
}

func deriveStorageKey(provider, providerID string) string {
	// TODO: use a proper secret from config
	secret := []byte("dating-dev-storage-key-secret")
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(provider + ":" + providerID))
	return hex.EncodeToString(mac.Sum(nil))[:16]
}

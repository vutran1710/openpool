package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID `json:"id" db:"id"`
	PublicID     string    `json:"public_id" db:"public_id"`
	StorageKey   string    `json:"storage_key" db:"storage_key"`
	AuthProvider string    `json:"auth_provider" db:"auth_provider"`
	ProviderID   string    `json:"provider_id" db:"provider_id"`
	DisplayName  string    `json:"display_name" db:"display_name"`
	Email        string    `json:"email,omitempty" db:"email"`
	AvatarURL    string    `json:"avatar_url,omitempty" db:"avatar_url"`
	Status       string    `json:"status" db:"status"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

type ProfileIndex struct {
	UserID      uuid.UUID `json:"user_id" db:"user_id"`
	DisplayName string    `json:"display_name" db:"display_name"`
	Bio         string    `json:"bio,omitempty" db:"bio"`
	City        string    `json:"city,omitempty" db:"city"`
	Interests   []string  `json:"interests" db:"interests"`
	LookingFor  string    `json:"looking_for,omitempty" db:"looking_for"`
	Discoverable bool     `json:"discoverable" db:"discoverable"`
	AvatarURL   string    `json:"avatar_url,omitempty" db:"avatar_url"`
	PublicID    string    `json:"public_id" db:"public_id"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

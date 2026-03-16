package svc

import (
	"encoding/hex"
	"fmt"

	"github.com/vutran1710/dating-dev/internal/cli/config"
)

type realPersistence struct {
	config ConfigService
	crypto CryptoService
}

func NewPersistenceService(cfg ConfigService, crypto CryptoService) PersistenceService {
	return &realPersistence{config: cfg, crypto: crypto}
}

func (s *realPersistence) SavePendingPool(pool config.PoolConfig) error {
	cfg, err := s.config.Load()
	if err != nil {
		return err
	}
	pool.Status = "pending"
	cfg.AddPool(pool)
	if cfg.Active == "" {
		cfg.Active = pool.Name
	}
	return s.config.Save(cfg)
}

func (s *realPersistence) MarkPoolActive(poolName, userHash string) error {
	cfg, err := s.config.Load()
	if err != nil {
		return err
	}
	for i, p := range cfg.Pools {
		if p.Name == poolName {
			cfg.Pools[i].Status = "active"
			cfg.Pools[i].UserHash = userHash
			break
		}
	}
	return s.config.Save(cfg)
}

func (s *realPersistence) MarkPoolRejected(poolName string) error {
	cfg, err := s.config.Load()
	if err != nil {
		return err
	}
	for i, p := range cfg.Pools {
		if p.Name == poolName {
			cfg.Pools[i].Status = "rejected"
			break
		}
	}
	return s.config.Save(cfg)
}

func (s *realPersistence) SaveEncryptedToken(encryptedHex string) error {
	cfg, err := s.config.Load()
	if err != nil {
		return err
	}
	cfg.User.EncryptedToken = encryptedHex
	return s.config.Save(cfg)
}

func (s *realPersistence) DecryptToken() (string, error) {
	cfg, err := s.config.Load()
	if err != nil {
		return "", err
	}
	if cfg.User.EncryptedToken == "" {
		return "", fmt.Errorf("no token stored")
	}

	_, priv, err := s.crypto.LoadKeyPair(s.config.KeysDir())
	if err != nil {
		return "", fmt.Errorf("loading keys: %w", err)
	}

	encrypted, err := hex.DecodeString(cfg.User.EncryptedToken)
	if err != nil {
		return "", fmt.Errorf("decoding token: %w", err)
	}

	plaintext, err := s.crypto.Decrypt(priv, encrypted)
	if err != nil {
		return "", fmt.Errorf("decrypting token: %w", err)
	}
	return string(plaintext), nil
}

func (s *realPersistence) SaveUserIdentity(displayName, username, provider, providerUserID string) error {
	cfg, err := s.config.Load()
	if err != nil {
		return err
	}
	cfg.User.DisplayName = displayName
	cfg.User.Username = username
	cfg.User.Provider = provider
	cfg.User.ProviderUserID = providerUserID
	return s.config.Save(cfg)
}

func (s *realPersistence) AddRegistry(repoURL string) error {
	cfg, err := s.config.Load()
	if err != nil {
		return err
	}
	cfg.AddRegistry(repoURL)
	if cfg.ActiveRegistry == "" {
		cfg.ActiveRegistry = repoURL
	}
	return s.config.Save(cfg)
}

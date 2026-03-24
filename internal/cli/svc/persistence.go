package svc

import (
	"encoding/hex"
	"fmt"

	"github.com/vutran1710/openpool/internal/cli/config"
	dbg "github.com/vutran1710/openpool/internal/debug"
)

type realPersistence struct {
	config ConfigService
	crypto CryptoService
}

func NewPersistenceService(cfg ConfigService, crypto CryptoService) PersistenceService {
	return &realPersistence{config: cfg, crypto: crypto}
}

func (s *realPersistence) SavePendingPool(pool config.PoolConfig) error {
	dbg.Log("persistence: SavePendingPool name=%s repo=%s issue=%d", pool.Name, pool.Repo, pool.PendingIssue)
	cfg, err := s.config.Load()
	if err != nil {
		dbg.Log("persistence: SavePendingPool load error: %v", err)
		return err
	}
	pool.Status = "pending"
	cfg.AddPool(pool)
	if cfg.Active == "" {
		cfg.Active = pool.Name
	}
	if err := s.config.Save(cfg); err != nil {
		dbg.Log("persistence: SavePendingPool save error: %v", err)
		return err
	}
	dbg.Log("persistence: SavePendingPool OK pools=%d", len(cfg.Pools))
	return nil
}

func (s *realPersistence) MarkPoolActive(poolName, userHash string) error {
	dbg.Log("persistence: MarkPoolActive name=%s hash=%s", poolName, userHash)
	cfg, err := s.config.Load()
	if err != nil {
		return err
	}
	found := false
	for i, p := range cfg.Pools {
		if p.Name == poolName {
			cfg.Pools[i].Status = "active"
			cfg.Pools[i].UserHash = userHash
			found = true
			break
		}
	}
	if !found {
		dbg.Log("persistence: MarkPoolActive pool %s not found", poolName)
	}
	return s.config.Save(cfg)
}

func (s *realPersistence) MarkPoolRejected(poolName string) error {
	dbg.Log("persistence: MarkPoolRejected name=%s", poolName)
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
	dbg.Log("persistence: SaveEncryptedToken len=%d", len(encryptedHex))
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
		dbg.Log("persistence: DecryptToken load error: %v", err)
		return "", err
	}
	if cfg.User.EncryptedToken == "" {
		dbg.Log("persistence: DecryptToken no token stored")
		return "", fmt.Errorf("no token stored")
	}

	_, priv, err := s.crypto.LoadKeyPair(s.config.KeysDir())
	if err != nil {
		dbg.Log("persistence: DecryptToken key error: %v", err)
		return "", fmt.Errorf("loading keys: %w", err)
	}

	encrypted, err := hex.DecodeString(cfg.User.EncryptedToken)
	if err != nil {
		dbg.Log("persistence: DecryptToken hex error: %v", err)
		return "", fmt.Errorf("decoding token: %w", err)
	}

	plaintext, err := s.crypto.Decrypt(priv, encrypted)
	if err != nil {
		dbg.Log("persistence: DecryptToken decrypt error: %v", err)
		return "", fmt.Errorf("decrypting token: %w", err)
	}
	dbg.Log("persistence: DecryptToken OK")
	return string(plaintext), nil
}

func (s *realPersistence) SaveUserIdentity(displayName, username, provider, providerUserID string) error {
	dbg.Log("persistence: SaveUserIdentity user=%s provider=%s", username, provider)
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
	dbg.Log("persistence: AddRegistry url=%s", repoURL)
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

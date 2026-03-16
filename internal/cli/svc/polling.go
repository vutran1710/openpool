package svc

import (
	"context"
	"time"
)

type realPolling struct {
	config ConfigService
	crypto CryptoService
	github GitHubService
	cancel context.CancelFunc
}

func NewPollingService(cfg ConfigService, crypto CryptoService, gh GitHubService) PollingService {
	return &realPolling{config: cfg, crypto: crypto, github: gh}
}

func (s *realPolling) Start(ctx context.Context, updates chan<- StatusUpdate) {
	ctx, s.cancel = context.WithCancel(ctx)

	go func() {
		// Initial delay
		select {
		case <-time.After(10 * time.Second):
		case <-ctx.Done():
			return
		}

		for {
			if update := s.PollOnce(ctx); update != nil {
				select {
				case updates <- *update:
				case <-ctx.Done():
					return
				}
			}

			select {
			case <-time.After(30 * time.Second):
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (s *realPolling) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *realPolling) PollOnce(ctx context.Context) *StatusUpdate {
	cfg, err := s.config.Load()
	if err != nil {
		return nil
	}

	// Decrypt token for API calls
	persistence := NewPersistenceService(s.config, s.crypto)
	token, err := persistence.DecryptToken()
	if err != nil {
		return nil
	}

	for _, p := range cfg.Pools {
		if p.Status != "pending" || p.PendingIssue == 0 {
			continue
		}

		pollCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		state, reason, err := s.github.GetIssue(pollCtx, p.Repo, token, p.PendingIssue)
		cancel()

		if err != nil {
			// Issue not found (404) or API error — clear stale data, reset to unjoined
			cfg, _ := s.config.Load()
			if cfg != nil {
				for i, pool := range cfg.Pools {
					if pool.Name == p.Name {
						cfg.Pools = append(cfg.Pools[:i], cfg.Pools[i+1:]...)
						break
					}
				}
				s.config.Save(cfg)
			}
			return nil
		}

		if state == "closed" {
			if reason == "completed" {
				persistence.MarkPoolActive(p.Name, "")
				return &StatusUpdate{PoolName: p.Name, Status: "active"}
			}
			persistence.MarkPoolRejected(p.Name)
			return &StatusUpdate{PoolName: p.Name, Status: "rejected"}
		}
	}

	return nil
}

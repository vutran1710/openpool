package svc

import (
	"context"
	"time"

	dbg "github.com/vutran1710/openpool/internal/debug"
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
	dbg.Log("polling: PollOnce start")
	cfg, err := s.config.Load()
	if err != nil {
		dbg.Log("polling: config load error: %v", err)
		return nil
	}

	persistence := NewPersistenceService(s.config, s.crypto)
	token, err := persistence.DecryptToken()
	if err != nil {
		dbg.Log("polling: token decrypt error: %v", err)
		return nil
	}

	for _, p := range cfg.Pools {
		if p.Status != "pending" || p.PendingIssue == 0 {
			continue
		}

		dbg.Log("polling: checking %s repo=%s issue=%d", p.Name, p.Repo, p.PendingIssue)
		pollCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		state, reason, err := s.github.GetIssue(pollCtx, p.Repo, token, p.PendingIssue)
		cancel()

		if err != nil {
			dbg.Log("polling: GetIssue error for %s: %v — removing stale entry", p.Name, err)
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

		dbg.Log("polling: %s state=%s reason=%s", p.Name, state, reason)
		if state == "closed" {
			if reason == "completed" {
				dbg.Log("polling: %s → active", p.Name)
				persistence.MarkPoolActive(p.Name, "")
				return &StatusUpdate{PoolName: p.Name, Status: "active"}
			}
			dbg.Log("polling: %s → rejected", p.Name)
			persistence.MarkPoolRejected(p.Name)
			return &StatusUpdate{PoolName: p.Name, Status: "rejected"}
		}
	}

	dbg.Log("polling: no updates")
	return nil
}

package cli

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/vutran1710/dating-dev/internal/cli/config"
	"github.com/vutran1710/dating-dev/internal/github"
)

func requirePool(cfg *config.Config) (*config.PoolConfig, error) {
	if !cfg.IsRegistered() {
		printWarning("Not registered. Run: dating auth register")
		return nil, fmt.Errorf("not registered")
	}

	pool := cfg.ActivePool()
	if pool == nil {
		printWarning("No active pool. Run: dating pool join <url>")
		return nil, fmt.Errorf("no active pool")
	}

	return pool, nil
}

func poolClient(pool *config.PoolConfig) *github.Pool {
	return github.NewPool(pool.Repo, "")
}

func poolClientWithToken(pool *config.PoolConfig, token string) *github.Pool {
	return github.NewPool(pool.Repo, token)
}

func prompt(reader *bufio.Reader, label string) string {
	fmt.Print(label)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

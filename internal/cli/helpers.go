package cli

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/vutran1710/openpool/internal/cli/config"
	"github.com/vutran1710/openpool/internal/github"
)

func requirePool(cfg *config.Config) (*config.PoolConfig, error) {
	if !cfg.IsRegistered() {
		printWarning("Not registered. Run: op auth register")
		return nil, fmt.Errorf("not registered")
	}

	pool := cfg.ActivePool()
	if pool == nil {
		printWarning("No active pool. Run: op pool join <url>")
		return nil, fmt.Errorf("no active pool")
	}

	return pool, nil
}

func poolClient(pool *config.PoolConfig) *github.Pool {
	return github.NewPoolWithClient(github.NewCLIOrHTTP(pool.Repo, ""))
}

func poolClientWithToken(pool *config.PoolConfig, token string) *github.Pool {
	return github.NewPoolWithClient(github.NewCLIOrHTTP(pool.Repo, token))
}

func prompt(reader *bufio.Reader, label string) string {
	fmt.Print(label)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

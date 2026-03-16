// Package debug provides a shared debug logger visible in the TUI status bar.
// Enable with DEBUG=1 or DEBUG=true environment variable.
package debug

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	Enabled = os.Getenv("DEBUG") == "1" || os.Getenv("DEBUG") == "true"
	log     logger
)

type logger struct {
	mu      sync.Mutex
	entries []string
}

func init() {
	log.entries = make([]string, 0, 100)
}

// Log records a debug message with timestamp.
func Log(format string, args ...any) {
	if !Enabled {
		return
	}
	log.mu.Lock()
	defer log.mu.Unlock()

	ts := time.Now().Format("15:04:05.000")
	msg := fmt.Sprintf("[%s] %s", ts, fmt.Sprintf(format, args...))
	log.entries = append(log.entries, msg)
	if len(log.entries) > 100 {
		log.entries = log.entries[len(log.entries)-100:]
	}
}

// Timer returns a func that logs elapsed time when called.
func Timer(label string) func() {
	if !Enabled {
		return func() {}
	}
	start := time.Now()
	return func() {
		Log("%s: %v", label, time.Since(start))
	}
}

// View returns the last N debug entries for display.
func View(n int) string {
	if !Enabled {
		return ""
	}
	log.mu.Lock()
	defer log.mu.Unlock()

	if len(log.entries) == 0 {
		return ""
	}
	start := len(log.entries) - n
	if start < 0 {
		start = 0
	}
	return strings.Join(log.entries[start:], "\n")
}

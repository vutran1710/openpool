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
	file    *os.File
}

func init() {
	log.entries = make([]string, 0, 100)
	if Enabled {
		home := os.Getenv("OPENPOOL_HOME")
		if home == "" {
			home = os.ExpandEnv("$HOME/.openpool")
		}
		os.MkdirAll(home, 0700)
		f, err := os.OpenFile(home+"/debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err == nil {
			log.file = f
		}
	}
}

// Close flushes and closes the log file.
func Close() {
	if log.file != nil {
		log.file.Close()
	}
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
	if log.file != nil {
		log.file.WriteString(msg + "\n")
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

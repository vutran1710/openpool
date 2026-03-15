package cli

import (
	"fmt"
	"sync"
	"time"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type spinner struct {
	message string
	done    chan struct{}
	wg      sync.WaitGroup
}

func newSpinner(message string) *spinner {
	s := &spinner{
		message: message,
		done:    make(chan struct{}),
	}
	s.wg.Add(1)
	go s.run()
	return s
}

func (s *spinner) run() {
	defer s.wg.Done()
	i := 0
	for {
		select {
		case <-s.done:
			// Clear the spinner line
			fmt.Printf("\r\033[K")
			return
		default:
			fmt.Printf("\r  %s %s", brand.Render(spinnerFrames[i%len(spinnerFrames)]), s.message)
			i++
			time.Sleep(80 * time.Millisecond)
		}
	}
}

func (s *spinner) stop() {
	close(s.done)
	s.wg.Wait()
}

// withSpinner runs fn while showing a spinner with the given message.
// Returns the result and prints success/error after.
func withSpinner[T any](message string, fn func() (T, error)) (T, error) {
	sp := newSpinner(message)
	result, err := fn()
	sp.stop()
	if err != nil {
		printError(err.Error())
	} else {
		printSuccess(message)
	}
	return result, err
}

// withSpinnerNoResult runs fn while showing a spinner. For void operations.
func withSpinnerNoResult(message string, fn func() error) error {
	sp := newSpinner(message)
	err := fn()
	sp.stop()
	if err != nil {
		printError(err.Error())
	} else {
		printSuccess(message)
	}
	return err
}

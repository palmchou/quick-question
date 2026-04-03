package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

func shouldShowSpinner(file *os.File) bool {
	if file == nil {
		return false
	}

	info, err := file.Stat()
	if err != nil {
		return false
	}

	return info.Mode()&os.ModeCharDevice != 0
}

type spinner struct {
	out      io.Writer
	enabled  bool
	message  string
	interval time.Duration
	frames   []byte
	done     chan struct{}
	stopped  chan struct{}
	mu       sync.Mutex
	lastLen  int
}

func startSpinner(out io.Writer, enabled bool, message string) *spinner {
	s := &spinner{
		out:      out,
		enabled:  enabled,
		message:  message,
		interval: 120 * time.Millisecond,
		frames:   []byte{'|', '/', '-', '\\'},
		done:     make(chan struct{}),
		stopped:  make(chan struct{}),
	}

	if !enabled {
		close(s.stopped)
		return s
	}

	go s.run()
	return s
}

func (s *spinner) Stop() {
	if !s.enabled {
		return
	}

	select {
	case <-s.done:
	default:
		close(s.done)
	}

	<-s.stopped
	s.clear()
}

func (s *spinner) run() {
	defer close(s.stopped)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for i := 0; ; i++ {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.render(i)
		}
	}
}

func (s *spinner) render(frame int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	text := fmt.Sprintf("\r%c %s", s.frames[frame%len(s.frames)], s.message)
	_, _ = io.WriteString(s.out, text)
	s.lastLen = len(text) - 1
}

func (s *spinner) clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.lastLen == 0 {
		return
	}

	_, _ = io.WriteString(s.out, "\r"+strings.Repeat(" ", s.lastLen)+"\r")
	s.lastLen = 0
}

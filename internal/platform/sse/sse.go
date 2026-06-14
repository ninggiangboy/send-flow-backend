package sse

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const DefaultHeartbeatInterval = 15 * time.Second

var ErrStreamingUnsupported = errors.New("sse: response writer does not support streaming")

type Options struct {
	HeartbeatInterval time.Duration
}

type Event struct {
	Event string
	ID    string
	Retry time.Duration
	Data  string
}

type Stream struct {
	w       http.ResponseWriter
	flusher http.Flusher
	done    chan struct{}
	wg      sync.WaitGroup

	mu sync.Mutex
}

func New(w http.ResponseWriter, r *http.Request, opts Options) (*Stream, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, ErrStreamingUnsupported
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	s := &Stream{
		w:       w,
		flusher: flusher,
		done:    make(chan struct{}),
	}

	interval := opts.HeartbeatInterval
	if interval == 0 {
		interval = DefaultHeartbeatInterval
	}
	if interval > 0 {
		s.wg.Add(1)
		go s.heartbeat(r.Context(), interval)
	}

	return s, nil
}

func (s *Stream) Close() {
	select {
	case <-s.done:
		return
	default:
		close(s.done)
		s.wg.Wait()
	}
}

func (s *Stream) WriteEvent(event Event) error {
	var b strings.Builder

	if event.Event != "" {
		fmt.Fprintf(&b, "event: %s\n", event.Event)
	}
	if event.ID != "" {
		fmt.Fprintf(&b, "id: %s\n", event.ID)
	}
	if event.Retry > 0 {
		fmt.Fprintf(&b, "retry: %d\n", event.Retry.Milliseconds())
	}

	for _, line := range strings.Split(event.Data, "\n") {
		fmt.Fprintf(&b, "data: %s\n", line)
	}
	b.WriteString("\n")

	return s.writeFrame(b.String())
}

func (s *Stream) WriteComment(comment string) error {
	return s.writeFrame(": " + comment + "\n\n")
}

func (s *Stream) heartbeat(ctx context.Context, interval time.Duration) {
	defer s.wg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.done:
			return
		case <-ticker.C:
			_ = s.WriteComment("keepalive")
		}
	}
}

func (s *Stream) writeFrame(frame string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := io.WriteString(s.w, frame); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

package realtime

import (
	"context"
	"log/slog"
	"sync"
)

type subscriber struct {
	ch   chan Event
	done func()
}

type channelSub struct {
	cancel     context.CancelFunc
	subs       map[*subscriber]struct{}
	subOnce    sync.Once
	unsubOnce  sync.Once
	unsubRedis func() error
}

type Hub struct {
	backplane *Backplane
	log       *slog.Logger

	mu     sync.RWMutex
	chans  map[string]*channelSub
	health bool
}

func NewHub(backplane *Backplane, log *slog.Logger) *Hub {
	return &Hub{
		backplane: backplane,
		log:       log.With("component", "realtime_hub"),
		chans:     make(map[string]*channelSub),
		health:    true,
	}
}

func (h *Hub) Subscribe(ctx context.Context, channel string) (<-chan Event, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()

	cs, ok := h.chans[channel]
	if !ok {
		cs = &channelSub{
			subs: make(map[*subscriber]struct{}),
		}
		h.chans[channel] = cs
	}

	sub := &subscriber{ch: make(chan Event, 32)}
	cs.subs[sub] = struct{}{}

	// Start Redis subscription on first subscriber
	cs.subOnce.Do(func() {
		subCtx, cancel := context.WithCancel(context.Background())
		cs.cancel = cancel

		redisCh, unsub, err := h.backplane.Subscribe(subCtx, channel)
		if err != nil {
			h.log.Error("failed to subscribe to redis channel", "channel", channel, "error", err)
			h.health = false
			cancel()
			return
		}
		cs.unsubRedis = unsub

		go h.fanout(subCtx, redisCh, channel, cs)
	})

	unsub := func() {
		h.mu.Lock()
		defer h.mu.Unlock()

		// sub may already have been removed by fanout (slow subscriber); only close once.
		if _, ok := cs.subs[sub]; ok {
			delete(cs.subs, sub)
			close(sub.ch)
		}

		if len(cs.subs) == 0 {
			cs.unsubOnce.Do(func() {
				cs.cancel()
				if cs.unsubRedis != nil {
					_ = cs.unsubRedis()
				}
				delete(h.chans, channel)
			})
		}
	}

	return sub.ch, unsub
}

func (h *Hub) Healthy() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.health
}

func (h *Hub) fanout(ctx context.Context, redisCh <-chan Event, channel string, cs *channelSub) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-redisCh:
			if !ok {
				return
			}
			h.mu.RLock()
			for sub := range cs.subs {
				select {
				case sub.ch <- ev:
				default:
					h.log.Warn("slow subscriber detected, closing",
						"channel", channel, "event_id", ev.EventID)
					close(sub.ch)
					delete(cs.subs, sub)
				}
			}
			h.mu.RUnlock()
		}
	}
}

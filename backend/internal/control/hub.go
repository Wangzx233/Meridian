package control

import (
	"context"
	"sync"
)

type EventHub struct {
	mu          sync.Mutex
	subscribers map[string]map[chan RunEvent]struct{}
}

func NewEventHub() *EventHub {
	return &EventHub{subscribers: map[string]map[chan RunEvent]struct{}{}}
}

func (h *EventHub) Subscribe(ctx context.Context, runID string) <-chan RunEvent {
	ch := make(chan RunEvent, 32)
	h.mu.Lock()
	if h.subscribers[runID] == nil {
		h.subscribers[runID] = map[chan RunEvent]struct{}{}
	}
	h.subscribers[runID][ch] = struct{}{}
	h.mu.Unlock()

	go func() {
		<-ctx.Done()
		h.mu.Lock()
		if subs := h.subscribers[runID]; subs != nil {
			delete(subs, ch)
			if len(subs) == 0 {
				delete(h.subscribers, runID)
			}
		}
		h.mu.Unlock()
		close(ch)
	}()
	return ch
}

func (h *EventHub) Publish(event RunEvent) {
	h.mu.Lock()
	subs := h.subscribers[event.RunID]
	targets := make([]chan RunEvent, 0, len(subs))
	for ch := range subs {
		targets = append(targets, ch)
	}
	h.mu.Unlock()
	for _, ch := range targets {
		select {
		case ch <- event:
		default:
		}
	}
}

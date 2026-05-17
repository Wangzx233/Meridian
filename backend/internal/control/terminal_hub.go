package control

import (
	"context"
	"sync"
)

type TerminalHub struct {
	mu   sync.Mutex
	subs map[string]map[chan RunnerEnvelope]struct{}
}

func NewTerminalHub() *TerminalHub {
	return &TerminalHub{subs: map[string]map[chan RunnerEnvelope]struct{}{}}
}

func (h *TerminalHub) Subscribe(ctx context.Context, terminalID string) chan RunnerEnvelope {
	ch := make(chan RunnerEnvelope, 64)
	h.mu.Lock()
	if h.subs[terminalID] == nil {
		h.subs[terminalID] = map[chan RunnerEnvelope]struct{}{}
	}
	h.subs[terminalID][ch] = struct{}{}
	h.mu.Unlock()

	go func() {
		<-ctx.Done()
		h.mu.Lock()
		delete(h.subs[terminalID], ch)
		if len(h.subs[terminalID]) == 0 {
			delete(h.subs, terminalID)
		}
		h.mu.Unlock()
		close(ch)
	}()

	return ch
}

func (h *TerminalHub) Publish(terminalID string, env RunnerEnvelope) {
	if terminalID == "" {
		return
	}
	h.mu.Lock()
	subs := make([]chan RunnerEnvelope, 0, len(h.subs[terminalID]))
	for ch := range h.subs[terminalID] {
		subs = append(subs, ch)
	}
	h.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- env:
		default:
		}
	}
}

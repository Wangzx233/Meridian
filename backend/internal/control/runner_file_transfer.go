package control

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type RunnerFileTransferManager struct {
	mu      sync.Mutex
	clients map[string]*RunnerFileTransferClient
}

type RunnerFileTransferClient struct {
	id      string
	conn    *websocket.Conn
	writeMu sync.Mutex
	mu      sync.Mutex
	pending map[string]chan RunnerEnvelope
}

func NewRunnerFileTransferManager() *RunnerFileTransferManager {
	return &RunnerFileTransferManager{clients: map[string]*RunnerFileTransferClient{}}
}

func (m *RunnerFileTransferManager) Register(runnerID string, conn *websocket.Conn) {
	m.mu.Lock()
	if old := m.clients[runnerID]; old != nil && old.conn != conn {
		_ = old.conn.Close()
	}
	m.clients[runnerID] = &RunnerFileTransferClient{
		id:      runnerID,
		conn:    conn,
		pending: map[string]chan RunnerEnvelope{},
	}
	m.mu.Unlock()
}

func (m *RunnerFileTransferManager) Unregister(runnerID string, conn *websocket.Conn) bool {
	m.mu.Lock()
	removed := false
	if current := m.clients[runnerID]; current != nil && current.conn == conn {
		delete(m.clients, runnerID)
		removed = true
	}
	m.mu.Unlock()
	return removed
}

func (m *RunnerFileTransferManager) Connected(runnerID string) bool {
	if runnerID == "" {
		return false
	}
	m.mu.Lock()
	connected := m.clients[runnerID] != nil
	m.mu.Unlock()
	return connected
}

func (m *RunnerFileTransferManager) WaitConnected(ctx context.Context, runnerID string, timeout time.Duration) bool {
	if m.Connected(runnerID) {
		return true
	}
	if timeout <= 0 {
		return false
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-waitCtx.Done():
			return false
		case <-ticker.C:
			if m.Connected(runnerID) {
				return true
			}
		}
	}
}

func (m *RunnerFileTransferManager) Close(runnerID string) {
	if runnerID == "" {
		return
	}
	m.mu.Lock()
	client := m.clients[runnerID]
	if client != nil {
		delete(m.clients, runnerID)
	}
	m.mu.Unlock()
	if client != nil {
		_ = client.conn.Close()
	}
}

func (m *RunnerFileTransferManager) Request(runnerID, typ string, payload any, data []byte, timeout time.Duration) (RunnerEnvelope, error) {
	if runnerID == "" {
		return RunnerEnvelope{}, ErrRunnerUnavailable
	}
	m.mu.Lock()
	client := m.clients[runnerID]
	m.mu.Unlock()
	if client == nil {
		return RunnerEnvelope{}, ErrRunnerUnavailable
	}
	messageID := randomID("msg")
	ch := make(chan RunnerEnvelope, 1)
	client.addPending(messageID, ch)
	defer client.removePending(messageID)

	if err := client.writeUploadRequest(typ, messageID, payload, data); err != nil {
		return RunnerEnvelope{}, err
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case env := <-ch:
		return env, nil
	case <-timer.C:
		return RunnerEnvelope{}, RunnerRequestTimeoutError{RunnerID: runnerID, MessageID: messageID, Type: typ, Timeout: timeout}
	}
}

func (m *RunnerFileTransferManager) HandleResponse(runnerID string, env RunnerEnvelope) bool {
	if runnerID == "" || env.MessageID == "" {
		return false
	}
	m.mu.Lock()
	client := m.clients[runnerID]
	m.mu.Unlock()
	if client == nil {
		return false
	}
	return client.deliver(env)
}

func (c *RunnerFileTransferClient) writeUploadRequest(typ, messageID string, payload any, data []byte) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	env := RunnerEnvelope{
		Type:      typ,
		MessageID: messageID,
		SentAt:    time.Now().UTC(),
		Payload:   raw,
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := c.conn.WriteJSON(env); err != nil {
		return err
	}
	if err := c.conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
		return fmt.Errorf("write upload data: %w", err)
	}
	return nil
}

func (c *RunnerFileTransferClient) addPending(messageID string, ch chan RunnerEnvelope) {
	c.mu.Lock()
	c.pending[messageID] = ch
	c.mu.Unlock()
}

func (c *RunnerFileTransferClient) removePending(messageID string) {
	c.mu.Lock()
	delete(c.pending, messageID)
	c.mu.Unlock()
}

func (c *RunnerFileTransferClient) deliver(env RunnerEnvelope) bool {
	c.mu.Lock()
	ch := c.pending[env.MessageID]
	if ch != nil {
		delete(c.pending, env.MessageID)
	}
	c.mu.Unlock()
	if ch == nil {
		return false
	}
	ch <- env
	return true
}

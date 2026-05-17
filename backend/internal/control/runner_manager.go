package control

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type RunnerManager struct {
	mu      sync.Mutex
	runners map[string]*RunnerClient
}

type RunnerClient struct {
	id           string
	conn         *websocket.Conn
	info         RunnerInfo
	capabilities map[string]any
	writeMu      sync.Mutex
	mu           sync.Mutex
	pending      map[string]chan RunnerEnvelope
}

type RunnerRequestTimeoutError struct {
	RunnerID  string
	MessageID string
	Type      string
	Timeout   time.Duration
}

func (e RunnerRequestTimeoutError) Error() string {
	return fmt.Sprintf("runner request %s timed out after %s", e.Type, e.Timeout)
}

func (e RunnerRequestTimeoutError) Is(target error) bool {
	return target == ErrRunnerRequestTimeout
}

func NewRunnerManager() *RunnerManager {
	return &RunnerManager{runners: map[string]*RunnerClient{}}
}

func (m *RunnerManager) Register(runnerID string, conn *websocket.Conn, info RunnerInfo, capabilities map[string]any) {
	m.mu.Lock()
	if old := m.runners[runnerID]; old != nil && old.conn != conn {
		_ = old.conn.Close()
	}
	if info.ConnectedAt.IsZero() {
		info.ConnectedAt = time.Now().UTC()
	}
	m.runners[runnerID] = &RunnerClient{
		id:           runnerID,
		conn:         conn,
		info:         info,
		capabilities: cloneCapabilities(capabilities),
		pending:      map[string]chan RunnerEnvelope{},
	}
	m.mu.Unlock()
}

func (m *RunnerManager) Unregister(runnerID string, conn *websocket.Conn) bool {
	m.mu.Lock()
	removed := false
	if current := m.runners[runnerID]; current != nil && current.conn == conn {
		delete(m.runners, runnerID)
		removed = true
	}
	m.mu.Unlock()
	return removed
}

func (m *RunnerManager) SendAssign(assign *RunAssignPayload) error {
	return m.send(assign.TargetRunnerID, "run.assign", assign)
}

func (m *RunnerManager) SendCancel(payload RunCancelPayload, runnerID string) error {
	return m.send(runnerID, "run.cancel", payload)
}

func (m *RunnerManager) Send(runnerID, typ string, payload any) error {
	return m.send(runnerID, typ, payload)
}

func (m *RunnerManager) Connected(runnerID string) bool {
	if runnerID == "" {
		return false
	}
	m.mu.Lock()
	connected := m.runners[runnerID] != nil
	m.mu.Unlock()
	return connected
}

func (m *RunnerManager) Capabilities(runnerID string) map[string]any {
	if runnerID == "" {
		return nil
	}
	m.mu.Lock()
	client := m.runners[runnerID]
	if client == nil {
		m.mu.Unlock()
		return nil
	}
	capabilities := cloneCapabilities(client.capabilities)
	m.mu.Unlock()
	return capabilities
}

func (m *RunnerManager) Info(runnerID string) *RunnerInfo {
	if runnerID == "" {
		return nil
	}
	m.mu.Lock()
	client := m.runners[runnerID]
	if client == nil {
		m.mu.Unlock()
		return nil
	}
	info := client.info
	m.mu.Unlock()
	return &info
}

func (m *RunnerManager) ConnectedRunnerIDs() map[string]bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]bool, len(m.runners))
	for runnerID := range m.runners {
		out[runnerID] = true
	}
	return out
}

func (m *RunnerManager) Supports(runnerID, capability string) bool {
	capabilities := m.Capabilities(runnerID)
	if capabilities == nil {
		return false
	}
	value, ok := capabilities[capability]
	if !ok {
		return false
	}
	supported, ok := value.(bool)
	return ok && supported
}

func (m *RunnerManager) Request(runnerID, typ string, payload any, timeout time.Duration) (RunnerEnvelope, error) {
	if runnerID == "" {
		return RunnerEnvelope{}, ErrRunnerUnavailable
	}
	m.mu.Lock()
	client := m.runners[runnerID]
	m.mu.Unlock()
	if client == nil {
		return RunnerEnvelope{}, ErrRunnerUnavailable
	}
	messageID := randomID("msg")
	ch := make(chan RunnerEnvelope, 1)
	client.addPending(messageID, ch)
	defer client.removePending(messageID)

	if err := client.write(typ, messageID, payload); err != nil {
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

func (m *RunnerManager) HandleResponse(runnerID string, env RunnerEnvelope) bool {
	if runnerID == "" || env.MessageID == "" {
		return false
	}
	m.mu.Lock()
	client := m.runners[runnerID]
	m.mu.Unlock()
	if client == nil {
		return false
	}
	return client.deliver(env)
}

func (m *RunnerManager) send(runnerID, typ string, payload any) error {
	if runnerID == "" {
		return ErrRunnerUnavailable
	}
	m.mu.Lock()
	client := m.runners[runnerID]
	m.mu.Unlock()
	if client == nil {
		return ErrRunnerUnavailable
	}
	return client.write(typ, randomID("msg"), payload)
}

func (c *RunnerClient) write(typ, messageID string, payload any) error {
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
	return c.conn.WriteJSON(env)
}

func (c *RunnerClient) addPending(messageID string, ch chan RunnerEnvelope) {
	c.mu.Lock()
	c.pending[messageID] = ch
	c.mu.Unlock()
}

func (c *RunnerClient) removePending(messageID string) {
	c.mu.Lock()
	delete(c.pending, messageID)
	c.mu.Unlock()
}

func (c *RunnerClient) deliver(env RunnerEnvelope) bool {
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

func cloneCapabilities(capabilities map[string]any) map[string]any {
	if len(capabilities) == 0 {
		return nil
	}
	clone := make(map[string]any, len(capabilities))
	for key, value := range capabilities {
		clone[key] = value
	}
	return clone
}

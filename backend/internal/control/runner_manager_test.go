package control

import (
	"errors"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestRunnerManagerConnectionCapabilities(t *testing.T) {
	manager := NewRunnerManager()
	conn := &websocket.Conn{}
	manager.Register("runner_desktop", conn, RunnerInfo{Hostname: "desktop", Version: "0.2.0"}, map[string]any{"fs_list": true, "codex_exec": true})

	if !manager.Connected("runner_desktop") {
		t.Fatalf("runner should be connected")
	}
	info := manager.Info("runner_desktop")
	if info == nil || info.Hostname != "desktop" || info.Version != "0.2.0" || info.ConnectedAt.IsZero() {
		t.Fatalf("runner info was not captured: %#v", info)
	}
	if !manager.Supports("runner_desktop", "fs_list") {
		t.Fatalf("runner should support fs_list")
	}
	if manager.Supports("runner_desktop", "missing") {
		t.Fatalf("runner should not support missing capability")
	}

	capabilities := manager.Capabilities("runner_desktop")
	capabilities["fs_list"] = false
	if !manager.Supports("runner_desktop", "fs_list") {
		t.Fatalf("capability snapshot should not mutate manager state")
	}
	connected := manager.ConnectedRunnerIDs()
	if !connected["runner_desktop"] {
		t.Fatalf("connected runner id snapshot should include runner_desktop")
	}

	if !manager.Unregister("runner_desktop", conn) {
		t.Fatalf("unregister should remove the current connection")
	}
	if manager.Connected("runner_desktop") {
		t.Fatalf("runner should be disconnected after unregister")
	}
}

func TestRunnerRequestTimeoutErrorMatchesSentinel(t *testing.T) {
	err := RunnerRequestTimeoutError{RunnerID: "runner_desktop", Type: "fs.list", Timeout: time.Millisecond}
	if !errors.Is(err, ErrRunnerRequestTimeout) {
		t.Fatalf("timeout error should match ErrRunnerRequestTimeout")
	}
}

package control

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

var runnerManagerTestUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

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

func TestRunnerFileTransferRequestWritesMetadataThenBinary(t *testing.T) {
	manager := NewRunnerFileTransferManager()
	serverConn, clientConn := newTestWebsocketPair(t)
	defer serverConn.Close()
	defer clientConn.Close()

	manager.Register("runner_desktop", serverConn)
	done := make(chan struct{})
	responseDone := make(chan struct{})
	go func() {
		defer close(responseDone)
		var env RunnerEnvelope
		if err := serverConn.ReadJSON(&env); err != nil {
			t.Errorf("read file transfer response: %v", err)
			return
		}
		manager.HandleResponse("runner_desktop", env)
	}()
	go func() {
		defer close(done)
		var env RunnerEnvelope
		if err := clientConn.ReadJSON(&env); err != nil {
			t.Errorf("read upload metadata: %v", err)
			return
		}
		if env.Type != "project.file.upload.stream" || env.MessageID == "" {
			t.Errorf("metadata envelope = %#v", env)
			return
		}
		var payload ProjectFileUploadStreamRequestPayload
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			t.Errorf("decode upload metadata: %v", err)
			return
		}
		if payload.ChunkBytes != 4 {
			t.Errorf("chunk bytes = %d, want 4", payload.ChunkBytes)
			return
		}
		messageType, data, err := clientConn.ReadMessage()
		if err != nil {
			t.Errorf("read upload data: %v", err)
			return
		}
		if messageType != websocket.BinaryMessage || string(data) != "data" {
			t.Errorf("data frame = %d %q", messageType, data)
			return
		}
		if err := clientConn.WriteJSON(RunnerEnvelope{
			Type:      "project.file.upload.stream.response",
			MessageID: env.MessageID,
			SentAt:    time.Now().UTC(),
			Payload: mustJSONForManagerTest(t, ProjectFileActionResult{
				Path:          payload.Path,
				UploadedBytes: payload.Offset + int64(len(data)),
				TotalSize:     payload.TotalSize,
				ResumeOffset:  payload.Offset + int64(len(data)),
			}),
		}); err != nil {
			t.Errorf("write response: %v", err)
		}
	}()

	env, err := manager.Request("runner_desktop", "project.file.upload.stream", ProjectFileUploadStreamRequestPayload{
		Path:       "upload.bin",
		Offset:     0,
		TotalSize:  4,
		ChunkBytes: 4,
	}, []byte("data"), time.Second)
	if err != nil {
		t.Fatalf("request file transfer: %v", err)
	}
	var result ProjectFileActionResult
	if err := json.Unmarshal(env.Payload, &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result.ResumeOffset != 4 {
		t.Fatalf("resume offset = %d, want 4", result.ResumeOffset)
	}
	<-done
	<-responseDone
}

func newTestWebsocketPair(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()
	serverConn := make(chan *websocket.Conn, 1)
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := runnerManagerTestUpgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade websocket pair: %v", err)
			return
		}
		serverConn <- conn
		<-r.Context().Done()
	}))
	t.Cleanup(httpServer.Close)

	clientConn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(httpServer.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial websocket pair: %v", err)
	}
	select {
	case conn := <-serverConn:
		return conn, clientConn
	case <-time.After(2 * time.Second):
		clientConn.Close()
		t.Fatal("server websocket was not accepted")
		return nil, nil
	}
}

func mustJSONForManagerTest(t *testing.T, value any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return raw
}

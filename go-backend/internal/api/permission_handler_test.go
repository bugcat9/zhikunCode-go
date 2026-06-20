package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go-backend/internal/permission"
)

func TestPermissionModeHandlerCanReadAndUpdateMode(t *testing.T) {
	resetPermissionBrokerForTest(t)
	t.Setenv("WORKSPACE_PATH", t.TempDir())

	router := NewRouter()

	put := httptest.NewRequest(http.MethodPut, "/api/permissions/mode", strings.NewReader(`{"mode":"read_only"}`))
	put.Header.Set("Content-Type", "application/json")
	putRecorder := httptest.NewRecorder()
	router.ServeHTTP(putRecorder, put)
	if putRecorder.Code != http.StatusOK {
		t.Fatalf("PUT mode status = %d, body = %s", putRecorder.Code, putRecorder.Body.String())
	}

	get := httptest.NewRequest(http.MethodGet, "/api/permissions/mode", nil)
	getRecorder := httptest.NewRecorder()
	router.ServeHTTP(getRecorder, get)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("GET mode status = %d, body = %s", getRecorder.Code, getRecorder.Body.String())
	}

	var body struct {
		Mode permission.Mode `json:"mode"`
	}
	if err := json.NewDecoder(getRecorder.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Mode != permission.ModeReadOnly {
		t.Fatalf("mode = %q, want %q", body.Mode, permission.ModeReadOnly)
	}
}

func TestPermissionDecisionHandlerResolvesPendingRequest(t *testing.T) {
	resetPermissionBrokerForTest(t)
	t.Setenv("WORKSPACE_PATH", t.TempDir())

	broker, err := getPermissionBroker()
	if err != nil {
		t.Fatal(err)
	}

	decisions := make(chan permission.PermissionDecision, 1)
	errors := make(chan error, 1)
	go func() {
		decision, err := broker.Request(context.Background(), permission.PermissionRequest{
			ID:        "call_1",
			ToolUseID: "call_1",
			ToolName:  "write_file",
			Input:     json.RawMessage(`{"path":"notes.txt","content":"hello"}`),
		})
		if err != nil {
			errors <- err
			return
		}
		decisions <- decision
	}()
	waitForPermissionPending(t, broker)

	router := NewRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/permissions/decision", strings.NewReader(`{"toolUseId":"call_1","decision":"deny"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("decision status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	select {
	case err := <-errors:
		t.Fatal(err)
	case decision := <-decisions:
		if decision.Decision != permission.DecisionDeny {
			t.Fatalf("decision = %#v, want deny", decision)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for pending permission to resolve")
	}
}

func resetPermissionBrokerForTest(t *testing.T) {
	t.Helper()

	permissionBrokerMu.Lock()
	previousBroker := permissionBroker
	previousMode := permissionMode
	permissionBroker = nil
	permissionMode = ""
	permissionBrokerMu.Unlock()

	t.Cleanup(func() {
		permissionBrokerMu.Lock()
		permissionBroker = previousBroker
		permissionMode = previousMode
		permissionBrokerMu.Unlock()
	})
}

func waitForPermissionPending(t *testing.T, broker *permission.MemoryBroker) {
	t.Helper()

	deadline := time.After(time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		if len(broker.Pending()) > 0 {
			return
		}
		select {
		case <-deadline:
			t.Fatal("permission request was not marked pending")
		case <-ticker.C:
		}
	}
}

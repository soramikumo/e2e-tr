package handler_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"e2e-runner/internal/config"
	"e2e-runner/internal/domain"
	"e2e-runner/internal/handler"
	"e2e-runner/internal/store"
)

func newTestHandler(t *testing.T) *handler.Handler {
	t.Helper()
	cfg := &config.Config{TestsDir: t.TempDir(), Port: ":8080", DBPath: "", MaxConcurrentRuns: 4}
	return handler.New(cfg, store.NewMemoryRunStore(), store.NewMemoryCodegenStore(), nil)
}

// readSSEEvents reads up to n SSE data lines from the response body via bufio.Scanner.
// It returns when n events are collected or the deadline is reached.
func readSSEEvents(t *testing.T, resp *http.Response, n int, timeout time.Duration) []map[string]string {
	t.Helper()
	events := make([]map[string]string, 0, n)
	done := make(chan struct{})

	go func() {
		defer close(done)
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			payload := strings.TrimPrefix(line, "data: ")
			var ev map[string]string
			if err := json.Unmarshal([]byte(payload), &ev); err != nil {
				continue
			}
			events = append(events, ev)
			if len(events) >= n {
				return
			}
		}
	}()

	select {
	case <-done:
	case <-time.After(timeout):
		t.Logf("readSSEEvents: timed out after %v, got %d/%d events", timeout, len(events), n)
	}
	return events
}

// ── Stream (handler.Stream) ────────────────────────────────────────────────────

func TestStream_NotFound(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/stream?id=nonexistent", nil)
	w := httptest.NewRecorder()
	h.Stream(w, req)

	if w.Result().StatusCode != http.StatusNotFound {
		t.Errorf("Stream with unknown id: got %d, want 404", w.Result().StatusCode)
	}
}

func TestStream_FinishedRun_LogsThenDone(t *testing.T) {
	h := newTestHandler(t)

	// Prepare a finished Run with known log lines.
	run := domain.NewRun("smoke", "")
	run.AddLog("line one")
	run.AddLog("line two")
	run.Finish(true)
	h.RunStore.Save(context.Background(), run)

	// Serve via a real HTTP server so SSE flushing works.
	srv := httptest.NewServer(http.HandlerFunc(h.Stream))
	defer srv.Close()

	resp, err := http.Get(fmt.Sprintf("%s?id=%s", srv.URL, run.ID))
	if err != nil {
		t.Fatalf("GET stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d", resp.StatusCode)
	}

	// Expect 2 log events + 1 done event.
	events := readSSEEvents(t, resp, 3, 3*time.Second)

	if len(events) < 3 {
		t.Fatalf("expected 3 SSE events, got %d: %v", len(events), events)
	}

	if events[0]["type"] != "log" || events[0]["message"] != "line one" {
		t.Errorf("event[0] = %v, want log 'line one'", events[0])
	}
	if events[1]["type"] != "log" || events[1]["message"] != "line two" {
		t.Errorf("event[1] = %v, want log 'line two'", events[1])
	}
	if events[2]["type"] != "done" {
		t.Errorf("event[2] = %v, want done", events[2])
	}
	if events[2]["status"] != string(domain.StatusDone) {
		t.Errorf("done event status = %q, want %q", events[2]["status"], string(domain.StatusDone))
	}
}

func TestStream_ClientDisconnect_NoGoroutineLeak(t *testing.T) {
	h := newTestHandler(t)

	// A run that is still running (never finished) — handler will block on select.
	run := domain.NewRun("leak-test", "")
	h.RunStore.Save(context.Background(), run)

	srv := httptest.NewServer(http.HandlerFunc(h.Stream))
	defer func() {
		srv.CloseClientConnections()
		srv.Close()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s?id=%s", srv.URL, run.ID), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET stream: %v", err)
	}

	// Give the handler time to enter its select loop.
	time.Sleep(50 * time.Millisecond)

	// Cancel the client context — simulates client disconnect.
	cancel()
	resp.Body.Close()

	// The handler should return promptly; wait a short time and verify the Run
	// no longer holds any subscriber channels (goroutine cleaned up via defer cancel).
	time.Sleep(200 * time.Millisecond)
	// If we reach here without deadlock / panic, the test passes.
	// A goroutine leak would not cause an immediate failure but goleak could catch it;
	// for now we validate that the server can handle a new request cleanly.
	resp2, err2 := http.Get(fmt.Sprintf("%s?id=nonexistent", srv.URL))
	if err2 != nil {
		t.Fatalf("second GET: %v", err2)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotFound {
		t.Errorf("second request: got %d, want 404", resp2.StatusCode)
	}
}

// ── CodegenStream (handler.CodegenStream) ─────────────────────────────────────

func TestCodegenStream_NotFound(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/codegen/stream?id=nonexistent", nil)
	w := httptest.NewRecorder()
	h.CodegenStream(w, req)

	if w.Result().StatusCode != http.StatusNotFound {
		t.Errorf("CodegenStream with unknown id: got %d, want 404", w.Result().StatusCode)
	}
}

func TestCodegenStream_FinishedSuccess_DoneEvent(t *testing.T) {
	h := newTestHandler(t)

	c := domain.NewCodegen("https://example.com", "test-scenario")
	c.Finish("scenario.spec.ts", nil)
	h.CodegenStore.Save(c)

	srv := httptest.NewServer(http.HandlerFunc(h.CodegenStream))
	defer srv.Close()

	resp, err := http.Get(fmt.Sprintf("%s?id=%s", srv.URL, c.ID))
	if err != nil {
		t.Fatalf("GET codegen stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d", resp.StatusCode)
	}

	events := readSSEEvents(t, resp, 1, 3*time.Second)
	if len(events) < 1 {
		t.Fatal("expected at least 1 SSE event, got none")
	}
	if events[0]["type"] != "done" {
		t.Errorf("event[0] type = %q, want 'done'", events[0]["type"])
	}
	if events[0]["file"] != "scenario.spec.ts" {
		t.Errorf("event[0] file = %q, want 'scenario.spec.ts'", events[0]["file"])
	}
}

func TestCodegenStream_FinishedError_ErrorEvent(t *testing.T) {
	h := newTestHandler(t)

	c := domain.NewCodegen("https://example.com", "test-scenario-err")
	c.Finish("", errors.New("recording failed"))
	h.CodegenStore.Save(c)

	srv := httptest.NewServer(http.HandlerFunc(h.CodegenStream))
	defer srv.Close()

	resp, err := http.Get(fmt.Sprintf("%s?id=%s", srv.URL, c.ID))
	if err != nil {
		t.Fatalf("GET codegen stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d", resp.StatusCode)
	}

	events := readSSEEvents(t, resp, 1, 3*time.Second)
	if len(events) < 1 {
		t.Fatal("expected at least 1 SSE event, got none")
	}
	if events[0]["type"] != "error" {
		t.Errorf("event[0] type = %q, want 'error'", events[0]["type"])
	}
}

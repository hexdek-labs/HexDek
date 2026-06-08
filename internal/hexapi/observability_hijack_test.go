package hexapi

import (
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// observability_hijack_test.go — pins that observabilityResponseWriter
// forwards http.Hijacker / http.Flusher / http.Pusher to the wrapped
// writer. The regression: the wrapper embeds the http.ResponseWriter
// *interface*, so a concrete Hijack on the underlying value is not
// promoted. Before the fix it implemented Flush but NOT Hijack, so
// every WebSocket upgrade (/ws/live, /ws/spectate, /ws/party) failed
// with "does not implement http.Hijacker" (HTTP 501) even though plain
// HTTP/1.1 connections are hijackable.

// hijackableRW is a minimal ResponseWriter that also implements
// Hijacker + Flusher + Pusher, recording whether each was invoked.
type hijackableRW struct {
	http.ResponseWriter
	hijacked bool
	flushed  bool
	pushed   string
}

func (h *hijackableRW) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h.hijacked = true
	return nil, nil, nil // nil conn is fine; we only assert the call threaded through
}
func (h *hijackableRW) Flush() { h.flushed = true }
func (h *hijackableRW) Push(target string, _ *http.PushOptions) error {
	h.pushed = target
	return nil
}

func newWrapped(inner http.ResponseWriter) *observabilityResponseWriter {
	return &observabilityResponseWriter{ResponseWriter: inner, peekCap: errorPeekCap}
}

func TestObservabilityWriter_SatisfiesHijacker(t *testing.T) {
	ow := newWrapped(&hijackableRW{ResponseWriter: httptest.NewRecorder()})
	if _, ok := interface{}(ow).(http.Hijacker); !ok {
		t.Fatal("observabilityResponseWriter must satisfy http.Hijacker — WS upgrades depend on it")
	}
}

func TestObservabilityWriter_HijackThreadsThrough(t *testing.T) {
	inner := &hijackableRW{ResponseWriter: httptest.NewRecorder()}
	ow := newWrapped(inner)
	if _, _, err := ow.Hijack(); err != nil {
		t.Fatalf("Hijack returned error: %v", err)
	}
	if !inner.hijacked {
		t.Error("Hijack did not thread through to the underlying writer")
	}
}

// When the underlying writer can't hijack, return an error rather than
// panicking — the WS handler surfaces it as a clean upgrade failure.
func TestObservabilityWriter_HijackErrorsWhenUnsupported(t *testing.T) {
	ow := newWrapped(httptest.NewRecorder()) // ResponseRecorder is not a Hijacker
	if _, _, err := ow.Hijack(); err == nil {
		t.Error("Hijack should error when the underlying writer is not a Hijacker")
	}
}

func TestObservabilityWriter_FlushAndPushThreadThrough(t *testing.T) {
	inner := &hijackableRW{ResponseWriter: httptest.NewRecorder()}
	ow := newWrapped(inner)

	if _, ok := interface{}(ow).(http.Flusher); !ok {
		t.Fatal("observabilityResponseWriter must satisfy http.Flusher — SSE depends on it")
	}
	ow.Flush()
	if !inner.flushed {
		t.Error("Flush did not thread through")
	}

	if _, ok := interface{}(ow).(http.Pusher); !ok {
		t.Fatal("observabilityResponseWriter must satisfy http.Pusher")
	}
	if err := ow.Push("/x", nil); err != nil {
		t.Fatalf("Push returned error: %v", err)
	}
	if inner.pushed != "/x" {
		t.Errorf("Push threaded target=%q, want /x", inner.pushed)
	}
}

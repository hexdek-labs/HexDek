package telemetry

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ga4_r60_test pins the small set of decision branches in ga4.go:
// NewGA4Client's env-driven init (both vars present = real client,
// either missing = nil), SendEvent's nil-receiver safety, and the
// JSON-marshal-then-POST happy path against an httptest server. The
// audit flagged ga4.go at 0% coverage because nothing in the unit-test
// suite ever exercised the GA4 emitter — these tests close that gap
// without sending real GA4 traffic.

// TestNewGA4Client_RequiresBothEnvVars verifies the gating: the client
// only initializes when BOTH HEXDEK_GA4_MEASUREMENT_ID and
// HEXDEK_GA4_API_SECRET are set. A missing-or-empty either-or returns
// nil so callers can safely no-op telemetry in dev / CI environments
// without paths that don't have a GA4 measurement ID.
func TestNewGA4Client_RequiresBothEnvVars(t *testing.T) {
	cases := []struct {
		name      string
		mid, sec  string
		wantNil   bool
	}{
		{"both set", "G-XYZ", "secret123", false},
		{"missing id", "", "secret123", true},
		{"missing secret", "G-XYZ", "", true},
		{"both empty", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HEXDEK_GA4_MEASUREMENT_ID", tc.mid)
			t.Setenv("HEXDEK_GA4_API_SECRET", tc.sec)
			c := NewGA4Client()
			if (c == nil) != tc.wantNil {
				t.Errorf("NewGA4Client() nil=%v, want nil=%v", c == nil, tc.wantNil)
			}
			if c != nil {
				if c.MeasurementID != tc.mid || c.APISecret != tc.sec {
					t.Errorf("client fields not populated: %+v", c)
				}
				if c.ClientID == "" {
					t.Errorf("ClientID should default to 'hexdek-engine-1', got empty")
				}
				if c.httpClient == nil {
					t.Errorf("httpClient should be initialized")
				}
			}
		})
	}
}

// TestSendEvent_NilReceiverSafe verifies SendEvent on a nil *GA4Client
// is a no-op (no panic, no network call). This is load-bearing because
// NewGA4Client returns nil in dev environments and all SendEvent call
// sites assume nil-safety.
func TestSendEvent_NilReceiverSafe(t *testing.T) {
	var c *GA4Client // nil
	// Must not panic.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("SendEvent on nil receiver panicked: %v", r)
		}
	}()
	c.SendEvent("test_event", map[string]interface{}{"k": "v"})
}

// TestSendEvent_PostsJSONPayload verifies the wire format: SendEvent
// marshals the event to JSON, POSTs to the GA4 endpoint with both
// query parameters set. We redirect the httpClient to an httptest
// server to inspect the request without sending real GA4 traffic.
func TestSendEvent_PostsJSONPayload(t *testing.T) {
	var (
		gotPath  string
		gotQuery string
		gotBody  []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		body, _ := io.ReadAll(r.Body)
		gotBody = body
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Build the client directly (NewGA4Client always points at the real
	// google-analytics.com URL; the SendEvent URL is hardcoded so we
	// can't redirect via env alone). We patch httpClient + use a
	// MeasurementID that we'll grep for in the captured query string.
	// However, the production code hardcodes the GA4 base URL, so we
	// can't fully redirect — instead, exercise the payload-build path
	// and verify json marshal didn't error by checking the client's
	// pre-post state through the public surface.
	//
	// Pragmatic compromise: build a client with bogus creds, call
	// SendEvent with an httptest-backed httpClient via a custom
	// transport that records the POST. We override URL via a custom
	// http.RoundTripper so the production URL constant doesn't matter.
	c := &GA4Client{
		MeasurementID: "G-TEST",
		APISecret:     "test-secret",
		ClientID:      "test-client",
		httpClient: &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				gotPath = req.URL.Path
				gotQuery = req.URL.RawQuery
				body, _ := io.ReadAll(req.Body)
				gotBody = body
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader("")),
					Header:     make(http.Header),
				}, nil
			}),
		},
	}
	c.SendEvent("game_complete", map[string]interface{}{"turns": 12, "winner": "Atraxa"})

	if gotPath != "/mp/collect" {
		t.Errorf("path = %q, want %q", gotPath, "/mp/collect")
	}
	if !strings.Contains(gotQuery, "measurement_id=G-TEST") {
		t.Errorf("query missing measurement_id: %q", gotQuery)
	}
	if !strings.Contains(gotQuery, "api_secret=test-secret") {
		t.Errorf("query missing api_secret: %q", gotQuery)
	}
	if !strings.Contains(string(gotBody), `"game_complete"`) {
		t.Errorf("body missing event name: %s", gotBody)
	}
	if !strings.Contains(string(gotBody), `"client_id":"test-client"`) {
		t.Errorf("body missing client_id: %s", gotBody)
	}
}

// TestSendEvent_NetworkErrorDoesNotPanic verifies that a failing POST
// (network error / timeout / dropped connection) returns silently
// without panicking. GA4 telemetry is best-effort fire-and-forget;
// failure to send must never bubble out into the calling game-engine
// code.
func TestSendEvent_NetworkErrorDoesNotPanic(t *testing.T) {
	c := &GA4Client{
		MeasurementID: "G-TEST",
		APISecret:     "test-secret",
		ClientID:      "test-client",
		httpClient: &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				return nil, io.ErrUnexpectedEOF
			}),
		},
	}
	// Must not panic. No error returned (GA4 emitter is fire-and-forget).
	c.SendEvent("test_event", nil)
}

// roundTripperFunc adapts a plain function into the http.RoundTripper
// interface so the GA4 tests can intercept the POST without spinning
// up an httptest.Server (the production URL constant is hardcoded to
// google-analytics.com so a real server wouldn't be reachable anyway).
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

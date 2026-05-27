package hexapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql/gqlerrors"
)

// Pins the R60 unification of subscription Error-frame payload shapes.
// Prior to the refactor, two code paths emitted divergent payloads:
//
//   - graphql.Subscribe validation errors (line 424-428): full
//     gqlerrors.FormattedError shape with message/locations/path,
//     but no extensions.code.
//
//   - emitError (line 447-449): bare []map[string]any{{"message": ...}},
//     no locations / path / extensions.
//
// Post-refactor, every Error-frame payload is a JSON array whose entries
// always carry: message (string) + extensions.code (string). graphql-go
// validation errors are re-stamped via stampErrorCodes before write;
// emitError-built errors use the gqlerrors.FormattedError shape directly.

// readErrorFrame opens a connection, runs init, sends Subscribe with the
// given payload, and returns the parsed error-frame payload.
func readErrorFrame(t *testing.T, payload []byte) []map[string]any {
	t.Helper()
	subs, _ := seedSubscriptionsHandler(t)
	srv := httptest.NewServer(http.HandlerFunc(subs.serve))
	t.Cleanup(srv.Close)

	conn, _, _ := dialSubscriptions(t, srv)
	t.Cleanup(func() { _ = conn.CloseNow() })
	sendMessage(t, conn, gqlwsMessage{Type: gqlwsConnectionInit})
	readNextNonPing(t, conn, 2*time.Second)

	sendMessage(t, conn, gqlwsMessage{ID: "op1", Type: gqlwsSubscribe, Payload: payload})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		msg := readMessage(t, conn, time.Until(deadline))
		if msg.Type == gqlwsPing {
			continue
		}
		if msg.Type != gqlwsError {
			t.Fatalf("expected error frame, got %q", msg.Type)
		}
		var entries []map[string]any
		if err := json.Unmarshal(msg.Payload, &entries); err != nil {
			t.Fatalf("error payload not a JSON array of objects: %v (body=%s)", err, msg.Payload)
		}
		return entries
	}
	t.Fatalf("no error frame observed within 2s")
	return nil
}

// TestSubscriptionErrors_ValidationCarriesCode: a malformed query reaches
// graphql.Subscribe and produces a FormattedError with no extensions —
// the runSubscription stamping pass must add extensions.code so the
// outgoing payload matches emitError's shape.
func TestSubscriptionErrors_ValidationCarriesCode(t *testing.T) {
	body, _ := json.Marshal(gqlwsSubscribePayload{Query: `subscription { gameEnded { id`})
	entries := readErrorFrame(t, body)

	if len(entries) == 0 {
		t.Fatalf("expected at least one error entry")
	}
	first := entries[0]
	msg, _ := first["message"].(string)
	if msg == "" {
		t.Errorf("entry missing message: %v", first)
	}
	ext, ok := first["extensions"].(map[string]any)
	if !ok {
		t.Fatalf("entry missing extensions object: %v", first)
	}
	code, _ := ext["code"].(string)
	if code != errCodeGraphQLValidation {
		t.Errorf("validation error code: want %q, got %q (entry=%v)", errCodeGraphQLValidation, code, first)
	}
}

// TestSubscriptionErrors_BadPayloadCarriesBadRequestCode: a Subscribe
// frame whose Payload is unparseable JSON exercises the emitError path
// (handleSubscribe line 340). Its payload must be a single FormattedError-
// shaped entry with extensions.code = BAD_REQUEST.
func TestSubscriptionErrors_BadPayloadCarriesBadRequestCode(t *testing.T) {
	// Valid JSON envelope but not a Subscribe payload struct (an array
	// where the handler expects {"query": "..."}).
	entries := readErrorFrame(t, []byte(`["not", "a", "subscribe", "payload"]`))

	if len(entries) != 1 {
		t.Fatalf("expected exactly one error entry, got %d (entries=%v)", len(entries), entries)
	}
	first := entries[0]
	msg, _ := first["message"].(string)
	if !strings.Contains(msg, "invalid Subscribe payload") {
		t.Errorf("entry message: want 'invalid Subscribe payload...', got %q", msg)
	}
	ext, ok := first["extensions"].(map[string]any)
	if !ok {
		t.Fatalf("entry missing extensions object: %v", first)
	}
	code, _ := ext["code"].(string)
	if code != errCodeBadRequest {
		t.Errorf("bad-request code: want %q, got %q (entry=%v)", errCodeBadRequest, code, first)
	}
}

// TestStampErrorCodes_DefaultFillsMissing: graphql-go validation errors
// arrive with no Extensions. The stamper must add the fallback code
// without touching any other field.
func TestStampErrorCodes_DefaultFillsMissing(t *testing.T) {
	in := []gqlerrors.FormattedError{
		{Message: "syntax error"},
	}
	out := stampErrorCodes(in, errCodeGraphQLValidation)

	if len(out) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(out))
	}
	if out[0].Message != "syntax error" {
		t.Errorf("message mutated: got %q", out[0].Message)
	}
	code, _ := out[0].Extensions["code"].(string)
	if code != errCodeGraphQLValidation {
		t.Errorf("code: want %q, got %q", errCodeGraphQLValidation, code)
	}
}

// TestStampErrorCodes_PreservesExistingCode: if a resolver already
// supplied an extensions.code (e.g. UNAUTHENTICATED from a future auth
// directive), the stamper must NOT overwrite it with the fallback.
func TestStampErrorCodes_PreservesExistingCode(t *testing.T) {
	in := []gqlerrors.FormattedError{
		{
			Message:    "must be logged in",
			Extensions: map[string]interface{}{"code": "UNAUTHENTICATED"},
		},
	}
	out := stampErrorCodes(in, errCodeGraphQLValidation)

	code, _ := out[0].Extensions["code"].(string)
	if code != "UNAUTHENTICATED" {
		t.Errorf("existing code overwritten: want %q, got %q", "UNAUTHENTICATED", code)
	}
}

// TestStampErrorCodes_DoesNotMutateInput: the stamper must operate on a
// copied Extensions map so a caller reusing the input slice isn't
// surprised by mutations.
func TestStampErrorCodes_DoesNotMutateInput(t *testing.T) {
	original := map[string]interface{}{"hint": "check syntax"}
	in := []gqlerrors.FormattedError{
		{Message: "boom", Extensions: original},
	}
	_ = stampErrorCodes(in, errCodeGraphQLValidation)

	if _, ok := original["code"]; ok {
		t.Errorf("stamper mutated caller's extensions map: %v", original)
	}
	if len(original) != 1 {
		t.Errorf("stamper grew caller's extensions: %v", original)
	}
}

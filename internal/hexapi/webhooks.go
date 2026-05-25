package hexapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hexdek/hexdek/internal/db"
)

// Webhook event types. Strings on the wire so we can add new events
// without a schema migration.
const (
	WebhookEventGameEnd = "game.end"
)

// supportedWebhookEvents enumerates the event types the API accepts
// at register time. Reject anything else with 400 — better to fail
// loudly than silently store a typo that will never fire.
var supportedWebhookEvents = map[string]bool{
	WebhookEventGameEnd: true,
}

// Default failure threshold past which the dispatcher auto-disables
// a webhook. Five consecutive transport errors / 5xx — enough to
// ride out a brief outage on the consumer side without spamming a
// dead endpoint forever.
const webhookFailureThreshold = 5

// Default per-delivery timeout. Webhook receivers should ack fast;
// anything past ten seconds is on its own.
const webhookDeliveryTimeout = 10 * time.Second

// WebhookDispatcher fires registered webhooks for engine events.
// Construct via NewWebhookDispatcher; wire into Handler.Webhooks and
// expose POST/GET/DELETE /api/webhooks via RegisterWebhookRoutes.
//
// The dispatcher owns no goroutines at rest — Fire spawns one short-
// lived goroutine per matching subscription. That keeps the engine
// loop's hot path predictable (no shared queue, no backpressure)
// at the cost of unbounded fan-out under pathological event rates;
// the auto-disable on consecutive failures is the backpressure.
type WebhookDispatcher struct {
	db     *sql.DB
	client *http.Client
	logf   func(string, ...any)

	// failureThreshold and deliveryTimeout are overridable in tests
	// so a failure-count assertion can hit the disable cutoff in
	// three iterations rather than five, and so a slow-server test
	// can run without waiting ten seconds.
	failureThreshold int
	deliveryTimeout  time.Duration

	// inflight is bumped each Fire call so tests can wait for fan-out
	// to drain without sleeping.
	inflight sync.WaitGroup
}

// NewWebhookDispatcher returns a dispatcher bound to sqlDB. The
// http.Client is owned by the dispatcher (so test wiring can swap a
// transport with a recorder) — pass nil to get a sensible default
// with the package's delivery timeout.
func NewWebhookDispatcher(sqlDB *sql.DB, client *http.Client) *WebhookDispatcher {
	if client == nil {
		client = &http.Client{Timeout: webhookDeliveryTimeout}
	}
	return &WebhookDispatcher{
		db:               sqlDB,
		client:           client,
		logf:             log.Printf,
		failureThreshold: webhookFailureThreshold,
		deliveryTimeout:  webhookDeliveryTimeout,
	}
}

// SetLogf overrides the dispatcher's log sink. Used by tests to
// pin the warn output without log.Printf scribbling to stderr.
func (d *WebhookDispatcher) SetLogf(f func(string, ...any)) {
	if f != nil {
		d.logf = f
	}
}

// SetFailureThreshold lets tests narrow the auto-disable cutoff.
// Values < 1 disable the auto-disable behaviour entirely.
func (d *WebhookDispatcher) SetFailureThreshold(n int) { d.failureThreshold = n }

// Wait blocks until every in-flight fan-out spawned by Fire has
// completed. Test-only — production callers use Fire and forget.
func (d *WebhookDispatcher) Wait() { d.inflight.Wait() }

// Fire loads every active webhook for eventType and spawns a
// short-lived goroutine per subscription to POST payload to its URL.
// Returns immediately; the engine loop never blocks on a slow
// consumer.
//
// payload is JSON-marshalled once; each delivery shares the same
// body bytes so the HMAC signature is identical across receivers.
// If marshalling fails the dispatcher logs and returns without
// firing — better silent than to POST garbage.
func (d *WebhookDispatcher) Fire(ctx context.Context, eventType string, payload any) {
	if d == nil || d.db == nil {
		return
	}
	if !supportedWebhookEvents[eventType] {
		d.logf("webhook: refusing to fire unsupported event %q", eventType)
		return
	}
	body, err := json.Marshal(payload)
	if err != nil {
		d.logf("webhook: marshal payload for %s: %v", eventType, err)
		return
	}
	rows, err := db.ListActiveWebhooksForEvent(ctx, d.db, eventType)
	if err != nil {
		d.logf("webhook: list %s subscribers: %v", eventType, err)
		return
	}
	for _, w := range rows {
		d.inflight.Add(1)
		go func(w db.Webhook) {
			defer d.inflight.Done()
			d.deliver(eventType, body, w)
		}(w)
	}
}

// deliver POSTs body to one subscriber, signed with the webhook's
// secret, and records the outcome via UpdateWebhookHealth. Failures
// past the configured threshold flip the row inactive in the same
// SQL UPDATE so the next Fire skips it.
func (d *WebhookDispatcher) deliver(eventType string, body []byte, w db.Webhook) {
	ctx, cancel := context.WithTimeout(context.Background(), d.deliveryTimeout)
	defer cancel()

	mac := hmac.New(sha256.New, []byte(w.Secret))
	mac.Write(body)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL, bytes.NewReader(body))
	if err != nil {
		d.logf("webhook %d: build request: %v", w.ID, err)
		_ = db.UpdateWebhookHealth(context.Background(), d.db, w.ID, -1, d.failureThreshold)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-HexDek-Event", eventType)
	req.Header.Set("X-HexDek-Signature", signature)
	req.Header.Set("X-HexDek-Timestamp", timestamp)
	req.Header.Set("X-HexDek-Webhook-Id", strconv.FormatInt(w.ID, 10))
	req.Header.Set("User-Agent", "hexdek-webhook/1")

	resp, err := d.client.Do(req)
	if err != nil {
		d.logf("webhook %d (%s): transport error: %v", w.ID, w.URL, err)
		_ = db.UpdateWebhookHealth(context.Background(), d.db, w.ID, -1, d.failureThreshold)
		return
	}
	// Drain + close per http.Client convention so the keep-alive
	// connection is returned to the pool.
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		d.logf("webhook %d (%s): non-2xx %d", w.ID, w.URL, resp.StatusCode)
	}
	_ = db.UpdateWebhookHealth(context.Background(), d.db, w.ID, resp.StatusCode, d.failureThreshold)
}

// FireGameEnd is the typed convenience the showmatch loop calls
// from RunGame's tail. Wraps the CompletedGame in the documented
// JSON shape — game_id / winner / commanders / deck_keys / turns /
// end_reason / finished_at — so consumers don't have to depend on
// the full CompletedGame struct (which carries timeline + final
// seats — too much for a webhook payload).
func (d *WebhookDispatcher) FireGameEnd(ctx context.Context, g CompletedGame) {
	if d == nil {
		return
	}
	payload := map[string]any{
		"event":       WebhookEventGameEnd,
		"game_id":     g.GameID,
		"winner":      g.Winner,
		"winner_name": g.WinnerName,
		"commanders":  g.Commanders,
		"deck_keys":   g.DeckKeys,
		"turns":       g.Turns,
		"end_reason":  g.EndReason,
		"finished_at": g.FinishedAt.UTC().Format(time.RFC3339),
	}
	d.Fire(ctx, WebhookEventGameEnd, payload)
}

// -------------------------------------------------------------------
// HTTP layer
// -------------------------------------------------------------------

// RegisterWebhookRoutes wires the register / list / delete handlers
// onto mux. Call from the same Register block that mounts the other
// hexapi endpoints. The Handler must have its DB field set; without
// a DB the handlers return 503.
func (h *Handler) RegisterWebhookRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/webhooks", h.handleRegisterWebhook)
	mux.HandleFunc("GET /api/webhooks", h.handleListWebhooks)
	mux.HandleFunc("DELETE /api/webhooks/{id}", h.handleDeleteWebhook)
}

// SetDB injects the SQLite handle the webhook handlers use. Kept
// separate so the handler struct can stay backwards-compatible —
// callers that don't construct a webhook DB pay nothing.
//
// (Companion to SetDB on the Showmatch path; that one feeds card
// stats. Webhooks need their own EnsureWebhooksSchema invocation
// at startup.)

// webhookRegisterRequest is the POST /api/webhooks JSON body.
type webhookRegisterRequest struct {
	URL       string `json:"url"`
	EventType string `json:"event_type"`
}

// webhookRegisterResponse is the body returned on successful
// registration. Secret is included ONCE — the caller is responsible
// for storing it (list / get endpoints intentionally omit it).
type webhookRegisterResponse struct {
	ID        int64  `json:"id"`
	URL       string `json:"url"`
	EventType string `json:"event_type"`
	Secret    string `json:"secret"`
	CreatedAt int64  `json:"created_at"`
}

// webhookListEntry is one row in GET /api/webhooks. No Secret field —
// once the client stored it on register, hexapi never echoes it.
type webhookListEntry struct {
	ID          int64  `json:"id"`
	URL         string `json:"url"`
	EventType   string `json:"event_type"`
	CreatedAt   int64  `json:"created_at"`
	LastFiredAt int64  `json:"last_fired_at,omitempty"`
	LastStatus  int    `json:"last_status,omitempty"`
	Failures    int    `json:"failures,omitempty"`
	Active      bool   `json:"active"`
}

func (h *Handler) handleRegisterWebhook(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		writeError(w, http.StatusServiceUnavailable, "database not configured")
		return
	}
	owner := strings.ToLower(strings.TrimSpace(r.Header.Get("X-HexDek-Owner")))
	if owner == "" {
		writeError(w, http.StatusUnauthorized, "missing X-HexDek-Owner header")
		return
	}
	var req webhookRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := validateWebhookURL(req.URL); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !supportedWebhookEvents[req.EventType] {
		writeError(w, http.StatusBadRequest, "unsupported event_type")
		return
	}
	secret, err := generateWebhookSecret()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "secret generation failed")
		return
	}
	id, err := db.InsertWebhook(r.Context(), h.db, owner, req.URL, req.EventType, secret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "insert failed")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(webhookRegisterResponse{
		ID:        id,
		URL:       req.URL,
		EventType: req.EventType,
		Secret:    secret,
		CreatedAt: time.Now().Unix(),
	})
}

func (h *Handler) handleListWebhooks(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		writeError(w, http.StatusServiceUnavailable, "database not configured")
		return
	}
	owner := strings.ToLower(strings.TrimSpace(r.Header.Get("X-HexDek-Owner")))
	if owner == "" {
		writeError(w, http.StatusUnauthorized, "missing X-HexDek-Owner header")
		return
	}
	rows, err := db.ListWebhooksByOwner(r.Context(), h.db, owner)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list failed")
		return
	}
	out := make([]webhookListEntry, 0, len(rows))
	for _, w := range rows {
		entry := webhookListEntry{
			ID:         w.ID,
			URL:        w.URL,
			EventType:  w.EventType,
			CreatedAt:  w.CreatedAt.Unix(),
			LastStatus: w.LastStatus,
			Failures:   w.Failures,
			Active:     w.Active,
		}
		if !w.LastFiredAt.IsZero() {
			entry.LastFiredAt = w.LastFiredAt.Unix()
		}
		out = append(out, entry)
	}
	writeJSON(w, map[string]any{"owner": owner, "webhooks": out})
}

func (h *Handler) handleDeleteWebhook(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		writeError(w, http.StatusServiceUnavailable, "database not configured")
		return
	}
	owner := strings.ToLower(strings.TrimSpace(r.Header.Get("X-HexDek-Owner")))
	if owner == "" {
		writeError(w, http.StatusUnauthorized, "missing X-HexDek-Owner header")
		return
	}
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid webhook id")
		return
	}
	n, err := db.DeleteWebhookByOwner(r.Context(), h.db, id, owner)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "delete failed")
		return
	}
	if n == 0 {
		// Could be a missing id OR an ownership mismatch — same 404
		// both ways so existence of someone else's webhook doesn't
		// leak to a probe.
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}
	writeJSON(w, map[string]any{"deleted": true, "id": id})
}

// -------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------

// validateWebhookURL enforces the surface a webhook URL is allowed
// to take. Requirements:
//   - parses cleanly via net/url
//   - scheme is http (loopback only) or https
//   - host is non-empty
//
// Public-internet SSRF protection (blocking RFC1918 ranges, link-
// local, etc.) is intentionally NOT enforced — HexDek is a single-
// engine deployment where the owner's own home server (192.168.1.*)
// is a legitimate target. The reject list is scheme-only.
func validateWebhookURL(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return errors.New("url required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid url: %v", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		// always allowed
	case "http":
		// only allowed for loopback so the test suite + local-dev
		// receivers work, but production endpoints have to use TLS.
		host := strings.ToLower(u.Hostname())
		if host != "localhost" && host != "127.0.0.1" && host != "::1" {
			return errors.New("http scheme only allowed for localhost; use https")
		}
	default:
		return errors.New("url scheme must be http or https")
	}
	if u.Host == "" {
		return errors.New("url must include a host")
	}
	return nil
}

// generateWebhookSecret returns a 32-byte hex-encoded random string
// suitable for HMAC keying. Sourced from crypto/rand — never a
// math/rand backend.
func generateWebhookSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

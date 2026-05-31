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
// a webhook. Five consecutive FAILED DELIVERIES (each delivery may
// retry up to MaxAttempts before counting as a failure) — enough to
// ride out a brief outage on the consumer side without spamming a
// dead endpoint forever.
const webhookFailureThreshold = 5

// Default per-attempt HTTP timeout. Webhook receivers should ack
// fast; anything past ten seconds is on its own.
const webhookDeliveryTimeout = 10 * time.Second

// Retry policy defaults. A delivery is tried up to MaxAttempts
// times for transient failures (5xx, 429, transport errors) with
// exponential backoff starting at BackoffBase and doubling each
// attempt, capped at BackoffMax. 4xx responses other than 429 are
// treated as permanent client errors and dead-letter immediately.
const (
	webhookDefaultMaxAttempts = 5
	webhookDefaultBackoffBase = 1 * time.Second
	webhookDefaultBackoffMax  = 30 * time.Second
)

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

	// Retry policy. Each delivery is tried up to maxAttempts times
	// for retriable failures (transport errors, 5xx, 429); each
	// retry waits backoffBase * 2^attempt, capped at backoffMax.
	// Tests override these to 1ms-scale so retry behaviour is
	// exercisable without real-time waits.
	maxAttempts int
	backoffBase time.Duration
	backoffMax  time.Duration

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
		maxAttempts:      webhookDefaultMaxAttempts,
		backoffBase:      webhookDefaultBackoffBase,
		backoffMax:       webhookDefaultBackoffMax,
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

// SetRetryPolicy overrides the per-delivery retry behaviour. Tests
// pass 1ms-scale durations so the backoff is exercisable in real
// time. maxAttempts < 1 means "single attempt, no retry"; pass 0
// or negative to disable retries entirely.
func (d *WebhookDispatcher) SetRetryPolicy(maxAttempts int, base, max time.Duration) {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	if base < 0 {
		base = 0
	}
	if max < base {
		max = base
	}
	d.maxAttempts = maxAttempts
	d.backoffBase = base
	d.backoffMax = max
}

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
// secret. Wraps the attempt loop in a retry-with-backoff envelope:
//
//   - 2xx        → success; failures reset, no retry.
//   - 4xx ≠ 429  → permanent client failure; dead-letter immediately,
//                  no retry (a bad URL / 401 / 410 won't recover).
//   - 5xx, 429   → transient; retry up to maxAttempts with backoff.
//                  429's Retry-After header is honored when numeric.
//   - transport  → transient; same retry treatment.
//
// On exhaustion, a webhook_dead_letters row is inserted carrying the
// full payload + last error so operators can replay. The webhooks
// row's failures counter is bumped ONCE per logical delivery (not
// per attempt) so the auto-disable threshold still measures
// consecutive-failed-deliveries, not consecutive-failed-attempts.
func (d *WebhookDispatcher) deliver(eventType string, body []byte, w db.Webhook) {
	firstAttempted := time.Now()
	maxAttempts := d.maxAttempts
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	var (
		lastStatus int
		lastErr    string
		attempt    int
		retryAfter time.Duration
	)

	for attempt = 1; attempt <= maxAttempts; attempt++ {
		status, err, ra := d.attemptDelivery(eventType, body, w)
		lastStatus = status
		retryAfter = ra
		if err != nil {
			lastErr = err.Error()
		} else {
			lastErr = ""
		}

		switch classifyDeliveryOutcome(status, err) {
		case deliverySuccess:
			_ = db.UpdateWebhookHealth(context.Background(), d.db, w.ID, status, d.failureThreshold)
			return
		case deliveryPermanent:
			// Don't retry — dead-letter immediately. failures still
			// bumps because the consumer-facing contract is "this
			// delivery did not succeed."
			d.recordDeadLetter(eventType, body, w, attempt, status, lastErr, firstAttempted)
			_ = db.UpdateWebhookHealth(context.Background(), d.db, w.ID, status, d.failureThreshold)
			return
		case deliveryTransient:
			// Fall through to retry (or exhaust below).
		}

		if attempt == maxAttempts {
			break
		}

		// Sleep with exponential backoff, honoring Retry-After when
		// the server provided it. Bounded by backoffMax so a buggy
		// server can't pin a goroutine for hours.
		wait := d.backoffFor(attempt)
		if retryAfter > 0 && retryAfter > wait {
			wait = retryAfter
		}
		if wait > d.backoffMax {
			wait = d.backoffMax
		}
		time.Sleep(wait)
	}

	// All attempts failed. Dead-letter + bump failures.
	// `attempt` is the loop counter at the moment the break fired,
	// which equals the total number of HTTP attempts made (the loop
	// runs 1..maxAttempts inclusive then breaks).
	d.logf("webhook %d (%s): exhausted %d attempts, last_status=%d err=%q",
		w.ID, w.URL, attempt, lastStatus, lastErr)
	d.recordDeadLetter(eventType, body, w, attempt, lastStatus, lastErr, firstAttempted)
	_ = db.UpdateWebhookHealth(context.Background(), d.db, w.ID, lastStatus, d.failureThreshold)
}

// deliveryOutcome classifies one HTTP-level result into the three
// states the retry loop branches on.
type deliveryOutcome int

const (
	deliverySuccess deliveryOutcome = iota
	deliveryTransient
	deliveryPermanent
)

// classifyDeliveryOutcome decides retry vs. give-up for one attempt.
// Transport errors and 5xx + 429 are transient; 4xx other than 429
// is permanent (URL bug, auth bug, target removed); 2xx is success.
// 3xx is treated as success — http.Client follows redirects by
// default, so we shouldn't normally see a raw 3xx, but if some
// pathological client did, treat it as "the consumer ack'd."
func classifyDeliveryOutcome(status int, err error) deliveryOutcome {
	if err != nil {
		return deliveryTransient
	}
	switch {
	case status >= 200 && status < 400:
		return deliverySuccess
	case status == http.StatusTooManyRequests:
		return deliveryTransient
	case status >= 400 && status < 500:
		return deliveryPermanent
	case status >= 500:
		return deliveryTransient
	default:
		// 1xx — unexpected for an application-level POST. Treat as
		// transient so the next attempt has a chance to do something
		// sensible.
		return deliveryTransient
	}
}

// backoffFor returns the wait before retry attempt `next` (1-based,
// so backoffFor(1) is the wait BEFORE the second attempt). Pure
// exponential with no jitter — the bounded fan-out (one goroutine
// per subscriber, max ~5 subscribers in practice) makes thundering
// herd a non-issue at this scale.
func (d *WebhookDispatcher) backoffFor(next int) time.Duration {
	if d.backoffBase <= 0 {
		return 0
	}
	wait := d.backoffBase
	for i := 1; i < next; i++ {
		wait *= 2
		if wait >= d.backoffMax {
			return d.backoffMax
		}
	}
	return wait
}

// attemptDelivery makes one HTTP POST. Returns (status, err,
// retryAfter). retryAfter is parsed from the Retry-After header
// when present + numeric; the caller can use it as a floor for
// the next backoff.
func (d *WebhookDispatcher) attemptDelivery(eventType string, body []byte, w db.Webhook) (int, error, time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), d.deliveryTimeout)
	defer cancel()

	mac := hmac.New(sha256.New, []byte(w.Secret))
	mac.Write(body)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL, bytes.NewReader(body))
	if err != nil {
		d.logf("webhook %d: build request: %v", w.ID, err)
		return -1, err, 0
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
		return -1, err, 0
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		d.logf("webhook %d (%s): non-2xx %d", w.ID, w.URL, resp.StatusCode)
	}
	return resp.StatusCode, nil, parseRetryAfter(resp.Header.Get("Retry-After"))
}

// parseRetryAfter understands the integer-seconds form of the
// Retry-After header. The HTTP-date form is rare on webhook
// receivers; we skip it to keep the parsing one-liner. Returns
// zero for missing / unparseable headers — the caller falls back
// to the configured backoff.
func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	secs, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || secs <= 0 {
		return 0
	}
	return time.Duration(secs) * time.Second
}

// recordDeadLetter inserts one row into webhook_dead_letters. Best-
// effort — log and continue if the insert fails; the operator-
// facing observability is the failure counter on the webhook row.
func (d *WebhookDispatcher) recordDeadLetter(eventType string, body []byte, w db.Webhook,
	attempts int, status int, lastErr string, firstAttempted time.Time) {
	now := time.Now()
	_, err := db.InsertWebhookDeadLetter(context.Background(), d.db, db.WebhookDeadLetter{
		WebhookID:      w.ID,
		EventType:      eventType,
		Payload:        append([]byte(nil), body...), // defensive copy
		Attempts:       attempts,
		LastStatus:     status,
		LastError:      lastErr,
		FirstAttempted: firstAttempted,
		LastAttempted:  now,
	})
	if err != nil {
		d.logf("webhook %d: dead-letter insert failed: %v", w.ID, err)
	}
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
	mux.HandleFunc("POST /api/webhooks", RequireCSRF(h.CSRFStore, h.handleRegisterWebhook))
	mux.HandleFunc("GET /api/webhooks", h.handleListWebhooks)
	mux.HandleFunc("DELETE /api/webhooks/{id}", RequireCSRF(h.CSRFStore, h.handleDeleteWebhook))
	mux.HandleFunc("GET /api/webhooks/{id}/dead-letters", h.handleListWebhookDeadLetters)
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

// -------------------------------------------------------------------
// Dead-letter listing
// -------------------------------------------------------------------

// webhookDeadLetterEntry is one row in the
// GET /api/webhooks/{id}/dead-letters response. Payload is returned
// as a string (the dispatcher writes JSON, so a string is the
// useful surface for the operator) — clients that want the raw
// bytes can decode it themselves.
type webhookDeadLetterEntry struct {
	ID             int64  `json:"id"`
	WebhookID      int64  `json:"webhook_id"`
	EventType      string `json:"event_type"`
	Payload        string `json:"payload"`
	Attempts       int    `json:"attempts"`
	LastStatus     int    `json:"last_status"`
	LastError      string `json:"last_error,omitempty"`
	FirstAttempted int64  `json:"first_attempted"`
	LastAttempted  int64  `json:"last_attempted"`
}

// handleListWebhookDeadLetters implements
// GET /api/webhooks/{id}/dead-letters. Owner-gated: only the owner of
// the webhook can list its dead-letters. A 404 is returned both for
// "no such webhook" AND "webhook owned by someone else" so the
// existence of someone else's subscription doesn't leak to a probe
// (mirrors handleDeleteWebhook's contract).
func (h *Handler) handleListWebhookDeadLetters(w http.ResponseWriter, r *http.Request) {
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

	// Ownership gate: confirm the webhook exists AND is owned by the
	// caller before reading its dead-letters. db.GetWebhook returns
	// sql.ErrNoRows on miss; both miss and ownership-mismatch surface
	// as 404 with the same message.
	hook, err := db.GetWebhook(r.Context(), h.db, id)
	if err != nil || hook.Owner != owner {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}

	limit := 50
	if q := strings.TrimSpace(r.URL.Query().Get("limit")); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 {
			if n > 500 {
				n = 500
			}
			limit = n
		}
	}

	rows, err := db.ListWebhookDeadLetters(r.Context(), h.db, id, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list failed")
		return
	}
	out := make([]webhookDeadLetterEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, webhookDeadLetterEntry{
			ID:             row.ID,
			WebhookID:      row.WebhookID,
			EventType:      row.EventType,
			Payload:        string(row.Payload),
			Attempts:       row.Attempts,
			LastStatus:     row.LastStatus,
			LastError:      row.LastError,
			FirstAttempted: row.FirstAttempted.Unix(),
			LastAttempted:  row.LastAttempted.Unix(),
		})
	}
	writeJSON(w, map[string]any{
		"webhook_id":   id,
		"dead_letters": out,
	})
}

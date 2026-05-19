// Package pincer is HexDek's session-tracking system — the "Temporal Pincer."
//
// Anonymous visitors get a UUID cookie (hexdek_session) on first visit. Every
// page view, deck view, game watch, or deck import is recorded against that
// UUID in SQLite. When the visitor authenticates, every UUID they've used
// across devices is stitched to their authenticated user_id, giving us a
// pre-auth → post-auth journey for the conversion funnel.
//
// The pincer closes from both directions: the anon cookie pins the session
// before login, the auth event pins it after. Stitching at the moment of
// auth preserves the entire pre-auth trail.
//
// Tables (created idempotently in EnsureSchema):
//   - pincer_session: one row per anon UUID. user_id is NULL until stitched.
//   - pincer_event:   append-only event log keyed by session UUID.
//   - pincer_stitch:  audit trail of (anon UUID → user_id) links.
package pincer

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	// CookieName is the session cookie set on every visitor's browser.
	CookieName = "hexdek_session"
	// CookieMaxAge keeps the cookie alive for a year by default.
	CookieMaxAge = 365 * 24 * 60 * 60

	// Standard event types. Anything else is fine too (RecordEvent takes a
	// free-form string), but these are the documented funnel touchpoints.
	EventPageViewed   = "page_viewed"
	EventDeckViewed   = "deck_viewed"
	EventGameWatched  = "game_watched"
	EventDeckImported = "deck_imported"
	EventLogin        = "login"
)

// Tracker is the pincer's facade over SQLite.
type Tracker struct {
	db         *sql.DB
	adminToken string // shared secret gating /api/analytics/sessions
	secure     bool   // sets the Secure flag on the cookie
}

// Options configures a Tracker. Both fields are optional.
type Options struct {
	// AdminToken gates /api/analytics/sessions. Empty string = endpoint
	// available to anyone (use only in dev). Compared case-sensitively.
	AdminToken string
	// SecureCookie toggles the Secure attribute on hexdek_session. Should
	// be true in production behind HTTPS, false for local http://localhost.
	SecureCookie bool
}

// New returns a Tracker with schema applied. Safe to call repeatedly.
func New(db *sql.DB, opts Options) (*Tracker, error) {
	t := &Tracker{db: db, adminToken: opts.AdminToken, secure: opts.SecureCookie}
	if err := t.EnsureSchema(context.Background()); err != nil {
		return nil, err
	}
	return t, nil
}

// EnsureSchema creates the pincer tables and indexes if they don't exist.
func (t *Tracker) EnsureSchema(ctx context.Context) error {
	const ddl = `
CREATE TABLE IF NOT EXISTS pincer_session (
    id            TEXT PRIMARY KEY,
    user_id       TEXT,                   -- NULL until stitched on auth
    first_seen_at INTEGER NOT NULL,
    last_seen_at  INTEGER NOT NULL,
    user_agent    TEXT NOT NULL DEFAULT '',
    ip_hash       TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_pincer_session_user ON pincer_session(user_id);
CREATE INDEX IF NOT EXISTS idx_pincer_session_lastseen ON pincer_session(last_seen_at);

CREATE TABLE IF NOT EXISTS pincer_event (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id   TEXT NOT NULL,
    event_type   TEXT NOT NULL,
    path         TEXT NOT NULL DEFAULT '',
    payload      TEXT NOT NULL DEFAULT '',
    occurred_at  INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_pincer_event_session ON pincer_event(session_id, occurred_at);
CREATE INDEX IF NOT EXISTS idx_pincer_event_type ON pincer_event(event_type, occurred_at);

CREATE TABLE IF NOT EXISTS pincer_stitch (
    session_id  TEXT NOT NULL,
    user_id     TEXT NOT NULL,
    stitched_at INTEGER NOT NULL,
    PRIMARY KEY (session_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_pincer_stitch_user ON pincer_stitch(user_id);
`
	_, err := t.db.ExecContext(ctx, ddl)
	return err
}

// ─────────────────────────── HTTP middleware ──────────────────────────

type ctxKey struct{}

var sessionCtxKey ctxKey

// FromContext returns the session UUID for the current request, or "" if
// no pincer middleware ran on this request.
func FromContext(ctx context.Context) string {
	id, _ := ctx.Value(sessionCtxKey).(string)
	return id
}

// Middleware attaches a hexdek_session cookie (creating one if missing) and
// records a page_viewed event for non-asset, non-API GET requests. The
// session UUID is available downstream via pincer.FromContext(r.Context()).
func (t *Tracker) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Don't issue cookies on preflight — that confuses some browsers,
		// and the actual request following will set it.
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		sid := readSessionCookie(r)
		fresh := false
		if sid == "" {
			sid = newUUID()
			fresh = true
			http.SetCookie(w, &http.Cookie{
				Name:     CookieName,
				Value:    sid,
				Path:     "/",
				MaxAge:   CookieMaxAge,
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
				Secure:   t.secure,
			})
		}

		// For brand-new sessions, write the row synchronously so that any
		// follow-up request carrying this cookie (e.g. an immediate POST
		// to /api/pincer/stitch on auth) finds the row to update — going
		// async here let Stitch's UPDATE silently affect 0 rows when the
		// goroutine hadn't run yet. Subsequent last_seen_at touches are
		// non-load-bearing and stay async.
		if fresh {
			t.touchSession(sid, true, r.UserAgent(), clientIPHash(r))
		} else {
			go t.touchSession(sid, false, r.UserAgent(), clientIPHash(r))
		}

		// Record a page view for content routes — skip API + static assets
		// so we don't spam events for every fetch.
		if r.Method == http.MethodGet && shouldTrackPage(r.URL.Path) {
			go t.recordEvent(sid, EventPageViewed, r.URL.Path, "")
		}
		// Map well-known URL patterns to typed funnel events. Doing this
		// here keeps the rest of the codebase free of pincer awareness.
		if eventType := classifyRequest(r); eventType != "" {
			go t.recordEvent(sid, eventType, r.URL.Path, "")
		}

		ctx := context.WithValue(r.Context(), sessionCtxKey, sid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func readSessionCookie(r *http.Request) string {
	c, err := r.Cookie(CookieName)
	if err != nil || c == nil {
		return ""
	}
	if !looksLikeUUID(c.Value) {
		return ""
	}
	return c.Value
}

// classifyRequest returns a typed funnel event for known endpoints, or ""
// if the request doesn't represent one. Pure path/method matching — runs
// before the handler resolves, so we record intent rather than success;
// a 404 GET /decks/foo/bar still counts as deck_viewed (funnel is about
// what the visitor tried, not what worked).
func classifyRequest(r *http.Request) string {
	p := r.URL.Path
	switch r.Method {
	case http.MethodGet:
		// Deck page (SPA route or API) — both indicate a deck view.
		if strings.HasPrefix(p, "/decks/") && countSlashes(p) == 3 {
			return EventDeckViewed
		}
		if strings.HasPrefix(p, "/api/decks/") && countSlashes(p) == 4 {
			return EventDeckViewed
		}
		// Live game watching — spectator HTTP poll or WS handshake.
		if p == "/api/live/game" || strings.HasPrefix(p, "/ws/showmatch") {
			return EventGameWatched
		}
	case http.MethodPost:
		if p == "/api/decks" || p == "/api/import/moxfield" {
			return EventDeckImported
		}
	}
	return ""
}

func countSlashes(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			n++
		}
	}
	return n
}

func shouldTrackPage(path string) bool {
	switch {
	case strings.HasPrefix(path, "/api/"),
		strings.HasPrefix(path, "/ws/"),
		strings.HasPrefix(path, "/assets/"),
		strings.HasPrefix(path, "/static/"),
		strings.HasSuffix(path, ".js"),
		strings.HasSuffix(path, ".css"),
		strings.HasSuffix(path, ".png"),
		strings.HasSuffix(path, ".jpg"),
		strings.HasSuffix(path, ".svg"),
		strings.HasSuffix(path, ".ico"),
		strings.HasSuffix(path, ".woff"),
		strings.HasSuffix(path, ".woff2"),
		path == "/health":
		return false
	}
	return true
}

// ─────────────────────────── persistence ──────────────────────────────

func (t *Tracker) touchSession(id string, fresh bool, ua, ipHash string) {
	now := time.Now().Unix()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if fresh {
		_, _ = t.db.ExecContext(ctx,
			`INSERT OR IGNORE INTO pincer_session
			   (id, user_id, first_seen_at, last_seen_at, user_agent, ip_hash)
			 VALUES (?, NULL, ?, ?, ?, ?)`,
			id, now, now, truncate(ua, 256), ipHash)
		return
	}
	_, _ = t.db.ExecContext(ctx,
		`UPDATE pincer_session SET last_seen_at = ? WHERE id = ?`, now, id)
}

func (t *Tracker) recordEvent(sessionID, eventType, path, payload string) {
	if sessionID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _ = t.db.ExecContext(ctx,
		`INSERT INTO pincer_event (session_id, event_type, path, payload, occurred_at)
		 VALUES (?, ?, ?, ?, ?)`,
		sessionID, eventType, truncate(path, 512), truncate(payload, 4096), time.Now().Unix())
}

// RecordEvent records a typed event against the session UUID currently
// attached to ctx. Safe to call without a session — drops silently.
// payload should be JSON-serializable (or empty); it's stored verbatim.
func (t *Tracker) RecordEvent(ctx context.Context, eventType, path, payload string) {
	t.recordEvent(FromContext(ctx), eventType, path, payload)
}

// ─────────────────────────── stitch on auth ───────────────────────────

// Stitch links a session UUID (typically the cookie on the request that
// just authenticated) to a user_id. Idempotent — calling twice is a no-op.
// Records a login event so the funnel can detect the auth moment.
func (t *Tracker) Stitch(ctx context.Context, sessionID, userID string) error {
	if sessionID == "" || userID == "" {
		return errors.New("pincer: empty sessionID or userID")
	}
	now := time.Now().Unix()
	tx, err := t.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx,
		`UPDATE pincer_session SET user_id = ? WHERE id = ? AND (user_id IS NULL OR user_id = ?)`,
		userID, sessionID, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT OR IGNORE INTO pincer_stitch (session_id, user_id, stitched_at) VALUES (?, ?, ?)`,
		sessionID, userID, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO pincer_event (session_id, event_type, path, payload, occurred_at) VALUES (?, ?, '', '', ?)`,
		sessionID, EventLogin, now); err != nil {
		return err
	}
	return tx.Commit()
}

// StitchAll links every UUID in sessionIDs to the user. Use when an
// authenticated user proves ownership of additional anon sessions (e.g. a
// 'I have another browser' link in the dashboard).
func (t *Tracker) StitchAll(ctx context.Context, sessionIDs []string, userID string) error {
	for _, id := range sessionIDs {
		if err := t.Stitch(ctx, id, userID); err != nil {
			return err
		}
	}
	return nil
}

// ─────────────────────────── analytics queries ────────────────────────

type SessionRow struct {
	ID          string `json:"id"`
	UserID      string `json:"user_id,omitempty"`
	FirstSeenAt int64  `json:"first_seen_at"`
	LastSeenAt  int64  `json:"last_seen_at"`
	EventCount  int    `json:"event_count"`
	UserAgent   string `json:"user_agent,omitempty"`
}

// ActiveSessions returns sessions seen within the lookback window, ordered
// by recency. limit caps the result set.
func (t *Tracker) ActiveSessions(ctx context.Context, lookback time.Duration, limit int) ([]SessionRow, error) {
	if limit <= 0 {
		limit = 100
	}
	since := time.Now().Add(-lookback).Unix()
	rows, err := t.db.QueryContext(ctx, `
		SELECT s.id, COALESCE(s.user_id, ''), s.first_seen_at, s.last_seen_at,
		       COALESCE((SELECT COUNT(*) FROM pincer_event e WHERE e.session_id = s.id), 0) AS event_count,
		       s.user_agent
		FROM pincer_session s
		WHERE s.last_seen_at >= ?
		ORDER BY s.last_seen_at DESC
		LIMIT ?`, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SessionRow
	for rows.Next() {
		var r SessionRow
		if err := rows.Scan(&r.ID, &r.UserID, &r.FirstSeenAt, &r.LastSeenAt, &r.EventCount, &r.UserAgent); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type Funnel struct {
	Window        string `json:"window"`
	AnonSessions  int    `json:"anon_sessions"`
	AuthSessions  int    `json:"auth_sessions"`
	DeckViews     int    `json:"deck_views"`
	GameWatches   int    `json:"game_watches"`
	DeckImports   int    `json:"deck_imports"`
	Logins        int    `json:"logins"`
	ConvRatePct   int    `json:"conv_rate_pct"` // auth_sessions * 100 / total
	TotalSessions int    `json:"total_sessions"`
}

// FunnelStats aggregates a single-window summary of the conversion funnel:
// total sessions, how many converted (got a user_id), and event counts for
// the standard touchpoints. Includes a coarse anon→auth conversion rate.
func (t *Tracker) FunnelStats(ctx context.Context, lookback time.Duration) (*Funnel, error) {
	since := time.Now().Add(-lookback).Unix()
	f := &Funnel{Window: lookback.String()}
	if err := t.db.QueryRowContext(ctx, `
		SELECT
		  COALESCE(SUM(CASE WHEN user_id IS NULL THEN 1 ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN user_id IS NOT NULL THEN 1 ELSE 0 END), 0),
		  COUNT(*)
		FROM pincer_session WHERE last_seen_at >= ?`, since,
	).Scan(&f.AnonSessions, &f.AuthSessions, &f.TotalSessions); err != nil {
		return nil, err
	}
	if err := t.db.QueryRowContext(ctx, `
		SELECT
		  COALESCE(SUM(CASE WHEN event_type = ? THEN 1 ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN event_type = ? THEN 1 ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN event_type = ? THEN 1 ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN event_type = ? THEN 1 ELSE 0 END), 0)
		FROM pincer_event WHERE occurred_at >= ?`,
		EventDeckViewed, EventGameWatched, EventDeckImported, EventLogin, since,
	).Scan(&f.DeckViews, &f.GameWatches, &f.DeckImports, &f.Logins); err != nil {
		return nil, err
	}
	if f.TotalSessions > 0 {
		f.ConvRatePct = f.AuthSessions * 100 / f.TotalSessions
	}
	return f, nil
}

type JourneyEvent struct {
	SessionID  string `json:"session_id"`
	EventType  string `json:"event_type"`
	Path       string `json:"path,omitempty"`
	Payload    string `json:"payload,omitempty"`
	OccurredAt int64  `json:"occurred_at"`
}

// UserJourney returns every event from every session that's been stitched
// to userID, ordered chronologically — the full pre/post-auth trail.
func (t *Tracker) UserJourney(ctx context.Context, userID string, limit int) ([]JourneyEvent, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := t.db.QueryContext(ctx, `
		SELECT e.session_id, e.event_type, e.path, e.payload, e.occurred_at
		FROM pincer_event e
		WHERE e.session_id IN (
		  SELECT session_id FROM pincer_stitch WHERE user_id = ?
		  UNION
		  SELECT id FROM pincer_session WHERE user_id = ?
		)
		ORDER BY e.occurred_at ASC
		LIMIT ?`, userID, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []JourneyEvent
	for rows.Next() {
		var j JourneyEvent
		if err := rows.Scan(&j.SessionID, &j.EventType, &j.Path, &j.Payload, &j.OccurredAt); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// ─────────────────────────── HTTP endpoints ───────────────────────────

// Register wires the analytics + stitch endpoints onto mux:
//
//	GET  /api/analytics/sessions?window=24h&user=<userID>   (admin-only)
//	POST /api/pincer/stitch    body: {"user_id": "..."}     (any visitor)
func (t *Tracker) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/analytics/sessions", t.handleAnalytics)
	mux.HandleFunc("POST /api/pincer/stitch", t.handleStitch)
}

func (t *Tracker) handleAnalytics(w http.ResponseWriter, r *http.Request) {
	if !t.adminAuthorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	window := parseDurationOr(r.URL.Query().Get("window"), 24*time.Hour)
	limit := parseIntOr(r.URL.Query().Get("limit"), 100)
	user := r.URL.Query().Get("user")

	resp := map[string]any{}
	funnel, err := t.FunnelStats(r.Context(), window)
	if err != nil {
		http.Error(w, "funnel: "+err.Error(), http.StatusInternalServerError)
		return
	}
	resp["funnel"] = funnel

	sessions, err := t.ActiveSessions(r.Context(), window, limit)
	if err != nil {
		http.Error(w, "sessions: "+err.Error(), http.StatusInternalServerError)
		return
	}
	resp["sessions"] = sessions

	if user != "" {
		journey, err := t.UserJourney(r.Context(), user, 500)
		if err != nil {
			http.Error(w, "journey: "+err.Error(), http.StatusInternalServerError)
			return
		}
		resp["journey"] = journey
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(resp)
}

func (t *Tracker) handleStitch(w http.ResponseWriter, r *http.Request) {
	sid := FromContext(r.Context())
	if sid == "" {
		// No middleware ran (or the cookie was rejected); fall back to
		// reading the cookie directly.
		sid = readSessionCookie(r)
	}
	if sid == "" {
		http.Error(w, "no session cookie", http.StatusBadRequest)
		return
	}
	var body struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.UserID == "" {
		http.Error(w, "missing user_id", http.StatusBadRequest)
		return
	}
	if err := t.Stitch(r.Context(), sid, body.UserID); err != nil {
		http.Error(w, "stitch: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"stitched": true, "session_id": sid, "user_id": body.UserID})
}

func (t *Tracker) adminAuthorized(r *http.Request) bool {
	if t.adminToken == "" {
		// No token configured — refuse rather than leak in prod by default.
		// Set ENABLE_ANALYTICS_OPEN=1 to opt into open access (dev only).
		return false
	}
	if got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "); got == t.adminToken {
		return true
	}
	if r.URL.Query().Get("token") == t.adminToken {
		return true
	}
	return false
}

// ─────────────────────────── helpers ──────────────────────────────────

// newUUID returns a v4-ish UUID string. crypto/rand-backed; doesn't pull
// in a third-party dep just for this.
func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func looksLikeUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
	}
	return true
}

// clientIPHash returns a short hash of the client's IP — we don't store
// the raw IP, just enough to disambiguate sessions for analytics.
func clientIPHash(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.RemoteAddr
	}
	if comma := strings.Index(ip, ","); comma > 0 {
		ip = ip[:comma]
	}
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return ""
	}
	// 8 hex chars from a fast non-crypto hash is plenty for clustering.
	var h uint32 = 2166136261
	for i := 0; i < len(ip); i++ {
		h ^= uint32(ip[i])
		h *= 16777619
	}
	return fmt.Sprintf("%08x", h)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func parseDurationOr(s string, fallback time.Duration) time.Duration {
	if s == "" {
		return fallback
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}

func parseIntOr(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return fallback
		}
		n = n*10 + int(c-'0')
	}
	if n <= 0 {
		return fallback
	}
	return n
}

package hexapi

import (
	"net"
	"net/http"
	"os"
	"strings"
)

// metricsAuth gates GET /metrics. Returns true when EITHER:
//   (a) the request arrived from a loopback or RFC1918 private
//       address (the standard "scrape from the box / behind a
//       trusted reverse proxy" deployment), OR
//   (b) the request carries an X-HexDek-Owner header matching the
//       HEXDEK_ADMIN_OWNER env var (the same admin-owner check used
//       by the admin_anomalies endpoints — lets a sysadmin curl
//       /metrics from outside the LAN by signing in as the admin).
//
// Security history (mirrors adminAnomalyAuth's note): r.Host is
// client-controlled and MUST NOT be trusted for this gate. RemoteAddr
// is the actual socket peer that the http server set, so it's safe.
// X-Forwarded-For is NOT trusted here either — that's also client-
// controlled and the loopback check would be defeatable by anyone
// who can send `X-Forwarded-For: 127.0.0.1`. If you sit behind Caddy
// or another trusted reverse proxy, the proxy itself talks to the
// hexapi server over loopback (the standard MISTY config), so the
// RemoteAddr check naturally allows scrapes that come THROUGH the
// proxy from anywhere — gate them at the proxy via admin-owner auth
// instead.
func metricsAuth(r *http.Request) bool {
	if isPrivateOrLoopback(r.RemoteAddr) {
		return true
	}
	expected := strings.ToLower(strings.TrimSpace(os.Getenv("HEXDEK_ADMIN_OWNER")))
	if expected == "" {
		return false
	}
	owner := strings.ToLower(strings.TrimSpace(r.Header.Get("X-HexDek-Owner")))
	return owner != "" && owner == expected
}

// isPrivateOrLoopback returns true when remoteAddr's host is a
// loopback (127.0.0.0/8, ::1) or RFC1918 private (10/8, 172.16/12,
// 192.168/16) address. Hostnames that fail to parse as IPs (rare;
// http.Server fills RemoteAddr with ip:port) fall through to false
// so a misparsed RemoteAddr can never grant access.
func isPrivateOrLoopback(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		// Try treating the whole thing as a bare IP (some test
		// harnesses set RemoteAddr without a port).
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() {
		return true
	}
	return ip.IsPrivate()
}

// HandleMetrics returns the GET /metrics handler. Gated via
// metricsAuth — unauthorized callers get the standard 403 in the
// unified ErrorResponse envelope so the rejection looks identical
// to every other hexapi auth failure. On success, writes the
// Prometheus text exposition format with the canonical Content-Type.
//
// When the registry is nil the handler returns 503 — better to
// surface "metrics not configured" loudly than to hand a scraper
// an empty-but-200 response that masks a startup misconfiguration.
func HandleMetrics(registry *MetricsRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !metricsAuth(r) {
			writeError(w, http.StatusForbidden, "metrics: access denied")
			return
		}
		if registry == nil {
			writeError(w, http.StatusServiceUnavailable, "metrics: not configured")
			return
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if err := registry.WritePrometheus(w); err != nil {
			// Write already started — can't change status. Log via the
			// observability middleware's eventual error_slug pickup.
			// Returning here without an explicit error body is fine;
			// Prometheus scrapers treat truncated output as a parse
			// error and surface that as a scrape failure metric.
			return
		}
	}
}

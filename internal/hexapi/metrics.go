package hexapi

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// MetricsRegistry is an in-process Prometheus-text-format metrics
// collector for the hexapi layer. Deliberately dependency-free
// (no prometheus/client_golang) — the exposition format is simple
// enough to render by hand, and avoiding the dep keeps the binary
// surface small + dodges the runtime/registry global-state pitfalls
// that come with the official client.
//
// Three metric families are recorded per request:
//
//   hexapi_request_count{endpoint,method,status}  — counter
//   hexapi_request_duration_seconds{endpoint,method}  — histogram
//   hexapi_error_count{endpoint,method,status,error_slug}  — counter
//
// Endpoint labels come from r.Pattern (Go 1.23+ ServeMux template,
// e.g. "GET /api/decks/{owner}/{id}") with the method prefix stripped,
// so {owner}/{id} stay as placeholders and cardinality is bounded by
// route count (~150) rather than blown out by per-user paths.
//
// All recording is mutex-guarded. Volume is low enough (~hundreds of
// req/s peak) that one mutex is fine; if it ever becomes hot, split
// to per-shard maps keyed on hash(endpoint).
type MetricsRegistry struct {
	mu sync.Mutex

	requestCount map[metricKey]uint64
	errorCount   map[errorMetricKey]uint64
	histograms   map[histKey]*histogramBuckets

	// buckets is the histogram boundary set, ascending. The bucket
	// counts cover the half-open intervals [0, b[0]), [b[0], b[1]),
	// ..., [b[n-1], +Inf).
	buckets []float64

	now func() time.Time // injectable for tests
}

type metricKey struct {
	Endpoint string
	Method   string
	Status   int
}

type errorMetricKey struct {
	Endpoint  string
	Method    string
	Status    int
	ErrorSlug string
}

type histKey struct {
	Endpoint string
	Method   string
}

type histogramBuckets struct {
	// counts has len(buckets)+1 entries — the trailing entry is the
	// +Inf bucket per Prometheus convention.
	counts []uint64
	sum    float64
	count  uint64
}

// DefaultMetricsBuckets is the histogram boundary set for HTTP
// request duration in seconds. Covers the typical hexapi latency
// range — fast cached reads at the low end, expensive Freya
// subprocess kickoffs / replay assembly at the high end. Pinned in
// a package var so the test suite can construct registries with the
// same shape the production server uses.
var DefaultMetricsBuckets = []float64{
	0.005, 0.010, 0.025, 0.050, 0.100, 0.250, 0.500, 1.0, 2.5, 5.0, 10.0,
}

// NewMetricsRegistry returns an empty registry initialized with the
// default histogram buckets. Pass a non-nil buckets slice to
// override (e.g. tests that want tight buckets to assert exact
// placement); buckets MUST be ascending.
func NewMetricsRegistry(buckets []float64) *MetricsRegistry {
	if buckets == nil {
		buckets = DefaultMetricsBuckets
	}
	cp := make([]float64, len(buckets))
	copy(cp, buckets)
	return &MetricsRegistry{
		requestCount: map[metricKey]uint64{},
		errorCount:   map[errorMetricKey]uint64{},
		histograms:   map[histKey]*histogramBuckets{},
		buckets:      cp,
		now:          time.Now,
	}
}

// RecordRequest increments the per-endpoint counters and observes the
// duration. `errorSlug` is the ErrorResponse.Error.Code value from
// the unified envelope (empty string when the response was a success
// or when no envelope was emitted) — when non-empty we also bump the
// per-error counter so dashboards can pivot on failure category.
//
// `endpoint` should be the route template (e.g. "/api/decks/{owner}/{id}")
// — see normalizeEndpoint for how this is derived from r.Pattern.
// `method` is the HTTP verb. `status` is the response status code
// (defaults to 200 when the handler never called WriteHeader).
func (m *MetricsRegistry) RecordRequest(endpoint, method string, status int, duration time.Duration, errorSlug string) {
	if m == nil || endpoint == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	m.requestCount[metricKey{Endpoint: endpoint, Method: method, Status: status}]++
	if errorSlug != "" {
		m.errorCount[errorMetricKey{
			Endpoint:  endpoint,
			Method:    method,
			Status:    status,
			ErrorSlug: errorSlug,
		}]++
	}

	hk := histKey{Endpoint: endpoint, Method: method}
	h, ok := m.histograms[hk]
	if !ok {
		h = &histogramBuckets{counts: make([]uint64, len(m.buckets)+1)}
		m.histograms[hk] = h
	}
	secs := duration.Seconds()
	if secs < 0 {
		secs = 0
	}
	h.sum += secs
	h.count++
	// Find the smallest bucket boundary >= secs. Linear scan is fine
	// for ~10 buckets; sort.SearchFloat64s would shave constant
	// factors but adds an import for negligible gain.
	idx := len(m.buckets) // +Inf bucket by default
	for i, b := range m.buckets {
		if secs <= b {
			idx = i
			break
		}
	}
	// Cumulative count: every bucket from idx through +Inf increments.
	// (Prometheus histograms are cumulative — bucket le="0.05" counts
	// everything with duration <= 0.05, NOT just events in the
	// [0.025, 0.05) interval.)
	for i := idx; i < len(h.counts); i++ {
		h.counts[i]++
	}
}

// WritePrometheus renders the current snapshot in Prometheus text
// exposition format (v0.0.4). The output is deterministic — labels
// are sorted alphabetically within each metric family — so test
// assertions and golden-file diffs work without flake. Caller is
// responsible for setting Content-Type:
// text/plain; version=0.0.4; charset=utf-8.
func (m *MetricsRegistry) WritePrometheus(w io.Writer) error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.writeRequestCount(w); err != nil {
		return err
	}
	if err := m.writeHistograms(w); err != nil {
		return err
	}
	return m.writeErrorCount(w)
}

func (m *MetricsRegistry) writeRequestCount(w io.Writer) error {
	if _, err := fmt.Fprintln(w, "# HELP hexapi_request_count Total number of HTTP requests received."); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "# TYPE hexapi_request_count counter"); err != nil {
		return err
	}
	keys := make([]metricKey, 0, len(m.requestCount))
	for k := range m.requestCount {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Endpoint != keys[j].Endpoint {
			return keys[i].Endpoint < keys[j].Endpoint
		}
		if keys[i].Method != keys[j].Method {
			return keys[i].Method < keys[j].Method
		}
		return keys[i].Status < keys[j].Status
	})
	for _, k := range keys {
		if _, err := fmt.Fprintf(w, "hexapi_request_count{endpoint=%q,method=%q,status=\"%d\"} %d\n",
			k.Endpoint, k.Method, k.Status, m.requestCount[k]); err != nil {
			return err
		}
	}
	return nil
}

func (m *MetricsRegistry) writeHistograms(w io.Writer) error {
	if _, err := fmt.Fprintln(w, "# HELP hexapi_request_duration_seconds HTTP request latency distribution."); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "# TYPE hexapi_request_duration_seconds histogram"); err != nil {
		return err
	}
	keys := make([]histKey, 0, len(m.histograms))
	for k := range m.histograms {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Endpoint != keys[j].Endpoint {
			return keys[i].Endpoint < keys[j].Endpoint
		}
		return keys[i].Method < keys[j].Method
	})
	for _, k := range keys {
		h := m.histograms[k]
		for i, b := range m.buckets {
			if _, err := fmt.Fprintf(w, "hexapi_request_duration_seconds_bucket{endpoint=%q,method=%q,le=%q} %d\n",
				k.Endpoint, k.Method, formatBucket(b), h.counts[i]); err != nil {
				return err
			}
		}
		// +Inf bucket.
		if _, err := fmt.Fprintf(w, "hexapi_request_duration_seconds_bucket{endpoint=%q,method=%q,le=\"+Inf\"} %d\n",
			k.Endpoint, k.Method, h.counts[len(h.counts)-1]); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "hexapi_request_duration_seconds_sum{endpoint=%q,method=%q} %g\n",
			k.Endpoint, k.Method, h.sum); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "hexapi_request_duration_seconds_count{endpoint=%q,method=%q} %d\n",
			k.Endpoint, k.Method, h.count); err != nil {
			return err
		}
	}
	return nil
}

func (m *MetricsRegistry) writeErrorCount(w io.Writer) error {
	if _, err := fmt.Fprintln(w, "# HELP hexapi_error_count Total number of HTTP error responses, by ErrorResponse.error.code slug."); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "# TYPE hexapi_error_count counter"); err != nil {
		return err
	}
	keys := make([]errorMetricKey, 0, len(m.errorCount))
	for k := range m.errorCount {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Endpoint != keys[j].Endpoint {
			return keys[i].Endpoint < keys[j].Endpoint
		}
		if keys[i].Method != keys[j].Method {
			return keys[i].Method < keys[j].Method
		}
		if keys[i].Status != keys[j].Status {
			return keys[i].Status < keys[j].Status
		}
		return keys[i].ErrorSlug < keys[j].ErrorSlug
	})
	for _, k := range keys {
		if _, err := fmt.Fprintf(w, "hexapi_error_count{endpoint=%q,method=%q,status=\"%d\",error_slug=%q} %d\n",
			k.Endpoint, k.Method, k.Status, k.ErrorSlug, m.errorCount[k]); err != nil {
			return err
		}
	}
	return nil
}

// formatBucket renders a bucket boundary in the canonical "0.005"
// format Prometheus expects (no scientific notation, no trailing
// zeros that change the label across releases).
func formatBucket(b float64) string {
	s := strconv.FormatFloat(b, 'f', -1, 64)
	// Make sure we always have at least one decimal place so
	// label-set diffs stay stable across versions ("5" vs "5.0").
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

// normalizeEndpoint strips the leading HTTP method (and following
// space) from a Go 1.23 ServeMux pattern. The pattern arrives shaped
// like "GET /api/decks/{owner}/{id}" — we want "/api/decks/{owner}/{id}"
// for the endpoint label since method already lives on its own label.
// When pattern is empty (handler not registered with method+pattern
// form, or sub-mux dispatch), returns "" so the caller can skip
// recording rather than poison the registry with a degenerate label.
func normalizeEndpoint(pattern string) string {
	if pattern == "" {
		return ""
	}
	if i := strings.IndexByte(pattern, ' '); i >= 0 {
		return pattern[i+1:]
	}
	return pattern
}

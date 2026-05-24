package hexapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/hexdek/hexdek/internal/anticheat"
)

func adminAnomalyAuth(r *http.Request) bool {
	owner := strings.ToLower(strings.TrimSpace(r.Header.Get("X-HexDek-Owner")))
	expected := strings.ToLower(strings.TrimSpace(os.Getenv("HEXDEK_ADMIN_OWNER")))
	if expected != "" {
		return owner != "" && owner == expected
	}
	host := r.Host
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

type flagJSON struct {
	ID            int64   `json:"id"`
	ContributorID string  `json:"contributor_id"`
	Metric        string  `json:"metric"`
	MetricValue   float64 `json:"metric_value"`
	PopMean       float64 `json:"pop_mean"`
	PopStdDev     float64 `json:"pop_stddev"`
	ZScore        float64 `json:"z_score"`
	Severity      int     `json:"severity"`
	DetectedAt    int64   `json:"detected_at"`
	ResolvedAt    *int64  `json:"resolved_at,omitempty"`
	ResolvedBy    string  `json:"resolved_by,omitempty"`
	ResolvedNote  string  `json:"resolved_note,omitempty"`
}

func toFlagJSON(f anticheat.Flag) flagJSON {
	out := flagJSON{
		ID:            f.ID,
		ContributorID: f.ContributorID,
		Metric:        f.Metric,
		MetricValue:   f.MetricValue,
		PopMean:       f.PopMean,
		PopStdDev:     f.PopStdDev,
		ZScore:        f.ZScore,
		Severity:      f.Severity,
		DetectedAt:    f.DetectedAt.Unix(),
		ResolvedBy:    f.ResolvedBy,
		ResolvedNote:  f.ResolvedNote,
	}
	if f.ResolvedAt != nil {
		t := f.ResolvedAt.Unix()
		out.ResolvedAt = &t
	}
	return out
}

func HandleListAnomalies(auditor *anticheat.StatisticalAuditor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !adminAnomalyAuth(r) {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		if auditor == nil {
			writeError(w, http.StatusServiceUnavailable, "anomaly auditor not configured")
			return
		}
		onlyActive := r.URL.Query().Get("include_resolved") != "1"
		limit := 0
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				limit = n
			}
		}
		flags, err := auditor.ListFlags(r.Context(), onlyActive, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "list flags: "+err.Error())
			return
		}
		out := make([]flagJSON, len(flags))
		for i, f := range flags {
			out[i] = toFlagJSON(f)
		}
		writeJSON(w, map[string]any{
			"flags":       out,
			"count":       len(out),
			"only_active": onlyActive,
		})
	}
}

func HandleResolveAnomaly(auditor *anticheat.StatisticalAuditor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !adminAnomalyAuth(r) {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		if auditor == nil {
			writeError(w, http.StatusServiceUnavailable, "anomaly auditor not configured")
			return
		}
		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			writeError(w, http.StatusBadRequest, "invalid id")
			return
		}
		var body struct {
			Note string `json:"note"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		by := strings.TrimSpace(r.Header.Get("X-HexDek-Owner"))

		if err := auditor.ResolveFlag(r.Context(), id, by, body.Note); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, "flag not found or already resolved")
				return
			}
			writeError(w, http.StatusInternalServerError, "resolve: "+err.Error())
			return
		}
		writeJSON(w, map[string]any{"resolved": true, "id": id})
	}
}

func RegisterAdminAnomalies(mux *http.ServeMux, auditor *anticheat.StatisticalAuditor) {
	mux.HandleFunc("GET /api/admin/anomalies", HandleListAnomalies(auditor))
	mux.HandleFunc("POST /api/admin/anomalies/{id}/resolve", HandleResolveAnomaly(auditor))
}

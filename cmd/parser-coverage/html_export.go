package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"strconv"
	"strings"
	"time"
)

//go:embed coverage.html.tmpl
var htmlTemplate string

// htmlCard is the per-card payload shape sent to the embedded JS UI.
// JSON tags are camelCase to match the JS naming convention used in
// the template — the CSV / history outputs use snake_case for shell /
// jq workflows, but in-browser JS conventionally reads camelCase
// without ceremony.
type htmlCard struct {
	Name        string   `json:"name"`
	Class       string   `json:"class"`
	TypeLine    string   `json:"typeLine"`
	PrimaryType string   `json:"primaryType"`
	SetName     string   `json:"setName"`
	Year        int      `json:"year"`
	Era         string   `json:"era"`
	Oracle      string   `json:"oracle"`
	OracleLen   int      `json:"oracleLen"`
	ParseErrors []string `json:"parseErrors,omitempty"`
}

// htmlPayload is the whole-page payload — the JS reads DATA.counts
// for the summary pills and DATA.cards for the table. Counts are
// always the FULL audit (so the summary stays accurate) even when
// cards is filtered to uncovered-only.
type htmlPayload struct {
	GeneratedAt string         `json:"generatedAt"`
	Counts      map[string]int `json:"counts"`
	Cards       []htmlCard     `json:"cards"`
}

// primaryType extracts the dominant card type from a Scryfall
// type_line, stripping supertypes (Legendary, Basic, Snow, …) so a
// "Legendary Creature — Elemental" returns "Creature" not "Legendary".
// Sagas, Battles, Planeswalkers, Tribal etc. all return their type
// word.
//
// Returns "" only for an empty type line; otherwise always non-empty
// so the type filter has a stable bucket per card.
func primaryType(typeLine string) string {
	main := typeLine
	if i := strings.Index(typeLine, "—"); i >= 0 {
		main = strings.TrimSpace(typeLine[:i])
	}
	supertypes := map[string]bool{
		"Legendary": true, "Basic": true, "Snow": true, "World": true,
		"Tribal": true, "Ongoing": true, "Elite": true, "Host": true, "Token": true,
	}
	for _, f := range strings.Fields(main) {
		if !supertypes[f] {
			return f
		}
	}
	// All-supertypes case (shouldn't happen on real cards but defensive):
	// fall back to the last word so the filter bucket is still deterministic.
	fields := strings.Fields(main)
	if len(fields) > 0 {
		return fields[len(fields)-1]
	}
	return ""
}

// extractYear pulls the YYYY out of a Scryfall released_at string
// (format "YYYY-MM-DD"). Returns 0 on parse failure so the era
// bucketer can route to "unknown" without panicking.
func extractYear(releasedAt string) int {
	if len(releasedAt) < 4 {
		return 0
	}
	y, err := strconv.Atoi(releasedAt[:4])
	if err != nil {
		return 0
	}
	return y
}

// eraFromYear maps a Scryfall release year to a human-readable era
// bucket. Boundaries match commonly cited Magic eras (classic /
// urza/masques / mirrodin to lorwyn / shards to khans / modern).
// Year 0 falls into "unknown" so cards with missing release dates
// don't all collapse into "classic".
func eraFromYear(year int) string {
	switch {
	case year == 0:
		return "unknown"
	case year < 1997:
		return "classic (1993-1996)"
	case year < 2003:
		return "early (1997-2002)"
	case year < 2010:
		return "modern early (2003-2009)"
	case year < 2018:
		return "modern late (2010-2017)"
	default:
		return "contemporary (2018+)"
	}
}

// resultsToHTMLPayload renders the full classified result set into
// the JSON payload the embedded UI consumes. includeOK controls
// whether OK / OK_VANILLA cards are emitted into the cards array
// (their counts are always reflected in Counts so the summary stays
// honest either way).
func resultsToHTMLPayload(results []result, includeOK bool, now time.Time) htmlPayload {
	counts := map[string]int{}
	cards := make([]htmlCard, 0, len(results))
	for _, r := range results {
		counts[r.Class.String()]++
		if !includeOK && (r.Class == classOK || r.Class == classOKVanilla) {
			continue
		}
		year := extractYear(r.ReleasedAt)
		cards = append(cards, htmlCard{
			Name:        r.Name,
			Class:       r.Class.String(),
			TypeLine:    r.TypeLine,
			PrimaryType: primaryType(r.TypeLine),
			SetName:     r.SetName,
			Year:        year,
			Era:         eraFromYear(year),
			Oracle:      r.OracleText,
			OracleLen:   len(r.OracleText),
			ParseErrors: r.ParseErrors,
		})
	}
	return htmlPayload{
		GeneratedAt: now.UTC().Format(time.RFC3339),
		Counts:      counts,
		Cards:       cards,
	}
}

// safeJSON marshals v to JSON and escapes any embedded "</" sequences
// to "</". This prevents an oracle text containing the literal
// string "</script>" (rare but possible — flavor-text-style content,
// hand-crafted parse errors) from prematurely closing the inline
// <script> tag the payload is embedded in. The escape is invisible
// at the JS level: JSON's string syntax decodes < back to '<'
// before the JS engine sees the value, so the JS side reads through
// unchanged.
//
// Returns template.JS so the html/template engine inserts the bytes
// verbatim into the <script> context (which expects JS source, not
// HTML-escaped text).
func safeJSON(v interface{}) (template.JS, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	b = bytes.ReplaceAll(b, []byte("</"), []byte(`\u003c/`))
	return template.JS(b), nil
}

// renderHTMLReport renders the embedded coverage.html.tmpl against
// the given result set and returns the page as a byte slice. The
// HTML is fully self-contained (no external CSS/JS/font requests)
// so it works offline, over file://, and through the in-memory
// --serve handler.
//
// Pull this out of writeHTMLReport so the --serve path can hold the
// bytes in memory without round-tripping through disk.
func renderHTMLReport(results []result, includeOK bool, now time.Time) ([]byte, error) {
	payload := resultsToHTMLPayload(results, includeOK, now)
	dataJS, err := safeJSON(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	tpl, err := template.New("coverage").Parse(htmlTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, struct {
		GeneratedAt string
		DataJSON    template.JS
	}{
		GeneratedAt: payload.GeneratedAt,
		DataJSON:    dataJS,
	}); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}
	return buf.Bytes(), nil
}

// writeHTMLReport renders the report and writes it to path. Thin
// wrapper around renderHTMLReport for the on-disk export flow.
func writeHTMLReport(path string, results []result, includeOK bool, now time.Time) error {
	b, err := renderHTMLReport(results, includeOK, now)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

package moxfield

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/oracle"
)

// validate_r60_test.go — pins moxfield.ValidateFormat behavior across
// Commander / Brawl / Modern legality checks, plus the end-to-end
// ValidateFormatFromURL convenience used by the CLI's --validate-format
// flag.

// ---------------------------------------------------------------------------
// Pure ValidateFormat unit tests (no I/O)
// ---------------------------------------------------------------------------

func leg(format, status string) map[string]string {
	return map[string]string{format: status}
}

func TestValidateFormat_AllLegalNoViolations(t *testing.T) {
	cards := []*oracle.Card{
		{Name: "Sol Ring", Legalities: leg("commander", "legal")},
		{Name: "Yuriko, the Tiger's Shadow", Legalities: leg("commander", "legal")},
		{Name: "Brainstorm", Legalities: leg("commander", "legal")},
	}
	r := ValidateFormat("commander", cards)
	if !r.IsClean() {
		t.Errorf("expected IsClean, got %d violations: %+v", len(r.Violations), r.Violations)
	}
	if len(r.SkippedUnknown) != 0 {
		t.Errorf("expected 0 skipped, got %v", r.SkippedUnknown)
	}
}

func TestValidateFormat_BannedFlagged(t *testing.T) {
	cards := []*oracle.Card{
		{Name: "Sol Ring", Legalities: leg("commander", "legal")},
		{Name: "Channel", Legalities: leg("commander", "banned")},
	}
	r := ValidateFormat("commander", cards)
	if len(r.Violations) != 1 {
		t.Fatalf("expected 1 violation, got %d: %+v", len(r.Violations), r.Violations)
	}
	if r.Violations[0].CardName != "Channel" {
		t.Errorf("violation card = %q, want Channel", r.Violations[0].CardName)
	}
	if r.Violations[0].Status != "banned" {
		t.Errorf("status = %q, want banned", r.Violations[0].Status)
	}
	if !strings.Contains(r.Violations[0].Reason, "banned in commander") {
		t.Errorf("reason should mention 'banned in commander': %q", r.Violations[0].Reason)
	}
	if r.IsClean() {
		t.Errorf("IsClean should be false with 1 violation")
	}
}

func TestValidateFormat_NotLegalFlagged(t *testing.T) {
	// Brawl historically excludes anything pre-Throne of Eldraine —
	// Lightning Bolt is "not_legal" rather than "banned".
	cards := []*oracle.Card{
		{Name: "Lightning Bolt", Legalities: leg("brawl", "not_legal")},
	}
	r := ValidateFormat("brawl", cards)
	if len(r.Violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(r.Violations))
	}
	if r.Violations[0].Status != "not_legal" {
		t.Errorf("status = %q, want not_legal", r.Violations[0].Status)
	}
	if !strings.Contains(r.Violations[0].Reason, "not legal in brawl") {
		t.Errorf("reason = %q, want 'not legal in brawl'", r.Violations[0].Reason)
	}
}

func TestValidateFormat_RestrictedFlagged(t *testing.T) {
	// Vintage-only status: max 1 copy. Treated as a violation so the
	// importer surfaces it; the importer doesn't yet enforce per-card
	// count.
	cards := []*oracle.Card{
		{Name: "Brainstorm", Legalities: leg("vintage", "restricted")},
	}
	r := ValidateFormat("vintage", cards)
	if len(r.Violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(r.Violations))
	}
	if !strings.Contains(r.Violations[0].Reason, "max 1 copy") {
		t.Errorf("restricted reason should mention max 1 copy: %q", r.Violations[0].Reason)
	}
}

func TestValidateFormat_FormatIsCaseInsensitive(t *testing.T) {
	cards := []*oracle.Card{
		{Name: "Channel", Legalities: leg("commander", "banned")},
	}
	r := ValidateFormat("COMMANDER", cards)
	if len(r.Violations) != 1 {
		t.Errorf("uppercase format should still match; got %d violations", len(r.Violations))
	}
}

func TestValidateFormat_LegalitiesNilSkipped(t *testing.T) {
	cards := []*oracle.Card{
		{Name: "Cardiac Mystery", Legalities: nil},
		{Name: "Sol Ring", Legalities: leg("commander", "legal")},
	}
	r := ValidateFormat("commander", cards)
	if len(r.Violations) != 0 {
		t.Errorf("nil-legalities card should NOT be a violation; got %+v", r.Violations)
	}
	if len(r.SkippedUnknown) != 1 || r.SkippedUnknown[0] != "Cardiac Mystery" {
		t.Errorf("expected SkippedUnknown=[Cardiac Mystery], got %v", r.SkippedUnknown)
	}
}

func TestValidateFormat_LegalitiesMissingTargetFormatSkipped(t *testing.T) {
	// Card has legalities for other formats but not the one we asked
	// about — should be skipped (unknown) rather than counted as legal.
	cards := []*oracle.Card{
		{Name: "Some Card", Legalities: map[string]string{
			"modern": "legal", "legacy": "legal",
		}},
	}
	r := ValidateFormat("commander", cards)
	if len(r.Violations) != 0 {
		t.Errorf("missing-target-format card should NOT be a violation; got %+v", r.Violations)
	}
	if len(r.SkippedUnknown) != 1 || r.SkippedUnknown[0] != "Some Card" {
		t.Errorf("expected SkippedUnknown=[Some Card], got %v", r.SkippedUnknown)
	}
}

func TestValidateFormat_EmptyFormatSkipsAll(t *testing.T) {
	cards := []*oracle.Card{
		{Name: "Sol Ring", Legalities: leg("commander", "legal")},
		{Name: "Channel", Legalities: leg("commander", "banned")},
	}
	r := ValidateFormat("", cards)
	if len(r.Violations) != 0 {
		t.Errorf("empty format should produce no violations; got %+v", r.Violations)
	}
	if len(r.SkippedUnknown) != 2 {
		t.Errorf("empty format should skip all cards; got %v", r.SkippedUnknown)
	}
}

func TestValidateFormat_DuplicateCardNotDoubleReported(t *testing.T) {
	c := &oracle.Card{Name: "Channel", Legalities: leg("commander", "banned")}
	r := ValidateFormat("commander", []*oracle.Card{c, c, c})
	if len(r.Violations) != 1 {
		t.Errorf("duplicate-pointer Channel should be reported once; got %d", len(r.Violations))
	}
}

func TestValidateFormat_ViolationsSortedByCardName(t *testing.T) {
	cards := []*oracle.Card{
		{Name: "Zur the Enchanter", Legalities: leg("commander", "banned")},
		{Name: "Channel", Legalities: leg("commander", "banned")},
		{Name: "Mox Sapphire", Legalities: leg("commander", "banned")},
	}
	r := ValidateFormat("commander", cards)
	if len(r.Violations) != 3 {
		t.Fatalf("expected 3 violations, got %d", len(r.Violations))
	}
	got := []string{r.Violations[0].CardName, r.Violations[1].CardName, r.Violations[2].CardName}
	want := []string{"Channel", "Mox Sapphire", "Zur the Enchanter"}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("sort: pos %d = %q, want %q (full got=%v)", i, got[i], want[i], got)
			break
		}
	}
}

func TestValidateFormat_SkippedUnknownSorted(t *testing.T) {
	cards := []*oracle.Card{
		{Name: "Zeta"},
		{Name: "Alpha"},
		{Name: "Mu"},
	}
	r := ValidateFormat("commander", cards)
	want := []string{"Alpha", "Mu", "Zeta"}
	for i := range want {
		if r.SkippedUnknown[i] != want[i] {
			t.Errorf("SkippedUnknown[%d] = %q, want %q (full=%v)", i, r.SkippedUnknown[i], want[i], r.SkippedUnknown)
			break
		}
	}
}

func TestValidateFormat_UnknownStatusFlagged(t *testing.T) {
	// Defensive: a future Scryfall status string should not silently
	// pass. Flag as a violation with the literal status so an operator
	// can investigate.
	cards := []*oracle.Card{
		{Name: "Time Walk", Legalities: leg("commander", "future_status_xyz")},
	}
	r := ValidateFormat("commander", cards)
	if len(r.Violations) != 1 {
		t.Fatalf("expected 1 violation for unknown status, got %d", len(r.Violations))
	}
	if r.Violations[0].Status != "future_status_xyz" {
		t.Errorf("status passthrough broken: %q", r.Violations[0].Status)
	}
	if !strings.Contains(r.Violations[0].Reason, "unknown legality status") {
		t.Errorf("reason should mention unknown status: %q", r.Violations[0].Reason)
	}
}

func TestValidateFormat_NilCardsSkipped(t *testing.T) {
	cards := []*oracle.Card{
		nil,
		{Name: "Sol Ring", Legalities: leg("commander", "legal")},
		nil,
	}
	r := ValidateFormat("commander", cards)
	if !r.IsClean() {
		t.Errorf("nil entries should be silently skipped; got %+v", r.Violations)
	}
}

// ---------------------------------------------------------------------------
// Format-specific real-card cases (sanity)
// ---------------------------------------------------------------------------

func TestValidateFormat_CommanderVsModern_DistinctVerdicts(t *testing.T) {
	// One *Card with legalities for both formats — confirms the
	// per-format dispatch picks up the right key.
	card := &oracle.Card{
		Name: "Sol Ring",
		Legalities: map[string]string{
			"commander": "legal",
			"modern":    "banned",
			"legacy":    "banned",
		},
	}
	rC := ValidateFormat("commander", []*oracle.Card{card})
	if !rC.IsClean() {
		t.Errorf("Sol Ring should be legal in commander; got %+v", rC.Violations)
	}
	rM := ValidateFormat("modern", []*oracle.Card{card})
	if len(rM.Violations) != 1 {
		t.Errorf("Sol Ring should be banned in modern; got %+v", rM.Violations)
	}
	rL := ValidateFormat("legacy", []*oracle.Card{card})
	if len(rL.Violations) != 1 {
		t.Errorf("Sol Ring should be banned in legacy; got %+v", rL.Violations)
	}
}

func TestValidateFormat_BrawlAgeGate(t *testing.T) {
	// Brawl restricts to a rolling print-window subset. A card like
	// Counterspell shipped in older sets is legal in legacy/vintage
	// but not_legal in brawl. Pin the per-format verdicts.
	card := &oracle.Card{
		Name: "Counterspell",
		Legalities: map[string]string{
			"commander": "legal",
			"legacy":    "legal",
			"vintage":   "legal",
			"brawl":     "not_legal",
		},
	}
	if !ValidateFormat("legacy", []*oracle.Card{card}).IsClean() {
		t.Errorf("Counterspell should be legal in legacy")
	}
	rBrawl := ValidateFormat("brawl", []*oracle.Card{card})
	if rBrawl.IsClean() {
		t.Errorf("Counterspell should be not_legal in brawl")
	}
}

// ---------------------------------------------------------------------------
// validateFormatFromAPIResponse: end-to-end with stubbed Scryfall + DB
// ---------------------------------------------------------------------------

// newScryfallStubWithLegalities spins an httptest server whose /cards/
// collection responses include the provided legalities map per card.
// Returns a cleanup that closes the server and restores scryfallBase.
func newScryfallStubWithLegalities(t *testing.T, legalitiesByName map[string]map[string]string) (counter *int32, cleanup func()) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/cards/collection"):
			var req struct {
				Identifiers []struct {
					Name string `json:"name"`
				} `json:"identifiers"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			data := []map[string]any{}
			for _, ident := range req.Identifiers {
				leg := legalitiesByName[ident.Name]
				data = append(data, map[string]any{
					"object":     "card",
					"id":         ident.Name + "-id",
					"name":       ident.Name,
					"type_line":  "Creature — Test",
					"legalities": leg,
				})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data":      data,
				"not_found": []map[string]string{},
			})
		case strings.Contains(r.URL.Path, "/cards/named"):
			name := r.URL.Query().Get("fuzzy")
			leg := legalitiesByName[name]
			_ = json.NewEncoder(w).Encode(map[string]any{
				"object":     "card",
				"id":         name + "-id",
				"name":       name,
				"type_line":  "Sorcery",
				"legalities": leg,
			})
		default:
			w.WriteHeader(404)
		}
	}))
	restore := oracle.SetScryfallBaseForTest(srv.URL)
	intCleanup := oracle.SetScryfallLimitIntervalForTest(5 * 1000 * 1000) // 5ms
	return &hits, func() {
		srv.Close()
		restore()
		intCleanup()
	}
}

func newMemoryDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func TestValidateFormatFromAPIResponse_FlagsBannedCommanderCard(t *testing.T) {
	_, cleanup := newScryfallStubWithLegalities(t, map[string]map[string]string{
		"Yuriko, the Tiger's Shadow": {"commander": "legal"},
		"Sol Ring":                   {"commander": "legal"},
		"Channel":                    {"commander": "banned"},
	})
	defer cleanup()

	data := &apiResponse{Name: "Test", Format: "commander"}
	data.Boards.Commanders.Cards = map[string]apiCardEntry{
		"c": {Quantity: 1, Card: apiCard{Name: "Yuriko, the Tiger's Shadow"}},
	}
	data.Boards.Mainboard.Cards = map[string]apiCardEntry{
		"m1": {Quantity: 1, Card: apiCard{Name: "Sol Ring"}},
		"m2": {Quantity: 1, Card: apiCard{Name: "Channel"}},
	}

	d := newMemoryDB(t)
	r := validateFormatFromAPIResponse(context.Background(), d, data)
	if r == nil {
		t.Fatal("nil report")
	}
	if r.IsClean() {
		t.Fatalf("expected violation for Channel, got clean report")
	}
	found := false
	for _, v := range r.Violations {
		if v.CardName == "Channel" && v.Status == "banned" {
			found = true
		}
	}
	if !found {
		t.Errorf("Channel banned violation not in report: %+v", r.Violations)
	}
	// Sol Ring + Yuriko both legal → not in violations list.
	for _, v := range r.Violations {
		if v.CardName == "Sol Ring" || v.CardName == "Yuriko, the Tiger's Shadow" {
			t.Errorf("legal card surfaced as violation: %+v", v)
		}
	}
}

func TestValidateFormatFromAPIResponse_SideboardCardsExcluded(t *testing.T) {
	_, cleanup := newScryfallStubWithLegalities(t, map[string]map[string]string{
		"Atraxa, Praetors' Voice": {"commander": "legal"},
		"Sol Ring":                {"commander": "legal"},
		"Channel":                 {"commander": "banned"},
	})
	defer cleanup()

	data := &apiResponse{Name: "Test", Format: "commander"}
	data.Boards.Commanders.Cards = map[string]apiCardEntry{
		"c": {Quantity: 1, Card: apiCard{Name: "Atraxa, Praetors' Voice"}},
	}
	data.Boards.Mainboard.Cards = map[string]apiCardEntry{
		"m": {Quantity: 1, Card: apiCard{Name: "Sol Ring"}},
	}
	// Channel banned in commander but only in sideboard/maybeboard —
	// must NOT trigger a violation because the importer's
	// validateFormatFromAPIResponse excludes non-playable boards.
	data.Boards.Sideboard.Cards = map[string]apiCardEntry{
		"s": {Quantity: 1, Card: apiCard{Name: "Channel"}},
	}
	data.Boards.Maybeboard.Cards = map[string]apiCardEntry{
		"mb": {Quantity: 1, Card: apiCard{Name: "Channel"}},
	}

	d := newMemoryDB(t)
	r := validateFormatFromAPIResponse(context.Background(), d, data)
	if !r.IsClean() {
		t.Errorf("sideboard/maybeboard violations must be excluded; got %+v", r.Violations)
	}
}

func TestValidateFormatFromAPIResponse_QuantityZeroSkipped(t *testing.T) {
	_, cleanup := newScryfallStubWithLegalities(t, map[string]map[string]string{
		"Yuriko, the Tiger's Shadow": {"commander": "legal"},
		"Channel":                    {"commander": "banned"},
	})
	defer cleanup()

	data := &apiResponse{Name: "Test", Format: "commander"}
	data.Boards.Commanders.Cards = map[string]apiCardEntry{
		"c": {Quantity: 1, Card: apiCard{Name: "Yuriko, the Tiger's Shadow"}},
	}
	data.Boards.Mainboard.Cards = map[string]apiCardEntry{
		// Channel banned but Quantity=0 — must be skipped.
		"zero": {Quantity: 0, Card: apiCard{Name: "Channel"}},
	}

	d := newMemoryDB(t)
	r := validateFormatFromAPIResponse(context.Background(), d, data)
	if !r.IsClean() {
		t.Errorf("quantity=0 entry should be skipped; got %+v", r.Violations)
	}
}

func TestValidateFormatFromAPIResponse_NilSafe(t *testing.T) {
	d := newMemoryDB(t)
	r := validateFormatFromAPIResponse(context.Background(), d, nil)
	if r == nil {
		t.Fatal("nil report on nil input")
	}
	if !r.IsClean() {
		t.Errorf("nil input should produce clean report, got %+v", r.Violations)
	}
}

// ---------------------------------------------------------------------------
// End-to-end via ValidateFormatFromURL (the public surface the CLI uses)
// ---------------------------------------------------------------------------

const fixtureValidateMoxJSON = `{
	"name": "Banned Channel Deck",
	"format": "commander",
	"boards": {
		"commanders": {"count": 1, "cards": {"c": {"quantity": 1, "card": {"name": "Yuriko, the Tiger's Shadow"}}}},
		"mainboard":  {"count": 2, "cards": {
			"m1": {"quantity": 1, "card": {"name": "Sol Ring"}},
			"m2": {"quantity": 1, "card": {"name": "Channel"}}
		}}
	}
}`

func newMoxStubReturning(t *testing.T, body string) func() {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	restore := SetAPIBaseForTest(srv.URL)
	tmp := t.TempDir()
	t.Setenv(cacheEnvDir, tmp)
	t.Setenv(cacheEnvDisable, "")
	t.Setenv(cacheEnvTTL, "")
	t.Setenv(snapshotEnvDir, filepath.Join(tmp, "snapshots"))
	resetCachesForTest()
	return func() {
		srv.Close()
		restore()
	}
}

func TestValidateFormatFromURL_EndToEnd(t *testing.T) {
	moxCleanup := newMoxStubReturning(t, fixtureValidateMoxJSON)
	defer moxCleanup()
	_, scryCleanup := newScryfallStubWithLegalities(t, map[string]map[string]string{
		"Yuriko, the Tiger's Shadow": {"commander": "legal"},
		"Sol Ring":                   {"commander": "legal"},
		"Channel":                    {"commander": "banned"},
	})
	defer scryCleanup()

	d := newMemoryDB(t)
	r := ValidateFormatFromURL(context.Background(), d, "https://moxfield.com/decks/val01")
	if r == nil {
		t.Fatal("nil report")
	}
	if r.Format != "commander" {
		t.Errorf("Format = %q, want commander", r.Format)
	}
	if r.IsClean() {
		t.Fatalf("expected violations; got clean. report=%+v", r)
	}
	if len(r.Violations) != 1 || r.Violations[0].CardName != "Channel" {
		t.Errorf("expected single Channel violation; got %+v", r.Violations)
	}
}

func TestValidateFormatFromURL_BadURLReturnsNil(t *testing.T) {
	d := newMemoryDB(t)
	r := ValidateFormatFromURL(context.Background(), d, "https://example.com/not-moxfield")
	if r != nil {
		t.Errorf("bad URL should return nil report, got %+v", r)
	}
}


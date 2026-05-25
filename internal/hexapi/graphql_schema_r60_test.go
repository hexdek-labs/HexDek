package hexapi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// graphql_schema_r60_test.go — pins the schema additions on top of
// the #321 POC: Deck type, deck(owner,id) query, decks(owner,limit)
// query, Game.decks cross-reference. Parsing + execution coverage:
//
//   - Schema compiles with the new types wired in
//   - deck(owner,id) round-trip + null on missing + path-traversal
//     guard
//   - decks(owner) filters; decks() without filter returns everything
//   - decks(limit) caps; default applies; sort is newest-first
//   - Game.decks cross-reference resolves each deckKey to a Deck;
//     missing deck files surface as null entries
//   - Field projection on Deck (the POC's load-bearing property,
//     verified for the new type)
//   - Variables thread through deck/decks queries
//
// Tests use a temp DecksDir seeded with two owners + a few deck
// files so the per-test scope is isolated and the assertions are
// deterministic.

// graphqlDeckFixture builds a temp decks tree:
//
//   <tmp>/alice/korvold.txt   (_b3_ marker)
//   <tmp>/alice/atraxa.txt    (_b4_ marker, newer mtime)
//   <tmp>/bob/muldrotha.txt   (_b2_ marker)
//   <tmp>/.versions/...       (reserved — must be excluded)
//
// Returns the decksDir path. Caller closes via t.Cleanup.
func graphqlDeckFixture(t *testing.T) string {
	t.Helper()
	tmp, err := os.MkdirTemp("", "hexapi-graphql-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmp) })

	plant := func(owner, id, body string, mtime time.Time) {
		dir := filepath.Join(tmp, owner)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, id+".txt")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		// Stamp mtime so the newest-first sort is deterministic.
		_ = os.Chtimes(path, mtime, mtime)
	}

	// _bN_ markers feed parseDeckFilename → bracket derivation.
	plant("alice", "korvold_b3_jund_value",
		"COMMANDER: Korvold, Fae-Cursed King\n1 Sol Ring\n1 Command Tower\n",
		time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC))
	plant("alice", "atraxa_b4_superfriends",
		"COMMANDER: Atraxa, Praetors' Voice\n1 Sol Ring\n1 Arcane Signet\n",
		time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC))
	plant("bob", "muldrotha_b2_jank",
		"COMMANDER: Muldrotha, the Gravetide\n1 Sol Ring\n",
		time.Date(2026, 5, 24, 8, 0, 0, 0, time.UTC))

	// Reserved dir that must be excluded from `decks` listings.
	_ = os.MkdirAll(filepath.Join(tmp, ".versions"), 0o755)
	_ = os.WriteFile(filepath.Join(tmp, ".versions", "ghost.txt"), []byte("x"), 0o644)

	return tmp
}

// --- Schema construction with new types ----------------------------------

func TestGraphQL_SchemaCompilesWithDeckType(t *testing.T) {
	// New constructor signature: must accept (sm, decksDir) and a
	// non-empty decksDir without complaint.
	h, err := NewGraphQLHandler(seedShowmatch(t), graphqlDeckFixture(t))
	if err != nil {
		t.Fatalf("NewGraphQLHandler with decksDir: %v", err)
	}
	if h == nil {
		t.Fatal("returned handler is nil")
	}
}

// --- deck(owner, id) query -----------------------------------------------

func TestGraphQL_DeckByOwnerID_Found(t *testing.T) {
	dir := graphqlDeckFixture(t)
	h, _ := NewGraphQLHandler(seedShowmatch(t), dir)
	resp := doGraphQL(t, h, `{ deck(owner: "alice", id: "korvold_b3_jund_value") { id owner commander bracket cardCount } }`, nil)
	if errs, ok := resp["errors"]; ok {
		t.Fatalf("unexpected errors: %v", errs)
	}
	d := resp["data"].(map[string]any)["deck"].(map[string]any)
	if d["id"] != "korvold_b3_jund_value" {
		t.Errorf("id = %v, want korvold_b3_jund_value", d["id"])
	}
	if d["owner"] != "alice" {
		t.Errorf("owner = %v, want alice", d["owner"])
	}
	if d["bracket"] != "3" {
		t.Errorf("bracket = %v, want 3 (from _b3_ filename marker)", d["bracket"])
	}
	if cc, ok := d["cardCount"].(float64); !ok || int(cc) < 1 {
		t.Errorf("cardCount = %v, want non-zero int", d["cardCount"])
	}
}

func TestGraphQL_DeckByOwnerID_MissingReturnsNull(t *testing.T) {
	h, _ := NewGraphQLHandler(seedShowmatch(t), graphqlDeckFixture(t))
	resp := doGraphQL(t, h, `{ deck(owner: "alice", id: "no_such_deck") { id } }`, nil)
	if errs, ok := resp["errors"]; ok {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if got := resp["data"].(map[string]any)["deck"]; got != nil {
		t.Errorf("deck: want nil for missing, got %v", got)
	}
}

func TestGraphQL_DeckByOwnerID_PathTraversalRejected(t *testing.T) {
	// validatePathComponent must run BEFORE the filesystem touch so
	// "../" / NUL-byte / slash-bearing owner or id strings can't escape
	// decksDir through the GraphQL surface (the REST handlers gate
	// the same way; the resolver must NOT be the soft spot).
	h, _ := NewGraphQLHandler(seedShowmatch(t), graphqlDeckFixture(t))
	for _, bad := range []struct{ owner, id string }{
		{"..", "korvold_b3_jund_value"},
		{"alice", "../../etc/passwd"},
		{"alice", "korvold/extra"},
		{"alice\x00", "korvold_b3_jund_value"},
		{"", "korvold_b3_jund_value"},
		{"alice", ""},
	} {
		body, _ := json.Marshal(graphqlRequest{
			Query: `query Get($o: String!, $i: String!) { deck(owner: $o, id: $i) { id } }`,
			Variables: map[string]any{"o": bad.owner, "i": bad.id},
		})
		_ = body // doGraphQL marshals from query+vars
		resp := doGraphQL(t, h, `query Get($o: String!, $i: String!) { deck(owner: $o, id: $i) { id } }`,
			map[string]any{"o": bad.owner, "i": bad.id})
		if errs, ok := resp["errors"]; ok {
			// GraphQL-level error is also acceptable — the point is
			// "no deck data returned." Skip the data check.
			_ = errs
			continue
		}
		if got := resp["data"].(map[string]any)["deck"]; got != nil {
			t.Errorf("owner=%q id=%q: traversal MUST yield null, got %v", bad.owner, bad.id, got)
		}
	}
}

// --- decks(owner, limit) query -------------------------------------------

func TestGraphQL_Decks_NoFilterReturnsAllOwners(t *testing.T) {
	h, _ := NewGraphQLHandler(seedShowmatch(t), graphqlDeckFixture(t))
	resp := doGraphQL(t, h, `{ decks { id owner } }`, nil)
	if errs, ok := resp["errors"]; ok {
		t.Fatalf("unexpected errors: %v", errs)
	}
	decks := resp["data"].(map[string]any)["decks"].([]any)
	if len(decks) != 3 {
		t.Fatalf("want 3 decks (alice x2 + bob x1), got %d", len(decks))
	}
	// Owners present in result must include both alice and bob.
	seen := map[string]int{}
	for _, d := range decks {
		seen[d.(map[string]any)["owner"].(string)]++
	}
	if seen["alice"] != 2 || seen["bob"] != 1 {
		t.Errorf("owner breakdown: want {alice:2, bob:1}, got %v", seen)
	}
}

func TestGraphQL_Decks_OwnerFilter(t *testing.T) {
	h, _ := NewGraphQLHandler(seedShowmatch(t), graphqlDeckFixture(t))
	resp := doGraphQL(t, h, `{ decks(owner: "bob") { id owner } }`, nil)
	decks := resp["data"].(map[string]any)["decks"].([]any)
	if len(decks) != 1 {
		t.Fatalf("want 1 bob deck, got %d", len(decks))
	}
	if owner := decks[0].(map[string]any)["owner"].(string); owner != "bob" {
		t.Errorf("filtered owner = %q, want bob", owner)
	}
}

func TestGraphQL_Decks_NewestFirstSort(t *testing.T) {
	h, _ := NewGraphQLHandler(seedShowmatch(t), graphqlDeckFixture(t))
	resp := doGraphQL(t, h, `{ decks(owner: "alice") { id } }`, nil)
	decks := resp["data"].(map[string]any)["decks"].([]any)
	if len(decks) != 2 {
		t.Fatalf("want 2 alice decks, got %d", len(decks))
	}
	// atraxa (mtime 12:00) before korvold (10:00).
	if decks[0].(map[string]any)["id"] != "atraxa_b4_superfriends" {
		t.Errorf("first alice deck = %v, want atraxa_b4_superfriends (newer mtime)",
			decks[0].(map[string]any)["id"])
	}
}

func TestGraphQL_Decks_LimitCaps(t *testing.T) {
	h, _ := NewGraphQLHandler(seedShowmatch(t), graphqlDeckFixture(t))
	resp := doGraphQL(t, h, `{ decks(limit: 2) { id } }`, nil)
	decks := resp["data"].(map[string]any)["decks"].([]any)
	if len(decks) != 2 {
		t.Errorf("limit=2: want 2 results, got %d", len(decks))
	}
}

func TestGraphQL_Decks_NegativeLimitErrors(t *testing.T) {
	h, _ := NewGraphQLHandler(seedShowmatch(t), graphqlDeckFixture(t))
	resp := doGraphQL(t, h, `{ decks(limit: -1) { id } }`, nil)
	if _, ok := resp["errors"]; !ok {
		t.Errorf("expected errors for negative limit, got %v", resp)
	}
}

func TestGraphQL_Decks_ExcludesReservedDirs(t *testing.T) {
	// .versions/ghost.txt was planted by the fixture; it must NOT
	// appear in the decks listing.
	h, _ := NewGraphQLHandler(seedShowmatch(t), graphqlDeckFixture(t))
	resp := doGraphQL(t, h, `{ decks { id owner } }`, nil)
	decks := resp["data"].(map[string]any)["decks"].([]any)
	for _, d := range decks {
		owner := d.(map[string]any)["owner"].(string)
		if owner == ".versions" {
			t.Errorf("reserved dir .versions leaked into decks listing: %v", d)
		}
	}
}

// --- Game.decks cross-reference ------------------------------------------

func TestGraphQL_GameDecks_CrossReferenceResolves(t *testing.T) {
	// Seed: the showmatch fixture's game #1 has deckKeys
	// ["alice/k", "bob/a", "carol/m", "dave/b"] which DON'T match the
	// deck fixture (the POC test seed used short ids). Reseed a
	// Showmatch with deckKeys that point at our fixture files so the
	// cross-reference actually resolves.
	dir := graphqlDeckFixture(t)
	sm := &Showmatch{
		gameHistory: []CompletedGame{
			{
				GameID:     42,
				Winner:     0,
				WinnerName: "Korvold",
				Commanders: []string{"Korvold", "Atraxa", "Muldrotha", "Ghost"},
				DeckKeys: []string{
					"alice/korvold_b3_jund_value",
					"alice/atraxa_b4_superfriends",
					"bob/muldrotha_b2_jank",
					"alice/does_not_exist", // proves null-on-missing for one entry
				},
				Turns:      11,
				EndReason:  "lethal",
				FinishedAt: time.Date(2026, 5, 24, 14, 0, 0, 0, time.UTC),
			},
		},
	}
	h, err := NewGraphQLHandler(sm, dir)
	if err != nil {
		t.Fatalf("NewGraphQLHandler: %v", err)
	}
	resp := doGraphQL(t, h, `{ game(id: 42) { decks { owner id bracket } } }`, nil)
	if errs, ok := resp["errors"]; ok {
		t.Fatalf("unexpected errors: %v", errs)
	}
	g := resp["data"].(map[string]any)["game"].(map[string]any)
	decks := g["decks"].([]any)
	if len(decks) != 4 {
		t.Fatalf("decks list length = %d, want 4 (one per deckKey)", len(decks))
	}
	// First three resolve to real Deck objects; fourth is null.
	for i, want := range []string{"alice", "alice", "bob", ""} {
		if want == "" {
			if decks[i] != nil {
				t.Errorf("decks[%d]: missing deckKey must surface as null, got %v", i, decks[i])
			}
			continue
		}
		d, ok := decks[i].(map[string]any)
		if !ok {
			t.Fatalf("decks[%d]: want object, got %T (%v)", i, decks[i], decks[i])
		}
		if d["owner"] != want {
			t.Errorf("decks[%d].owner = %v, want %v", i, d["owner"], want)
		}
	}
	// Spot-check that bracket flowed through the cross-reference too.
	if b := decks[0].(map[string]any)["bracket"]; b != "3" {
		t.Errorf("decks[0].bracket = %v, want 3 (from _b3_ marker)", b)
	}
}

// --- Field projection on Deck --------------------------------------------

func TestGraphQL_Deck_FieldProjection(t *testing.T) {
	// Same load-bearing claim as the POC test for Game: only the
	// requested fields appear in the response.
	h, _ := NewGraphQLHandler(seedShowmatch(t), graphqlDeckFixture(t))
	resp := doGraphQL(t, h, `{ deck(owner: "alice", id: "korvold_b3_jund_value") { id owner } }`, nil)
	d := resp["data"].(map[string]any)["deck"].(map[string]any)
	if _, ok := d["id"]; !ok {
		t.Errorf("id missing")
	}
	if _, ok := d["owner"]; !ok {
		t.Errorf("owner missing")
	}
	for _, omitted := range []string{"commander", "bracket", "color", "archetype", "cardCount", "importedAt", "name"} {
		if _, ok := d[omitted]; ok {
			t.Errorf("field projection failed: %s returned despite not being requested (response=%v)",
				omitted, d)
		}
	}
}

// --- Variables thread through new queries --------------------------------

func TestGraphQL_Variables_OnDeckQuery(t *testing.T) {
	h, _ := NewGraphQLHandler(seedShowmatch(t), graphqlDeckFixture(t))
	q := `query GetDeck($o: String!, $i: String!) { deck(owner: $o, id: $i) { commander bracket } }`
	resp := doGraphQL(t, h, q, map[string]any{"o": "bob", "i": "muldrotha_b2_jank"})
	if errs, ok := resp["errors"]; ok {
		t.Fatalf("unexpected errors: %v", errs)
	}
	d := resp["data"].(map[string]any)["deck"].(map[string]any)
	if d["bracket"] != "2" {
		t.Errorf("bracket via vars = %v, want 2", d["bracket"])
	}
}

func TestGraphQL_Variables_OnDecksQuery(t *testing.T) {
	h, _ := NewGraphQLHandler(seedShowmatch(t), graphqlDeckFixture(t))
	q := `query GetDecks($o: String, $n: Int!) { decks(owner: $o, limit: $n) { id } }`
	resp := doGraphQL(t, h, q, map[string]any{"o": "alice", "n": 1})
	if errs, ok := resp["errors"]; ok {
		t.Fatalf("unexpected errors: %v", errs)
	}
	decks := resp["data"].(map[string]any)["decks"].([]any)
	if len(decks) != 1 {
		t.Errorf("limit=1 via vars: want 1 result, got %d", len(decks))
	}
}

// --- Empty decksDir degrades gracefully ----------------------------------

func TestGraphQL_EmptyDecksDirReturnsEmptyOrNull(t *testing.T) {
	// Schema construction with decksDir="" must succeed; the deck
	// queries simply return null/empty so the graph degrades
	// gracefully on partial wiring.
	h, err := NewGraphQLHandler(seedShowmatch(t), "")
	if err != nil {
		t.Fatalf("NewGraphQLHandler with empty decksDir: %v", err)
	}
	resp := doGraphQL(t, h, `{ deck(owner: "alice", id: "korvold") { id } decks { id } }`, nil)
	if errs, ok := resp["errors"]; ok {
		t.Fatalf("unexpected errors: %v", errs)
	}
	data := resp["data"].(map[string]any)
	if data["deck"] != nil {
		t.Errorf("deck on empty decksDir: want null, got %v", data["deck"])
	}
	if decks, ok := data["decks"].([]any); !ok || len(decks) != 0 {
		t.Errorf("decks on empty decksDir: want empty list, got %v", data["decks"])
	}
}

// --- loadGraphQLDeckSummary helper unit tests ----------------------------

func TestLoadGraphQLDeckSummary_NilOnInvalidPath(t *testing.T) {
	dir := graphqlDeckFixture(t)
	for _, c := range []struct{ owner, id string }{
		{"..", "x"}, {"alice", "../escape"}, {"", "x"}, {"alice", ""},
		{"alice\x00", "x"}, {"alice", "x\x00"},
	} {
		if ds := loadGraphQLDeckSummary(dir, c.owner, c.id); ds != nil {
			t.Errorf("loadGraphQLDeckSummary(owner=%q, id=%q) = non-nil, want nil (path validation)",
				c.owner, c.id)
		}
	}
}

func TestLoadGraphQLDeckSummary_NilOnEmptyDecksDir(t *testing.T) {
	if ds := loadGraphQLDeckSummary("", "alice", "korvold"); ds != nil {
		t.Errorf("empty decksDir: want nil, got %v", ds)
	}
}

func TestLoadGraphQLDeckSummary_NilOnMissingFile(t *testing.T) {
	dir := graphqlDeckFixture(t)
	if ds := loadGraphQLDeckSummary(dir, "alice", "definitely_not_here"); ds != nil {
		t.Errorf("missing deck: want nil, got %v", ds)
	}
}

func TestLoadGraphQLDeckSummary_PopulatesFields(t *testing.T) {
	dir := graphqlDeckFixture(t)
	ds := loadGraphQLDeckSummary(dir, "alice", "korvold_b3_jund_value")
	if ds == nil {
		t.Fatal("expected populated DeckSummary, got nil")
	}
	if ds.Owner != "alice" || ds.ID != "korvold_b3_jund_value" {
		t.Errorf("owner/id mismatch: %+v", ds)
	}
	if ds.Bracket != "3" {
		t.Errorf("bracket = %q, want 3", ds.Bracket)
	}
	if ds.ImportedAt.IsZero() {
		t.Errorf("ImportedAt should be set from mtime")
	}
}

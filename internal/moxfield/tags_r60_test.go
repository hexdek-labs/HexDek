package moxfield

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

// tags_r60_test.go — pins Moxfield user-applied tag capture: the
// Hubs[] (canonical community labels — Combo, Token, Stax) and Tags[]
// (free-form per-user strings) get merged, deduped case-insensitively,
// sorted, and surfaced via FetchDeckMeta + an `// Tags:` comment in
// formatDecklist output. Verified against realistic v3 API JSON.

// ---------------------------------------------------------------------------
// deckTags helper: input shapes
// ---------------------------------------------------------------------------

func TestDeckTags_HubsOnly(t *testing.T) {
	r := &apiResponse{
		Hubs: []apiHub{
			{Name: "Combo", Slug: "combo"},
			{Name: "Token", Slug: "token"},
		},
	}
	got := r.deckTags()
	want := []string{"Combo", "Token"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("deckTags(Hubs only) = %v, want %v", got, want)
	}
}

func TestDeckTags_TagsOnly(t *testing.T) {
	r := &apiResponse{
		Tags: []string{"Voltron", "Equipment"},
	}
	got := r.deckTags()
	want := []string{"Equipment", "Voltron"} // sorted
	if !reflect.DeepEqual(got, want) {
		t.Errorf("deckTags(Tags only) = %v, want %v", got, want)
	}
}

func TestDeckTags_BothMergedAndDeduped(t *testing.T) {
	r := &apiResponse{
		Hubs: []apiHub{
			{Name: "Combo", Slug: "combo"},
			{Name: "Token", Slug: "token"},
		},
		Tags: []string{
			"combo",       // dupe of Hub "Combo" — Hub case wins (iterated first)
			"Aristocrats", // unique
			"TOKEN",       // dupe of Hub "Token"
		},
	}
	got := r.deckTags()
	want := []string{"Aristocrats", "Combo", "Token"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("deckTags(both, dedup) = %v, want %v", got, want)
	}
}

func TestDeckTags_DeterministicSortAcrossRuns(t *testing.T) {
	r := &apiResponse{
		Hubs: []apiHub{
			{Name: "Tribal"},
			{Name: "Combo"},
			{Name: "Stax"},
		},
	}
	first := r.deckTags()
	for i := 0; i < 30; i++ {
		got := r.deckTags()
		if !reflect.DeepEqual(got, first) {
			t.Fatalf("deckTags non-deterministic: run %d = %v, first = %v", i, got, first)
		}
	}
	// Spot-check the sort.
	want := []string{"Combo", "Stax", "Tribal"}
	if !reflect.DeepEqual(first, want) {
		t.Errorf("deckTags sort = %v, want %v", first, want)
	}
}

func TestDeckTags_EmptyAndWhitespaceDropped(t *testing.T) {
	r := &apiResponse{
		Hubs: []apiHub{
			{Name: ""},
			{Name: "  "},
			{Name: "Combo"},
		},
		Tags: []string{"", "\t\n", "Token"},
	}
	got := r.deckTags()
	want := []string{"Combo", "Token"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("deckTags(empties dropped) = %v, want %v", got, want)
	}
}

func TestDeckTags_NoTagsReturnsEmpty(t *testing.T) {
	r := &apiResponse{}
	got := r.deckTags()
	if len(got) != 0 {
		t.Errorf("deckTags(no tags) should be empty; got %v", got)
	}
}

// ---------------------------------------------------------------------------
// Fixture-based JSON parsing + formatDecklist integration
// ---------------------------------------------------------------------------

const fixtureHubsOnly = `{
	"name": "Krenko Token Shower",
	"format": "commander",
	"hubs": [
		{"name": "Tribal", "slug": "tribal"},
		{"name": "Token", "slug": "token"},
		{"name": "Goblins", "slug": "goblins"}
	],
	"boards": {
		"commanders": {"count": 1, "cards": {"c": {"quantity": 1, "card": {"name": "Krenko, Mob Boss"}}}},
		"mainboard":  {"count": 1, "cards": {"m": {"quantity": 1, "card": {"name": "Goblin Bombardment"}}}}
	}
}`

func TestFormatDecklist_HubsBecomeTagsComment(t *testing.T) {
	var r apiResponse
	if err := json.Unmarshal([]byte(fixtureHubsOnly), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := formatDecklist(&r)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	// Expect: "// Tags: Goblins, Token, Tribal\n" (alphabetical)
	want := "// Tags: Goblins, Token, Tribal\n"
	if !strings.Contains(out, want) {
		t.Errorf("missing %q in output:\n%s", want, out)
	}
}

func TestFormatDecklist_TagsLineAppearsBeforeCommanders(t *testing.T) {
	var r apiResponse
	if err := json.Unmarshal([]byte(fixtureHubsOnly), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := formatDecklist(&r)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	idxTags := strings.Index(out, "// Tags:")
	idxCommander := strings.Index(out, "COMMANDER:")
	if idxTags < 0 || idxCommander < 0 {
		t.Fatalf("missing tags or commander line:\n%s", out)
	}
	if idxTags >= idxCommander {
		t.Errorf("// Tags: should precede COMMANDER (tags@%d commander@%d)", idxTags, idxCommander)
	}
}

const fixtureMixedTags = `{
	"name": "Mixed Tags Deck",
	"format": "commander",
	"hubs": [
		{"name": "Combo", "slug": "combo"},
		{"name": "Storm", "slug": "storm"}
	],
	"tags": ["combo", "homebrew", "STORM"],
	"boards": {
		"commanders": {"count": 1, "cards": {"c": {"quantity": 1, "card": {"name": "Mizzix of the Izmagnus"}}}},
		"mainboard":  {"count": 1, "cards": {"m": {"quantity": 1, "card": {"name": "Sol Ring"}}}}
	}
}`

func TestFormatDecklist_HubsAndTagsMergedDeduped(t *testing.T) {
	var r apiResponse
	if err := json.Unmarshal([]byte(fixtureMixedTags), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := formatDecklist(&r)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	// "combo" + "Combo" → "Combo"; "STORM" + "Storm" → "Storm";
	// "homebrew" unique.
	want := "// Tags: Combo, Storm, homebrew\n"
	if !strings.Contains(out, want) {
		t.Errorf("missing %q (merged+deduped+sorted) in:\n%s", want, out)
	}
	// Negative: dupe variants must not appear standalone.
	for _, forbidden := range []string{
		"// Tags: combo,",
		", combo,",
		", STORM,",
		"// Tags: STORM",
	} {
		if strings.Contains(out, forbidden) {
			t.Errorf("dedup leaked %q in:\n%s", forbidden, out)
		}
	}
}

func TestFormatDecklist_NoTagsLineWhenNoTags(t *testing.T) {
	const noTags = `{
		"name": "Untagged",
		"format": "commander",
		"boards": {
			"commanders": {"count": 1, "cards": {"c": {"quantity": 1, "card": {"name": "Atraxa, Praetors' Voice"}}}},
			"mainboard":  {"count": 1, "cards": {"m": {"quantity": 1, "card": {"name": "Sol Ring"}}}}
		}
	}`
	var r apiResponse
	if err := json.Unmarshal([]byte(noTags), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := formatDecklist(&r)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	if strings.Contains(out, "// Tags:") {
		t.Errorf("no-tag deck should not emit `// Tags:`; got:\n%s", out)
	}
}

func TestFormatDecklist_TagsOnlyNoCardsStillErrors(t *testing.T) {
	// Sanity: a deck with tags but zero cards must still error rather
	// than silently produce a tags-only file. The internal cards-buffer
	// check guards this — emit-then-check on a single builder would
	// have let "// Tags: X\n" pass as non-empty.
	const tagsOnly = `{
		"name": "Just Tags",
		"format": "commander",
		"hubs": [{"name": "Combo"}],
		"boards": {}
	}`
	var r apiResponse
	if err := json.Unmarshal([]byte(tagsOnly), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, err := formatDecklist(&r); err == nil {
		t.Fatal("expected error for cards-less deck even when tags present")
	}
}

// ---------------------------------------------------------------------------
// FetchDeckMeta API — exercised through stub server, shares the cache
// ---------------------------------------------------------------------------

const fixtureFetchMeta = `{
	"name": "Yuriko Ninjas",
	"format": "commander",
	"hubs": [
		{"name": "Ninjas", "slug": "ninjas"},
		{"name": "Combo", "slug": "combo"}
	],
	"tags": ["control", "combo"],
	"boards": {
		"commanders": {"count": 1, "cards": {"c": {"quantity": 1, "card": {"name": "Yuriko, the Tiger's Shadow"}}}},
		"mainboard":  {"count": 1, "cards": {"m": {"quantity": 1, "card": {"name": "Sol Ring"}}}}
	}
}`

func newTagsStub(t *testing.T, body string) (counter *int32, cleanup func()) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	origBase := apiBaseVar
	apiBaseVar = srv.URL

	tmp := t.TempDir()
	t.Setenv(cacheEnvDir, tmp)
	t.Setenv(cacheEnvDisable, "")
	t.Setenv(cacheEnvTTL, "")
	t.Setenv(snapshotEnvDir, filepath.Join(tmp, "snapshots"))
	resetCachesForTest()

	return &hits, func() {
		srv.Close()
		apiBaseVar = origBase
	}
}

func TestFetchDeckMeta_ReturnsNameAndTags(t *testing.T) {
	_, cleanup := newTagsStub(t, fixtureFetchMeta)
	defer cleanup()

	meta, err := FetchDeckMeta("https://moxfield.com/decks/meta01")
	if err != nil {
		t.Fatalf("FetchDeckMeta: %v", err)
	}
	if meta.Name != "Yuriko Ninjas" {
		t.Errorf("Name = %q, want %q", meta.Name, "Yuriko Ninjas")
	}
	want := []string{"Combo", "Ninjas", "control"}
	if !reflect.DeepEqual(meta.Tags, want) {
		t.Errorf("Tags = %v, want %v", meta.Tags, want)
	}
}

func TestFetchDeckMeta_SharesCacheWithFetchDeck(t *testing.T) {
	hits, cleanup := newTagsStub(t, fixtureFetchMeta)
	defer cleanup()

	// All three back-to-back calls must hit the API at most once.
	if _, err := FetchDeckMeta("https://moxfield.com/decks/cache01"); err != nil {
		t.Fatalf("meta: %v", err)
	}
	if _, err := FetchDeckName("https://moxfield.com/decks/cache01"); err != nil {
		t.Fatalf("name: %v", err)
	}
	if _, err := FetchDeck("https://moxfield.com/decks/cache01"); err != nil {
		t.Fatalf("deck: %v", err)
	}
	if got := atomic.LoadInt32(hits); got != 1 {
		t.Fatalf("meta+name+deck should share 1 HTTP call; got %d", got)
	}
}

func TestFetchDeckMeta_BadURLErrors(t *testing.T) {
	_, cleanup := newTagsStub(t, fixtureFetchMeta)
	defer cleanup()
	if _, err := FetchDeckMeta("https://example.com/not-moxfield"); err == nil {
		t.Fatal("expected error for non-Moxfield URL")
	}
}

// End-to-end: FetchDeck (text path) must include the // Tags: line in
// its output when the API returns tags.
func TestFetchDeck_IncludesTagsCommentInOutput(t *testing.T) {
	_, cleanup := newTagsStub(t, fixtureFetchMeta)
	defer cleanup()

	text, err := FetchDeck("https://moxfield.com/decks/text01")
	if err != nil {
		t.Fatalf("FetchDeck: %v", err)
	}
	if !strings.Contains(text, "// Tags: Combo, Ninjas, control\n") {
		t.Errorf("text path missing // Tags: line:\n%s", text)
	}
	if !strings.Contains(text, "COMMANDER: Yuriko, the Tiger's Shadow\n") {
		t.Errorf("missing commander line:\n%s", text)
	}
}

// Hub slug + description fields not used by us shouldn't break
// parsing or leak into output.
func TestFormatDecklist_HubExtrasIgnored(t *testing.T) {
	const fixtureHubExtras = `{
		"name": "Extras Test",
		"format": "commander",
		"hubs": [
			{
				"name": "+1/+1 Counters",
				"slug": "1-1-counters",
				"description": "Decks focused on placing +1/+1 counters",
				"image_url": "https://moxfield.com/img/counters.png",
				"cardCount": 9876
			}
		],
		"boards": {
			"commanders": {"count": 1, "cards": {"c": {"quantity": 1, "card": {"name": "Atraxa, Praetors' Voice"}}}},
			"mainboard":  {"count": 1, "cards": {"m": {"quantity": 1, "card": {"name": "Sol Ring"}}}}
		}
	}`
	var r apiResponse
	if err := json.Unmarshal([]byte(fixtureHubExtras), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := formatDecklist(&r)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	if !strings.Contains(out, "// Tags: +1/+1 Counters\n") {
		t.Errorf("missing +1/+1 Counters tag in:\n%s", out)
	}
	for _, leaked := range []string{
		"1-1-counters", "Decks focused", "image_url", "moxfield.com/img",
		"cardCount", "9876", "slug",
	} {
		if strings.Contains(out, leaked) {
			t.Errorf("Hub noise field %q leaked into output:\n%s", leaked, out)
		}
	}
}

// Deck with many tags — make sure the join is stable and doesn't add
// trailing commas or whitespace.
func TestFormatDecklist_ManyTagsCleanJoin(t *testing.T) {
	const many = `{
		"name": "Many",
		"format": "commander",
		"hubs": [{"name":"A"},{"name":"B"},{"name":"C"},{"name":"D"},{"name":"E"}],
		"boards": {
			"commanders": {"count": 1, "cards": {"c": {"quantity": 1, "card": {"name": "Atraxa, Praetors' Voice"}}}},
			"mainboard":  {"count": 1, "cards": {"m": {"quantity": 1, "card": {"name": "Sol Ring"}}}}
		}
	}`
	var r apiResponse
	if err := json.Unmarshal([]byte(many), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := formatDecklist(&r)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	// Exact-match the tags line — no leading whitespace, no trailing
	// comma, single space after each comma.
	want := "// Tags: A, B, C, D, E\n"
	if !strings.HasPrefix(out, want) {
		t.Errorf("tags line should be first; want prefix %q, got:\n%s", want, out)
	}
}

// Quick smoke: a JSON missing the hubs/tags fields entirely
// (untagged deck) still parses without error and produces no tags
// line — pins backwards compat against legacy cache files.
func TestFormatDecklist_LegacyCacheWithoutHubsField(t *testing.T) {
	const legacy = `{
		"name": "Legacy",
		"format": "commander",
		"mainboard": {"m": {"quantity": 99, "card": {"name": "Island"}}},
		"commanders": {"c": {"quantity": 1, "card": {"name": "Yuriko, the Tiger's Shadow"}}}
	}`
	var r apiResponse
	if err := json.Unmarshal([]byte(legacy), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := formatDecklist(&r)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	if strings.Contains(out, "// Tags:") {
		t.Errorf("legacy fixture has no tags; should not emit // Tags:; got:\n%s", out)
	}
	if !strings.Contains(out, "COMMANDER: Yuriko") {
		t.Errorf("legacy commander missing:\n%s", out)
	}
	_ = fmt.Sprintf // touch import in case future edits drop usage
}

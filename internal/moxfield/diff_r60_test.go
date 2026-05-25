package moxfield

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// diff_r60_test.go — pins moxfield.DiffSnapshots + DiffLatestSnapshots,
// the version-history feature surfaced on re-import.

// helper: build an apiResponse with a single mainboard card
func deckJSON(t *testing.T, name, format string, hubs []string, boards map[string]map[string]int) []byte {
	t.Helper()
	mkBoard := func(cards map[string]int) map[string]any {
		entries := map[string]any{}
		for n, q := range cards {
			entries[n] = map[string]any{
				"quantity": q,
				"card":     map[string]any{"name": n},
			}
		}
		return map[string]any{"count": len(entries), "cards": entries}
	}
	hubObjs := []map[string]string{}
	for _, h := range hubs {
		hubObjs = append(hubObjs, map[string]string{"name": h})
	}
	obj := map[string]any{
		"name":   name,
		"format": format,
		"hubs":   hubObjs,
		"boards": map[string]any{
			"commanders":  mkBoard(boards["commanders"]),
			"mainboard":   mkBoard(boards["mainboard"]),
			"sideboard":   mkBoard(boards["sideboard"]),
			"companions":  mkBoard(boards["companions"]),
			"maybeboard":  mkBoard(boards["maybeboard"]),
			"considering": mkBoard(boards["considering"]),
		},
	}
	bytes, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	return bytes
}

// ---------------------------------------------------------------------------
// DiffSnapshots unit tests
// ---------------------------------------------------------------------------

func TestDiff_IdenticalSnapshotsEmpty(t *testing.T) {
	raw := deckJSON(t, "Same", "commander", []string{"Combo"}, map[string]map[string]int{
		"commanders": {"Atraxa, Praetors' Voice": 1},
		"mainboard":  {"Sol Ring": 1, "Mana Crypt": 1},
	})
	d, err := DiffSnapshots(raw, raw)
	if err != nil {
		t.Fatalf("DiffSnapshots: %v", err)
	}
	if !d.IsEmpty() {
		t.Errorf("identical snapshots must produce empty diff; got %s", d.String())
	}
}

func TestDiff_AddedAndRemovedCards(t *testing.T) {
	oldRaw := deckJSON(t, "X", "commander", nil, map[string]map[string]int{
		"commanders": {"Yuriko, the Tiger's Shadow": 1},
		"mainboard":  {"Sol Ring": 1, "Brainstorm": 1, "Counterspell": 1},
	})
	newRaw := deckJSON(t, "X", "commander", nil, map[string]map[string]int{
		"commanders": {"Yuriko, the Tiger's Shadow": 1},
		"mainboard":  {"Sol Ring": 1, "Mana Crypt": 1, "Force of Will": 1}, // -Brainstorm -Counterspell +Mana Crypt +Force of Will
	})
	d, err := DiffSnapshots(oldRaw, newRaw)
	if err != nil {
		t.Fatalf("DiffSnapshots: %v", err)
	}
	if d.IsEmpty() {
		t.Fatal("expected non-empty diff")
	}
	mb := d.Boards["mainboard"]
	if mb == nil {
		t.Fatalf("missing mainboard diff")
	}
	addedNames := []string{}
	for _, c := range mb.Added {
		addedNames = append(addedNames, c.Name)
	}
	if !reflect.DeepEqual(addedNames, []string{"Force of Will", "Mana Crypt"}) {
		t.Errorf("Added (sorted) = %v, want [Force of Will, Mana Crypt]", addedNames)
	}
	removedNames := []string{}
	for _, c := range mb.Removed {
		removedNames = append(removedNames, c.Name)
	}
	if !reflect.DeepEqual(removedNames, []string{"Brainstorm", "Counterspell"}) {
		t.Errorf("Removed (sorted) = %v, want [Brainstorm, Counterspell]", removedNames)
	}
	if len(mb.Changed) != 0 {
		t.Errorf("no qty changes expected; got %v", mb.Changed)
	}
}

func TestDiff_QuantityChange(t *testing.T) {
	oldRaw := deckJSON(t, "X", "modern", nil, map[string]map[string]int{
		"mainboard": {"Lightning Bolt": 4, "Island": 20},
	})
	newRaw := deckJSON(t, "X", "modern", nil, map[string]map[string]int{
		"mainboard": {"Lightning Bolt": 2, "Island": 22},
	})
	d, _ := DiffSnapshots(oldRaw, newRaw)
	mb := d.Boards["mainboard"]
	if len(mb.Changed) != 2 {
		t.Fatalf("expected 2 qty changes, got %v", mb.Changed)
	}
	want := map[string][2]int{
		"Lightning Bolt": {4, 2},
		"Island":         {20, 22},
	}
	for _, c := range mb.Changed {
		w, ok := want[c.Name]
		if !ok {
			t.Errorf("unexpected qty change for %s", c.Name)
			continue
		}
		if c.OldQty != w[0] || c.NewQty != w[1] {
			t.Errorf("%s: %d→%d, want %d→%d", c.Name, c.OldQty, c.NewQty, w[0], w[1])
		}
	}
	if len(mb.Added) > 0 || len(mb.Removed) > 0 {
		t.Errorf("qty-only change shouldn't produce Add/Remove; got +%v -%v", mb.Added, mb.Removed)
	}
}

func TestDiff_NameAndFormatChange(t *testing.T) {
	oldRaw := deckJSON(t, "Old Name", "commander", nil, map[string]map[string]int{
		"commanders": {"Atraxa, Praetors' Voice": 1},
		"mainboard":  {"Sol Ring": 1},
	})
	newRaw := deckJSON(t, "Shiny New Name", "brawl", nil, map[string]map[string]int{
		"commanders": {"Atraxa, Praetors' Voice": 1},
		"mainboard":  {"Sol Ring": 1},
	})
	d, _ := DiffSnapshots(oldRaw, newRaw)
	if !d.NameChanged {
		t.Errorf("NameChanged should be true")
	}
	if d.NameOld != "Old Name" || d.NameNew != "Shiny New Name" {
		t.Errorf("name diff: %q→%q", d.NameOld, d.NameNew)
	}
	if !d.FormatChanged {
		t.Errorf("FormatChanged should be true")
	}
	if d.FormatOld != "commander" || d.FormatNew != "brawl" {
		t.Errorf("format diff: %q→%q", d.FormatOld, d.FormatNew)
	}
}

func TestDiff_TagsAddedAndRemoved(t *testing.T) {
	oldRaw := deckJSON(t, "X", "commander", []string{"Combo", "Tribal"}, map[string]map[string]int{
		"commanders": {"Atraxa, Praetors' Voice": 1},
		"mainboard":  {"Sol Ring": 1},
	})
	newRaw := deckJSON(t, "X", "commander", []string{"Combo", "Storm", "Token"}, map[string]map[string]int{
		"commanders": {"Atraxa, Praetors' Voice": 1},
		"mainboard":  {"Sol Ring": 1},
	})
	d, _ := DiffSnapshots(oldRaw, newRaw)
	if !reflect.DeepEqual(d.TagsAdded, []string{"Storm", "Token"}) {
		t.Errorf("TagsAdded = %v, want [Storm Token]", d.TagsAdded)
	}
	if !reflect.DeepEqual(d.TagsRemoved, []string{"Tribal"}) {
		t.Errorf("TagsRemoved = %v, want [Tribal]", d.TagsRemoved)
	}
}

func TestDiff_PerBoardIsolation(t *testing.T) {
	// Changes in mainboard must not be reported on sideboard, and vice versa.
	oldRaw := deckJSON(t, "X", "commander", nil, map[string]map[string]int{
		"commanders": {"Krenko, Mob Boss": 1},
		"mainboard":  {"Goblin Bombardment": 1},
		"sideboard":  {"Pithing Needle": 1},
	})
	newRaw := deckJSON(t, "X", "commander", nil, map[string]map[string]int{
		"commanders": {"Krenko, Mob Boss": 1},
		"mainboard":  {"Skullclamp": 1},      // mainboard swap
		"sideboard":  {"Pithing Needle": 1},  // unchanged
	})
	d, _ := DiffSnapshots(oldRaw, newRaw)
	mb := d.Boards["mainboard"]
	if len(mb.Added) != 1 || mb.Added[0].Name != "Skullclamp" {
		t.Errorf("mainboard Added = %+v, want [Skullclamp]", mb.Added)
	}
	if len(mb.Removed) != 1 || mb.Removed[0].Name != "Goblin Bombardment" {
		t.Errorf("mainboard Removed = %+v, want [Goblin Bombardment]", mb.Removed)
	}
	sb := d.Boards["sideboard"]
	if sb == nil {
		t.Fatal("sideboard diff should exist (board present in both sides)")
	}
	if !sb.IsEmpty() {
		t.Errorf("sideboard unchanged should be empty; got Added=%v Removed=%v Changed=%v",
			sb.Added, sb.Removed, sb.Changed)
	}
}

func TestDiff_BoardAppearsOrDisappears(t *testing.T) {
	// New maybeboard appearing → all entries in Added.
	oldRaw := deckJSON(t, "X", "commander", nil, map[string]map[string]int{
		"commanders": {"Yuriko, the Tiger's Shadow": 1},
		"mainboard":  {"Sol Ring": 1},
	})
	newRaw := deckJSON(t, "X", "commander", nil, map[string]map[string]int{
		"commanders": {"Yuriko, the Tiger's Shadow": 1},
		"mainboard":  {"Sol Ring": 1},
		"maybeboard": {"Bitterblossom": 1, "Force of Will": 1},
	})
	d, _ := DiffSnapshots(oldRaw, newRaw)
	mb := d.Boards["maybeboard"]
	if mb == nil {
		t.Fatal("maybeboard diff missing")
	}
	if len(mb.Added) != 2 {
		t.Errorf("expected 2 added to new maybeboard, got %v", mb.Added)
	}
	if len(mb.Removed) != 0 {
		t.Errorf("nothing should be removed from never-existed maybeboard, got %v", mb.Removed)
	}
}

func TestDiff_FirstImportAgainstNothing(t *testing.T) {
	// Old side empty → every card in new is an Add.
	newRaw := deckJSON(t, "X", "commander", []string{"Combo"}, map[string]map[string]int{
		"commanders": {"Atraxa, Praetors' Voice": 1},
		"mainboard":  {"Sol Ring": 1, "Mana Crypt": 1},
	})
	d, err := DiffSnapshots(nil, newRaw)
	if err != nil {
		t.Fatalf("DiffSnapshots: %v", err)
	}
	mb := d.Boards["mainboard"]
	if len(mb.Added) != 2 {
		t.Errorf("first-import mainboard should be 2 Added, got %v", mb.Added)
	}
	c := d.Boards["commanders"]
	if len(c.Added) != 1 {
		t.Errorf("first-import commanders should be 1 Added, got %v", c.Added)
	}
	if !d.NameChanged || d.NameOld != "" || d.NameNew != "X" {
		t.Errorf("name should appear as change from '' to X; got %+v", d)
	}
	if !reflect.DeepEqual(d.TagsAdded, []string{"Combo"}) {
		t.Errorf("Combo should appear as tag-added; got %v", d.TagsAdded)
	}
}

func TestDiff_MalformedJSONErrors(t *testing.T) {
	_, err := DiffSnapshots([]byte(`{not valid`), []byte(`{}`))
	if err == nil {
		t.Error("expected error on malformed old")
	}
	_, err = DiffSnapshots([]byte(`{}`), []byte(`{not valid`))
	if err == nil {
		t.Error("expected error on malformed new")
	}
}

func TestDiff_StringSkipsUnchangedBoards(t *testing.T) {
	oldRaw := deckJSON(t, "X", "commander", nil, map[string]map[string]int{
		"commanders": {"Atraxa, Praetors' Voice": 1},
		"mainboard":  {"Sol Ring": 1, "Mana Crypt": 1},
		"sideboard":  {"Pithing Needle": 1},
	})
	newRaw := deckJSON(t, "X", "commander", nil, map[string]map[string]int{
		"commanders": {"Atraxa, Praetors' Voice": 1},
		"mainboard":  {"Sol Ring": 1, "Mana Vault": 1}, // swap Mana Crypt → Mana Vault
		"sideboard":  {"Pithing Needle": 1},
	})
	d, _ := DiffSnapshots(oldRaw, newRaw)
	s := d.String()
	if strings.Contains(s, "commanders:") {
		t.Errorf("commanders unchanged but appears in output:\n%s", s)
	}
	if strings.Contains(s, "sideboard:") {
		t.Errorf("sideboard unchanged but appears in output:\n%s", s)
	}
	if !strings.Contains(s, "mainboard:") {
		t.Errorf("mainboard changed but missing from output:\n%s", s)
	}
	if !strings.Contains(s, "+ 1 Mana Vault") {
		t.Errorf("missing add line in:\n%s", s)
	}
	if !strings.Contains(s, "- 1 Mana Crypt") {
		t.Errorf("missing remove line in:\n%s", s)
	}
}

func TestDiff_StringEmptyDiff(t *testing.T) {
	raw := deckJSON(t, "X", "commander", nil, map[string]map[string]int{
		"commanders": {"Atraxa, Praetors' Voice": 1},
		"mainboard":  {"Sol Ring": 1},
	})
	d, _ := DiffSnapshots(raw, raw)
	if got := d.String(); got != "(no changes)" {
		t.Errorf("empty diff string = %q, want %q", got, "(no changes)")
	}
}

func TestDiff_StringIncludesQuantityChange(t *testing.T) {
	oldRaw := deckJSON(t, "X", "modern", nil, map[string]map[string]int{
		"mainboard": {"Lightning Bolt": 4, "Island": 18},
	})
	newRaw := deckJSON(t, "X", "modern", nil, map[string]map[string]int{
		"mainboard": {"Lightning Bolt": 2, "Island": 20},
	})
	d, _ := DiffSnapshots(oldRaw, newRaw)
	s := d.String()
	if !strings.Contains(s, "~ Lightning Bolt: 4 → 2") {
		t.Errorf("missing quantity-change line for Lightning Bolt in:\n%s", s)
	}
	if !strings.Contains(s, "~ Island: 18 → 20") {
		t.Errorf("missing quantity-change line for Island in:\n%s", s)
	}
}

// ---------------------------------------------------------------------------
// DiffLatestSnapshots integration with snapshot system
// ---------------------------------------------------------------------------

// snapshotDiffEnv isolates cache + snapshot dirs and returns the
// snapshot dir path.
func snapshotDiffEnv(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv(cacheEnvDir, tmp)
	t.Setenv(cacheEnvDisable, "")
	t.Setenv(snapshotEnvDir, filepath.Join(tmp, "snapshots"))
	t.Setenv(snapshotEnvDisable, "")
	resetCachesForTest()
}

func TestDiffLatestSnapshots_NoSnapshotsReturnsNil(t *testing.T) {
	snapshotDiffEnv(t)
	d, err := DiffLatestSnapshots("never-imported")
	if err != nil {
		t.Errorf("expected nil err for no-snapshots, got %v", err)
	}
	if d != nil {
		t.Errorf("expected nil diff for no-snapshots, got %+v", d)
	}
}

func TestDiffLatestSnapshots_OneSnapshotReturnsNil(t *testing.T) {
	snapshotDiffEnv(t)
	raw := deckJSON(t, "Only", "commander", nil, map[string]map[string]int{
		"commanders": {"Atraxa, Praetors' Voice": 1},
		"mainboard":  {"Sol Ring": 1},
	})
	if _, err := SaveSnapshot("only01", raw); err != nil {
		t.Fatalf("save: %v", err)
	}
	d, err := DiffLatestSnapshots("only01")
	if err != nil {
		t.Errorf("err = %v", err)
	}
	if d != nil {
		t.Errorf("one snapshot should not produce a diff; got %+v", d)
	}
}

func TestDiffLatestSnapshots_TwoSnapshotsReturnsDiff(t *testing.T) {
	snapshotDiffEnv(t)
	v1 := deckJSON(t, "Evolving", "commander", nil, map[string]map[string]int{
		"commanders": {"Atraxa, Praetors' Voice": 1},
		"mainboard":  {"Sol Ring": 1, "Mana Crypt": 1},
	})
	v2 := deckJSON(t, "Evolving", "commander", nil, map[string]map[string]int{
		"commanders": {"Atraxa, Praetors' Voice": 1},
		"mainboard":  {"Sol Ring": 1, "Mana Vault": 1}, // swap Crypt → Vault
	})
	if _, err := SaveSnapshot("evolve01", v1); err != nil {
		t.Fatalf("save v1: %v", err)
	}
	// Millisecond-precision filenames ensure distinct ordering even
	// when saves are close together; a small sleep is still useful so
	// the wall clock has clearly advanced.
	time.Sleep(50 * time.Millisecond)
	if _, err := SaveSnapshot("evolve01", v2); err != nil {
		t.Fatalf("save v2: %v", err)
	}

	d, err := DiffLatestSnapshots("evolve01")
	if err != nil {
		t.Fatalf("DiffLatestSnapshots: %v", err)
	}
	if d == nil || d.IsEmpty() {
		t.Fatalf("expected non-empty diff; got %+v", d)
	}
	mb := d.Boards["mainboard"]
	if len(mb.Added) != 1 || mb.Added[0].Name != "Mana Vault" {
		t.Errorf("Added = %v, want [Mana Vault]", mb.Added)
	}
	if len(mb.Removed) != 1 || mb.Removed[0].Name != "Mana Crypt" {
		t.Errorf("Removed = %v, want [Mana Crypt]", mb.Removed)
	}
	if d.DeckID != "evolve01" {
		t.Errorf("DeckID = %q, want evolve01", d.DeckID)
	}
}

// End-to-end: fetchDeckRaw → snapshot saved; change upstream; refetch;
// DiffLatestSnapshots surfaces the change.
func TestDiffLatestSnapshots_EndToEndViaFetchDeckRaw(t *testing.T) {
	v1 := deckJSON(t, "E2E", "commander", nil, map[string]map[string]int{
		"commanders": {"Yuriko, the Tiger's Shadow": 1},
		"mainboard":  {"Sol Ring": 1, "Brainstorm": 1},
	})
	v2 := deckJSON(t, "E2E", "commander", nil, map[string]map[string]int{
		"commanders": {"Yuriko, the Tiger's Shadow": 1},
		"mainboard":  {"Sol Ring": 1, "Counterspell": 1}, // swap Brainstorm → Counterspell
	})

	var body atomic.Value
	body.Store(string(v1))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body.Load().(string)))
	}))
	defer srv.Close()

	origBase := apiBaseVar
	apiBaseVar = srv.URL
	defer func() { apiBaseVar = origBase }()
	snapshotDiffEnv(t)

	if _, err := fetchDeckRaw("e2e01"); err != nil {
		t.Fatalf("v1 fetch: %v", err)
	}
	body.Store(string(v2))
	Refresh("e2e01")
	time.Sleep(50 * time.Millisecond)
	if _, err := fetchDeckRaw("e2e01"); err != nil {
		t.Fatalf("v2 fetch: %v", err)
	}

	d, err := DiffLatestSnapshots("e2e01")
	if err != nil {
		t.Fatalf("DiffLatestSnapshots: %v", err)
	}
	if d == nil || d.IsEmpty() {
		t.Fatalf("expected diff after upstream change; got %+v", d)
	}
	mb := d.Boards["mainboard"]
	if len(mb.Added) != 1 || mb.Added[0].Name != "Counterspell" {
		t.Errorf("Added = %+v, want [Counterspell]", mb.Added)
	}
	if len(mb.Removed) != 1 || mb.Removed[0].Name != "Brainstorm" {
		t.Errorf("Removed = %+v, want [Brainstorm]", mb.Removed)
	}
}

// Defensive: a board's flattenBoardToNameQty sums duplicate entries
// (Moxfield typically dedupes but custom card edits can not).
func TestDiff_DuplicateEntriesSummed(t *testing.T) {
	// Two entries for "Forest" in the same board's cards map — Moxfield
	// shouldn't do this but if it does, our diff must add the
	// quantities rather than dropping one.
	raw := map[string]any{
		"name":   "DupTest",
		"format": "commander",
		"boards": map[string]any{
			"mainboard": map[string]any{
				"count": 2,
				"cards": map[string]any{
					"forest_a": map[string]any{
						"quantity": 5,
						"card":     map[string]any{"name": "Forest"},
					},
					"forest_b": map[string]any{
						"quantity": 3,
						"card":     map[string]any{"name": "Forest"},
					},
				},
			},
		},
	}
	bytes, _ := json.Marshal(raw)
	var ar apiResponse
	_ = json.Unmarshal(bytes, &ar)
	flat := flattenBoardToNameQty(ar.mainboard())
	if flat["Forest"] != 8 {
		t.Errorf("duplicate entries should sum; Forest qty = %d, want 8", flat["Forest"])
	}
}

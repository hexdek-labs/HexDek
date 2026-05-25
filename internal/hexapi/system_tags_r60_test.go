package hexapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// normalizeSystemTagValue pure-helper tests — no DB / FS needed.

func TestNormalizeSystemTagValue_LowercasesAndHyphenates(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Combo", "combo"},
		{"Voltron", "voltron"},
		{"Lands Matter", "lands-matter"},
		{"Group Hug", "group-hug"},
		{"  Stax  ", "stax"},
		{"Reanimator/Combo", "reanimator-combo"},
		{"Tokens  Go-Wide", "tokens-go-wide"},
		{"", ""},
		{"   ", ""},
		{"\t\n", ""},
		{"Aggro\x07", "aggro"}, // strips control chars
	}
	for _, c := range cases {
		if got := normalizeSystemTagValue(c.in); got != c.want {
			t.Errorf("normalize(%q): got %q, want %q", c.in, got, c.want)
		}
	}
}

// Cap at MaxTagLen - len(SystemTagPrefix) so the eventual prefixed
// tag fits inside the user-tag length limit (32 chars total).
func TestNormalizeSystemTagValue_LengthCapped(t *testing.T) {
	longArchetype := "verylongarchetypenamethatexceedsthecap"
	got := normalizeSystemTagValue(longArchetype)
	if len(SystemTagPrefix+got) > MaxTagLen {
		t.Errorf("prefixed length %d exceeds MaxTagLen=%d (raw=%q)",
			len(SystemTagPrefix+got), MaxTagLen, got)
	}
}

// loadFreyaSystemTags reads strategy.json and returns the prefixed
// archetype tag. Setup: write a fake strategy.json under
// decks/owner/freya/id.strategy.json.
func writeStrategyFixture(t *testing.T, root, owner, id string, payload map[string]any) {
	t.Helper()
	dir := filepath.Join(root, owner, "freya")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	b, _ := json.Marshal(payload)
	if err := os.WriteFile(filepath.Join(dir, id+".strategy.json"), b, 0o644); err != nil {
		t.Fatalf("write strategy: %v", err)
	}
}

func TestLoadFreyaSystemTags_ReadsPrimaryArchetype(t *testing.T) {
	tmp := t.TempDir()
	writeStrategyFixture(t, tmp, "alice", "krenko", map[string]any{
		"primary_archetype": "Combo",
		"bracket":           4,
	})

	got := loadFreyaSystemTags(tmp, "alice", "krenko")
	want := []string{"archetype:combo"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestLoadFreyaSystemTags_MultiWordArchetype(t *testing.T) {
	tmp := t.TempDir()
	writeStrategyFixture(t, tmp, "alice", "lumra", map[string]any{
		"primary_archetype": "Lands Matter",
	})

	got := loadFreyaSystemTags(tmp, "alice", "lumra")
	if len(got) != 1 || got[0] != "archetype:lands-matter" {
		t.Errorf("multi-word archetype: got %v, want [archetype:lands-matter]", got)
	}
}

func TestLoadFreyaSystemTags_MissingStrategyReturnsNil(t *testing.T) {
	tmp := t.TempDir()
	// No strategy.json written.
	if got := loadFreyaSystemTags(tmp, "alice", "krenko"); got != nil {
		t.Errorf("missing strategy should return nil; got %v", got)
	}
}

func TestLoadFreyaSystemTags_EmptyArchetypeReturnsNil(t *testing.T) {
	tmp := t.TempDir()
	writeStrategyFixture(t, tmp, "alice", "krenko", map[string]any{
		"primary_archetype": "",
		"bracket":           3,
	})

	if got := loadFreyaSystemTags(tmp, "alice", "krenko"); got != nil {
		t.Errorf("empty archetype should return nil; got %v", got)
	}
}

func TestLoadFreyaSystemTags_WhitespaceArchetypeReturnsNil(t *testing.T) {
	tmp := t.TempDir()
	writeStrategyFixture(t, tmp, "alice", "krenko", map[string]any{
		"primary_archetype": "   \t",
	})

	if got := loadFreyaSystemTags(tmp, "alice", "krenko"); got != nil {
		t.Errorf("whitespace-only archetype should return nil; got %v", got)
	}
}

func TestLoadFreyaSystemTags_MalformedJSONReturnsNil(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "alice", "freya")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "krenko.strategy.json"), []byte("{{ broken"), 0o644)

	if got := loadFreyaSystemTags(tmp, "alice", "krenko"); got != nil {
		t.Errorf("malformed strategy should return nil; got %v", got)
	}
}

func TestLoadFreyaSystemTags_EmptyArgsReturnsNil(t *testing.T) {
	if got := loadFreyaSystemTags("", "alice", "krenko"); got != nil {
		t.Errorf("empty decksDir should return nil")
	}
	if got := loadFreyaSystemTags("/tmp", "", "krenko"); got != nil {
		t.Errorf("empty owner should return nil")
	}
	if got := loadFreyaSystemTags("/tmp", "alice", ""); got != nil {
		t.Errorf("empty id should return nil")
	}
}

// End-to-end: GET /api/decks/{owner}/{id} surfaces system_tags
// alongside the existing tags field.
func setupSystemTagsHandler(t *testing.T) (*Handler, http.Handler, string) {
	t.Helper()
	tmp := t.TempDir()
	decksDir := filepath.Join(tmp, "decks")
	if err := os.MkdirAll(filepath.Join(decksDir, "alice"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Minimal deck file so handleGetDeck finds it.
	if err := os.WriteFile(
		filepath.Join(decksDir, "alice", "krenko.txt"),
		[]byte("COMMANDER: Krenko, Mob Boss\n1 Sol Ring\n20 Mountain\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	d, err := sql.Open("sqlite", filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	if err := EnsureDeckMetaSchema(context.Background(), d); err != nil {
		t.Fatalf("schema: %v", err)
	}

	h := &Handler{DecksDir: decksDir}
	h.SetDB(d)
	mux := http.NewServeMux()
	h.Register(mux)
	return h, mux, decksDir
}

func TestHandleGetDeck_SurfacesSystemTagsWhenStrategyExists(t *testing.T) {
	_, mux, decksDir := setupSystemTagsHandler(t)
	writeStrategyFixture(t, decksDir, "alice", "krenko", map[string]any{
		"primary_archetype": "Aggro",
	})

	req := httptest.NewRequest("GET", "/api/decks/alice/krenko", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	sysTags, _ := resp["system_tags"].([]any)
	if len(sysTags) != 1 || sysTags[0] != "archetype:aggro" {
		t.Errorf("system_tags: want [archetype:aggro], got %v", resp["system_tags"])
	}
}

func TestHandleGetDeck_OmitsSystemTagsWhenNoStrategy(t *testing.T) {
	_, mux, _ := setupSystemTagsHandler(t)
	// No strategy.json written — Freya hasn't run.

	req := httptest.NewRequest("GET", "/api/decks/alice/krenko", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	// JSON unmarshal of `null` lands as nil — either "missing key"
	// or "nil value" is acceptable. The wrong outcome is a non-nil
	// non-empty array.
	if v, ok := resp["system_tags"]; ok && v != nil {
		if arr, isArr := v.([]any); !isArr || len(arr) != 0 {
			t.Errorf("system_tags should be missing or empty when no strategy; got %v", v)
		}
	}
}

// System tags and user tags coexist independently. User can add
// any tag (including the bare archetype name) without colliding
// with the system tag.
func TestHandleGetDeck_SystemTagsAndUserTagsCoexist(t *testing.T) {
	_, mux, decksDir := setupSystemTagsHandler(t)
	writeStrategyFixture(t, decksDir, "alice", "krenko", map[string]any{
		"primary_archetype": "Combo",
	})

	// User adds a custom "combo" tag (same word as system archetype).
	patchReq := httptest.NewRequest("PATCH", "/api/decks/alice/krenko",
		strings.NewReader(`{"tags": ["combo", "budget"]}`))
	patchReq.Header.Set("X-HexDek-Owner", "alice")
	patchReq.Header.Set("Content-Type", "application/json")
	patchRec := httptest.NewRecorder()
	mux.ServeHTTP(patchRec, patchReq)
	if patchRec.Code != 200 {
		t.Fatalf("patch: %d body=%s", patchRec.Code, patchRec.Body.String())
	}

	req := httptest.NewRequest("GET", "/api/decks/alice/krenko", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)

	userTags, _ := resp["tags"].([]any)
	sysTags, _ := resp["system_tags"].([]any)
	if len(userTags) != 2 {
		t.Errorf("user tags: want 2, got %v", userTags)
	}
	if len(sysTags) != 1 || sysTags[0] != "archetype:combo" {
		t.Errorf("system tags: want [archetype:combo], got %v", sysTags)
	}
	// Crucial: user "combo" tag and system "archetype:combo" tag
	// are distinct strings — they don't collide in either array.
	for _, ut := range userTags {
		if s, _ := ut.(string); s == "archetype:combo" {
			t.Errorf("user-tag array should not contain the prefixed system tag; got %v", userTags)
		}
	}
}

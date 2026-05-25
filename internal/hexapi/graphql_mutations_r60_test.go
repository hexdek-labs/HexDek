package hexapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// graphql_mutations_r60_test.go — exercises the new GraphQL mutations
// (importDeck + deleteDeck) end-to-end through the HTTP transport so
// the caller-owner plumbing is part of the picture. Coverage:
//
//   - Schema compiles with Mutation root
//   - importDeck happy path: writes file, returns Deck, defaults
//     applied for blank owner/name
//   - importDeck collision: second import with same name lands at
//     {name}_2 (mirrors handleImportDeck's behavior)
//   - importDeck empty deckList → GraphQL error
//   - importDeck sanitizes owner + name (uppercase / spaces → safe
//     slug per sanitizeFilename)
//   - deleteDeck happy path: caller matches owner → file removed,
//     returns true
//   - deleteDeck missing caller header → GraphQL error (NOT silent
//     false — the client can't confuse it with "deck didn't exist")
//   - deleteDeck wrong caller → GraphQL error
//   - deleteDeck missing file with valid caller → returns false
//     (no-op, not an error)
//   - deleteDeck path-traversal rejected
//   - Cross-mutation: import then delete round-trip works end-to-end
//   - Helper unit tests for writeImportedDeckFile

// doGraphQLAuthed runs one POST /graphql request with an
// X-HexDek-Owner header so mutation resolvers see the caller.
func doGraphQLAuthed(t *testing.T, h *GraphQLHandler, owner, query string, vars map[string]any) map[string]any {
	t.Helper()
	body, err := json.Marshal(graphqlRequest{Query: query, Variables: vars})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if owner != "" {
		req.Header.Set("X-HexDek-Owner", owner)
	}
	rr := httptest.NewRecorder()
	h.serve(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (body=%q)", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return resp
}

// mutationsFixture builds a temp decksDir with one existing deck so
// delete/collision tests have something to work with. Returns the dir.
func mutationsFixture(t *testing.T) string {
	t.Helper()
	tmp, err := os.MkdirTemp("", "hexapi-graphql-mut-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmp) })
	if err := os.MkdirAll(filepath.Join(tmp, "alice"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Pre-plant alice's existing deck so delete + collision tests
	// have a target.
	_ = os.WriteFile(filepath.Join(tmp, "alice", "korvold.txt"),
		[]byte("COMMANDER: Korvold\n1 Sol Ring\n"), 0o644)
	return tmp
}

// --- Schema construction --------------------------------------------------

func TestGraphQL_SchemaCompilesWithMutationRoot(t *testing.T) {
	h, err := NewGraphQLHandler(seedShowmatch(t), mutationsFixture(t))
	if err != nil {
		t.Fatalf("NewGraphQLHandler: %v", err)
	}
	// __schema query exposes the Mutation root if it was wired.
	resp := doGraphQL(t, h, `{ __schema { mutationType { name } } }`, nil)
	if errs, ok := resp["errors"]; ok {
		t.Fatalf("introspection errors: %v", errs)
	}
	mt, ok := resp["data"].(map[string]any)["__schema"].(map[string]any)["mutationType"].(map[string]any)
	if !ok {
		t.Fatalf("Mutation root not registered on the schema (data=%v)", resp["data"])
	}
	if mt["name"] != "Mutation" {
		t.Errorf("Mutation type name = %v, want Mutation", mt["name"])
	}
}

// --- importDeck mutation --------------------------------------------------

func TestGraphQL_ImportDeck_HappyPath(t *testing.T) {
	dir := mutationsFixture(t)
	h, _ := NewGraphQLHandler(seedShowmatch(t), dir)
	q := `mutation { importDeck(input: {deckList: "COMMANDER: Atraxa\n1 Sol Ring\n", owner: "alice", name: "atraxa_b4"}) { id owner commander } }`
	resp := doGraphQL(t, h, q, nil)
	if errs, ok := resp["errors"]; ok {
		t.Fatalf("unexpected errors: %v", errs)
	}
	d := resp["data"].(map[string]any)["importDeck"].(map[string]any)
	if d["owner"] != "alice" {
		t.Errorf("owner = %v, want alice", d["owner"])
	}
	if d["id"] != "atraxa_b4" {
		t.Errorf("id = %v, want atraxa_b4", d["id"])
	}
	// Filesystem should now contain the file.
	if _, err := os.Stat(filepath.Join(dir, "alice", "atraxa_b4.txt")); err != nil {
		t.Errorf("file not written: %v", err)
	}
}

func TestGraphQL_ImportDeck_DefaultsAppliedWhenBlank(t *testing.T) {
	// Mirrors handleImportDeck: blank owner → "imported", blank name
	// → "imported_deck" → fileID "imported_deck".
	dir := mutationsFixture(t)
	h, _ := NewGraphQLHandler(seedShowmatch(t), dir)
	resp := doGraphQL(t, h,
		`mutation { importDeck(input: {deckList: "COMMANDER: x\n"}) { owner id } }`, nil)
	if errs, ok := resp["errors"]; ok {
		t.Fatalf("unexpected errors: %v", errs)
	}
	d := resp["data"].(map[string]any)["importDeck"].(map[string]any)
	if d["owner"] != "imported" {
		t.Errorf("default owner = %v, want \"imported\"", d["owner"])
	}
	if d["id"] != "imported_deck" {
		t.Errorf("default id = %v, want \"imported_deck\"", d["id"])
	}
}

func TestGraphQL_ImportDeck_CollisionGetsNumericSuffix(t *testing.T) {
	// Mirrors handleImportDeck's "_2.._100" collision suffix loop.
	dir := mutationsFixture(t)
	h, _ := NewGraphQLHandler(seedShowmatch(t), dir)
	q := `mutation Imp($n: String!) { importDeck(input: {deckList: "COMMANDER: dup\n", owner: "bob", name: $n}) { id } }`

	resp := doGraphQLAuthed(t, h, "", q, map[string]any{"n": "twin"})
	if errs, ok := resp["errors"]; ok {
		t.Fatalf("first import: %v", errs)
	}
	first := resp["data"].(map[string]any)["importDeck"].(map[string]any)["id"]
	if first != "twin" {
		t.Errorf("first import id = %v, want twin", first)
	}

	resp = doGraphQLAuthed(t, h, "", q, map[string]any{"n": "twin"})
	if errs, ok := resp["errors"]; ok {
		t.Fatalf("second import: %v", errs)
	}
	second := resp["data"].(map[string]any)["importDeck"].(map[string]any)["id"]
	if second != "twin_2" {
		t.Errorf("second (collision) import id = %v, want twin_2", second)
	}
}

func TestGraphQL_ImportDeck_EmptyDeckListErrors(t *testing.T) {
	h, _ := NewGraphQLHandler(seedShowmatch(t), mutationsFixture(t))
	resp := doGraphQL(t, h,
		`mutation { importDeck(input: {deckList: "   ", owner: "alice"}) { id } }`, nil)
	if _, ok := resp["errors"]; !ok {
		t.Errorf("blank deckList must error, got %v", resp)
	}
	// data.importDeck should be null when the mutation errored.
	if d := resp["data"].(map[string]any)["importDeck"]; d != nil {
		t.Errorf("data.importDeck should be null on error, got %v", d)
	}
}

func TestGraphQL_ImportDeck_OwnerAndNameSanitized(t *testing.T) {
	// sanitizeFilename lowercases + strips non-[a-z0-9_-] (spaces +
	// commas become underscores). Verify the mutation reaches the
	// same shape as handleImportDeck would.
	dir := mutationsFixture(t)
	h, _ := NewGraphQLHandler(seedShowmatch(t), dir)
	resp := doGraphQL(t, h,
		`mutation { importDeck(input: {deckList: "COMMANDER: x\n", owner: "Alice McCool!", name: "Big Bad Deck"}) { id owner } }`,
		nil)
	if errs, ok := resp["errors"]; ok {
		t.Fatalf("unexpected errors: %v", errs)
	}
	d := resp["data"].(map[string]any)["importDeck"].(map[string]any)
	// Alice McCool! → "alice_mccool" (spaces→underscore, lowercased, ! stripped)
	if d["owner"] != "alice_mccool" {
		t.Errorf("sanitized owner = %v, want alice_mccool", d["owner"])
	}
	// "Big Bad Deck" → "big_bad_deck"
	if d["id"] != "big_bad_deck" {
		t.Errorf("sanitized id = %v, want big_bad_deck", d["id"])
	}
}

// --- deleteDeck mutation --------------------------------------------------

func TestGraphQL_DeleteDeck_HappyPath(t *testing.T) {
	dir := mutationsFixture(t)
	h, _ := NewGraphQLHandler(seedShowmatch(t), dir)
	resp := doGraphQLAuthed(t, h, "alice",
		`mutation { deleteDeck(owner: "alice", id: "korvold") }`, nil)
	if errs, ok := resp["errors"]; ok {
		t.Fatalf("unexpected errors: %v", errs)
	}
	ok := resp["data"].(map[string]any)["deleteDeck"].(bool)
	if !ok {
		t.Error("deleteDeck returned false on successful delete; want true")
	}
	if _, err := os.Stat(filepath.Join(dir, "alice", "korvold.txt")); !os.IsNotExist(err) {
		t.Errorf("file should be deleted, stat err = %v", err)
	}
}

func TestGraphQL_DeleteDeck_MissingCallerHeaderErrors(t *testing.T) {
	// Critical security pin: the absence of X-HexDek-Owner MUST
	// surface as an error, not a silent "false" that the client could
	// misread as "the deck didn't exist." Both shapes are distinct on
	// the wire so the SPA can prompt re-login vs surface "no such deck".
	dir := mutationsFixture(t)
	h, _ := NewGraphQLHandler(seedShowmatch(t), dir)
	resp := doGraphQLAuthed(t, h, "", // no caller header
		`mutation { deleteDeck(owner: "alice", id: "korvold") }`, nil)
	if _, ok := resp["errors"]; !ok {
		t.Errorf("missing caller header must error, got %v", resp)
	}
	if _, err := os.Stat(filepath.Join(dir, "alice", "korvold.txt")); err != nil {
		t.Errorf("file must still exist after rejected delete: %v", err)
	}
}

func TestGraphQL_DeleteDeck_WrongCallerErrors(t *testing.T) {
	dir := mutationsFixture(t)
	h, _ := NewGraphQLHandler(seedShowmatch(t), dir)
	resp := doGraphQLAuthed(t, h, "mallory", // not alice
		`mutation { deleteDeck(owner: "alice", id: "korvold") }`, nil)
	if errs, ok := resp["errors"]; !ok {
		t.Errorf("wrong caller must error, got %v", resp)
	} else {
		// Defensive: surface the error message in case the resolver
		// returns the wrong shape (e.g. confused with "not found").
		t.Logf("expected error message: %v", errs)
	}
	if _, err := os.Stat(filepath.Join(dir, "alice", "korvold.txt")); err != nil {
		t.Errorf("file must still exist after rejected delete: %v", err)
	}
}

func TestGraphQL_DeleteDeck_MissingFileReturnsFalseNotError(t *testing.T) {
	// "Deck didn't exist" is a no-op outcome, NOT an error — same
	// shape as a REST DELETE returning 404 vs 200.
	dir := mutationsFixture(t)
	h, _ := NewGraphQLHandler(seedShowmatch(t), dir)
	resp := doGraphQLAuthed(t, h, "alice",
		`mutation { deleteDeck(owner: "alice", id: "no_such_deck") }`, nil)
	if errs, ok := resp["errors"]; ok {
		t.Fatalf("missing file must NOT error (use false), got %v", errs)
	}
	got := resp["data"].(map[string]any)["deleteDeck"].(bool)
	if got {
		t.Errorf("delete of missing file = true, want false")
	}
}

func TestGraphQL_DeleteDeck_PathTraversalErrors(t *testing.T) {
	dir := mutationsFixture(t)
	h, _ := NewGraphQLHandler(seedShowmatch(t), dir)
	for _, bad := range []struct{ owner, id string }{
		{"..", "korvold"},
		{"alice", "../escape"},
		{"alice\x00", "korvold"},
		{"alice", "korvold\x00"},
	} {
		q := `mutation D($o: String!, $i: String!) { deleteDeck(owner: $o, id: $i) }`
		resp := doGraphQLAuthed(t, h, "alice", q,
			map[string]any{"o": bad.owner, "i": bad.id})
		if _, ok := resp["errors"]; !ok {
			t.Errorf("owner=%q id=%q: path traversal MUST error, got %v",
				bad.owner, bad.id, resp)
		}
	}
}

// --- Cross-mutation roundtrip --------------------------------------------

func TestGraphQL_ImportThenDelete_Roundtrip(t *testing.T) {
	dir := mutationsFixture(t)
	h, _ := NewGraphQLHandler(seedShowmatch(t), dir)
	// Import a fresh deck as alice.
	resp := doGraphQLAuthed(t, h, "alice",
		`mutation { importDeck(input: {deckList: "COMMANDER: roundtrip\n", owner: "alice", name: "roundtrip"}) { id owner } }`,
		nil)
	if errs, ok := resp["errors"]; ok {
		t.Fatalf("import: %v", errs)
	}
	imported := resp["data"].(map[string]any)["importDeck"].(map[string]any)
	id, _ := imported["id"].(string)
	if id == "" {
		t.Fatal("import returned empty id")
	}
	// Then delete it via the same call shape.
	resp = doGraphQLAuthed(t, h, "alice",
		`mutation D($i: String!) { deleteDeck(owner: "alice", id: $i) }`,
		map[string]any{"i": id})
	if errs, ok := resp["errors"]; ok {
		t.Fatalf("delete: %v", errs)
	}
	if !resp["data"].(map[string]any)["deleteDeck"].(bool) {
		t.Errorf("delete returned false on freshly-imported deck")
	}
	if _, err := os.Stat(filepath.Join(dir, "alice", id+".txt")); !os.IsNotExist(err) {
		t.Errorf("file should be gone after delete roundtrip, stat err=%v", err)
	}
}

// --- writeImportedDeckFile helper unit tests ------------------------------

func TestWriteImportedDeckFile_HappyPath(t *testing.T) {
	dir := mutationsFixture(t)
	owner, id, err := writeImportedDeckFile(dir, "carol", "first_deck", "COMMANDER: x\n")
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if owner != "carol" || id != "first_deck" {
		t.Errorf("owner/id = %q/%q, want carol/first_deck", owner, id)
	}
	if _, err := os.Stat(filepath.Join(dir, "carol", "first_deck.txt")); err != nil {
		t.Errorf("file missing: %v", err)
	}
}

func TestWriteImportedDeckFile_BlankInputsApplyDefaults(t *testing.T) {
	dir := mutationsFixture(t)
	owner, id, err := writeImportedDeckFile(dir, "", "", "COMMANDER: x\n")
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if owner != "imported" || id != "imported_deck" {
		t.Errorf("defaults: got %q/%q, want imported/imported_deck", owner, id)
	}
}

func TestWriteImportedDeckFile_EmptyDecksDirErrors(t *testing.T) {
	if _, _, err := writeImportedDeckFile("", "alice", "korvold", "x"); err == nil {
		t.Errorf("empty decksDir must error")
	}
}

func TestWriteImportedDeckFile_OwnerSanitizedToEmptyErrors(t *testing.T) {
	dir := mutationsFixture(t)
	// "!!!" sanitizes to "" → must error rather than fall back to "imported"
	// (the caller passed a non-blank owner; surface the corruption).
	if _, _, err := writeImportedDeckFile(dir, "!!!", "x", "x"); err == nil {
		t.Errorf("owner that sanitizes to empty must error")
	}
}

func TestWriteImportedDeckFile_CollisionSuffix(t *testing.T) {
	dir := mutationsFixture(t)
	_, id1, err := writeImportedDeckFile(dir, "dave", "dup", "x")
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	_, id2, err := writeImportedDeckFile(dir, "dave", "dup", "x")
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if id1 != "dup" || id2 != "dup_2" {
		t.Errorf("collision ids = %q,%q, want dup,dup_2", id1, id2)
	}
}

// --- callerOwner context plumbing unit test ------------------------------

func TestCallerOwner_ContextRoundtrip(t *testing.T) {
	ctx := context.Background()
	if got := callerOwner(ctx); got != "" {
		t.Errorf("empty context: want \"\", got %q", got)
	}
	ctx = withCallerOwner(ctx, "alice")
	if got := callerOwner(ctx); got != "alice" {
		t.Errorf("after with: want alice, got %q", got)
	}
}

func TestServe_StashesCallerOwnerLowercased(t *testing.T) {
	// Integration: serve() must lowercase + trim the header so
	// resolvers see a stable form regardless of how the client sent it.
	h, _ := NewGraphQLHandler(seedShowmatch(t), mutationsFixture(t))

	// We can't directly observe what the resolver saw, but we CAN
	// exercise deleteDeck's ownership check: caller "ALICE" with
	// uppercase header should match owner "alice".
	body, _ := json.Marshal(graphqlRequest{
		Query: `mutation { deleteDeck(owner: "alice", id: "korvold") }`,
	})
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-HexDek-Owner", "  ALICE  ") // uppercase + padded
	rr := httptest.NewRecorder()
	h.serve(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	var resp map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if errs, ok := resp["errors"]; ok {
		t.Fatalf("uppercase+padded ALICE header should match lowercase owner; got errors %v",
			errs)
	}
	if !resp["data"].(map[string]any)["deleteDeck"].(bool) {
		t.Error("delete failed despite proper caller normalization")
	}
}

// Ensure we still import strings somewhere — keep the unused-import
// guard quiet if a future refactor drops the only usage.
var _ = strings.ToLower

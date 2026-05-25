package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hexdek/hexdek/internal/moxfield"
)

// import_moxfield_test.go — end-to-end exercise of runImportMoxfield
// against realistic Moxfield v3 API responses. Stubs the Moxfield API
// with httptest and pins:
//   - DFCs preserve `//` through to disk
//   - Partners emit two COMMANDER: lines deterministically
//   - Companion goes through the `// Companion:` channel
//   - Foil/price/etched/etc. noise fields don't leak into the .txt
//   - Output file has the standard `# <name>` + `# Source: <url>` header
//   - Filename derives from sanitized deck name
//   - HTTP error from Moxfield surfaces as an error (no log.Fatal)

const ctmoxDFC = `{
	"name": "Valki DFC Pile",
	"format": "commander",
	"boards": {
		"commanders": {
			"count": 1,
			"cards": {
				"c": {
					"quantity": 1,
					"isFoil": false,
					"card": {
						"name": "Valki, God of Lies // Tibalt, Cosmic Impostor",
						"prices": {"usd": "8.99", "usd_foil": "21.00"},
						"scryfall_id": "59cf0fc7-ca5b-4189-b074-77a2d3e5c2b0"
					}
				}
			}
		},
		"mainboard": {
			"count": 2,
			"cards": {
				"m1": {"quantity": 1, "card": {"name": "Malakir Rebirth // Malakir Mire"}},
				"m2": {"quantity": 1, "card": {"name": "Sol Ring", "isFoil": true, "prices": {"usd": "3.00"}}}
			}
		}
	}
}`

const ctmoxPartner = `{
	"name": "Akiri+Silas Hatebears",
	"format": "commander",
	"boards": {
		"commanders": {
			"count": 2,
			"cards": {
				"a": {"quantity": 1, "card": {"name": "Akiri, Line-Slinger", "prices": {"usd": "3.10"}}},
				"b": {"quantity": 1, "card": {"name": "Silas Renn, Seeker Adept", "prices": {"usd": "2.40"}}}
			}
		},
		"mainboard": {
			"count": 1,
			"cards": {"m1": {"quantity": 1, "card": {"name": "Stoneforge Mystic"}}}
		}
	}
}`

const ctmoxCompanion = `{
	"name": "Edgar with Lurrus",
	"format": "commander",
	"boards": {
		"commanders": {"count": 1, "cards": {"c1": {"quantity": 1, "card": {"name": "Edgar Markov"}}}},
		"mainboard":  {"count": 1, "cards": {"m1": {"quantity": 1, "card": {"name": "Sol Ring"}}}},
		"companions": {"count": 1, "cards": {"comp1": {"quantity": 1, "card": {"name": "Lurrus of the Dream-Den"}}}}
	}
}`

// newCLITestEnv sets up an httptest server that returns the given JSON
// for any v3 deck fetch, isolates the Moxfield disk cache to t.TempDir,
// skips the Freya shell-out, and resets in-process caches. Returns the
// hit counter + the cleanup that restores all globals.
func newCLITestEnv(t *testing.T, body string) (counter *int32, cleanup func()) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	restore := moxfield.SetAPIBaseForTest(srv.URL)

	// Isolate the Moxfield disk cache + snapshot dir + import registry
	// so this test doesn't reuse the user's real ~/.cache /
	// ~/.local/share entries or step on parallel tests.
	tmp := t.TempDir()
	t.Setenv("HEXDEK_MOXFIELD_CACHE_DIR", tmp)
	t.Setenv("HEXDEK_MOXFIELD_CACHE", "")
	t.Setenv("HEXDEK_MOXFIELD_CACHE_TTL", "")
	t.Setenv("HEXDEK_MOXFIELD_SNAPSHOT_DIR", tmp+"/snapshots")
	t.Setenv("HEXDEK_MOXFIELD_IMPORTS_PATH", tmp+"/imports.json")
	t.Setenv("HEXDEK_IMPORT_SKIP_FREYA", "1")
	// runImportMoxfield uses a fresh deck ID, so in-process dedupe is
	// irrelevant; the Refresh call here is defensive against tests that
	// reuse the same deck ID.
	moxfield.Refresh("dfc01")
	moxfield.Refresh("part01")
	moxfield.Refresh("comp01")

	return &hits, func() {
		srv.Close()
		restore()
	}
}

func runOne(t *testing.T, fixture, deckID string) (outPath, contents string) {
	t.Helper()
	_, cleanup := newCLITestEnv(t, fixture)
	defer cleanup()

	outDir := t.TempDir()
	url := fmt.Sprintf("https://moxfield.com/decks/%s", deckID)
	moxfield.Refresh(deckID) // ensure no carry-over from prior subtests

	out, err := runImportMoxfield(url, outDir)
	if err != nil {
		t.Fatalf("runImportMoxfield(%s): %v", deckID, err)
	}
	bytes, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read written deck file: %v", err)
	}
	return out, string(bytes)
}

func TestCLI_DFCImport_PreservesSlashAndWritesHeader(t *testing.T) {
	outPath, body := runOne(t, ctmoxDFC, "dfc01")

	// Header lines from runImportMoxfield.
	if !strings.Contains(body, "# Valki DFC Pile") {
		t.Errorf("expected `# <deck name>` header, got:\n%s", body)
	}
	if !strings.Contains(body, "# Source: https://moxfield.com/decks/dfc01") {
		t.Errorf("expected `# Source:` header, got:\n%s", body)
	}

	// DFC name must round-trip with "//".
	if !strings.Contains(body, "COMMANDER: Valki, God of Lies // Tibalt, Cosmic Impostor\n") {
		t.Errorf("DFC commander missing `//` separator in:\n%s", body)
	}
	if !strings.Contains(body, "1 Malakir Rebirth // Malakir Mire\n") {
		t.Errorf("DFC mainboard line missing `//` separator in:\n%s", body)
	}
	// Filename derived from sanitized deck name.
	if base := filepath.Base(outPath); base != "valki_dfc_pile.txt" {
		t.Errorf("filename = %q, want %q", base, "valki_dfc_pile.txt")
	}
	// None of the API-noise fields should leak into the written file.
	for _, leaked := range []string{"prices", "usd", "8.99", "scryfall_id", "isFoil", "21.00"} {
		if strings.Contains(body, leaked) {
			t.Errorf("Moxfield noise field %q leaked into deck file:\n%s", leaked, body)
		}
	}
}

func TestCLI_PartnerImport_TwoCommandersDeterministicOrder(t *testing.T) {
	_, body := runOne(t, ctmoxPartner, "part01")
	if got := strings.Count(body, "COMMANDER: "); got != 2 {
		t.Fatalf("partner deck must write exactly 2 COMMANDER lines, got %d:\n%s", got, body)
	}
	// Akiri's entry key sorts before Silas's; deterministic order is
	// load-bearing because the downstream deckparser treats the first
	// commander as primary.
	idxA := strings.Index(body, "COMMANDER: Akiri, Line-Slinger")
	idxS := strings.Index(body, "COMMANDER: Silas Renn, Seeker Adept")
	if idxA < 0 || idxS < 0 || idxA >= idxS {
		t.Errorf("partner ordering broken; Akiri should precede Silas: Akiri@%d Silas@%d\n%s",
			idxA, idxS, body)
	}
}

func TestCLI_CompanionImport_RoutesToCompanionLine(t *testing.T) {
	_, body := runOne(t, ctmoxCompanion, "comp01")
	if !strings.Contains(body, "// Companion: 1 Lurrus of the Dream-Den\n") {
		t.Errorf("companion must appear under `// Companion:` channel; got:\n%s", body)
	}
	// And NOT promoted.
	if strings.Contains(body, "COMMANDER: Lurrus") {
		t.Errorf("Lurrus must not be promoted to COMMANDER:\n%s", body)
	}
	if strings.Contains(body, "\n1 Lurrus of the Dream-Den\n") {
		t.Errorf("Lurrus must not appear as a bare mainboard entry:\n%s", body)
	}
	// Edgar still primary.
	if !strings.Contains(body, "COMMANDER: Edgar Markov\n") {
		t.Errorf("primary commander Edgar Markov missing:\n%s", body)
	}
}

// HTTP failure surfaces as an error rather than terminating the process.
func TestCLI_APIError_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
		fmt.Fprintln(w, "service unavailable")
	}))
	defer srv.Close()
	restore := moxfield.SetAPIBaseForTest(srv.URL)
	defer restore()

	tmp := t.TempDir()
	t.Setenv("HEXDEK_MOXFIELD_CACHE_DIR", tmp)
	t.Setenv("HEXDEK_MOXFIELD_CACHE", "")
	t.Setenv("HEXDEK_MOXFIELD_SNAPSHOT_DIR", tmp+"/snapshots")
	t.Setenv("HEXDEK_MOXFIELD_IMPORTS_PATH", tmp+"/imports.json")
	t.Setenv("HEXDEK_IMPORT_SKIP_FREYA", "1")
	moxfield.Refresh("err01")

	_, err := runImportMoxfield("https://moxfield.com/decks/err01", t.TempDir())
	if err == nil {
		t.Fatal("expected error on 503 response")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("error should mention 503; got: %v", err)
	}
}

// Bad URL (no deck ID extractable) errors out without hitting the API.
func TestCLI_BadURL_ErrorsWithoutAPICall(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	restore := moxfield.SetAPIBaseForTest(srv.URL)
	defer restore()
	t.Setenv("HEXDEK_IMPORT_SKIP_FREYA", "1")

	_, err := runImportMoxfield("https://example.com/not-a-moxfield-deck", t.TempDir())
	if err == nil {
		t.Fatal("expected error for non-Moxfield URL")
	}
	if got := atomic.LoadInt32(&hits); got != 0 {
		t.Errorf("bad URL should not hit API, got %d hits", got)
	}
}

// Verify the FetchDeckName + FetchDeck pair in the CLI flow actually
// shares a single HTTP call (this is the perf fix from the previous
// branch; pin it from the CLI side so a regression at the CLI layer
// — e.g., constructing a fresh moxfield.Client — is caught).
func TestCLI_DeckNameAndDeckShareSingleHTTPCall(t *testing.T) {
	hits, cleanup := newCLITestEnv(t, ctmoxDFC)
	defer cleanup()
	moxfield.Refresh("dfc01")

	if _, err := runImportMoxfield("https://moxfield.com/decks/dfc01", t.TempDir()); err != nil {
		t.Fatalf("runImportMoxfield: %v", err)
	}
	if got := atomic.LoadInt32(hits); got != 1 {
		t.Fatalf("CLI flow expected 1 HTTP hit (cache should dedupe FetchDeckName+FetchDeck), got %d", got)
	}
}

// Foil-and-price-laden card entries don't break the import — the
// importer must ignore Moxfield's per-printing decoration without
// erroring or polluting the .txt.
func TestCLI_FoilAndPriceFieldsNeitherErrorNorLeak(t *testing.T) {
	body := `{
		"name": "Foil Soup",
		"format": "commander",
		"boards": {
			"commanders": {"count": 1, "cards": {"c1": {
				"quantity": 1,
				"isFoil": true,
				"isEtched": false,
				"isAlter": true,
				"useCmcOverride": true,
				"useManaCostOverride": true,
				"useColorIdentityOverride": true,
				"finish": "etched",
				"condition": "NM",
				"language": "en",
				"prices": {"usd": "120.00", "usd_foil": "240.00", "usd_etched": "300.00", "tix": "5.00", "eur": "110.00"},
				"card": {
					"name": "Liliana of the Veil",
					"scryfall_id": "00000000-0000-0000-0000-000000000000",
					"set": "isd",
					"collector_number": "105*",
					"image_uris": {"normal": "https://example.com/art.jpg"},
					"card_faces": []
				}
			}}},
			"mainboard": {"count": 1, "cards": {"m1": {"quantity": 1, "card": {"name": "Swamp"}}}}
		}
	}`
	hits, cleanup := newCLITestEnv(t, body)
	defer cleanup()
	moxfield.Refresh("foil01")

	out, err := runImportMoxfield("https://moxfield.com/decks/foil01", t.TempDir())
	if err != nil {
		t.Fatalf("runImportMoxfield: %v", err)
	}
	contents, _ := os.ReadFile(out)
	text := string(contents)
	for _, leaked := range []string{
		"isFoil", "isEtched", "isAlter", "useCmcOverride",
		"prices", "120.00", "240.00", "300.00", "5.00",
		"scryfall_id", "00000000", "isd", "image_uris",
		"collector_number", "card_faces", "condition", "language",
	} {
		if strings.Contains(text, leaked) {
			t.Errorf("printing-decoration field %q leaked into .txt:\n%s", leaked, text)
		}
	}
	if !strings.Contains(text, "COMMANDER: Liliana of the Veil\n") {
		t.Errorf("commander name missing from decorated import:\n%s", text)
	}
	if got := atomic.LoadInt32(hits); got != 1 {
		t.Errorf("expected 1 HTTP hit, got %d", got)
	}
}

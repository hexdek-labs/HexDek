package hexapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/deckparser"
)

// health_dependencies_r60_test.go — pins the r60 k8s-style
// dependencies sub-object on GET /api/health. Each dependency
// surfaces "ok" or "fail" so an orchestrator / dashboard can render
// a per-component status grid without parsing Go-specific JSON.
//
// Covered dependencies:
//   - scylla: aliases the SQLite-DB liveness probe (HexDek doesn't use
//     ScyllaDB; "scylla" is the orchestrator-side name for the
//     persistent store). Mirrors the top-level db_reachable boolean.
//   - freya: hexdek-freya analysis binary discoverable on PATH or
//     at ./hexdek-freya (matches handler.go runFreya's discovery path)
//   - hat: in-process YggdrasilHat AI player — requires the
//     Showmatch's AST corpus + MetaDB to be loaded.

// healthGet fires GET /api/health and parses the response. Centralized
// so per-test bodies stay focused.
func healthGet(t *testing.T, h *Handler) healthResponse {
	t.Helper()
	mux := http.NewServeMux()
	h.Register(mux)
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (body=%q)", w.Code, w.Body.String())
	}
	var resp healthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v (body=%q)", err, w.Body.String())
	}
	return resp
}

// --- Shape ---------------------------------------------------------------

func TestHealthDependencies_AllThreeKeysPresent(t *testing.T) {
	resp := healthGet(t, &Handler{})
	for _, k := range []string{"scylla", "freya", "hat"} {
		if _, ok := resp.Dependencies[k]; !ok {
			t.Errorf("dependencies missing key %q (have %v)", k, resp.Dependencies)
		}
	}
}

// Every dependency value MUST be either "ok" or "fail" — defends a
// future refactor that introduces a third state (degraded, warming)
// without explicit operator coordination.
func TestHealthDependencies_ValuesAreOkOrFail(t *testing.T) {
	resp := healthGet(t, &Handler{})
	for k, v := range resp.Dependencies {
		if v != HealthOK && v != HealthFail {
			t.Errorf("dependencies[%q]=%q, want %q or %q", k, v, HealthOK, HealthFail)
		}
	}
}

// HealthOK and HealthFail are the canonical wire strings. Pin so
// they don't drift from "ok"/"fail" (orchestrators string-match).
func TestHealthDependencies_ConstantValues(t *testing.T) {
	if HealthOK != "ok" {
		t.Errorf("HealthOK=%q, want %q", HealthOK, "ok")
	}
	if HealthFail != "fail" {
		t.Errorf("HealthFail=%q, want %q", HealthFail, "fail")
	}
}

// --- scylla --------------------------------------------------------------

// scylla aliases the existing db_reachable probe — when the DB is
// nil (test/dev binary), both report fail.
func TestHealthDependencies_ScyllaMatchesDBReachable_NoDB(t *testing.T) {
	resp := healthGet(t, &Handler{})
	if resp.DBReachable {
		t.Fatal("setup: nil DB should report unreachable")
	}
	if resp.Dependencies["scylla"] != HealthFail {
		t.Errorf("scylla=%q, want %q when db_reachable=false", resp.Dependencies["scylla"], HealthFail)
	}
}

// --- freya ---------------------------------------------------------------

// freya status reflects binary discoverability. Default test runs
// have no hexdek-freya on PATH, so the dependency reports fail.
// Defends against a future health-handler that always reports ok
// without actually probing.
func TestHealthDependencies_FreyaFailWhenBinaryMissing(t *testing.T) {
	// Save + clear PATH so exec.LookPath misses, and switch to a
	// temp working directory so ./hexdek-freya doesn't accidentally
	// resolve to a sibling file.
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", "")
	defer t.Setenv("PATH", origPath)

	origWD, _ := os.Getwd()
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	resp := healthGet(t, &Handler{})
	if resp.Dependencies["freya"] != HealthFail {
		t.Errorf("freya=%q, want %q (no binary on PATH or in cwd)",
			resp.Dependencies["freya"], HealthFail)
	}
}

// freya status reports ok when a hexdek-freya binary exists at
// ./hexdek-freya relative to the working directory. Defends the
// `./hexdek-freya` fallback branch in freyaBinaryAvailable.
func TestHealthDependencies_FreyaOKWhenLocalBinaryPresent(t *testing.T) {
	tmp := t.TempDir()
	// Plant a fake (empty) executable so os.Stat hits.
	freyaPath := filepath.Join(tmp, "hexdek-freya")
	if err := os.WriteFile(freyaPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", "")
	defer t.Setenv("PATH", origPath)

	origWD, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	resp := healthGet(t, &Handler{})
	if resp.Dependencies["freya"] != HealthOK {
		t.Errorf("freya=%q, want %q (./hexdek-freya present)",
			resp.Dependencies["freya"], HealthOK)
	}
}

// --- hat -----------------------------------------------------------------

// hat needs the Showmatch's MetaDB loaded. Without a Showmatch, fail.
func TestHealthDependencies_HatFailWhenShowmatchMissing(t *testing.T) {
	resp := healthGet(t, &Handler{}) // no Showmatch
	if resp.Dependencies["hat"] != HealthFail {
		t.Errorf("hat=%q, want %q (no Showmatch)", resp.Dependencies["hat"], HealthFail)
	}
}

// hat also fails when Showmatch is constructed but its meta hasn't
// loaded yet (background load in flight or load failed).
func TestHealthDependencies_HatFailWhenMetaNotLoaded(t *testing.T) {
	h := &Handler{Showmatch: &Showmatch{}} // meta nil
	resp := healthGet(t, h)
	if resp.Dependencies["hat"] != HealthFail {
		t.Errorf("hat=%q, want %q (meta not loaded)", resp.Dependencies["hat"], HealthFail)
	}
}

// hat reports ok once meta is loaded. Build a minimal MetaDB via the
// public JSONL loader so tests don't need access to the unexported
// byName field.
func TestHealthDependencies_HatOKWhenMetaLoaded(t *testing.T) {
	meta, err := deckparser.LoadMetaReader(strings.NewReader(`{"name":"Sol Ring","type_line":"Artifact","mana_cost":"{1}","cmc":1,"colors":[]}`))
	if err != nil {
		t.Fatalf("load meta: %v", err)
	}
	h := &Handler{Showmatch: &Showmatch{meta: meta}}
	resp := healthGet(t, h)
	if resp.Dependencies["hat"] != HealthOK {
		t.Errorf("hat=%q, want %q (meta loaded)", resp.Dependencies["hat"], HealthOK)
	}
}

// --- Backwards-compat ---------------------------------------------------

// db_reachable stays a top-level boolean alongside the new
// dependencies map — existing monitors that read the boolean directly
// MUST keep working.
func TestHealthDependencies_DBReachableStillTopLevel(t *testing.T) {
	mux := http.NewServeMux()
	h := &Handler{}
	h.Register(mux)
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var raw map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := raw["db_reachable"]; !ok {
		t.Error("db_reachable missing — backwards-compat broken")
	}
	// scylla under dependencies must agree with top-level db_reachable.
	deps, ok := raw["dependencies"].(map[string]any)
	if !ok {
		t.Fatal("dependencies missing or wrong type")
	}
	dbBool := raw["db_reachable"].(bool)
	wantScylla := HealthFail
	if dbBool {
		wantScylla = HealthOK
	}
	if deps["scylla"] != wantScylla {
		t.Errorf("scylla=%v, want %q to mirror db_reachable=%v", deps["scylla"], wantScylla, dbBool)
	}
}


package moxfield

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// ----- imported-deck registry -----
//
// A persistent JSON file that tracks every Moxfield deck the user has
// successfully imported through hexdek-import. Distinct from both the
// fetch CACHE (TTL'd, ephemeral) and SNAPSHOTS (per-fetch raw-JSON
// archive): the import registry is the "you have imported this deck"
// truth set, sized by the user's actual import history rather than by
// every API touch.
//
// Use cases:
//   - Delta imports: feed a folder of URLs, skip ones already imported
//     unless their upstream hash has changed.
//   - "List my decks" dashboards (CLI `--list-imports`).
//   - Stale detection: compare each registered deck's current upstream
//     hash to LastSnapshotHash and surface those that have changed
//     since import (CLI `--check-stale`).
//
// Format (default $XDG_DATA_HOME/hexdek/moxfield-imports.json):
//
//   {
//     "version": 1,
//     "records": [
//       {"deck_id": "...", "url": "...", "deck_name": "...",
//        "output_path": "...", "first_imported_at": "...",
//        "last_imported_at": "...", "import_count": 3,
//        "last_snapshot_hash": "a1b2c3d4e5f6", "last_snapshot_path": "..."}
//     ]
//   }
//
// Concurrency: read-modify-write within a process is mutex-guarded
// (importsMu). Cross-process is racy — last writer wins. The
// downstream tooling treats the registry as a discovery hint, not
// authoritative state, so the rare cross-process race that drops one
// record's update is acceptable. The actual deck data (snapshots,
// .txt outputs) is preserved separately.
//
// Disable entirely with HEXDEK_MOXFIELD_IMPORTS=0; override path with
// HEXDEK_MOXFIELD_IMPORTS_PATH=/some/file.json.

const (
	importsEnvPath    = "HEXDEK_MOXFIELD_IMPORTS_PATH"
	importsEnvDisable = "HEXDEK_MOXFIELD_IMPORTS"
	importsSchemaVer  = 1
)

// ImportRecord is a single deck's registry entry.
type ImportRecord struct {
	DeckID           string    `json:"deck_id"`
	URL              string    `json:"url"`
	DeckName         string    `json:"deck_name"`
	OutputPath       string    `json:"output_path"`
	FirstImportedAt  time.Time `json:"first_imported_at"`
	LastImportedAt   time.Time `json:"last_imported_at"`
	ImportCount      int       `json:"import_count"`
	LastSnapshotHash string    `json:"last_snapshot_hash"`
	LastSnapshotPath string    `json:"last_snapshot_path"`
}

// importsFile is the on-disk wire format. Wrapped in a version-tagged
// envelope so future schema changes can be migrated cleanly.
type importsFile struct {
	Version int            `json:"version"`
	Records []ImportRecord `json:"records"`
}

var importsMu sync.Mutex

func importsEnabled() bool { return os.Getenv(importsEnvDisable) != "0" }

func importsPath() string {
	if p := os.Getenv(importsEnvPath); p != "" {
		return p
	}
	base := snapshotBaseDir()
	if base == "" {
		return ""
	}
	// snapshotBaseDir() returns .../moxfield-snapshots/; sibling
	// file moxfield-imports.json lives one level up.
	return filepath.Join(filepath.Dir(base), "moxfield-imports.json")
}

// loadImports reads + parses the registry file. Returns an empty
// envelope (no error) when the file doesn't exist yet. Caller must
// hold importsMu.
func loadImports() (*importsFile, error) {
	path := importsPath()
	if path == "" {
		return &importsFile{Version: importsSchemaVer}, nil
	}
	bytes, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &importsFile{Version: importsSchemaVer}, nil
		}
		return nil, fmt.Errorf("imports: read %s: %w", path, err)
	}
	if len(bytes) == 0 {
		return &importsFile{Version: importsSchemaVer}, nil
	}
	var f importsFile
	if err := json.Unmarshal(bytes, &f); err != nil {
		return nil, fmt.Errorf("imports: parse %s: %w", path, err)
	}
	return &f, nil
}

// saveImports writes the envelope atomically via temp-file + rename
// so a crash mid-write leaves the prior file intact rather than
// truncating to empty. Caller must hold importsMu.
func saveImports(f *importsFile) error {
	path := importsPath()
	if path == "" {
		return fmt.Errorf("imports: no writable path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("imports: mkdir: %w", err)
	}
	f.Version = importsSchemaVer
	bytes, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("imports: marshal: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, bytes, 0644); err != nil {
		return fmt.Errorf("imports: write tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("imports: rename: %w", err)
	}
	return nil
}

// RecordImport upserts a registry entry for a successfully-imported
// deck. New entries start with ImportCount=1 and equal First/Last
// timestamps. Re-importing an existing deck bumps ImportCount, updates
// LastImportedAt + LastSnapshotHash/Path, preserves FirstImportedAt,
// and refreshes URL/DeckName/OutputPath in case the user re-ran the
// import with different flags.
//
// Returns nil (no-op) if HEXDEK_MOXFIELD_IMPORTS=0 disables the
// registry. Returns an error on validation failure (empty DeckID) or
// IO failure (load/save of the imports registry) — production callers
// can log and continue without blocking the import.
func RecordImport(rec ImportRecord) error {
	if !importsEnabled() {
		return nil
	}
	if rec.DeckID == "" {
		return fmt.Errorf("imports: RecordImport with empty DeckID")
	}
	importsMu.Lock()
	defer importsMu.Unlock()

	f, err := loadImports()
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	if rec.LastImportedAt.IsZero() {
		rec.LastImportedAt = now
	}
	found := false
	for i := range f.Records {
		if f.Records[i].DeckID == rec.DeckID {
			existing := f.Records[i]
			merged := existing
			merged.URL = rec.URL
			merged.DeckName = rec.DeckName
			merged.OutputPath = rec.OutputPath
			merged.LastImportedAt = rec.LastImportedAt
			merged.ImportCount = existing.ImportCount + 1
			if rec.LastSnapshotHash != "" {
				merged.LastSnapshotHash = rec.LastSnapshotHash
				merged.LastSnapshotPath = rec.LastSnapshotPath
			}
			f.Records[i] = merged
			found = true
			break
		}
	}
	if !found {
		if rec.FirstImportedAt.IsZero() {
			rec.FirstImportedAt = rec.LastImportedAt
		}
		if rec.ImportCount <= 0 {
			rec.ImportCount = 1
		}
		f.Records = append(f.Records, rec)
	}
	return saveImports(f)
}

// GetImport returns the registry entry for a deck ID, or nil if not
// recorded. Returns an error only on registry IO failure (not on
// missing record).
func GetImport(deckID string) (*ImportRecord, error) {
	importsMu.Lock()
	defer importsMu.Unlock()
	f, err := loadImports()
	if err != nil {
		return nil, err
	}
	for i := range f.Records {
		if f.Records[i].DeckID == deckID {
			cp := f.Records[i]
			return &cp, nil
		}
	}
	return nil, nil
}

// HasImported is a thin convenience.
func HasImported(deckID string) bool {
	rec, _ := GetImport(deckID)
	return rec != nil
}

// ListImports returns all registry entries, sorted by LastImportedAt
// descending (most-recently-imported first).
func ListImports() ([]ImportRecord, error) {
	importsMu.Lock()
	defer importsMu.Unlock()
	f, err := loadImports()
	if err != nil {
		return nil, err
	}
	out := make([]ImportRecord, len(f.Records))
	copy(out, f.Records)
	sort.Slice(out, func(i, j int) bool {
		return out[i].LastImportedAt.After(out[j].LastImportedAt)
	})
	return out, nil
}

// FindStale walks each registered deck, force-refreshes the moxfield
// fetch cache, re-fetches the upstream JSON, and returns the records
// whose newest snapshot hash now differs from the registry's
// LastSnapshotHash — i.e., the Moxfield owner has edited the deck
// since this user last imported it.
//
// Stale candidates:
//   - registry.LastSnapshotHash != newest_snapshot.Hash (real edit)
// Skipped:
//   - records with empty LastSnapshotHash (no baseline to compare)
//   - decks whose fetch errors (upstream 404, network down) — kept
//     in registry, not flagged stale; the .txt + prior snapshot
//     remain authoritative
//
// `limit > 0` caps how many records are checked (0 = all). Each
// checked deck hits the Moxfield API; expect ~150ms per deck against
// the live API.
func FindStale(ctx context.Context, limit int) ([]ImportRecord, error) {
	records, err := ListImports()
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(records) > limit {
		records = records[:limit]
	}
	var stale []ImportRecord
	for _, rec := range records {
		if rec.LastSnapshotHash == "" {
			continue
		}
		// Bust the cache so we genuinely re-check upstream rather than
		// confirming our own stale data.
		Refresh(rec.DeckID)
		if _, err := fetchDeckRaw(rec.DeckID); err != nil {
			// Upstream gone / network down — keep the registry entry,
			// don't claim it's stale.
			continue
		}
		snaps, _ := ListSnapshots(rec.DeckID)
		if len(snaps) == 0 {
			continue
		}
		if snaps[0].Hash != rec.LastSnapshotHash {
			stale = append(stale, rec)
		}
		// Honour ctx cancellation between fetches.
		select {
		case <-ctx.Done():
			return stale, ctx.Err()
		default:
		}
	}
	return stale, nil
}

// resetImportsForTest wipes both the in-memory state (none — we use
// pure-IO functions) and the on-disk file. Test-only.
func resetImportsForTest() {
	importsMu.Lock()
	defer importsMu.Unlock()
	if p := importsPath(); p != "" {
		_ = os.Remove(p)
	}
}

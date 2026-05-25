package moxfield

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ----- deck snapshots -----
//
// Distinct from the deck CACHE (which is TTL'd ephemeral storage for
// "give me the deck fast on re-import"), snapshots are an append-only
// history of every Moxfield API response we received for a given
// deck ID. They survive cache invalidation and survive Moxfield-side
// edits / deletions so a deck's exact import-time state can always
// be replayed.
//
// On-disk layout:
//   $HEXDEK_MOXFIELD_SNAPSHOT_DIR/{deck_id}/{timestamp}_{hash}.json
//   default base: $XDG_DATA_HOME/hexdek/moxfield-snapshots/
//                 or ~/.local/share/hexdek/moxfield-snapshots/
//
// The filename encodes both ordering (UTC timestamp, ISO8601) and
// content identity (sha256 prefix). ListSnapshots just lists the
// directory and parses filenames — no separate manifest file to keep
// in sync.
//
// Dedup: SaveSnapshot scans existing snapshots and reuses the file
// when the hash matches an existing entry. The Deduped flag lets the
// caller distinguish "wrote a new file" from "matched existing."
// Concurrent SaveSnapshot calls for the same deck can race past the
// dedup scan — accepted as a rare cost; worst case is two files with
// identical content, no correctness impact.
//
// Disable entirely with HEXDEK_MOXFIELD_SNAPSHOTS=0.

const (
	snapshotEnvDir     = "HEXDEK_MOXFIELD_SNAPSHOT_DIR"
	snapshotEnvDisable = "HEXDEK_MOXFIELD_SNAPSHOTS"
	snapshotHashLen    = 12 // 6 bytes hex; collision risk ~1 in 2^48 for one deck
	snapshotTimeLayout = "20060102T150405Z"
)

// Snapshot describes a single point-in-time capture of a deck's raw
// Moxfield API JSON.
type Snapshot struct {
	DeckID    string
	Timestamp time.Time
	Hash      string // sha256 hex prefix (snapshotHashLen chars)
	Path      string // absolute path to the JSON file on disk
	Size      int64
	// Deduped is true on SaveSnapshot when the supplied rawJSON's
	// hash matched an existing snapshot for this deck — i.e. no new
	// file was written and Path/Timestamp refer to the prior entry.
	Deduped bool
}

func snapshotsEnabled() bool {
	return os.Getenv(snapshotEnvDisable) != "0"
}

func snapshotBaseDir() string {
	if d := os.Getenv(snapshotEnvDir); d != "" {
		return d
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "hexdek", "moxfield-snapshots")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share", "hexdek", "moxfield-snapshots")
}

func snapshotDeckDir(deckID string) string {
	base := snapshotBaseDir()
	if base == "" {
		return ""
	}
	// Sanitize deck ID — same allowlist used by the moxfield cache,
	// preventing a Moxfield-side ID format change from letting weird
	// chars escape the deck dir.
	safe := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, deckID)
	return filepath.Join(base, safe)
}

// SaveSnapshot persists rawJSON to a new file under the deck's
// snapshot directory. If a snapshot with the same content hash
// already exists, no file is written and the existing Snapshot is
// returned with Deduped=true.
//
// Best-effort: returns a non-nil error only when something is
// genuinely broken (mkdir failure, write failure). Snapshots being
// disabled via env returns a zero Snapshot with nil error so callers
// can ignore the result safely.
func SaveSnapshot(deckID string, rawJSON []byte) (Snapshot, error) {
	if !snapshotsEnabled() {
		return Snapshot{}, nil
	}
	if deckID == "" {
		return Snapshot{}, fmt.Errorf("moxfield snapshot: empty deck ID")
	}
	if len(rawJSON) == 0 {
		return Snapshot{}, fmt.Errorf("moxfield snapshot: empty rawJSON")
	}
	dir := snapshotDeckDir(deckID)
	if dir == "" {
		return Snapshot{}, fmt.Errorf("moxfield snapshot: cannot resolve snapshot dir")
	}

	sum := sha256.Sum256(rawJSON)
	hash := hex.EncodeToString(sum[:])[:snapshotHashLen]

	// Look for an existing snapshot with the same hash. We must scan
	// before mkdir/write to avoid creating an empty dir for a
	// deduped-no-op save.
	existing, _ := ListSnapshots(deckID)
	for _, s := range existing {
		if s.Hash == hash {
			s.Deduped = true
			return s, nil
		}
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return Snapshot{}, fmt.Errorf("moxfield snapshot: mkdir %s: %w", dir, err)
	}

	now := time.Now().UTC()
	name := fmt.Sprintf("%s_%s.json", now.Format(snapshotTimeLayout), hash)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, rawJSON, 0644); err != nil {
		return Snapshot{}, fmt.Errorf("moxfield snapshot: write %s: %w", path, err)
	}
	return Snapshot{
		DeckID:    deckID,
		Timestamp: now,
		Hash:      hash,
		Path:      path,
		Size:      int64(len(rawJSON)),
	}, nil
}

// ListSnapshots returns all snapshots for a deck, newest first.
// Returns a nil slice when the deck has no snapshot dir yet (not an
// error — fresh decks naturally have no history).
func ListSnapshots(deckID string) ([]Snapshot, error) {
	dir := snapshotDeckDir(deckID)
	if dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var snaps []Snapshot
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		// {timestamp}_{hash}.json
		base := strings.TrimSuffix(e.Name(), ".json")
		parts := strings.SplitN(base, "_", 2)
		if len(parts) != 2 {
			continue
		}
		ts, err := time.Parse(snapshotTimeLayout, parts[0])
		if err != nil {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		snaps = append(snaps, Snapshot{
			DeckID:    deckID,
			Timestamp: ts,
			Hash:      parts[1],
			Path:      filepath.Join(dir, e.Name()),
			Size:      info.Size(),
		})
	}
	sort.Slice(snaps, func(i, j int) bool {
		return snaps[i].Timestamp.After(snaps[j].Timestamp)
	})
	return snaps, nil
}

// LoadSnapshot reads a snapshot file's raw JSON bytes from disk. The
// caller can json.Unmarshal into whatever shape they want — typically
// the unexported apiResponse via a re-fetch through fetchDeckRaw with
// the cache primed, or directly via the public API if more callers
// emerge.
func LoadSnapshot(path string) ([]byte, error) {
	return os.ReadFile(path)
}

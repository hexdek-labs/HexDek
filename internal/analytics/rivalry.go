package analytics

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Rivalry tracks the head-to-head record between two decks across
// tournament runs. Keyed by a canonical pair (alphabetically sorted
// commander names) so A-vs-B and B-vs-A map to the same record.
type Rivalry struct {
	CommanderA string `json:"commander_a"`
	CommanderB string `json:"commander_b"`
	AWins      int    `json:"a_wins"`
	BWins      int    `json:"b_wins"`
	TotalGames int    `json:"total_games"`
	LastPlayed string `json:"last_played"`
}

// RivalrySummary is a user-facing view of a single rival for one deck.
type RivalrySummary struct {
	Opponent   string  `json:"opponent"`
	Wins       int     `json:"wins"`
	Losses     int     `json:"losses"`
	TotalGames int     `json:"total_games"`
	WinRate    float64 `json:"win_rate"`
	LastPlayed string  `json:"last_played"`
}

// canonicalKey returns (a, b) with a <= b lexicographically, plus a
// bool indicating whether the inputs were swapped.
func canonicalKey(a, b string) (string, string, bool) {
	if a <= b {
		return a, b, false
	}
	return b, a, true
}

func rivalryMapKey(a, b string) string { return a + " ⚔ " + b }

// PersistRivalries reads the existing matchups file, merges new matchup
// data from a tournament result, and writes back atomically. Follows
// the same append-only read-merge-write pattern as Muninn/Huginn.
//
// matchupWins[A][B] = games A won when both A and B were present.
// matchupGames[A][B] = total games where both A and B were present.
func PersistRivalries(dir string, matchupWins, matchupGames map[string]map[string]int) error {
	if len(matchupGames) == 0 {
		return nil
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path := filepath.Join(dir, "matchups.json")
	existing, _ := loadRivalries(path)
	index := make(map[string]*Rivalry, len(existing))
	for i := range existing {
		r := &existing[i]
		index[rivalryMapKey(r.CommanderA, r.CommanderB)] = r
	}

	now := time.Now().UTC().Format(time.RFC3339)

	for nameA, opponents := range matchupGames {
		for nameB, games := range opponents {
			if games == 0 || nameA >= nameB {
				continue
			}
			ca, cb, _ := canonicalKey(nameA, nameB)
			key := rivalryMapKey(ca, cb)
			r, ok := index[key]
			if !ok {
				r = &Rivalry{CommanderA: ca, CommanderB: cb}
				index[key] = r
				existing = append(existing, *r)
				r = &existing[len(existing)-1]
				index[key] = r
			}

			aWins := 0
			if w, ok2 := matchupWins[nameA]; ok2 {
				aWins = w[nameB]
			}
			bWins := 0
			if w, ok2 := matchupWins[nameB]; ok2 {
				bWins = w[nameA]
			}

			if ca == nameA {
				r.AWins += aWins
				r.BWins += bWins
			} else {
				r.AWins += bWins
				r.BWins += aWins
			}
			r.TotalGames += games
			r.LastPlayed = now
		}
	}

	// Rebuild from index.
	out := make([]Rivalry, 0, len(index))
	for _, r := range index {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TotalGames > out[j].TotalGames })

	return atomicWriteJSON(path, out)
}

// LoadRivalries reads all rivalries from disk. Missing file is not an
// error; a corrupt file surfaces the JSON parse error so callers can
// distinguish "no history yet" from "stored history is unreadable".
func LoadRivalries(dir string) ([]Rivalry, error) {
	path := filepath.Join(dir, "matchups.json")
	return loadRivalries(path)
}

// TopRivals returns the top N rivals for a given commander, sorted by
// total games played (most played first).
func TopRivals(rivalries []Rivalry, commander string, n int) []RivalrySummary {
	var matches []RivalrySummary
	for _, r := range rivalries {
		var s RivalrySummary
		switch {
		case r.CommanderA == commander:
			s = RivalrySummary{
				Opponent:   r.CommanderB,
				Wins:       r.AWins,
				Losses:     r.BWins,
				TotalGames: r.TotalGames,
				LastPlayed: r.LastPlayed,
			}
		case r.CommanderB == commander:
			s = RivalrySummary{
				Opponent:   r.CommanderA,
				Wins:       r.BWins,
				Losses:     r.AWins,
				TotalGames: r.TotalGames,
				LastPlayed: r.LastPlayed,
			}
		default:
			continue
		}
		if s.TotalGames > 0 {
			s.WinRate = float64(s.Wins) / float64(s.TotalGames)
		}
		matches = append(matches, s)
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].TotalGames > matches[j].TotalGames
	})

	if n > 0 && len(matches) > n {
		matches = matches[:n]
	}
	return matches
}

func loadRivalries(path string) ([]Rivalry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Rivalry
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func atomicWriteJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

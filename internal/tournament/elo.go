package tournament

import "math"

const (
	eloDefault = 1500.0
	eloK       = 32.0
)

// ELORatings tracks per-commander ELO ratings across a tournament.
// Updated after each game using multiplayer ELO (each pair of
// participants plays a virtual match; winner gets K-adjusted gain,
// losers get K-adjusted loss).
type ELORatings struct {
	Rating map[string]float64
	Games  map[string]int
}

// NewELORatings initializes ratings for the given commander names.
func NewELORatings(names []string) *ELORatings {
	e := &ELORatings{
		Rating: make(map[string]float64, len(names)),
		Games:  make(map[string]int, len(names)),
	}
	for _, n := range names {
		e.Rating[n] = eloDefault
	}
	return e
}

// Update processes a single game outcome. In multiplayer, we treat each
// pair of participants as a virtual head-to-head: the winner scores 1.0
// against every other participant, draw scores 0.5 for all. Ratings are
// updated pairwise with K scaled by 1/nOpponents to prevent inflation.
func (e *ELORatings) Update(winnerName string, participantNames []string) {
	n := len(participantNames)
	if n < 2 {
		return
	}
	kScaled := eloK / float64(n-1)

	for _, name := range participantNames {
		e.Games[name]++
	}

	if winnerName == "" {
		// Draw: all participants exchange 0.5-expected ratings.
		for i := 0; i < n; i++ {
			for j := i + 1; j < n; j++ {
				a, b := participantNames[i], participantNames[j]
				ea := expectedScore(e.Rating[a], e.Rating[b])
				eb := 1.0 - ea
				e.Rating[a] += kScaled * (0.5 - ea)
				e.Rating[b] += kScaled * (0.5 - eb)
			}
		}
		return
	}

	for _, loser := range participantNames {
		if loser == winnerName {
			continue
		}
		eW := expectedScore(e.Rating[winnerName], e.Rating[loser])
		eL := 1.0 - eW
		e.Rating[winnerName] += kScaled * (1.0 - eW)
		e.Rating[loser] += kScaled * (0.0 - eL)
	}
}

func expectedScore(ratingA, ratingB float64) float64 {
	return 1.0 / (1.0 + math.Pow(10, (ratingB-ratingA)/400.0))
}

// Snapshot returns a copy of current ratings sorted by rating descending.
type ELOEntry struct {
	Commander string
	Rating    float64
	Games     int
}

func (e *ELORatings) Snapshot() []ELOEntry {
	entries := make([]ELOEntry, 0, len(e.Rating))
	for name, r := range e.Rating {
		entries = append(entries, ELOEntry{
			Commander: name,
			Rating:    r,
			Games:     e.Games[name],
		})
	}
	// Sort descending by rating.
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].Rating > entries[i].Rating {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
	return entries
}

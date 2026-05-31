package analytics

import (
	"sort"
	"strings"
)

// DeckFingerprint is a per-commander multi-game profile suitable for
// opponent recognition by 3rd Eye / Yggdrasil's OpponentProfile. The
// frame is "across N analyzed games where commander X sat, what
// pattern of cards / timings / win conditions did the deck exhibit?"
//
// Use cases:
//   1. Hat opponent prior: when the hat sees an opponent commander it
//      has played against before, LookupFingerprint(name, list)
//      returns a fingerprint whose Archetype / AvgTurnToFirstCombo /
//      WinConditionMix can pre-seed OpponentProfile.Archetype before
//      the hat has observed enough plays to classify on its own.
//   2. Deckbuilder reference: the top-CardFrequency entries are a
//      cheap "what cards is this commander always playing?" list,
//      useful when stalking a meta to identify the obvious staples
//      vs. tech choices.
//   3. Meta study: WinConditionMix surfaces decks whose strategy
//      shifted (e.g., a commander that used to combo out at 65% but
//      now closes via combat damage 70% of the time signals a
//      gameplan pivot in the corpus).
type DeckFingerprint struct {
	CommanderName string
	GamesObserved int

	// CardFrequency lists every non-land, non-token, non-commander
	// card seen across the analyzed games for this commander, with
	// the fraction of games it was successfully cast (TurnCast > 0).
	// Sorted by Frequency desc, then by GamesCast desc, then by Name
	// asc for stability.
	CardFrequency []CardFrequencyEntry

	// Threat timing benchmarks. 0.0 means "never observed".
	// All averages are conditional means — only games where the
	// event fired contribute to the denominator.
	AvgTurnToFirstResolution float64 // first commander cast from command zone
	AvgTurnToFirstCombo      float64 // first per_card_win event (ComboAssembled)
	AvgFirstBlood            float64 // first damage dealt to a player
	AvgGameLength            float64 // mean TotalTurns across observed games

	// WinConditionMix is the fraction of games (when commander won)
	// closed via each WinCondition category ("combat_damage", "combo",
	// "commander_damage", "life_drain", "poison", "turn_cap", etc.).
	// Sums to 1.0 across wins; 0.0 if commander never won.
	WinConditionMix map[string]float64
	GamesWon        int

	// Archetype is the heuristic classification derived from the
	// fingerprint signature: "aggro", "combo", "control", "stax",
	// "midrange", "unknown". ClassifyArchetype reads the fingerprint
	// fields, no external state.
	Archetype string

	// ArchetypeConfidence is in [0, 1]; higher means stronger
	// signal-card hit-count and / or sharper WinConditionMix peak.
	// Reports / hat consumers should treat <0.4 as "unknown for
	// practical purposes".
	ArchetypeConfidence float64
}

// CardFrequencyEntry is one row of the CardFrequency list.
type CardFrequencyEntry struct {
	Name      string
	GamesCast int
	Frequency float64 // GamesCast / GamesObserved
}

// BuildDeckFingerprints walks the analyses and produces one
// DeckFingerprint per distinct CommanderName. Empty input returns
// nil. Output sorted by GamesObserved desc, then by CommanderName asc.
//
// Card frequency uses the same lands/tokens exclusion as the other
// metrics (KeystoneImpact / WinRateWhenCast). Per-game dedupe by
// (gameIdx, cardName) so a card cast 3 times in one game still counts
// once. Mirror games (commander in two seats) collapse to one sample
// per game per the standard per-game dedupe.
func BuildDeckFingerprints(analyses []*GameAnalysis) []DeckFingerprint {
	if len(analyses) == 0 {
		return nil
	}

	type cardAcc struct {
		gamesCast int
	}
	type fpAcc struct {
		games int
		// Card name → tally; key set is the union of all observed
		// CardsPlayed across games where this commander played.
		cards map[string]*cardAcc
		// Per-game dedupe set: which (gameIdx, cardName) we already
		// counted toward this commander's frequency.
		gameCardSeen map[gameCardKey]bool

		firstResolutionSum   int
		firstResolutionCount int
		firstComboSum        int
		firstComboCount      int
		firstBloodSum        int
		firstBloodCount      int
		gameLengthSum        int
		gamesWon             int
		winConditions        map[string]int
	}
	byCmdr := make(map[string]*fpAcc)
	getAcc := func(name string) *fpAcc {
		a, ok := byCmdr[name]
		if !ok {
			a = &fpAcc{
				cards:         make(map[string]*cardAcc),
				gameCardSeen:  make(map[gameCardKey]bool),
				winConditions: make(map[string]int),
			}
			byCmdr[name] = a
		}
		return a
	}

	for gameIdx, ga := range analyses {
		if ga == nil {
			continue
		}
		// Per-game seat tracking — a commander appearing in 2 seats
		// (mirror) is one game, not two, for GamesObserved.
		seenCmdrThisGame := make(map[string]bool)
		for seatIdx := range ga.Players {
			pa := &ga.Players[seatIdx]
			name := pa.CommanderName
			if name == "" {
				continue
			}
			a := getAcc(name)
			if !seenCmdrThisGame[name] {
				seenCmdrThisGame[name] = true
				a.games++
				if ga.TotalTurns > 0 {
					a.gameLengthSum += ga.TotalTurns
				}
				if ga.FirstBlood > 0 {
					a.firstBloodSum += ga.FirstBlood
					a.firstBloodCount++
				}
				if ga.ComboAssembled > 0 {
					a.firstComboSum += ga.ComboAssembled
					a.firstComboCount++
				}
			}

			// Commander resolution turn.
			if turn := findCardTurnCast(pa, name); turn > 0 {
				a.firstResolutionSum += turn
				a.firstResolutionCount++
			}

			// Card frequency: dedupe by (gameIdx, cardName) across
			// seats so mirrors don't double-count.
			for ci := range pa.CardsPlayed {
				cp := &pa.CardsPlayed[ci]
				if cp.TurnCast <= 0 {
					continue
				}
				if cp.IsLand || cp.IsToken || isTokenByName(cp.Name) {
					continue
				}
				// Skip the commander itself — it's a known constant,
				// the frequency line for it would always read 100%.
				if cp.Name == name {
					continue
				}
				key := gameCardKey{gameIdx: gameIdx, card: cp.Name}
				if a.gameCardSeen[key] {
					continue
				}
				a.gameCardSeen[key] = true
				ca, ok := a.cards[cp.Name]
				if !ok {
					ca = &cardAcc{}
					a.cards[cp.Name] = ca
				}
				ca.gamesCast++
			}

			// Win-condition mix: only winning seats contribute.
			if pa.Won {
				a.gamesWon++
				if ga.WinCondition != "" {
					a.winConditions[ga.WinCondition]++
				}
			}
		}
	}

	out := make([]DeckFingerprint, 0, len(byCmdr))
	for name, a := range byCmdr {
		fp := DeckFingerprint{
			CommanderName: name,
			GamesObserved: a.games,
			GamesWon:      a.gamesWon,
		}
		if a.games > 0 {
			fp.AvgGameLength = float64(a.gameLengthSum) / float64(a.games)
		}
		if a.firstResolutionCount > 0 {
			fp.AvgTurnToFirstResolution = float64(a.firstResolutionSum) / float64(a.firstResolutionCount)
		}
		if a.firstComboCount > 0 {
			fp.AvgTurnToFirstCombo = float64(a.firstComboSum) / float64(a.firstComboCount)
		}
		if a.firstBloodCount > 0 {
			fp.AvgFirstBlood = float64(a.firstBloodSum) / float64(a.firstBloodCount)
		}

		// Card frequency entries.
		entries := make([]CardFrequencyEntry, 0, len(a.cards))
		for cardName, ca := range a.cards {
			freq := 0.0
			if a.games > 0 {
				freq = float64(ca.gamesCast) / float64(a.games)
			}
			entries = append(entries, CardFrequencyEntry{
				Name:      cardName,
				GamesCast: ca.gamesCast,
				Frequency: freq,
			})
		}
		sort.SliceStable(entries, func(i, j int) bool {
			if entries[i].Frequency != entries[j].Frequency {
				return entries[i].Frequency > entries[j].Frequency
			}
			if entries[i].GamesCast != entries[j].GamesCast {
				return entries[i].GamesCast > entries[j].GamesCast
			}
			return entries[i].Name < entries[j].Name
		})
		fp.CardFrequency = entries

		// Win-condition mix.
		if a.gamesWon > 0 {
			mix := make(map[string]float64, len(a.winConditions))
			for cond, n := range a.winConditions {
				mix[cond] = float64(n) / float64(a.gamesWon)
			}
			fp.WinConditionMix = mix
		}

		// Archetype heuristic.
		fp.Archetype, fp.ArchetypeConfidence = ClassifyArchetype(&fp)

		out = append(out, fp)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].GamesObserved != out[j].GamesObserved {
			return out[i].GamesObserved > out[j].GamesObserved
		}
		return out[i].CommanderName < out[j].CommanderName
	})

	return out
}

// LookupFingerprint scans `fingerprints` for the first entry whose
// CommanderName matches `name`. Case-insensitive. Returns nil when
// not found.
//
// Pure utility — designed for hat 3rd Eye to seed OpponentProfile
// from a precomputed corpus of opponent fingerprints. The hat owns
// nothing about the analytics package itself; the caller arranges
// the fingerprint slice (typically loaded once at hat init from a
// Heimdall-emitted JSON / passed through tournament harness).
func LookupFingerprint(name string, fingerprints []DeckFingerprint) *DeckFingerprint {
	if name == "" {
		return nil
	}
	want := strings.ToLower(strings.TrimSpace(name))
	for i := range fingerprints {
		if strings.ToLower(fingerprints[i].CommanderName) == want {
			return &fingerprints[i]
		}
	}
	return nil
}

// gameCardKey is a per-game dedupe key for (gameIdx, cardName). The
// fingerprint aggregator uses it to ensure mirror seats / multi-cast
// don't double-count toward CardFrequency. Same semantics as the
// function-scoped gameCastKey in card_rankings.go::RankCards, kept
// at package scope here so other aggregators can adopt the same
// pattern in future batches.
type gameCardKey struct {
	gameIdx int
	card    string
}

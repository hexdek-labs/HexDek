package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/hexdek/hexdek/internal/analytics"
	"github.com/hexdek/hexdek/internal/tournament"
)

// ---------------------------------------------------------------------------
// Freya tournament summary report — distills a TournamentResult JSON
// (produced by cmd/hexdek-tournament/) into a coaching-shaped narrative:
//
//   - Performance tiers — top quartile / mid / bottom by winrate
//   - Archetype meta breakdown — commander → archetype heuristic
//   - Surprise upset matchups — H2H wins by lower-ELO decks
//   - OMW% leaders (Swiss only)
//   - Win-condition analysis — kill methods + top win-contribution cards
//
// CLI surface: freya --tournament path/to/tournament.json
// ---------------------------------------------------------------------------

// TournamentSummary is the structured report extracted from a
// TournamentResult. All sections are optional — only populated when
// the source result carries the underlying data. JSON-serializable
// so downstream tools (Decks-screen tournament-replay panel,
// Heimdall) can consume the same shape.
type TournamentSummary struct {
	Games        int     `json:"games"`
	NSeats       int     `json:"n_seats"`
	Mode         string  `json:"mode"`
	TotalDecks   int     `json:"total_decks"`
	AvgTurnToEnd float64 `json:"avg_turn_to_end"`

	Tiers []PerformanceTier `json:"tiers"`

	ArchetypeBreakdown   map[string]int `json:"archetype_breakdown,omitempty"`
	ArchetypeWinrate     map[string]float64 `json:"archetype_winrate,omitempty"`
	BestArchetype        string  `json:"best_archetype,omitempty"`
	BestArchetypeWinrate float64 `json:"best_archetype_winrate,omitempty"`

	Upsets []UpsetMatch `json:"upsets,omitempty"`

	OMWLeaders []OMWLeader `json:"omw_leaders,omitempty"`

	WinConditions []CommanderWinPattern `json:"win_conditions,omitempty"`

	TopCards []TopCard `json:"top_cards,omitempty"`
}

// PerformanceTier groups commanders by winrate quartile.
type PerformanceTier struct {
	Tier       string              `json:"tier"`        // "top" / "mid" / "bottom"
	Commanders []CommanderRanking  `json:"commanders"`
}

// CommanderRanking is one commander's tournament-wide performance.
type CommanderRanking struct {
	Commander string  `json:"commander"`
	Winrate   float64 `json:"winrate"` // 0-1
	Wins      int     `json:"wins"`
	Games     int     `json:"games"`
	ELO       float64 `json:"elo,omitempty"`
}

// UpsetMatch is a head-to-head pairing where the lower-rated deck
// won the matchup outright. The ELO gap is signed (LoserELO −
// WinnerELO so positive means the loser was favored). Reported only
// when the gap is at least upsetMinELOGap.
type UpsetMatch struct {
	Winner     string  `json:"winner"`
	Loser      string  `json:"loser"`
	WinnerELO  float64 `json:"winner_elo"`
	LoserELO   float64 `json:"loser_elo"`
	ELOGap     float64 `json:"elo_gap"`
	Games      int     `json:"games"`
	WinnerWins int     `json:"winner_wins"`
}

// OMWLeader is a Swiss standing top-spot by opponent match-win %.
type OMWLeader struct {
	Commander string  `json:"commander"`
	OMW       float64 `json:"omw"`
	Points    int     `json:"points"`
	Wins      int     `json:"wins"`
}

// CommanderWinPattern names a commander's most common kill method
// + most common lethal card across their wins. Derived from
// KillRecords; only populated when audit was enabled during the
// tournament run.
type CommanderWinPattern struct {
	Commander     string `json:"commander"`
	Wins          int    `json:"wins"`
	TopMethod     string `json:"top_method,omitempty"`
	TopMethodPct  int    `json:"top_method_pct,omitempty"` // 0-100
	TopLethalCard string `json:"top_lethal_card,omitempty"`
}

// TopCard is the per-card win-contribution leaderboard entry.
type TopCard struct {
	Card            string  `json:"card"`
	WinContribution float64 `json:"win_contribution"` // 0-1
	GamesWon        int     `json:"games_won"`
	KillShotRate    float64 `json:"kill_shot_rate,omitempty"`
}

// Detection thresholds.
const (
	// upsetMinELOGap is the ELO differential needed before an H2H
	// reversal counts as an "upset" rather than noise. 100 points
	// corresponds to a ~64% expected win rate for the higher-rated
	// side — meaningfully favored but not a blowout. Below 100 the
	// "upset" is mostly statistical drift.
	upsetMinELOGap = 100.0

	// upsetMinH2HGames is the minimum head-to-head game count required
	// before an upset gets reported — a single-game flip on a 4-seat
	// pod is too noisy to call a real upset. 2 games keeps the bar
	// reasonable for small tournaments while still filtering out
	// single-game noise.
	upsetMinH2HGames = 2

	// upsetMinWinPct is the H2H winrate the lower-rated side must
	// hit to count. 0.50 = the lower-rated side at least split
	// (which is itself surprising vs a 100-point ELO gap).
	upsetMinWinPct = 0.50

	// Per-section caps so the rendered report stays scannable.
	tierTopCap        = 5
	upsetCap          = 5
	omwLeaderCap      = 5
	winConditionCap   = 8
	topCardCap        = 10
)

// LoadTournamentResult parses a TournamentResult JSON file. Returns
// the canonical type from internal/tournament so callers can
// re-share the in-memory result with other tools.
func LoadTournamentResult(path string) (*tournament.TournamentResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read tournament %s: %w", path, err)
	}
	var r tournament.TournamentResult
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parse tournament %s: %w", path, err)
	}
	return &r, nil
}

// BuildTournamentSummary computes the structured summary from a
// TournamentResult. Sections that depend on optional fields
// (SwissStandings, CardRankings, KillRecords) gracefully omit
// when the data isn't present.
func BuildTournamentSummary(r *tournament.TournamentResult) *TournamentSummary {
	if r == nil {
		return nil
	}
	summary := &TournamentSummary{
		Games:        r.Games,
		NSeats:       r.NSeats,
		Mode:         r.Mode,
		TotalDecks:   len(r.CommanderNames),
		AvgTurnToEnd: r.AvgTurns,
	}

	rankings := buildCommanderRankings(r)
	summary.Tiers = buildPerformanceTiers(rankings)
	summary.ArchetypeBreakdown, summary.ArchetypeWinrate, summary.BestArchetype, summary.BestArchetypeWinrate =
		buildArchetypeBreakdown(rankings)
	summary.Upsets = buildUpsetMatches(r)
	summary.OMWLeaders = buildOMWLeaders(r.SwissStandings)
	summary.WinConditions = buildWinConditions(r)
	summary.TopCards = buildTopCards(r.CardRankings)

	return summary
}

// buildCommanderRankings walks WinsByCommander + GamesPlayedByCommander
// + ELO into a unified per-commander ranking sorted by winrate desc
// (with games-played as the stability key — a 1/1 deck shouldn't
// outrank a 8/16 deck at 50% even though both are at 100%/50% on
// their own).
func buildCommanderRankings(r *tournament.TournamentResult) []CommanderRanking {
	eloByName := map[string]float64{}
	for _, e := range r.ELO {
		eloByName[e.Commander] = e.Rating
	}

	out := make([]CommanderRanking, 0, len(r.CommanderNames))
	for _, name := range r.CommanderNames {
		wins := r.WinsByCommander[name]
		games := totalGamesForCommander(r, name)
		if games == 0 {
			continue
		}
		out = append(out, CommanderRanking{
			Commander: name,
			Wins:      wins,
			Games:     games,
			Winrate:   float64(wins) / float64(games),
			ELO:       eloByName[name],
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Winrate != out[j].Winrate {
			return out[i].Winrate > out[j].Winrate
		}
		if out[i].Wins != out[j].Wins {
			return out[i].Wins > out[j].Wins
		}
		return out[i].Commander < out[j].Commander
	})
	return out
}

// totalGamesForCommander prefers GamesPlayedByCommander when present
// (set by round-robin / Swiss); falls back to r.Games (all decks
// played every game — true for rotate mode and pool mode).
func totalGamesForCommander(r *tournament.TournamentResult, name string) int {
	if r.GamesPlayedByCommander != nil {
		if n, ok := r.GamesPlayedByCommander[name]; ok {
			return n
		}
	}
	return r.Games
}

// buildPerformanceTiers splits commanders into top quartile / mid /
// bottom quartile by winrate. Returns 3 tiers in fixed order so
// downstream renderers don't have to sort.
func buildPerformanceTiers(rankings []CommanderRanking) []PerformanceTier {
	if len(rankings) == 0 {
		return nil
	}
	q1 := len(rankings) / 4
	if q1 < 1 {
		q1 = 1
	}
	q3 := len(rankings) - q1
	if q3 < q1 {
		q3 = q1
	}
	cap := func(s []CommanderRanking) []CommanderRanking {
		if len(s) > tierTopCap {
			return s[:tierTopCap]
		}
		return s
	}
	tiers := []PerformanceTier{
		{Tier: "top", Commanders: cap(rankings[:q1])},
	}
	if q3 > q1 {
		tiers = append(tiers, PerformanceTier{Tier: "mid", Commanders: cap(rankings[q1:q3])})
	}
	if q3 < len(rankings) {
		tiers = append(tiers, PerformanceTier{Tier: "bottom", Commanders: cap(rankings[q3:])})
	}
	return tiers
}

// commanderArchetypeHints maps known commanders to their canonical
// archetype label. Curated, not exhaustive — unknown commanders fall
// through to "Unclassified" so the rollup degrades gracefully on a
// tournament with novel commanders. Extending this map is a one-line
// change with no API impact.
var commanderArchetypeHints = map[string]string{
	"Atraxa, Praetors' Voice":         "Superfriends",
	"Atraxa, Grand Unifier":           "Reanimator",
	"Krenko, Mob Boss":                "Aggro",
	"Edgar Markov":                    "Aggro",
	"Yawgmoth, Thran Physician":       "Aristocrats",
	"Meren of Clan Nel Toth":          "Reanimator",
	"Karador, Ghost Chieftain":        "Reanimator",
	"Uril, the Miststalker":           "Voltron",
	"Voltron":                         "Voltron",
	"Thrasios, Triton Hero":           "Combo",
	"Tymna the Weaver":                "Combo",
	"Kraum, Ludevic's Opus":           "Combo",
	"Korvold, Fae-Cursed King":        "Aristocrats",
	"Talrand, Sky Summoner":           "Spellslinger",
	"Niv-Mizzet, Parun":               "Spellslinger",
	"The Locust God":                  "Spellslinger",
	"Sythis, Harvest's Hand":          "Enchantress",
	"Tuvasa the Sunlit":               "Enchantress",
	"Sigarda, Host of Herons":         "Voltron",
	"Selvala, Heart of the Wilds":     "Combo",
	"Najeela, the Blade-Blossom":      "Extra Combats",
	"Aurelia, the Warleader":          "Extra Combats",
	"Urza, Lord High Artificer":       "Artifacts",
	"Breya, Etherium Shaper":          "Artifacts",
	"Lord Windgrace":                  "Lands Matter",
	"Omnath, Locus of Creation":       "Lands Matter",
	"Azusa, Lost but Seeking":         "Lands Matter",
	"Lazav, the Multifarious":         "Theft / Clone",
	"The Mimeoplasm":                  "Theft / Clone",
	"Brago, King Eternal":             "Blink",
	"Roon of the Hidden Realm":        "Blink",
	"Phenax, God of Deception":        "Mill",
	"Bruvac the Grandiloquent":        "Mill",
}

// archetypeForCommander returns the curated archetype label for the
// given commander name (case-insensitive), or "Unclassified" when no
// hint exists. Doc-grade hint table — tests pin canonical entries so
// regressions are caught at CI.
func archetypeForCommander(name string) string {
	if name == "" {
		return "Unclassified"
	}
	if v, ok := commanderArchetypeHints[name]; ok {
		return v
	}
	// Case-insensitive fallback for tournaments that exported
	// commander names with non-canonical casing.
	lower := strings.ToLower(name)
	for k, v := range commanderArchetypeHints {
		if strings.EqualFold(k, lower) {
			return v
		}
	}
	return "Unclassified"
}

// buildArchetypeBreakdown produces the meta-distribution rollup:
// commander count per archetype + per-archetype average winrate +
// the best-performing archetype. Best-archetype detection requires
// at least 2 commanders in that archetype bucket (a single 100%
// winrate commander isn't a meta signal — it's a single deck).
func buildArchetypeBreakdown(rankings []CommanderRanking) (
	breakdown map[string]int,
	winrate map[string]float64,
	bestArchetype string,
	bestWinrate float64,
) {
	if len(rankings) == 0 {
		return nil, nil, "", 0
	}
	breakdown = map[string]int{}
	totalWinrate := map[string]float64{}
	for _, r := range rankings {
		arch := archetypeForCommander(r.Commander)
		breakdown[arch]++
		totalWinrate[arch] += r.Winrate
	}
	winrate = map[string]float64{}
	for arch, count := range breakdown {
		winrate[arch] = totalWinrate[arch] / float64(count)
	}

	bestWinrate = -1
	for arch, w := range winrate {
		// Skip Unclassified for "best archetype" — it's a fallback
		// bucket, not a real archetype.
		if arch == "Unclassified" {
			continue
		}
		// Require >=2 commanders in the bucket — single-deck buckets
		// don't reflect archetype strength, they reflect deck strength.
		if breakdown[arch] < 2 {
			continue
		}
		if w > bestWinrate {
			bestWinrate = w
			bestArchetype = arch
		}
	}
	if bestArchetype == "" {
		bestWinrate = 0
	}
	return
}

// buildUpsetMatches scans the MatchupMatrix for head-to-head pairings
// where the lower-ELO commander won the H2H series outright (>=50%
// over >=upsetMinH2HGames), with an ELO gap of >=upsetMinELOGap.
// Sorted by gap descending — biggest surprise first.
func buildUpsetMatches(r *tournament.TournamentResult) []UpsetMatch {
	if r.MatchupMatrix == nil || r.MatchupGames == nil || len(r.ELO) == 0 {
		return nil
	}
	eloByName := map[string]float64{}
	for _, e := range r.ELO {
		eloByName[e.Commander] = e.Rating
	}

	seen := map[string]bool{}
	var out []UpsetMatch
	for a, opponents := range r.MatchupMatrix {
		for b, aWins := range opponents {
			if a == b {
				continue
			}
			pairKey := canonicalPairKey(a, b)
			if seen[pairKey] {
				continue
			}
			seen[pairKey] = true

			games := r.MatchupGames[a][b]
			if games < upsetMinH2HGames {
				continue
			}
			eloA, okA := eloByName[a]
			eloB, okB := eloByName[b]
			if !okA || !okB {
				continue
			}
			bWins := r.MatchupMatrix[b][a]
			gap := math.Abs(eloA - eloB)
			if gap < upsetMinELOGap {
				continue
			}

			// Who's the favored side and who's the underdog?
			var underdog, favored string
			var underdogWins, favoredWins int
			var underdogELO, favoredELO float64
			if eloA < eloB {
				underdog, favored = a, b
				underdogWins, favoredWins = aWins, bWins
				underdogELO, favoredELO = eloA, eloB
			} else {
				underdog, favored = b, a
				underdogWins, favoredWins = bWins, aWins
				underdogELO, favoredELO = eloB, eloA
			}

			underdogPct := float64(underdogWins) / float64(underdogWins+favoredWins)
			if underdogPct < upsetMinWinPct {
				continue
			}
			_ = favoredWins // surfaced via games count
			out = append(out, UpsetMatch{
				Winner:     underdog,
				Loser:      favored,
				WinnerELO:  underdogELO,
				LoserELO:   favoredELO,
				ELOGap:     gap,
				Games:      games,
				WinnerWins: underdogWins,
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ELOGap != out[j].ELOGap {
			return out[i].ELOGap > out[j].ELOGap
		}
		return out[i].Winner < out[j].Winner
	})
	if len(out) > upsetCap {
		out = out[:upsetCap]
	}
	return out
}

func canonicalPairKey(a, b string) string {
	if a < b {
		return a + "|" + b
	}
	return b + "|" + a
}

// buildOMWLeaders pulls the top opponent-match-win-% entries from
// SwissStandings. Empty for non-Swiss tournament modes.
func buildOMWLeaders(standings []tournament.SwissStanding) []OMWLeader {
	if len(standings) == 0 {
		return nil
	}
	sorted := append([]tournament.SwissStanding(nil), standings...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].OpponentMatchWinPct != sorted[j].OpponentMatchWinPct {
			return sorted[i].OpponentMatchWinPct > sorted[j].OpponentMatchWinPct
		}
		if sorted[i].Points != sorted[j].Points {
			return sorted[i].Points > sorted[j].Points
		}
		return sorted[i].CommanderName < sorted[j].CommanderName
	})
	if len(sorted) > omwLeaderCap {
		sorted = sorted[:omwLeaderCap]
	}
	out := make([]OMWLeader, 0, len(sorted))
	for _, s := range sorted {
		out = append(out, OMWLeader{
			Commander: s.CommanderName,
			OMW:       s.OpponentMatchWinPct,
			Points:    s.Points,
			Wins:      s.Wins,
		})
	}
	return out
}

// buildWinConditions aggregates KillRecords per commander into a
// "what did each winner kill with most often?" rollup. Method and
// LethalCard tallies are independent — TopMethod is the most common
// kill mechanic; TopLethalCard is the most common named card across
// all kills that named one.
func buildWinConditions(r *tournament.TournamentResult) []CommanderWinPattern {
	if len(r.KillRecords) == 0 {
		return nil
	}
	methodCount := map[string]map[string]int{}     // commander → method → count
	lethalCardCount := map[string]map[string]int{} // commander → card → count
	winsByCommander := map[string]int{}            // commander → kill total
	for _, k := range r.KillRecords {
		if k.KillerCommander == "" {
			continue
		}
		if methodCount[k.KillerCommander] == nil {
			methodCount[k.KillerCommander] = map[string]int{}
			lethalCardCount[k.KillerCommander] = map[string]int{}
		}
		method := k.Method
		if method == "" {
			method = "unknown"
		}
		methodCount[k.KillerCommander][method]++
		winsByCommander[k.KillerCommander]++
		if k.LethalCard != "" {
			lethalCardCount[k.KillerCommander][k.LethalCard]++
		}
	}

	out := make([]CommanderWinPattern, 0, len(winsByCommander))
	for commander, wins := range winsByCommander {
		topMethod, topMethodCount := topKey(methodCount[commander])
		topCard, _ := topKey(lethalCardCount[commander])
		methodPct := 0
		if wins > 0 {
			methodPct = int(math.Round(100.0 * float64(topMethodCount) / float64(wins)))
		}
		out = append(out, CommanderWinPattern{
			Commander:     commander,
			Wins:          wins,
			TopMethod:     topMethod,
			TopMethodPct:  methodPct,
			TopLethalCard: topCard,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Wins != out[j].Wins {
			return out[i].Wins > out[j].Wins
		}
		return out[i].Commander < out[j].Commander
	})
	if len(out) > winConditionCap {
		out = out[:winConditionCap]
	}
	return out
}

// topKey returns the key with the highest count in a map[string]int,
// plus its count. Empty key + 0 when the map is empty.
func topKey(m map[string]int) (string, int) {
	bestKey := ""
	bestCount := 0
	for k, v := range m {
		if v > bestCount || (v == bestCount && k < bestKey) {
			bestKey = k
			bestCount = v
		}
	}
	return bestKey, bestCount
}

// buildTopCards distills CardRankings into the top-N by WinContribution.
// Caller-friendly slim shape — drops the dead-in-hand / counter-rate
// fields that don't matter for a tournament summary.
func buildTopCards(rankings []analytics.CardRanking) []TopCard {
	if len(rankings) == 0 {
		return nil
	}
	sorted := append([]analytics.CardRanking(nil), rankings...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].WinContribution != sorted[j].WinContribution {
			return sorted[i].WinContribution > sorted[j].WinContribution
		}
		if sorted[i].GamesWon != sorted[j].GamesWon {
			return sorted[i].GamesWon > sorted[j].GamesWon
		}
		return sorted[i].Name < sorted[j].Name
	})
	if len(sorted) > topCardCap {
		sorted = sorted[:topCardCap]
	}
	out := make([]TopCard, 0, len(sorted))
	for _, c := range sorted {
		out = append(out, TopCard{
			Card:            c.Name,
			WinContribution: c.WinContribution,
			GamesWon:        c.GamesWon,
			KillShotRate:    c.KillShotRate,
		})
	}
	return out
}

// FormatTournamentSummary renders the structured summary as plain
// text suitable for terminal output.
func FormatTournamentSummary(s *TournamentSummary) string {
	if s == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("FREYA -- Tournament Summary\n")
	sb.WriteString("=============================\n")
	sb.WriteString(fmt.Sprintf("Mode: %s   Games: %d   Decks: %d   Seats: %d   Avg turn: %.1f\n\n",
		orDash(s.Mode), s.Games, s.TotalDecks, s.NSeats, s.AvgTurnToEnd))

	for _, tier := range s.Tiers {
		label := strings.ToUpper(tier.Tier)
		sb.WriteString(fmt.Sprintf("[%s tier] %d commanders\n", label, len(tier.Commanders)))
		for _, c := range tier.Commanders {
			eloTag := ""
			if c.ELO > 0 {
				eloTag = fmt.Sprintf("   ELO %.0f", c.ELO)
			}
			sb.WriteString(fmt.Sprintf("  %-32s  %3d/%3d  (%.1f%%)%s\n",
				c.Commander, c.Wins, c.Games, 100*c.Winrate, eloTag))
		}
		sb.WriteString("\n")
	}

	if len(s.ArchetypeBreakdown) > 0 {
		sb.WriteString("[Archetype meta]\n")
		// Sort archetypes by commander count desc for the panel.
		archs := make([]string, 0, len(s.ArchetypeBreakdown))
		for a := range s.ArchetypeBreakdown {
			archs = append(archs, a)
		}
		sort.SliceStable(archs, func(i, j int) bool {
			if s.ArchetypeBreakdown[archs[i]] != s.ArchetypeBreakdown[archs[j]] {
				return s.ArchetypeBreakdown[archs[i]] > s.ArchetypeBreakdown[archs[j]]
			}
			return archs[i] < archs[j]
		})
		for _, a := range archs {
			sb.WriteString(fmt.Sprintf("  %-22s %d decks, avg %.1f%% winrate\n",
				a, s.ArchetypeBreakdown[a], 100*s.ArchetypeWinrate[a]))
		}
		if s.BestArchetype != "" {
			sb.WriteString(fmt.Sprintf("  → Best archetype: %s at %.1f%% avg winrate\n",
				s.BestArchetype, 100*s.BestArchetypeWinrate))
		}
		sb.WriteString("\n")
	}

	if len(s.Upsets) > 0 {
		sb.WriteString("[Surprise upsets] (lower-ELO commander took the H2H)\n")
		for _, u := range s.Upsets {
			sb.WriteString(fmt.Sprintf("  %s (ELO %.0f) beat %s (ELO %.0f) %d/%d  [gap %.0f]\n",
				u.Winner, u.WinnerELO, u.Loser, u.LoserELO,
				u.WinnerWins, u.Games, u.ELOGap))
		}
		sb.WriteString("\n")
	}

	if len(s.OMWLeaders) > 0 {
		sb.WriteString("[OMW% leaders] (Swiss strength-of-schedule)\n")
		for _, l := range s.OMWLeaders {
			sb.WriteString(fmt.Sprintf("  %-32s OMW %.3f   pts %d   wins %d\n",
				l.Commander, l.OMW, l.Points, l.Wins))
		}
		sb.WriteString("\n")
	}

	if len(s.WinConditions) > 0 {
		sb.WriteString("[Win conditions] (top kill method per commander)\n")
		for _, w := range s.WinConditions {
			lethal := ""
			if w.TopLethalCard != "" {
				lethal = fmt.Sprintf(" via %s", w.TopLethalCard)
			}
			sb.WriteString(fmt.Sprintf("  %-32s %d wins — %s (%d%%)%s\n",
				w.Commander, w.Wins, w.TopMethod, w.TopMethodPct, lethal))
		}
		sb.WriteString("\n")
	}

	if len(s.TopCards) > 0 {
		sb.WriteString("[Top cards] (by win contribution)\n")
		for _, c := range s.TopCards {
			killTag := ""
			if c.KillShotRate > 0 {
				killTag = fmt.Sprintf(", kill-shot %.0f%%", 100*c.KillShotRate)
			}
			sb.WriteString(fmt.Sprintf("  %-32s contrib %.2f, %d wins%s\n",
				c.Card, c.WinContribution, c.GamesWon, killTag))
		}
	}

	return sb.String()
}

// PrintTournamentSummary writes the summary to w. JSON format emits
// the structured TournamentSummary directly; text format uses
// FormatTournamentSummary.
func PrintTournamentSummary(w io.Writer, s *TournamentSummary, format string) {
	if format == "json" {
		_ = json.NewEncoder(w).Encode(s)
		return
	}
	fmt.Fprint(w, FormatTournamentSummary(s))
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

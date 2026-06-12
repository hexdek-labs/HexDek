package analytics

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/judge"
	"github.com/hexdek/hexdek/internal/seedcontract"
)

// KillRecord captures a single elimination event: who killed whom,
// how, and when.
type KillRecord struct {
	KillerCommander string `json:"killer_commander"`
	VictimCommander string `json:"victim_commander"`
	KillerSeat      int    `json:"killer_seat"`
	VictimSeat      int    `json:"victim_seat"`
	Method          string `json:"method"`
	// MethodCanonical is Method normalized through
	// seedcontract.CanonicalKillMethod (consolidation step 2) so
	// cross-subsystem consumers compare against win_reason without
	// re-mapping. Method keeps the finer-grained analytics vocabulary
	// (combat_damage / noncombat_damage / life_drain / decking…) that
	// the persisted threat graph and method breakdowns rely on.
	MethodCanonical string `json:"method_canonical,omitempty"`
	LethalCard      string `json:"lethal_card,omitempty"`
	Turn            int    `json:"turn"`
	GameID          string `json:"game_id,omitempty"`
	Timestamp       string `json:"timestamp"`
}

// ThreatEdge aggregates kills between two commanders across games.
type ThreatEdge struct {
	KillerCommander string         `json:"killer_commander"`
	VictimCommander string         `json:"victim_commander"`
	Kills           int            `json:"kills"`
	Deaths          int            `json:"deaths"`
	TotalGames      int            `json:"total_games"`
	MethodBreakdown map[string]int `json:"method_breakdown"`
	TopLethalCards  []string       `json:"top_lethal_cards,omitempty"`
	LastSeen        string         `json:"last_seen"`
}

// KingmakerRecord tracks when a deck eliminates the eventual winner's
// rivals, effectively kingmaking.
type KingmakerRecord struct {
	KingmakerCommander string `json:"kingmaker_commander"`
	BeneficiaryWinner  string `json:"beneficiary_winner"`
	VictimsKilled      int    `json:"victims_killed"`
	GameID             string `json:"game_id,omitempty"`
}

// ThreatGraphSummary is the user-facing view for a single deck.
type ThreatGraphSummary struct {
	Commander      string       `json:"commander"`
	TopKills       []ThreatEdge `json:"top_kills"`
	TopDeaths      []ThreatEdge `json:"top_deaths"`
	KingmakerScore float64      `json:"kingmaker_score"`
	TotalKills     int          `json:"total_kills"`
	TotalDeaths    int          `json:"total_deaths"`
}

// ExtractKillRecords walks the event log of a completed game and
// infers who killed each eliminated player by finding the last
// damage source before the elimination event.
func ExtractKillRecords(events []gameengine.Event, nSeats int, commanderNames []string, winnerSeat int, gameID string) []KillRecord {
	if len(events) == 0 || len(commanderNames) == 0 {
		return nil
	}

	now := time.Now().UTC().Format(time.RFC3339)
	var records []KillRecord

	for i, ev := range events {
		if ev.Kind != "seat_eliminated" || ev.Seat < 0 || ev.Seat >= nSeats {
			continue
		}
		victimSeat := ev.Seat
		victimName := safeCommanderName(commanderNames, victimSeat)
		reason := ""
		lossCategory := ""
		lossSourceCard := ""
		if ev.Details != nil {
			if r, ok := ev.Details["reason"].(string); ok {
				reason = r
			}
			// Structured loss cause (consolidation step 2) — stamped on
			// the elimination event by HandleSeatElimination when the
			// engine wrote Seat.LossDetail. Preferred over substring-
			// parsing the freeform reason.
			if c, ok := ev.Details["loss_category"].(string); ok {
				lossCategory = c
			}
			if sc, ok := ev.Details["loss_source_card"].(string); ok {
				lossSourceCard = sc
			}
		}

		killerSeat, method, lethalCard := inferKiller(events[:i], victimSeat, reason, lossCategory, lossSourceCard, nSeats)

		if killerSeat < 0 || killerSeat == victimSeat {
			continue
		}

		turn := 0
		if ev.Details != nil {
			if t, ok := ev.Details["turn"].(int); ok {
				turn = t
			}
		}
		if turn == 0 {
			turn = inferTurn(events[:i])
		}

		records = append(records, KillRecord{
			KillerCommander: safeCommanderName(commanderNames, killerSeat),
			VictimCommander: victimName,
			KillerSeat:      killerSeat,
			VictimSeat:      victimSeat,
			Method:          method,
			MethodCanonical: seedcontract.CanonicalKillMethod(method),
			LethalCard:      lethalCard,
			Turn:            turn,
			GameID:          gameID,
			Timestamp:       now,
		})
	}

	return records
}

// inferKiller walks backwards from the elimination event to find the
// seat that dealt the lethal blow. lossCategory/lossSourceCard are the
// structured loss cause from the elimination event (consolidation step
// 2; empty for pre-LossDetail replays) — when present they replace the
// freeform-substring sniffing of lossReason; the substring checks below
// are the LEGACY FALLBACK, kept for old replays. Do not extend them.
func inferKiller(events []gameengine.Event, victimSeat int, lossReason, lossCategory, lossSourceCard string, nSeats int) (killerSeat int, method, lethalCard string) {
	killerSeat = -1

	// Commander damage losses encode the killer's commander name.
	if lossCategory == judge.LossCategoryCommanderDamage ||
		(lossCategory == "" && strings.Contains(lossReason, "21+ commander damage from")) {
		method = "commander_damage"
		for j := len(events) - 1; j >= 0; j-- {
			ev := &events[j]
			if ev.Kind == "damage" && ev.Target == victimSeat && ev.Seat >= 0 && ev.Seat != victimSeat {
				if isCmdr, _ := detailBool(ev, "commander"); isCmdr {
					killerSeat = ev.Seat
					lethalCard = ev.Source
					return
				}
			}
		}
	}

	// Poison counter loss.
	if lossCategory == judge.LossCategoryPoison ||
		(lossCategory == "" && strings.Contains(lossReason, "poison")) {
		method = "poison"
		for j := len(events) - 1; j >= 0; j-- {
			ev := &events[j]
			if ev.Kind == "damage" && ev.Target == victimSeat && ev.Seat >= 0 && ev.Seat != victimSeat {
				if infect, _ := detailBool(ev, "infect"); infect {
					killerSeat = ev.Seat
					lethalCard = ev.Source
					return
				}
			}
		}
	}

	// Life total 0 or less — find the damage source that crossed 0.
	if lossCategory == judge.LossCategoryLife ||
		(lossCategory == "" && (strings.Contains(lossReason, "life total 0") || lossReason == "")) {
		runningLife := 0
		for j := len(events) - 1; j >= 0; j-- {
			ev := &events[j]

			if ev.Kind == "damage" && ev.Amount > 0 && ev.Target == victimSeat && ev.Seat >= 0 && ev.Seat != victimSeat {
				if runningLife+ev.Amount > 0 {
					killerSeat = ev.Seat
					lethalCard = ev.Source
					isCombat, _ := detailBool(ev, "combat")
					if isCombat {
						method = "combat_damage"
					} else {
						method = "noncombat_damage"
					}
					return
				}
				runningLife += ev.Amount
			}

			if ev.Kind == "life_change" && ev.Seat == victimSeat && ev.Amount < 0 {
				// Life loss (not damage) — e.g., Exsanguinate, Sanguine Bond.
				if ev.Source != "" && runningLife-ev.Amount > 0 {
					// The source is a card name, not a seat. We need to find
					// who controls that source.
					controllerSeat := findSourceController(events[:j], ev.Source, nSeats)
					if controllerSeat >= 0 && controllerSeat != victimSeat {
						killerSeat = controllerSeat
						lethalCard = ev.Source
						method = "life_drain"
						return
					}
				}
				runningLife -= ev.Amount
			}
		}
	}

	// Concession / pact failure / other — attribute to no one specific.
	if lossCategory == judge.LossCategoryConcession ||
		(lossCategory == "" && strings.Contains(lossReason, "concession")) {
		method = "concession"
		return -1, method, ""
	}
	if lossCategory == judge.LossCategoryEmptyLibrary ||
		(lossCategory == "" && strings.Contains(lossReason, "drew from empty library")) {
		method = "decking"
		// Try to find who milled them.
		for j := len(events) - 1; j >= 0; j-- {
			ev := &events[j]
			if ev.Kind == "mill" && ev.Target == victimSeat && ev.Seat >= 0 && ev.Seat != victimSeat {
				killerSeat = ev.Seat
				lethalCard = ev.Source
				return
			}
		}
		return -1, method, ""
	}

	// Fallback: last damage dealer.
	for j := len(events) - 1; j >= 0; j-- {
		ev := &events[j]
		if ev.Kind == "damage" && ev.Target == victimSeat && ev.Seat >= 0 && ev.Seat != victimSeat && ev.Amount > 0 {
			killerSeat = ev.Seat
			lethalCard = ev.Source
			method = "damage"
			return
		}
	}

	return -1, "unknown", ""
}

func findSourceController(events []gameengine.Event, cardName string, nSeats int) int {
	for j := len(events) - 1; j >= 0; j-- {
		ev := &events[j]
		if ev.Source == cardName && ev.Seat >= 0 && ev.Seat < nSeats {
			return ev.Seat
		}
	}
	return -1
}

func inferTurn(events []gameengine.Event) int {
	for j := len(events) - 1; j >= 0; j-- {
		if events[j].Kind == "turn_start" || events[j].Kind == "phase_change" {
			if t, ok := events[j].Details["turn"].(int); ok {
				return t
			}
		}
	}
	return 0
}

func safeCommanderName(names []string, seat int) string {
	if seat >= 0 && seat < len(names) {
		return names[seat]
	}
	return "unknown"
}

// PersistThreatGraph reads the existing threat graph, merges new kill
// records, and writes back atomically.
func PersistThreatGraph(dir string, records []KillRecord) error {
	if len(records) == 0 {
		return nil
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path := filepath.Join(dir, "threat_graph.json")
	existing, _ := loadThreatGraph(path)

	// Merge new records into the edge map.
	type edgeKey struct{ killer, victim string }
	edges := make(map[edgeKey]*ThreatEdge, len(existing))
	for i := range existing {
		e := &existing[i]
		edges[edgeKey{e.KillerCommander, e.VictimCommander}] = e
	}

	for _, kr := range records {
		key := edgeKey{kr.KillerCommander, kr.VictimCommander}
		e, ok := edges[key]
		if !ok {
			e = &ThreatEdge{
				KillerCommander: kr.KillerCommander,
				VictimCommander: kr.VictimCommander,
				MethodBreakdown: map[string]int{},
			}
			edges[key] = e
		}
		e.Kills++
		e.TotalGames++
		if kr.Method != "" {
			e.MethodBreakdown[kr.Method]++
		}
		e.LastSeen = kr.Timestamp

		// Track top lethal cards (keep unique, max 5).
		if kr.LethalCard != "" {
			found := false
			for _, c := range e.TopLethalCards {
				if c == kr.LethalCard {
					found = true
					break
				}
			}
			if !found && len(e.TopLethalCards) < 5 {
				e.TopLethalCards = append(e.TopLethalCards, kr.LethalCard)
			}
		}
	}

	// Also populate the reverse Deaths count for symmetric queries.
	for key, e := range edges {
		reverseKey := edgeKey{key.victim, key.killer}
		if rev, ok := edges[reverseKey]; ok {
			e.Deaths = rev.Kills
		}
	}

	out := make([]ThreatEdge, 0, len(edges))
	for _, e := range edges {
		out = append(out, *e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Kills > out[j].Kills })

	return atomicWriteJSON(path, out)
}

// LoadThreatGraph reads the threat graph from disk. Missing file is not
// an error; a corrupt file surfaces the JSON parse error so callers can
// distinguish "no graph yet" from "stored graph is unreadable".
func LoadThreatGraph(dir string) ([]ThreatEdge, error) {
	path := filepath.Join(dir, "threat_graph.json")
	return loadThreatGraph(path)
}

// ThreatSummaryFor builds a user-facing threat graph summary for one
// commander, showing top kills, top deaths, and kingmaker score.
func ThreatSummaryFor(edges []ThreatEdge, commander string, topN int) ThreatGraphSummary {
	s := ThreatGraphSummary{Commander: commander}

	var kills, deaths []ThreatEdge
	for _, e := range edges {
		if e.KillerCommander == commander {
			s.TotalKills += e.Kills
			kills = append(kills, e)
		}
		if e.VictimCommander == commander {
			s.TotalDeaths += e.Kills
			deaths = append(deaths, e)
		}
	}

	sort.Slice(kills, func(i, j int) bool { return kills[i].Kills > kills[j].Kills })
	sort.Slice(deaths, func(i, j int) bool { return deaths[i].Kills > deaths[j].Kills })

	if topN > 0 && len(kills) > topN {
		kills = kills[:topN]
	}
	if topN > 0 && len(deaths) > topN {
		deaths = deaths[:topN]
	}

	s.TopKills = kills
	s.TopDeaths = deaths

	return s
}

func loadThreatGraph(path string) ([]ThreatEdge, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []ThreatEdge
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

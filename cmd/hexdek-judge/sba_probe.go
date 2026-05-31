package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// CR §704 State-Based Action probe.
//
// Fourth batch-mode probe in hexdek-judge after --check-mana-costs
// (CR §202.2), --check-commander (CR §903.5/.5b/.6/.4 + banned), and
// --check-deck-construction (CR §903.5 + §903.5b + §903.4 + bracket
// shape). This one operates on a saved game-state SNAPSHOT and reports
// every §704.5 / §704.6 SBA condition that exists in the snapshot
// but should have already been resolved — i.e. an SBA that "should
// have fired but did not."
//
// Use cases:
//   - Judge-mode replay analysis: a Loki crash snapshot, a real game
//     replay frame, or a hand-rolled state dump that's suspected to be
//     post-SBA-pause. The probe surfaces every unfired SBA with a CR
//     citation so the user can confirm whether the engine missed an
//     SBA or whether the state is mid-resolution (legitimately
//     pre-SBA).
//   - Test fixtures for new engine work: a developer writing a new SBA
//     handler can hand-author a snapshot that captures the
//     condition + expected violation, run the probe to confirm the
//     condition is detectable, then run the engine on the same input
//     and compare.
//   - CI gate: a game-replay corpus can be probed periodically to
//     verify the engine never produces a snapshot with an unresolved
//     SBA.
//
// The input snapshot is a SIMPLIFIED JSON shape that captures only
// the SBA-relevant facts — life total, poison counters, library
// count, battlefield permanents (types, P/T, marked damage, counters,
// attachments), command-zone contents, commander-damage matrix. This
// is intentionally NOT the engine's internal GameState struct: that
// struct churns with every engine release, while this probe needs a
// STABLE schema that downstream tooling (replay loaders, CI hooks,
// hand-authored test fixtures) can rely on.
//
// Sub-rules covered (9 of the 22 §704.5 letters — the snapshot-
// detectable subset):
//
//   §704.5a — A player whose life total is 0 or less loses.
//   §704.5c — A player with ten or more poison counters loses.
//   §704.5f — A creature with toughness ≤ 0 is put into its owner's
//             graveyard.
//   §704.5g — A creature with marked damage ≥ toughness (positive
//             toughness) is destroyed.
//   §704.5h — A planeswalker with 0 loyalty is put into its owner's
//             graveyard.
//   §704.5i — Legend rule: if a player controls 2+ legendary
//             permanents with the same name, all but one go to the
//             graveyard.
//   §704.5j — World rule: if there are 2+ world permanents on the
//             battlefield, all but the newest-timestamped one are
//             put into their owners' graveyards.
//   §704.5p — Pair removal: a permanent with both a +1/+1 and a
//             -1/-1 counter has them removed in pairs (in a clean
//             snapshot, neither should remain — both flags being
//             non-zero is the violation).
//   §704.6c — A player who has been dealt 21+ combat damage from a
//             single commander loses.
//
// Sub-rules NOT covered (snapshot-irrelevant or state-transitional):
//   §704.5b — Drew from empty library — requires turn-flag tracking
//             that doesn't survive in a static snapshot.
//   §704.5d, .5e — Token/copy in non-battlefield zone ceases — the
//             snapshot wouldn't normally include such ephemera.
//   §704.5k, .5m, .5n — Aura/Equipment attachment legality — the
//             attachment-target legality check requires deep
//             oracle-text introspection (target restriction, color
//             matching, type matching) that's out of scope for a
//             schema-stable static probe. Engine SBAs cover these
//             at runtime; the probe could be extended later if a
//             specific attachment-illegal pattern proves common.
//   §704.5q — Counter pair-removal mechanics — covered by §704.5p
//             in the snapshot view (both flags present = violation).
//   §704.5r — Counter cleanup — runtime-mutation rule, not a
//             snapshot violation.
//   §704.5s, .5t, .5u, .5v, .5w, .5x, .5y, .5z — Saga / Battle /
//             Day-Night / Role / Speed / Class — could be added
//             incrementally; the present set covers the high-value
//             game-loss / death rules.
//   §704.6d — Commander in graveyard/exile → optional command zone —
//             a CHOICE-driven SBA where the player elects whether
//             to redirect; a snapshot doesn't capture whether the
//             choice has been offered, so this isn't a violation
//             vs. a pending choice.

// SBASnapshot is the input schema — a simplified game-state shape
// containing only the fields the probe needs.
type SBASnapshot struct {
	Format string         `json:"format,omitempty"` // "commander" | "vintage" | etc — affects nothing in the probe but useful for human triage
	Turn   int            `json:"turn,omitempty"`
	Seats  []SBASeat      `json:"seats"`
}

// SBASeat is one player's state. CommandZone is only populated for
// Commander format. CommanderDamage[dealerIdx][commanderName] = dmg
// — mirrors the engine's per-commander damage matrix.
type SBASeat struct {
	Idx             int                       `json:"idx"`
	Life            int                       `json:"life"`
	PoisonCounters  int                       `json:"poison_counters,omitempty"`
	LibraryCount    int                       `json:"library_count,omitempty"`
	Lost            bool                      `json:"lost,omitempty"`
	LeftGame        bool                      `json:"left_game,omitempty"`
	Battlefield     []SBAPermanent            `json:"battlefield"`
	CommandZone     []string                  `json:"command_zone,omitempty"`
	CommanderDamage map[string]map[string]int `json:"commander_damage,omitempty"`
}

// SBAPermanent is a single permanent on the battlefield. Types are
// lowercase strings; "creature", "planeswalker", "artifact", "land",
// "enchantment", "battle" are the relevant values. Supertypes
// ("legendary", "snow", "world") live in Supertypes.
type SBAPermanent struct {
	Name         string         `json:"name"`
	Owner        int            `json:"owner"`
	Controller   int            `json:"controller"`
	Types        []string       `json:"types"`
	Supertypes   []string       `json:"supertypes,omitempty"`
	BasePower    int            `json:"base_power,omitempty"`
	BaseToughness int           `json:"base_toughness,omitempty"`
	MarkedDamage int            `json:"marked_damage,omitempty"`
	Loyalty      int            `json:"loyalty,omitempty"`
	Defense      int            `json:"defense,omitempty"`
	Counters     map[string]int `json:"counters,omitempty"`
	Tapped       bool           `json:"tapped,omitempty"`
	PhasedOut    bool           `json:"phased_out,omitempty"`
	Timestamp    int            `json:"timestamp,omitempty"`
}

// SBAReport is the top-level JSON output shape.
type SBAReport struct {
	Rule            string           `json:"rule"`
	SnapshotPath    string           `json:"snapshot_path"`
	TurnNumber      int              `json:"turn,omitempty"`
	SeatCount       int              `json:"seat_count"`
	Violations      []SBAViolation   `json:"violations"`
	ViolationsByRule map[string]int  `json:"violations_by_rule"`
	Valid           bool             `json:"valid"`
}

// SBAViolation is one unfired SBA detected in the snapshot.
type SBAViolation struct {
	Rule        string `json:"rule"`        // "704.5a" | "704.5c" | "704.5f" | …
	RuleName    string `json:"rule_name"`   // human-readable: "life total ≤ 0", "10+ poison counters", …
	Seat        int    `json:"seat"`
	PermanentName string `json:"permanent_name,omitempty"` // empty for player-level rules (5a/5c/6c)
	Detail      string `json:"detail"`
}

// hasType reports whether the permanent has the given lowercase type.
func (p *SBAPermanent) hasType(t string) bool {
	for _, s := range p.Types {
		if strings.EqualFold(s, t) {
			return true
		}
	}
	return false
}

// isLegendary reports whether the permanent is supertype Legendary.
func (p *SBAPermanent) isLegendary() bool {
	for _, s := range p.Supertypes {
		if strings.EqualFold(s, "legendary") {
			return true
		}
	}
	return false
}

// isWorld reports whether the permanent is supertype World.
func (p *SBAPermanent) isWorld() bool {
	for _, s := range p.Supertypes {
		if strings.EqualFold(s, "world") {
			return true
		}
	}
	return false
}

// effectiveToughness returns base toughness + counters contribution
// from +1/+1 and -1/-1. The probe treats the +1/+1 / -1/-1 counters
// independently — pair-removal (§704.5p / §704.5q) is a separate rule
// the probe also checks; for §704.5f/g the question is "what's the
// current toughness right now."
func (p *SBAPermanent) effectiveToughness() int {
	t := p.BaseToughness
	if p.Counters != nil {
		t += p.Counters["+1/+1"]
		t -= p.Counters["-1/-1"]
	}
	return t
}

// detectSBAViolations walks the snapshot and returns every violation
// in deterministic order: by rule letter first, then by seat, then
// by permanent name.
func detectSBAViolations(s *SBASnapshot) []SBAViolation {
	var out []SBAViolation
	for _, seat := range s.Seats {
		seat := seat
		// Skip seats that have already left the game — CR §800.4a
		// cleanup runs once. Their state is preserved for audit but
		// the SBAs are by definition already resolved.
		if seat.LeftGame {
			continue
		}

		// §704.5a — life ≤ 0
		if seat.Life <= 0 && !seat.Lost {
			out = append(out, SBAViolation{
				Rule:     "704.5a",
				RuleName: "life total ≤ 0",
				Seat:     seat.Idx,
				Detail:   fmt.Sprintf("seat %d at life=%d but Lost=false", seat.Idx, seat.Life),
			})
		}

		// §704.5c — 10+ poison counters
		if seat.PoisonCounters >= 10 && !seat.Lost {
			out = append(out, SBAViolation{
				Rule:     "704.5c",
				RuleName: "ten or more poison counters",
				Seat:     seat.Idx,
				Detail:   fmt.Sprintf("seat %d at poison=%d but Lost=false", seat.Idx, seat.PoisonCounters),
			})
		}

		// §704.6c — 21+ commander damage from a single source
		for dealer, perCmdr := range seat.CommanderDamage {
			for cmdrName, dmg := range perCmdr {
				if dmg >= 21 && !seat.Lost {
					out = append(out, SBAViolation{
						Rule:     "704.6c",
						RuleName: "21+ combat damage from a single commander",
						Seat:     seat.Idx,
						Detail:   fmt.Sprintf("seat %d took %d commander damage from %s (dealer %s) but Lost=false", seat.Idx, dmg, cmdrName, dealer),
					})
				}
			}
		}

		// Per-permanent rules: §704.5f, .5g, .5h, .5p
		// Legend-rule (704.5i) and world-rule (704.5j) need a second
		// scan with aggregation across permanents.
		for _, p := range seat.Battlefield {
			p := p
			if p.PhasedOut {
				continue
			}
			if p.hasType("creature") {
				t := p.effectiveToughness()
				// §704.5f — toughness ≤ 0 sends creature to graveyard
				if t <= 0 {
					out = append(out, SBAViolation{
						Rule:          "704.5f",
						RuleName:      "creature with toughness ≤ 0",
						Seat:          seat.Idx,
						PermanentName: p.Name,
						Detail:        fmt.Sprintf("%s on seat %d has effective toughness=%d (base=%d, counters=%v) but is still on the battlefield", p.Name, seat.Idx, t, p.BaseToughness, p.Counters),
					})
				} else if p.MarkedDamage >= t && t > 0 {
					// §704.5g — marked damage ≥ toughness destroys
					out = append(out, SBAViolation{
						Rule:          "704.5g",
						RuleName:      "creature with marked damage ≥ toughness",
						Seat:          seat.Idx,
						PermanentName: p.Name,
						Detail:        fmt.Sprintf("%s on seat %d has marked_damage=%d ≥ toughness=%d but is still on the battlefield", p.Name, seat.Idx, p.MarkedDamage, t),
					})
				}
			}
			if p.hasType("planeswalker") {
				loyalty := p.Loyalty
				if p.Counters != nil {
					loyalty += p.Counters["loyalty"]
				}
				// §704.5h — planeswalker with 0 loyalty
				if loyalty <= 0 {
					out = append(out, SBAViolation{
						Rule:          "704.5h",
						RuleName:      "planeswalker with 0 loyalty",
						Seat:          seat.Idx,
						PermanentName: p.Name,
						Detail:        fmt.Sprintf("%s on seat %d has loyalty=%d but is still on the battlefield", p.Name, seat.Idx, loyalty),
					})
				}
			}
			// §704.5p — both +1/+1 and -1/-1 counters present (pair-
			// removal should have collapsed them). The rule prescribes
			// pair removal until at most one kind remains; in a clean
			// post-SBA snapshot, no permanent should have BOTH.
			if p.Counters != nil {
				plus := p.Counters["+1/+1"]
				minus := p.Counters["-1/-1"]
				if plus > 0 && minus > 0 {
					out = append(out, SBAViolation{
						Rule:          "704.5p",
						RuleName:      "permanent with un-paired +1/+1 and -1/-1 counters",
						Seat:          seat.Idx,
						PermanentName: p.Name,
						Detail:        fmt.Sprintf("%s on seat %d has +1/+1=%d AND -1/-1=%d simultaneously — pair removal should have collapsed them", p.Name, seat.Idx, plus, minus),
					})
				}
			}
		}

		// §704.5i — legend rule. Group seat's legendary permanents by
		// name; any group with size ≥ 2 is a violation.
		legendCounts := map[string]int{}
		for _, p := range seat.Battlefield {
			if p.PhasedOut {
				continue
			}
			if p.isLegendary() {
				legendCounts[p.Name]++
			}
		}
		for name, n := range legendCounts {
			if n >= 2 {
				out = append(out, SBAViolation{
					Rule:          "704.5i",
					RuleName:      "legend rule — duplicate legendary permanents under one controller",
					Seat:          seat.Idx,
					PermanentName: name,
					Detail:        fmt.Sprintf("seat %d controls %d copies of legendary %s — all but one should go to the graveyard", seat.Idx, n, name),
				})
			}
		}
	}

	// §704.5j — world rule (cross-seat, since the world supertype is
	// global). Any state with 2+ world permanents on the battlefield
	// is a violation; all but the newest-timestamped one are put into
	// their owners' graveyards.
	type worldRef struct {
		name string
		seat int
	}
	var worlds []worldRef
	for _, seat := range s.Seats {
		if seat.LeftGame {
			continue
		}
		for _, p := range seat.Battlefield {
			if p.PhasedOut {
				continue
			}
			if p.isWorld() {
				worlds = append(worlds, worldRef{name: p.Name, seat: seat.Idx})
			}
		}
	}
	if len(worlds) >= 2 {
		// One violation per world beyond the first — same shape as
		// the legend-rule violation but at the cross-seat level.
		for _, w := range worlds {
			out = append(out, SBAViolation{
				Rule:          "704.5j",
				RuleName:      "world rule — multiple world permanents on the battlefield",
				Seat:          w.seat,
				PermanentName: w.name,
				Detail:        fmt.Sprintf("%d world permanents on the battlefield across all seats — all but the newest-timestamped should go to the graveyard", len(worlds)),
			})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Rule != out[j].Rule {
			return out[i].Rule < out[j].Rule
		}
		if out[i].Seat != out[j].Seat {
			return out[i].Seat < out[j].Seat
		}
		return out[i].PermanentName < out[j].PermanentName
	})
	return out
}

// runSBAProbe is the CLI entry point.
func runSBAProbe(snapshotPath, outPath string) (*SBAReport, error) {
	if snapshotPath == "" {
		return nil, fmt.Errorf("--check-sba requires --snapshot <path>")
	}
	f, err := os.Open(snapshotPath)
	if err != nil {
		return nil, fmt.Errorf("open snapshot: %w", err)
	}
	defer f.Close()
	var snap SBASnapshot
	if err := json.NewDecoder(f).Decode(&snap); err != nil {
		return nil, fmt.Errorf("decode snapshot: %w", err)
	}
	violations := detectSBAViolations(&snap)
	rep := &SBAReport{
		Rule:             "CR §704",
		SnapshotPath:     snapshotPath,
		TurnNumber:       snap.Turn,
		SeatCount:        len(snap.Seats),
		Violations:       violations,
		ViolationsByRule: map[string]int{},
		Valid:            len(violations) == 0,
	}
	for _, v := range violations {
		rep.ViolationsByRule[v.Rule]++
	}
	return rep, writeSBAReport(rep, outPath)
}

func writeSBAReport(rep *SBAReport, outPath string) error {
	var w io.Writer = os.Stdout
	if outPath != "" {
		f, err := os.Create(outPath)
		if err != nil {
			return fmt.Errorf("create %s: %w", outPath, err)
		}
		defer f.Close()
		w = f
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}

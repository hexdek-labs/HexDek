package hat

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// OpponentProfile is the rolling, per-opponent tally of what we have
// observed them do this game, plus a derived classification. Lives
// inside YggdrasilHat as opponentProfiles[seat]. Initialized on first
// ObserveEvent and reset on game_start.
//
// Archetype values mirror the StrategyProfile constants on our own
// hat ("aggro", "combo", "control", "midrange") plus "unknown" for
// the early game when we haven't seen enough plays to commit.
//
// Confidence ramps each turn the classification holds: a snap call on
// turn 3 starts at 0.6 and crawls toward 0.9 as more plays land
// without contradicting the archetype.
//
// ThreatLevel is a [0,1] readout we expose for use by decision
// functions — recomputed every time classifyOpponent runs. It blends
// raw board power with archetype urgency (combo near win = max
// threat regardless of board, aggro at 4+ creatures = high threat).
type OpponentProfile struct {
	Archetype       string
	Confidence      float64
	ThreatLevel     float64
	CreaturesPlayed int
	SpellsPlayed    int
	LandsPlayed     int
	TutorsUsed      int
	RemovalUsed     int
	CountersUsed    int
	ComboSignals    int

	// Recency tracking: turn-number of the most recent action of each
	// kind. A tutor on turn 2 followed by 8 quiet turns is a much
	// weaker "about to pop off" signal than a tutor on the prior turn,
	// but the running totals above can't tell those apart. 0 means
	// "never observed". Set by recordOpponentPlay; consumed by
	// computeThreatLevel via TutoredWithin / ComboSignalWithin.
	LastTutorTurn       int
	LastComboSignalTurn int
	LastCounterTurn     int
	LastRemovalTurn     int

	// Internal bookkeeping.
	firstClassifiedTurn int
	lastClassifiedTurn  int
	stableTurns         int // consecutive turns the archetype has held
}

// R60 round 5 — archetype-bias confidence curve.
//
// archetypeBiasMinConfidence is the floor below which an OpponentProfile
// classification is too uncertain to act on. Returns 0 multiplier — the
// decision falls back to non-archetype signals.
//
// archetypeBiasHighThreshold marks where confidence transitions from
// "soft signal" to "we KNOW this archetype." Above this threshold the
// multiplier curves up super-linearly so a 0.90 stable classification
// has noticeably more pull on decisions than a 0.75 borderline call.
//
// archetypeBiasHighSlope is the post-threshold gradient. With
// MinConfidence=0.4, HighThreshold=0.75, HighSlope=2.0:
//   conf 0.40 → mult 0.40  (baseline)
//   conf 0.60 → mult 0.60
//   conf 0.75 → mult 0.75  (no boost yet)
//   conf 0.90 → mult 1.20  (+60% over linear)
//   conf 0.95 → mult 1.35  (+42% over linear at the cap)
//
// Tuned so the pre-R60r5 linear pass-through at moderate confidence
// is preserved (no behavior change for 0.4-0.75 cases) but a hat
// that has watched an opponent tutor three turns in a row commits
// noticeably harder to pressuring their setup.
const (
	archetypeBiasMinConfidence  = 0.4
	archetypeBiasHighThreshold  = 0.75
	archetypeBiasHighSlope      = 2.0
)

// archetypeBiasMultiplier returns the [0, ~1.4] confidence-derived
// multiplier the hat applies to opponent-archetype-driven decision
// biases. Below archetypeBiasMinConfidence the multiplier is 0 (no
// archetype-driven adjustment). Linear through archetypeBiasHighThreshold
// (so prior behavior at moderate confidence is preserved bit-for-bit).
// Super-linear above the threshold so a high-confidence read commits
// the hat harder than a borderline guess. Caps via the natural ceiling
// from Confidence being capped at 0.95 in classifyOpponent.
func archetypeBiasMultiplier(conf float64) float64 {
	if conf < archetypeBiasMinConfidence {
		return 0
	}
	mult := conf
	if conf > archetypeBiasHighThreshold {
		mult += (conf - archetypeBiasHighThreshold) * archetypeBiasHighSlope
	}
	return mult
}

// TutoredWithin reports whether the opponent has tutored within the
// last `n` turns of `turn`. Returns false when no tutor has ever been
// observed (LastTutorTurn == 0).
func (p *OpponentProfile) TutoredWithin(turn, n int) bool {
	if p == nil || p.LastTutorTurn <= 0 || n <= 0 {
		return false
	}
	return turn-p.LastTutorTurn < n
}

// ComboSignalWithin: same shape for combo-piece cast events.
func (p *OpponentProfile) ComboSignalWithin(turn, n int) bool {
	if p == nil || p.LastComboSignalTurn <= 0 || n <= 0 {
		return false
	}
	return turn-p.LastComboSignalTurn < n
}

// classifyOpponent derives an OpponentProfile snapshot for a single
// seat. Reads the per-event tallies maintained by recordOpponentPlay
// plus current board state to assign Archetype, Confidence, and
// ThreatLevel.
//
// Classification rules (from observable plays):
//   - 3+ creatures by turn ≤3 → "aggro" (start 0.6).
//   - Tutored + held mana 2+ turns + few board plays → "combo" (0.7).
//   - Used removal + counters + draw + few creatures → "control" (0.6).
//   - Mix of creatures + value pieces → "midrange" (0.4).
//   - Below thresholds or too early → "unknown" (0.0).
//
// Confidence stickies up by +0.05 per stable turn, capped at 0.95.
// Calling this on the same seat repeatedly within a turn returns the
// cached profile without re-incrementing stableTurns.
func (h *YggdrasilHat) classifyOpponent(gs *gameengine.GameState, oppSeat int) *OpponentProfile {
	if oppSeat < 0 || oppSeat >= len(h.opponentProfiles) {
		return nil
	}
	prof := h.opponentProfiles[oppSeat]
	if prof == nil {
		prof = &OpponentProfile{Archetype: "unknown"}
		h.opponentProfiles[oppSeat] = prof
	}

	turn := 1
	if gs != nil {
		turn = gs.Turn
		if turn < 1 {
			turn = 1
		}
	}

	// Determine the candidate archetype from accumulated stats. Order
	// matters: combo and control shapes win over aggro when their
	// signals are strong, since aggro is a default for "lots of cheap
	// creatures" which combo decks may briefly look like.
	candidate := "unknown"
	baseConf := 0.0

	// Combo: tutored + sandbagging mana + few board plays.
	heldMana := 0
	if oppSeat < len(h.opponentHeldMana) {
		heldMana = h.opponentHeldMana[oppSeat]
	}
	if prof.TutorsUsed >= 1 && heldMana >= 2 && prof.CreaturesPlayed <= 3 {
		candidate = "combo"
		baseConf = 0.7
	} else if prof.ComboSignals >= 2 {
		candidate = "combo"
		baseConf = 0.65
	} else if prof.RemovalUsed+prof.CountersUsed >= 3 && prof.CreaturesPlayed <= 3 {
		// Control: removal + counters, light board.
		candidate = "control"
		baseConf = 0.6
	} else if prof.CreaturesPlayed >= 3 && (turn <= 3 || prof.CreaturesPlayed >= turn) {
		// Aggro: piling on creatures fast — either early (3 by turn 3)
		// or sustained (one creature per turn average).
		candidate = "aggro"
		baseConf = 0.6
	} else if prof.CreaturesPlayed >= 2 && prof.SpellsPlayed >= 4 && prof.RemovalUsed >= 1 {
		// Midrange: some board + some interaction (the removal floor
		// distinguishes from pure aggro running 4+ cheap creatures).
		candidate = "midrange"
		baseConf = 0.4
	}

	// If candidate is the same as last classification, ratchet
	// confidence upward (one bump per turn, not per call).
	if candidate == prof.Archetype && candidate != "unknown" {
		if turn > prof.lastClassifiedTurn {
			prof.stableTurns++
			prof.lastClassifiedTurn = turn
		}
		conf := baseConf + 0.05*float64(prof.stableTurns)
		if conf > 0.95 {
			conf = 0.95
		}
		prof.Confidence = conf
	} else if candidate == "unknown" {
		// Don't blow away a prior classification just because the
		// momentary shape ambiguous; decay confidence slowly.
		if prof.Archetype != "unknown" {
			prof.Confidence *= 0.95
			if prof.Confidence < 0.2 {
				prof.Archetype = "unknown"
				prof.Confidence = 0
				prof.stableTurns = 0
				prof.firstClassifiedTurn = 0
			}
		}
	} else {
		// New classification — reset stability counter.
		if prof.Archetype != candidate {
			prof.firstClassifiedTurn = turn
			prof.stableTurns = 0
		}
		prof.Archetype = candidate
		prof.Confidence = baseConf
		prof.lastClassifiedTurn = turn
	}

	prof.ThreatLevel = computeThreatLevel(gs, oppSeat, prof)
	return prof
}

// computeThreatLevel blends board power, life, and archetype urgency
// into a [0,1] danger reading. Combo opponents about to assemble are
// always max threat regardless of life total; aggro with a wide board
// scales with permanent count; control rises with their hand size.
func computeThreatLevel(gs *gameengine.GameState, oppSeat int, prof *OpponentProfile) float64 {
	if gs == nil || oppSeat < 0 || oppSeat >= len(gs.Seats) || gs.Seats[oppSeat] == nil {
		return 0
	}
	s := gs.Seats[oppSeat]
	if s.Lost || s.LeftGame {
		return 0
	}

	level := 0.0

	// Board pressure: creatures + permanents.
	creatures := 0
	totalPower := 0
	for _, p := range s.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		creatures++
		totalPower += gs.PowerOf(p)
	}
	level += float64(creatures) * 0.04
	level += float64(totalPower) * 0.02

	// Hand size adds latent threat (more answers / threats hidden).
	level += float64(len(s.Hand)) * 0.02

	// Archetype-specific bumps. Two changes from the static version:
	//
	//   - Multiplied by prof.Confidence. A 0.55 snap call on a half-
	//     observed opponent shouldn't fully amplify their threat the
	//     same as a 0.90 stable classification. The board / hand-size
	//     terms above stay raw because they're direct observations.
	//
	//   - Combo opponents read their imminence off RECENT tutors, not
	//     ever-tutored. A combo deck that tutored on turn 2 and sat
	//     back for 8 turns is far less likely to pop off than one who
	//     tutored last turn; the old `TutorsUsed >= 2` flag couldn't
	//     tell those apart. Recency window matches typical cEDH
	//     execute-turn timing: a tutor within the last 2 turns is a
	//     setup signal, within the last 4 is a soft signal.
	turn := 1
	if gs != nil && gs.Turn > 0 {
		turn = gs.Turn
	}
	switch prof.Archetype {
	case "combo":
		level += 0.25 * prof.Confidence
		switch {
		case prof.TutoredWithin(turn, 2):
			level += 0.20 * prof.Confidence
		case prof.TutoredWithin(turn, 4):
			level += 0.10 * prof.Confidence
		case prof.TutorsUsed >= 2:
			// Old/stale tutors still bump, but at a discount — they
			// signal "this is a combo deck" more than "popping off
			// imminently".
			level += 0.05 * prof.Confidence
		}
	case "aggro":
		// Aggro becomes scary fast — wide board + low life pressure.
		if creatures >= 4 {
			level += 0.25 * prof.Confidence
		}
	case "control":
		// Mostly an indirect threat — they slow us down rather than
		// kill us. Bump for hand size only.
		level += 0.05 * prof.Confidence
	}

	if level > 1.0 {
		level = 1.0
	}
	return level
}

// recordOpponentPlay updates the rolling per-opponent tallies for one
// observed event. Called from ObserveEvent on cast / play_land /
// permanent_etb / tutor / search_library kinds. The classifier reads
// these tallies on demand via classifyOpponent.
//
// The card is identified by source name; the engine resolver writes
// the spell's printed name into Event.Source. We use that to detect
// removal / counter / combo-piece patterns without needing the full
// card record. Combo-piece detection reuses h.comboPieceSet (built
// from Freya), so unknown decks that still hit our combo-piece DB
// flag immediately.
func (h *YggdrasilHat) recordOpponentPlay(eventKind, sourceName string, oppSeat int, card *gameengine.Card, turn int) {
	if oppSeat < 0 || oppSeat >= len(h.opponentProfiles) {
		return
	}
	prof := h.opponentProfiles[oppSeat]
	if prof == nil {
		prof = &OpponentProfile{Archetype: "unknown"}
		h.opponentProfiles[oppSeat] = prof
	}

	switch eventKind {
	case "cast":
		prof.SpellsPlayed++
		// Classify the spell. We may have either a *Card pointer (best
		// case) or just the name; both paths fall through to substring
		// matching on the lowered source name as a last resort.
		if card != nil {
			if cardHasType(card, "creature") {
				prof.CreaturesPlayed++
			}
			if gameengine.CardHasCounterSpell(card) || hasOracleHint(card, "counter target") {
				prof.CountersUsed++
				if turn > 0 {
					prof.LastCounterTurn = turn
				}
			}
			ot := cardOracleText(card)
			if isRemovalText(ot) {
				prof.RemovalUsed++
				if turn > 0 {
					prof.LastRemovalTurn = turn
				}
			}
			if isTutorText(ot) {
				prof.TutorsUsed++
				if turn > 0 {
					prof.LastTutorTurn = turn
				}
			}
		}
		// Combo-piece DB hit (works even without a Card pointer).
		if sourceName != "" && h.comboPieceSet[sourceName] {
			prof.ComboSignals++
			if turn > 0 {
				prof.LastComboSignalTurn = turn
			}
		}
	case "permanent_etb", "creature_etb":
		// ETB events come for our own perms too; the caller should
		// have filtered to opponent seats already.
		if card != nil && cardHasType(card, "creature") {
			prof.CreaturesPlayed++
		}
	case "play_land":
		prof.LandsPlayed++
	case "tutor", "search_library":
		prof.TutorsUsed++
		if turn > 0 {
			prof.LastTutorTurn = turn
		}
	}
}

// findCardByName scans the seat's reachable zones for a card with
// the given printed name, returning the first match. Used by
// recordOpponentPlay to recover the *Card pointer from a name-only
// event so we can classify the spell's effect / type. Returns nil if
// no match is found — recordOpponentPlay degrades to combo-piece
// lookup in that case.
func findCardByName(gs *gameengine.GameState, seatIdx int, name string) *gameengine.Card {
	if gs == nil || name == "" || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	// Stack first — a freshly-cast spell still lives here when the
	// "cast" event fires.
	for _, item := range gs.Stack {
		if item == nil || item.Card == nil {
			continue
		}
		if item.Card.DisplayName() == name {
			return item.Card
		}
	}
	s := gs.Seats[seatIdx]
	if s == nil {
		return nil
	}
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.Card.DisplayName() == name {
			return p.Card
		}
	}
	for _, c := range s.Graveyard {
		if c == nil {
			continue
		}
		if c.DisplayName() == name {
			return c
		}
	}
	return nil
}

// cardOracleText returns the lowered oracle text of `c`, augmented
// by any "oracle:..." prefixed tokens stored in card.Types. The
// AST-derived path (gameengine.OracleTextLower) is authoritative for
// real corpus cards; the Types-suffix path lets tests seed phrases
// without building a full CardAST.
func cardOracleText(c *gameengine.Card) string {
	if c == nil {
		return ""
	}
	ot := gameengine.OracleTextLower(c)
	for _, t := range c.Types {
		if strings.HasPrefix(t, "oracle:") {
			ot += " " + strings.ToLower(strings.TrimPrefix(t, "oracle:"))
		}
	}
	return ot
}

// hasOracleHint is true if `c`'s combined oracle text (AST + Types
// "oracle:" tokens) contains the substring `hint` (lowered).
func hasOracleHint(c *gameengine.Card, hint string) bool {
	if c == nil {
		return false
	}
	return strings.Contains(cardOracleText(c), strings.ToLower(hint))
}

// cardHasType wraps a HasType-style check at this layer to avoid
// pulling in the per_card helper. Mirrors the pattern used by the
// hat's internal classifiers.
func cardHasType(c *gameengine.Card, t string) bool {
	if c == nil {
		return false
	}
	want := strings.ToLower(t)
	for _, got := range c.Types {
		if strings.ToLower(got) == want {
			return true
		}
	}
	return false
}

// isRemovalText returns true for oracle text that smells like single-
// target removal — destroy / exile / -X/-X / "deals N damage to
// target". Substring-only; intentionally over-broad rather than
// missing entries.
func isRemovalText(ot string) bool {
	if ot == "" {
		return false
	}
	if strings.Contains(ot, "destroy target") {
		return true
	}
	if strings.Contains(ot, "exile target") {
		return true
	}
	if strings.Contains(ot, "deals") && strings.Contains(ot, "damage to target") {
		return true
	}
	if strings.Contains(ot, "target creature gets -") {
		return true
	}
	if strings.Contains(ot, "destroy all") || strings.Contains(ot, "exile all") {
		return true
	}
	return false
}

// isTutorText covers the canonical tutor wording without trying to
// catch every modal/conditional tutor. The classifier double-checks
// against the search_library / tutor event kinds anyway.
func isTutorText(ot string) bool {
	if ot == "" {
		return false
	}
	if strings.Contains(ot, "search your library for") {
		return true
	}
	return false
}

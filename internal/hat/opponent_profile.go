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

	// Contradictions counts observations that don't fit the current
	// archetype classification (R60 round 5 — "I've been wrong before"
	// decay). Combo casting Wrath, aggro casting Counterspell, control
	// slamming a 6/6 — each bumps this counter AND immediately
	// subtracts contradictDecayStep from Confidence, so we lose faith
	// in a stale read 3x faster than the +0.05/turn ramp builds it.
	// Reset to 0 on game_start and when the classification changes.
	Contradictions int

	// MetaConfidence (R60 round 5 — meta-confidence layer) is the
	// "how confident IS the confidence?" signal in [0, 1]. The raw
	// Confidence field says "I think this opponent is combo at 0.65" —
	// MetaConfidence says "but my sample size is thin / my read has
	// been volatile / I've seen contradicting actions, so trust the
	// 0.65 less than its face value." Composed by computeMetaConfidence
	// from three factors: sample (total observations), stability
	// (turns this archetype has held), and contradiction history.
	// Recomputed every classifyOpponent call. Consumed by
	// `effectiveArchetypeBias` which the downstream targeting /
	// removal-selection sites use instead of the raw bias multiplier.
	MetaConfidence float64

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

// R60 round 5 — meta-confidence layer.
//
// The raw Confidence field is "how sure am I this opponent is combo."
// MetaConfidence is "how sure am I that the 0.65 reading IS RIGHT."
// Borderline cases — thin sample, volatile read, recent contradictions
// — should have low meta-confidence so the hat doesn't lean as hard
// on the classification as the raw confidence value would suggest.
//
// Three factors, each in roughly [0.5, 1.0]:
//
//   - sampleFactor: scales with total observations the classifier
//     has seen. Few observations = thin sample = unreliable.
//   - stabilityFactor: scales with stableTurns. A read that just
//     flipped to combo last turn is less reliable than one that's
//     held for five turns.
//   - contradictionFactor: dampens with the Contradictions counter.
//     A high Confidence WITH contradictions is a stale stuck read.
//
// Product capped at 1.0. Below the floor we expose `metaFloor` so a
// fresh game with one observation still produces a non-zero meta
// value — we don't want to fully zero the archetype bias on the
// first observation, just dampen it.
const (
	metaSampleFullAt       = 8    // observations at which sample factor hits 1.0
	metaStabilityFullAt    = 6    // stableTurns at which stability factor hits 1.0
	metaContradictionStep  = 0.15 // factor reduction per contradiction
	metaContradictionFloor = 0.5  // contradiction factor can't drop below this
	// Sample / stability floors are set high enough that a fresh
	// thin-sample classification still gets a usable meta value —
	// the layer is meant to discount borderline reads, not erase
	// them. At full saturation we hit 1.0; at the worst case
	// (1 observation, 0 stable turns, 0 contradictions) meta still
	// returns ~0.35 — visible damping without flipping the sign of
	// the bias.
	metaSampleFloor    = 0.5
	metaStabilityFloor = 0.6
)

// computeMetaConfidence returns the [0, 1] meta-confidence for an
// OpponentProfile. Pure function — caller is responsible for providing
// the current observation count and reading prof's stableTurns +
// Contradictions. Returns 0 for a nil profile.
func computeMetaConfidence(prof *OpponentProfile, observations int) float64 {
	if prof == nil {
		return 0
	}
	if prof.Archetype == "unknown" || prof.Archetype == "" {
		// No classification → meta-confidence is meaningless; downstream
		// callers should already gate on Confidence > 0 before consulting
		// meta. Return 0 to make accidental consumption safe.
		return 0
	}

	// Sample factor: linear ramp from floor to 1.0 across the
	// metaSampleFullAt-observation window. Few observations → thin
	// sample → low meta.
	sampleFactor := metaSampleFloor +
		(1.0-metaSampleFloor)*float64(observations)/float64(metaSampleFullAt)
	if sampleFactor > 1.0 {
		sampleFactor = 1.0
	}

	// Stability factor: linear ramp from floor to 1.0 across the
	// metaStabilityFullAt-turn window. New classification → volatile
	// → low meta.
	stabilityFactor := metaStabilityFloor +
		(1.0-metaStabilityFloor)*float64(prof.stableTurns)/float64(metaStabilityFullAt)
	if stabilityFactor > 1.0 {
		stabilityFactor = 1.0
	}

	// Contradiction factor: each recorded contradiction dampens by
	// metaContradictionStep, floored at metaContradictionFloor.
	contradictionFactor := 1.0 - float64(prof.Contradictions)*metaContradictionStep
	if contradictionFactor < metaContradictionFloor {
		contradictionFactor = metaContradictionFloor
	}

	meta := sampleFactor * stabilityFactor * contradictionFactor
	if meta > 1.0 {
		meta = 1.0
	}
	if meta < 0 {
		meta = 0
	}
	return meta
}

// effectiveArchetypeBias is the public surface downstream call sites
// (attack-target, removal-selection) should prefer over
// archetypeBiasMultiplier alone. It folds in MetaConfidence so a
// borderline classification with a thin sample or recent
// contradictions dampens the bias even at moderate raw confidence:
//
//   bias = archetypeBiasMultiplier(prof.Confidence) * prof.MetaConfidence
//
// A 0.85 Confidence with full MetaConfidence (1.0) still produces the
// 1.05 super-linear multiplier from #307. The same 0.85 Confidence
// with MetaConfidence=0.55 (thin sample + 1 contradiction) drops to
// ~0.58 — pulling out of the super-linear zone and letting other
// signals re-balance the decision.
//
// Returns 0 for a nil profile or below-floor confidence (matching
// archetypeBiasMultiplier's semantics).
func effectiveArchetypeBias(prof *OpponentProfile) float64 {
	if prof == nil {
		return 0
	}
	return archetypeBiasMultiplier(prof.Confidence) * prof.MetaConfidence
}

// R60 round 5 — "I've been wrong before" contradiction decay.
//
// The ramp-up rate for confidence is +0.05 per stable turn (see
// classifyOpponent). The decay when a candidate goes unknown is
// *0.95 per turn (≈5% drop). When an OBSERVED ACTION contradicts the
// current classification (combo deck casting Wrath, aggro deck
// casting Counterspell, etc.) we want to lose confidence FASTER —
// the read was wrong, not just stale. contradictDecayStep is the
// per-contradiction confidence subtracted; sized to be 3x the ramp
// rate so a single contradiction undoes three turns of "I'm sure
// they're combo" stickiness.
//
// contradictMaxPenaltyPerTurn caps the per-turn penalty so a
// scripted multi-spell turn (Wrath + Counterspell + Wrath copy)
// doesn't snap confidence to zero in one observation window. The
// classifier still sees every contradiction (Contradictions
// increments unbounded for diagnostics) but the confidence drop is
// gated.
const (
	contradictDecayStep         = 0.15
	contradictMaxPenaltyPerTurn = 0.30
)

// isMassRemovalText returns true for oracle text that smells like a
// board wipe — "destroy all", "exile all", "deals N damage to each
// creature", "creatures get -X/-X" sweep wording. Distinct from
// isRemovalText (which bundles single-target and mass under one flag)
// so the archetype-contradiction detector can rule combo decks out
// without flagging single-target removal.
func isMassRemovalText(ot string) bool {
	if ot == "" {
		return false
	}
	if strings.Contains(ot, "destroy all") || strings.Contains(ot, "exile all") {
		return true
	}
	if strings.Contains(ot, "damage to each creature") || strings.Contains(ot, "damage to each opponent") {
		return true
	}
	if strings.Contains(ot, "all creatures get -") || strings.Contains(ot, "creatures you don't control get -") {
		return true
	}
	return false
}

// contradictsArchetype reports whether a cast event is inconsistent
// with the opponent's current classification. The check is
// intentionally narrow — only HIGH-SIGNAL mismatches count, since
// false positives here drive the hat to drop confidence on a deck
// that's just playing a flexible card. Each archetype defines its
// canonical NON-PLAYS:
//
//   - combo: casts a board wipe (combo lists run targeted answers,
//     not mass removal — Wrath in a combo deck means we misread).
//   - aggro: casts a counterspell OR a board wipe (aggro decks
//     pressure the board, they don't tap out to undo it).
//   - control: slams an attack-relevant 5+ CMC creature (control
//     plays utility creatures and finishers, but a 5+ CMC threat
//     being played AND swinging suggests midrange/aggro/voltron).
//
// Returns false for "unknown" and "midrange" — those are the
// catch-all classifications and we don't have a sharp NON-PLAY to
// contradict against without producing false positives.
func contradictsArchetype(arch string, card *gameengine.Card) bool {
	if arch == "" || arch == "unknown" || arch == "midrange" || card == nil {
		return false
	}
	ot := cardOracleText(card)
	switch arch {
	case "combo":
		return isMassRemovalText(ot)
	case "aggro":
		if gameengine.CardHasCounterSpell(card) || hasOracleHint(card, "counter target") {
			return true
		}
		return isMassRemovalText(ot)
	case "control":
		if !cardHasType(card, "creature") {
			return false
		}
		return cardCMC(card) >= 5
	}
	return false
}

// cardCMC returns the converted mana cost / mana value of `c`, falling
// back to scanning the test-only "cost:N" type suffix used by minimal
// test fixtures when CMC is not populated directly.
func cardCMC(c *gameengine.Card) int {
	if c == nil {
		return 0
	}
	if c.CMC > 0 {
		return c.CMC
	}
	for _, t := range c.Types {
		if strings.HasPrefix(t, "cost:") {
			n := 0
			for _, r := range strings.TrimPrefix(t, "cost:") {
				if r < '0' || r > '9' {
					return n
				}
				n = n*10 + int(r-'0')
			}
			return n
		}
	}
	return 0
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
		// New classification — reset stability counter AND contradiction
		// counter (R60r5). Contradictions tagged against the old
		// archetype don't apply to the new one — a deck that looked
		// like combo and contradicted itself by casting Wrath probably
		// IS the control deck the new candidate now reads as, so the
		// prior contradictions were actually evidence FOR this
		// classification, not against it.
		if prof.Archetype != candidate {
			prof.firstClassifiedTurn = turn
			prof.stableTurns = 0
			prof.Contradictions = 0
		}
		prof.Archetype = candidate
		prof.Confidence = baseConf
		prof.lastClassifiedTurn = turn
	}

	// R60 round 5 — meta-confidence layer. Total observations = the
	// raw input to the sample factor. SpellsPlayed already counts
	// every cast; LandsPlayed is a stable per-turn signal that
	// complements it without double-counting creature-vs-spell
	// distinctions.
	observations := prof.SpellsPlayed + prof.LandsPlayed
	prof.MetaConfidence = computeMetaConfidence(prof, observations)

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

			// R60 round 5 — "I've been wrong before" decay. If this
			// cast contradicts the current classification (combo
			// casting Wrath, aggro casting Counterspell, control
			// slamming a 6/6) we want to lose faith FASTER than the
			// +0.05/turn ramp built it. contradictDecayStep is the
			// per-contradiction subtract; contradictMaxPenaltyPerTurn
			// caps the total drop per observation window so a single
			// scripted turn doesn't snap confidence to zero. The
			// classifier itself is untouched here — the next
			// classifyOpponent call still re-evaluates the candidate;
			// we just tag this read as less trustworthy in the
			// meantime.
			if contradictsArchetype(prof.Archetype, card) {
				prof.Contradictions++
				if prof.Confidence > 0 {
					penalty := contradictDecayStep
					if penalty > contradictMaxPenaltyPerTurn {
						penalty = contradictMaxPenaltyPerTurn
					}
					prof.Confidence -= penalty
					if prof.Confidence < 0 {
						prof.Confidence = 0
					}
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

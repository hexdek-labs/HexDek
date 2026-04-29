package gameengine

// Phase 10 — Pluggable player-decision protocol (Hat).
//
// A Hat encapsulates every decision the rules leave up to a player.
// The engine CALLS these methods; the engine MUST NEVER INSPECT what
// kind of Hat a seat has. Hats are swappable at any time — tournament
// runners load N hats into N seats and run.
//
// Architectural directive from the project lead (7174n1c):
//   "hats are swappable, engine never inspects them."
//
// Rationale — Comprehensive Rules pointer map
// ----------------------------------------------------------------------
//   §103.4   Mulligan decision                       → ChooseMulligan
//   §116.1b  Choosing which spell / land to play     → ChooseLandToPlay /
//                                                      ChooseCastFromHand
//   §117.3   Priority / passing                      → ChooseResponse
//   §506.1   Each attacker chooses its defender      → ChooseAttackers /
//                                                      ChooseAttackTarget
//   §509.1   Defending player declares blockers      → AssignBlockers
//   §601.2c  Choosing modes for a modal spell        → ChooseMode
//   §608.2a  Each target is chosen                   → ChooseTarget
//   §616.1   Affected-player picks replacement order → OrderReplacements
//   §701.8   Discard (player chooses unless told)    → ChooseDiscard
//   §903.8   Whether to cast commander (+ tax)       → ShouldCastCommander
//   §903.9a  Commander-zone redirect choice          → ShouldRedirectCommanderZone
//
// The interface lives in `gameengine` (not `internal/hat`) so that
// `Seat.Hat` can reference it without creating an import cycle between
// the engine (which needs the interface) and the implementations (which
// need engine types).
//
// Concrete implementations: `github.com/hexdek/hexdek/internal/hat`
//   - `hat.GreedyHat` — baseline heuristic, byte-equivalent to pre-Phase-10
//                       engine behavior (so parity tests don't drift).
//   - `hat.PokerHat`  — HOLD/CALL/RAISE adaptive hat with 7-dim threat score.

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ThreatScoreBasic returns a simple threat score for `target` as seen
// from `source`. Mirrors Python's baseline threat_score (pre-7-dim):
// lowest-life first (inverse), then board power, then hand size. Used
// as the default by GreedyHat's attack-target picker.
//
// This function lives in gameengine because it's tiny and callers in
// both gameengine itself (pickAttackDefender) and the hat package
// can share it without redundant copies.
func ThreatScoreBasic(gs *GameState, sourceSeat int, target *Seat) float64 {
	if target == nil || target.Lost {
		return -1_000_000
	}
	score := 0.0
	// Lower life = more pressurable — inverse of life.
	if target.Life > 0 {
		score += 40.0 / float64(target.Life)
	} else {
		score += 100
	}
	// Board power.
	for _, p := range target.Battlefield {
		if p != nil && p.IsCreature() {
			pw := p.Power()
			if pw > 0 {
				score += float64(pw) * 0.5
			}
		}
	}
	// Hand size — 5+ is scary.
	h := len(target.Hand)
	if h >= 5 {
		score += float64(h-4) * 0.5
	}
	return score
}

// AvailableManaEstimate returns a quick estimate of this seat's
// spendable mana. Uses the typed pool total (authoritative after
// EnsureTypedPool) plus untapped lands and mana artifacts on the
// battlefield. Hats call this as a gating predicate; the engine's real
// cost check during CastSpell is authoritative.
func AvailableManaEstimate(gs *GameState, seat *Seat) int {
	if seat == nil {
		return 0
	}
	EnsureTypedPool(seat)
	n := seat.Mana.Total()
	for _, p := range seat.Battlefield {
		if p == nil || p.Tapped {
			continue
		}
		if p.IsLand() {
			n++
			continue
		}
		// Count untapped mana artifacts.
		if IsArtifactOnly(p) && !ArtifactHasDestructiveCost(p) {
			n += ArtifactManaPotential(p)
		}
	}
	return n
}

// OracleTextLower returns the lowercased oracle text for a card. We
// don't carry oracle_text on the runtime Card directly — instead we
// reconstruct a minimal proxy from the AST's ability raw strings.
// Good enough for string-matching heuristics ("win the game",
// "destroy all", "draw three cards"). Returns "" for nil / AST-less
// cards (tokens).
func OracleTextLower(card *Card) string {
	if card == nil || card.AST == nil {
		return ""
	}
	if card.oracleTextReady {
		return card.OracleTextCache
	}
	var sb strings.Builder
	for _, ab := range card.AST.Abilities {
		switch a := ab.(type) {
		case *gameast.Keyword:
			sb.WriteString(a.Name)
			sb.WriteByte(' ')
		case *gameast.Static:
			sb.WriteString(a.Raw)
			sb.WriteByte(' ')
		case *gameast.Activated:
			sb.WriteString(a.Raw)
			sb.WriteByte(' ')
		case *gameast.Triggered:
			sb.WriteString(a.Raw)
			sb.WriteByte(' ')
		}
	}
	card.OracleTextCache = strings.ToLower(sb.String())
	card.oracleTextReady = true
	return card.OracleTextCache
}

// Activation is a legal activated-ability choice surfaced to a Hat.
// Index is the position in Permanent.Card.AST.Abilities of the chosen
// ability; the engine re-dispatches once the Hat picks one.
type Activation struct {
	Permanent *Permanent
	Ability   int
}

// Hat is the pluggable player-decision protocol. Every method receives
// the GameState as *GameState so the Hat can read ANY public state —
// that's fine because Hats are private per seat; the engine side
// never introspects a Hat.
//
// Mutation: Hats MUST NOT mutate *GameState. The contract is:
//   - methods return a decision the engine applies
//   - ObserveEvent is the only method that updates the Hat's OWN internal
//     state; it is called for every seat on every logged event
//
// Nil-safety: every method must tolerate empty / nil inputs — the
// engine calls them in edge positions (no legal attackers, empty hand,
// dead opponents). A Hat that panics on an empty legal list would hang
// the tournament runner.
type Hat interface {
	// -- Card selection --------------------------------------------------

	// ChooseMulligan returns true to mulligan the opening hand per §103.4.
	// `hand` is the current opener (7 cards first time, 6 on the second
	// call, and so on). The engine applies the mulligan machinery.
	ChooseMulligan(gs *GameState, seatIdx int, hand []*Card) bool

	// ChooseLandToPlay returns which land in hand to play this main phase,
	// or nil to skip the land drop. `lands` is the pre-filtered set of
	// lands currently in hand.
	ChooseLandToPlay(gs *GameState, seatIdx int, lands []*Card) *Card

	// ChooseCastFromHand returns which castable spell to cast next, or nil
	// to pass. `castable` is the engine-filtered set of affordable spells.
	ChooseCastFromHand(gs *GameState, seatIdx int, castable []*Card) *Card

	// ChooseActivation returns which activated ability to fire, or nil
	// to pass. `options` lists legal activations already filtered for
	// cost + timing restrictions.
	ChooseActivation(gs *GameState, seatIdx int, options []Activation) *Activation

	// -- Combat ----------------------------------------------------------

	// ChooseAttackers returns which of the legal attackers to declare.
	// Order of the returned slice is preserved for determinism.
	ChooseAttackers(gs *GameState, seatIdx int, legal []*Permanent) []*Permanent

	// ChooseAttackTarget returns the seat index `attacker` should attack.
	// `legalDefenders` is the set of living opponents (+ any planeswalker
	// seats the engine chose to surface). §506.1.
	ChooseAttackTarget(gs *GameState, seatIdx int, attacker *Permanent, legalDefenders []int) int

	// AssignBlockers maps each attacker to the blocker(s) assigned to it.
	// A missing / empty slice means the attacker is unblocked.
	// Blocker list order matters for damage assignment (§509.1h).
	AssignBlockers(gs *GameState, seatIdx int, attackers []*Permanent) map[*Permanent][]*Permanent

	// -- Stack / priority ------------------------------------------------

	// ChooseResponse returns a *StackItem (counter, removal, etc.) to put
	// on the stack in response, or nil to pass priority. §117.3.
	ChooseResponse(gs *GameState, seatIdx int, stackTop *StackItem) *StackItem

	// -- Targeting / modes -----------------------------------------------

	// ChooseTarget picks one target from `legal`. The engine passes `filter`
	// so the hat can apply filter-specific preferences (e.g. prefer a
	// highest-toughness creature for a buff vs lowest-toughness for a kill).
	ChooseTarget(gs *GameState, seatIdx int, filter gameast.Filter, legal []Target) Target

	// ChooseMode returns the selected mode INDEX for a modal effect.
	// `modes` is the list of options (§601.2c). Returns -1 to choose none.
	ChooseMode(gs *GameState, seatIdx int, modes []gameast.Effect) int

	// -- Commander -------------------------------------------------------

	// ShouldCastCommander returns true to cast the named commander from
	// the command zone this priority window. Engine already checked
	// mana affordability (`tax` is the current §903.8 surcharge).
	ShouldCastCommander(gs *GameState, seatIdx int, commanderName string, tax int) bool

	// ShouldRedirectCommanderZone returns true to apply the §903.9a / b
	// replacement and send the commander to the command zone INSTEAD of
	// `to` (hand / library / graveyard / exile). §903.9a wording: "the
	// OWNER may have that commander go to the command zone instead."
	ShouldRedirectCommanderZone(gs *GameState, seatIdx int, commander *Card, to string) bool

	// -- Replacements / discard ------------------------------------------

	// OrderReplacements returns the input slice re-ordered per the
	// affected player's choice. §616.1 says the AFFECTED player orders
	// the effects if more than one category-equivalent replacement is
	// applicable to the same event.
	OrderReplacements(gs *GameState, seatIdx int, candidates []*ReplacementEffect) []*ReplacementEffect

	// ChooseDiscard returns which N cards to discard. §701.8.
	ChooseDiscard(gs *GameState, seatIdx int, hand []*Card, n int) []*Card

	// -- Trigger ordering (§603.3b APNAP) --------------------------------

	// OrderTriggers returns the input slice of triggered-ability stack
	// items reordered per the controller's choice. CR §603.3b says when
	// multiple triggered abilities controlled by the same player go on
	// the stack simultaneously, that player chooses the order. The engine
	// calls this AFTER APNAP grouping — each call receives only the
	// triggers controlled by seatIdx. The returned order determines
	// stack push order (first element pushed first = resolves last).
	OrderTriggers(gs *GameState, seatIdx int, triggers []*StackItem) []*StackItem

	// -- X-cost announcement -----------------------------------------------

	// ChooseX returns the value of X for a spell with X in its mana cost.
	// Called by CastSpell when the card's mana cost contains X (CR §107.3).
	// `availableMana` is the mana the caster has after subtracting the non-X
	// portion of the cost. The Hat must return a value in [0, availableMana].
	ChooseX(gs *GameState, seatIdx int, card *Card, availableMana int) int

	// -- London mulligan bottom-card selection ----------------------------

	// ChooseBottomCards returns which N cards from hand to put on the
	// bottom of the library during the London mulligan (CR §103.5).
	// `count` is the number of cards to bottom (= number of mulligans
	// taken so far). The returned slice must have exactly `count` cards
	// from `hand`.
	ChooseBottomCards(gs *GameState, seatIdx int, hand []*Card, count int) []*Card

	// -- Scry / Surveil (§701.18 / §701.46) ----------------------------

	// ChooseScry is called when the player scries N cards. `cards` are
	// the top N cards of the library. The hat returns two slices:
	//   - `top`: cards to keep on top of the library, in order (first
	//     element will be the new top card)
	//   - `bottom`: cards to put on the bottom of the library, in order
	// Every card in `cards` must appear in exactly one of the two slices.
	ChooseScry(gs *GameState, seatIdx int, cards []*Card) (top []*Card, bottom []*Card)

	// ChooseSurveil is called when the player surveils N cards (§701.46).
	// `cards` are the top N cards of the library. The hat returns:
	//   - `graveyard`: cards to put into the graveyard
	//   - `top`: cards to keep on top of the library, in order
	// Every card in `cards` must appear in exactly one of the two slices.
	ChooseSurveil(gs *GameState, seatIdx int, cards []*Card) (graveyard []*Card, top []*Card)

	// -- Put-back (Brainstorm-style) --------------------------------------

	// ChoosePutBack returns `count` cards from `hand` to put back on top
	// of the library, in order (first element = new top card). Used by
	// Brainstorm ("draw 3, put 2 back") and similar effects. The returned
	// slice must have exactly `count` cards chosen from `hand`.
	ChoosePutBack(gs *GameState, seatIdx int, hand []*Card, count int) []*Card

	// -- Concession (conviction-based) -----------------------------------

	// ShouldConcede returns true if this hat wants to concede the game.
	// Called at the start of each turn. CR §104.3a: "A player can concede
	// the game at any time." Hats that don't track conviction return false.
	ShouldConcede(gs *GameState, seatIdx int) bool

	// -- Event observation (adaptive hats) -------------------------------

	// ObserveEvent is called for EVERY seat on EVERY logged event so
	// adaptive hats (e.g. PokerHat) can update internal state. The engine
	// broadcasts to every Hat; hats may filter by `event.Seat == seatIdx`
	// to look at their own actions only.
	//
	// MUST NOT mutate *GameState.
	ObserveEvent(gs *GameState, seatIdx int, event *Event)
}

// ConcedeGame marks a seat as having conceded. CR §104.3a.
func ConcedeGame(gs *GameState, seatIdx int) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || seat.Lost || seat.LeftGame {
		return
	}
	seat.Lost = true
	seat.LossReason = "concession"
	gs.LogEvent(Event{
		Kind:   "concession",
		Seat:   seatIdx,
		Target: -1,
		Details: map[string]interface{}{
			"rule":   "104.3a",
			"reason": "conviction_concession",
			"turn":   gs.Turn,
		},
	})
	HandleSeatElimination(gs, seatIdx)
}

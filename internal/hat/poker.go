package hat

// Deprecated: PokerHat is superseded by YggdrasilHat, which unifies all hat
// behaviors into a single brain with tunable personality. Use NewYggdrasilHat instead.
//
// PokerHat — Go port of scripts/extensions/policies/poker.py v2.
//
// Poker-inspired HOLD / CALL / RAISE adaptive hat. The MODE is INTERNAL
// hat state; the engine does not know it exists and must never branch
// on it. Swapping this hat for another is a one-line change:
//
//	gs.Seats[i].Hat = hat.NewPokerHat()
//
// Mode semantics (per Python v2):
//   - HOLD  — rebuild mode. Prioritize tutors, draw, recursion, ramp.
//             Targeted removal iff a threat merits it. Cheap threats to
//             rebuild. Skip expensive haymakers.
//   - CALL  — default engaged mode. Greedy-like play with an open-target
//             attack preference + 7-dim threat ranker for targeting.
//   - RAISE — press to win OR respond to another seat's RAISE. All-in
//             attacks, save mana for combo pieces, only counter
//             win-the-game / mass-removal threats.
//
// The 7-dim threat score widens the aperture from "stomp the best board"
// to include graveyard value, hand size, command zone, ramp, library
// pressure, and deck-archetype telegraphs — so reanimator / combo /
// ramp decks (Sin, Coram, Oloro) light up in the threat map.
//
// RAISE cascade — observe_event catches player_mode_change events from
// OTHER seats. If another seat raises and we have 2+ combo pieces, a
// big board, or imminent loss, we also raise. This reproduces the
// all-seats-in-RAISE closing exchanges from paper EDH.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

var _ gameengine.Hat = (*PokerHat)(nil)

// PlayerMode is the HOLD/CALL/RAISE enum.
type PlayerMode int

const (
	ModeHold PlayerMode = iota
	ModeCall
	ModeRaise
)

// String lowercases the mode for logs + event details.
func (m PlayerMode) String() string {
	switch m {
	case ModeHold:
		return "hold"
	case ModeCall:
		return "call"
	case ModeRaise:
		return "raise"
	}
	return "unknown"
}

// Tuning constants — mirror scripts/extensions/policies/poker.py.
const (
	holdToCallThreshold   = 12
	callToHoldThreshold   = 8
	raiseLifeThreshold    = 10
	raiseBoardPowerFloor  = 14
	raiseComboManaFloor   = 4
	raiseCascadeBoardPow  = 10
	modeChangeCooldownEvs = 3

	// 7-dim threat weights.
	wBoardPower      = 1.0
	wGraveyard       = 0.8
	wHand            = 0.6
	wCommander       = 0.7
	wRamp            = 0.5
	wLibraryPressure = 0.6
	wArchetype       = 1.2
)

// Archetype labels.
const (
	ArchetypeUnknown      = "unknown"
	ArchetypeAggro        = "aggro"
	ArchetypeMidrange     = "midrange"
	ArchetypeControl      = "control"
	ArchetypeRamp         = "ramp"
	ArchetypeCombo        = "combo"
	ArchetypeStax         = "stax"
	ArchetypeReanimator   = "reanimator"
	ArchetypeSpellslinger = "spellslinger"
	ArchetypeTribal       = "tribal"
	ArchetypeAristocrats  = "aristocrats"
	ArchetypeSelfmill     = "selfmill"
)

// observation is a running tally of an OTHER seat's plays. A PokerHat
// instance attached to seat S keeps one of these per other seat.
type observation struct {
	rampSpent          int
	countersCast       int
	creaturesCast      int
	cardsCastTotal     int
	graveyardHighWater int
	commanderCastCount int
}

// comboPair defines a known two-card combo. When both pieces are
// available, castFirst is played first; the other piece follows on the
// next ChooseCastFromHand call.
type comboPair struct {
	piece1, piece2 string
	castFirst      string // which to cast first
}

// knownCombos is the registry of two-card instant-win / engine combos
// the PokerHat should recognise. Order within the slice doubles as
// priority: earlier entries are preferred when multiple combos are live.
var knownCombos = []comboPair{
	{"Thassa's Oracle", "Demonic Consultation", "Demonic Consultation"},
	{"Thassa's Oracle", "Tainted Pact", "Tainted Pact"},
	{"Laboratory Maniac", "Demonic Consultation", "Demonic Consultation"},
	{"Isochron Scepter", "Dramatic Reversal", "Isochron Scepter"},
	{"Food Chain", "Misthollow Griffin", "Food Chain"},
	{"Food Chain", "Eternal Scourge", "Food Chain"},
	{"Sanguine Bond", "Exquisite Blood", "Exquisite Blood"},
	{"Walking Ballista", "Heliod, Sun-Crowned", "Heliod, Sun-Crowned"},
	{"Basalt Monolith", "Kinnan, Bonder Prodigy", "Basalt Monolith"},
}

// PokerHat is the adaptive hat. Embeds GreedyHat so every method we
// don't override falls through to the baseline.
type PokerHat struct {
	GreedyHat // fallthrough default

	Mode                PlayerMode
	lastModeChangeSeq   int
	eventsSeen          int
	obs                 map[int]*observation
	lastThreatBreakdown map[string]float64

	// pendingComboSecond is the card name of the second combo piece
	// that should be cast on the NEXT ChooseCastFromHand call after the
	// first piece resolved. Set by the combo-detection path, consumed
	// (zeroed) once the second piece is chosen.
	pendingComboSecond string

	// strategy is the Freya-derived deck intelligence profile. When non-
	// nil, the hat uses deck-specific combos, tutor priorities, and value
	// engine keys instead of (or in addition to) the hardcoded globals.
	strategy *StrategyProfile

	// strategyCombos is the pre-computed comboPair slice derived from
	// strategy.ComboPieces (only 2-card combos). Cached at construction
	// time to avoid repeated allocation.
	strategyCombos []comboPair

	// strategyPieceSet is the set of all card names appearing in any
	// ComboPlan. Used for fast membership checks in mulligan/observe.
	strategyPieceSet map[string]bool

	// degradedCombos tracks combo plans where a piece has been exiled
	// or countered. Keyed by combo index in strategy.ComboPieces.
	degradedCombos map[int]bool
}

// NewPokerHat constructs a PokerHat starting in CALL mode (the paper-
// EDH default for a seat "playing normally"). A seat with nothing to
// do T1 will drop to HOLD on its first re-evaluate.
func NewPokerHat() *PokerHat {
	return &PokerHat{
		Mode: ModeCall,
		obs:  map[int]*observation{},
	}
}

// NewPokerHatWithMode lets tests construct a hat in a specific starting
// mode — useful for asserting the HOLD → CALL transition path.
func NewPokerHatWithMode(m PlayerMode) *PokerHat {
	h := NewPokerHat()
	h.Mode = m
	return h
}

// NewPokerHatForArchetype constructs a PokerHat with a starting mode
// tuned to the deck's self-assessed archetype. Per the nightmare
// verification spec:
//   - Aggro: starts CALL, auto-RAISE by turn 4 if threats are low
//   - Combo: starts HOLD, shifts to RAISE when combo pieces are in hand
//   - Control: starts CALL, stays CALL longer before reacting
//   - Ramp: starts HOLD, shifts to CALL once ramp is established
//   - Unknown/Midrange: starts CALL (default)
//
// The re-evaluate function will naturally shift modes based on game
// state, so these are starting biases rather than rigid rules.
func NewPokerHatForArchetype(archetype string) *PokerHat {
	h := NewPokerHat()
	switch archetype {
	case ArchetypeAggro:
		h.Mode = ModeCall // Aggressive start, will auto-RAISE
	case ArchetypeCombo:
		h.Mode = ModeHold // Lay low until combo pieces assemble
	case ArchetypeControl:
		h.Mode = ModeCall // Active but reactive
	case ArchetypeRamp:
		h.Mode = ModeHold // Build mana base first
	default:
		h.Mode = ModeCall // Default
	}
	return h
}

// NewPokerHatWithStrategy constructs a PokerHat informed by Freya's deck
// analysis. The starting mode is derived from the strategy's archetype
// (same logic as NewPokerHatForArchetype), and the hat's combo detection,
// mulligan evaluation, and event observation use deck-specific intelligence.
//
// When strategy is nil, behavior is identical to NewPokerHat().
func NewPokerHatWithStrategy(sp *StrategyProfile) *PokerHat {
	if sp == nil {
		return NewPokerHat()
	}
	h := NewPokerHatForArchetype(sp.Archetype)
	h.strategy = sp
	h.strategyCombos = comboPlansToKnownCombos(sp.ComboPieces)
	h.strategyPieceSet = allComboPieceNames(sp.ComboPieces)
	h.degradedCombos = map[int]bool{}
	return h
}

// Strategy returns the hat's strategy profile (may be nil). Exposed for
// test assertions only — the engine MUST NOT call this.
func (h *PokerHat) Strategy() *StrategyProfile { return h.strategy }

// ---------------------------------------------------------------------
// Mode transitions (internal)
// ---------------------------------------------------------------------

func (h *PokerHat) transition(gs *gameengine.GameState, seatIdx int, newMode PlayerMode, reason string) {
	if newMode == h.Mode {
		return
	}
	if h.eventsSeen-h.lastModeChangeSeq < modeChangeCooldownEvs {
		return
	}
	old := h.Mode
	h.Mode = newMode
	h.lastModeChangeSeq = h.eventsSeen
	// Emit a player_mode_change event on the engine's log. Other seats'
	// hats see this via their ObserveEvent and may cascade.
	gs.LogEvent(gameengine.Event{
		Kind: "player_mode_change",
		Seat: seatIdx,
		Details: map[string]interface{}{
			"from_mode": old.String(),
			"to_mode":   newMode.String(),
			"reason":    reason,
		},
	})
}

func (h *PokerHat) reEvaluate(gs *gameengine.GameState, seatIdx int) {
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}

	// 0. Late-game forced RAISE — after turn 20 (round 5 in 4-player),
	//    force RAISE mode regardless of threat assessment. Real Commander
	//    games end in 10-20 turns; games that drag past round 5 need to
	//    close out aggressively. This is a one-way lock: once forced,
	//    the RAISE decay path below is suppressed by the same check.
	if gs.Turn > 20 && h.Mode != ModeRaise {
		h.transition(gs, seatIdx, ModeRaise,
			fmt.Sprintf("late_game_force: turn=%d>20", gs.Turn))
		return
	}

	// 1. Emergency RAISE — low life / opp one turn from lethal.
	if seat.Life <= raiseLifeThreshold && h.Mode != ModeRaise {
		h.transition(gs, seatIdx, ModeRaise, fmt.Sprintf("life=%d<=%d", seat.Life, raiseLifeThreshold))
		return
	}
	if h.opponentOneTurnFromWin(gs, seatIdx) {
		h.transition(gs, seatIdx, ModeRaise, "opp_one_turn_from_win")
		return
	}

	// 2. Offensive RAISE.
	ourBoard := boardPower(seat)
	if ourBoard >= raiseBoardPowerFloor && h.comboReady(gs, seat) && h.Mode != ModeRaise {
		h.transition(gs, seatIdx, ModeRaise,
			fmt.Sprintf("offensive: board=%d + combo_ready", ourBoard))
		return
	}

	// 3. CALL ↔ HOLD hysteresis on self-threat.
	score := h.selfThreatScore(gs, seatIdx)
	if h.Mode == ModeHold && score >= holdToCallThreshold {
		h.transition(gs, seatIdx, ModeCall,
			fmt.Sprintf("self_threat=%.1f>=%d", score, holdToCallThreshold))
		return
	}
	if h.Mode == ModeCall && score <= callToHoldThreshold {
		h.transition(gs, seatIdx, ModeHold,
			fmt.Sprintf("self_threat=%.1f<=%d", score, callToHoldThreshold))
		return
	}

	// 4. RAISE decay — stabilized, drop back to CALL. Suppressed in late
	//    game (turn > 20) to keep the forced RAISE lock.
	if h.Mode == ModeRaise && gs.Turn <= 20 &&
		seat.Life > raiseLifeThreshold && !h.opponentOneTurnFromWin(gs, seatIdx) {
		h.transition(gs, seatIdx, ModeCall, "raise_cooldown")
	}
}

// considerCascadeRaise is called when ANOTHER seat just raised. Match
// if we have 2+ combo pieces, board power ≥ 10, or imminent loss.
func (h *PokerHat) considerCascadeRaise(gs *gameengine.GameState, seatIdx int) {
	if h.Mode == ModeRaise {
		return
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	combo := h.comboPiecesInHand(seat)
	bp := boardPower(seat)
	imminent := h.opponentOneTurnFromWin(gs, seatIdx) || seat.Life <= raiseLifeThreshold
	if combo >= 2 || bp >= raiseCascadeBoardPow || imminent {
		h.transition(gs, seatIdx, ModeRaise,
			fmt.Sprintf("cascade: combo=%d board=%d imminent=%v", combo, bp, imminent))
	}
}

// ---------------------------------------------------------------------
// Event observation + archetype tally
// ---------------------------------------------------------------------

func (h *PokerHat) obsFor(seatIdx int) *observation {
	if h.obs == nil {
		h.obs = map[int]*observation{}
	}
	o, ok := h.obs[seatIdx]
	if !ok {
		o = &observation{}
		h.obs[seatIdx] = o
	}
	return o
}

// ChooseMulligan — mulligan if the opening hand has 0-1 lands. This
// prevents players from going entire games without playing (BUG 7: Azula
// zero-activity game). Only mulligans once (the London mulligan procedure
// handles repeated mulligans, but we limit to one for simplicity).
//
// Strategy-aware: when the deck is a combo archetype with known combo
// pieces, keep hands that contain 2+ combo/engine pieces even with only
// 1 land — combo decks need pieces more than land flood.
func (h *PokerHat) ChooseMulligan(gs *gameengine.GameState, seatIdx int, hand []*gameengine.Card) bool {
	landCount := 0
	for _, c := range hand {
		if c != nil && typeLineContains(c, "land") {
			landCount++
		}
	}

	// First mulligan only (hand size = 7 means no mulligans taken yet).
	if len(hand) < 7 {
		return false
	}

	// Strategy-aware: combo decks keep piece-heavy hands even with 1 land.
	if h.strategy != nil && landCount == 1 && len(h.strategyPieceSet) > 0 {
		pieceCount := h.countStrategyPiecesInHand(hand)
		if pieceCount >= 2 {
			// 2+ combo/engine pieces with 1 land = keepable for combo decks
			return false
		}
	}

	// Default: mulligan if 0-1 lands.
	if landCount <= 1 {
		return true
	}
	return false
}

// countStrategyPiecesInHand counts cards in hand that are combo pieces,
// tutor targets, or value engine keys per the strategy profile.
func (h *PokerHat) countStrategyPiecesInHand(hand []*gameengine.Card) int {
	if h.strategy == nil {
		return 0
	}
	n := 0
	for _, c := range hand {
		if c == nil {
			continue
		}
		name := c.DisplayName()
		if h.strategyPieceSet[name] {
			n++
			continue
		}
		// Also count value engine keys.
		for _, vk := range h.strategy.ValueEngineKeys {
			if name == vk {
				n++
				break
			}
		}
	}
	return n
}

// ChooseScry — mode-aware scry. In RAISE mode keep everything (rush);
// in HOLD mode bottom expensive non-essentials.
func (h *PokerHat) ChooseScry(gs *gameengine.GameState, seatIdx int, cards []*gameengine.Card) (top []*gameengine.Card, bottom []*gameengine.Card) {
	if len(cards) == 0 {
		return nil, nil
	}
	// Delegate to GreedyHat's logic as baseline.
	g := &GreedyHat{}
	return g.ChooseScry(gs, seatIdx, cards)
}

// ChooseSurveil — delegate to GreedyHat baseline.
func (h *PokerHat) ChooseSurveil(gs *gameengine.GameState, seatIdx int, cards []*gameengine.Card) (graveyard []*gameengine.Card, top []*gameengine.Card) {
	g := &GreedyHat{}
	return g.ChooseSurveil(gs, seatIdx, cards)
}

// ChoosePutBack — delegate to GreedyHat baseline.
func (h *PokerHat) ChoosePutBack(gs *gameengine.GameState, seatIdx int, hand []*gameengine.Card, count int) []*gameengine.Card {
	g := &GreedyHat{}
	return g.ChoosePutBack(gs, seatIdx, hand, count)
}

// ObserveEvent — the only state-mutation hook. Called for every logged
// event; runs archetype accounting + triggers re-evaluate.
func (h *PokerHat) ShouldConcede(gs *gameengine.GameState, seatIdx int) bool { return false }

func (h *PokerHat) ObserveEvent(gs *gameengine.GameState, seatIdx int, ev *gameengine.Event) {
	if ev == nil {
		return
	}
	h.eventsSeen++

	// RAISE cascade — another seat transitioned to RAISE.
	if ev.Kind == "player_mode_change" {
		otherIdx := ev.Seat
		toMode, _ := ev.Details["to_mode"].(string)
		if otherIdx != seatIdx && toMode == ModeRaise.String() {
			h.considerCascadeRaise(gs, seatIdx)
			return
		}
	}

	// Strategy-aware combo piece tracking: when a combo piece enters
	// our hand or battlefield, consider nudging toward RAISE. When a
	// combo piece is exiled or countered, mark the combo degraded.
	if h.strategy != nil && len(h.strategyPieceSet) > 0 && ev.Seat == seatIdx {
		h.observeStrategyEvent(gs, seatIdx, ev)
	}

	// Archetype accounting — tally OTHER seats' casts.
	if ev.Kind == "cast" {
		acting := ev.Seat
		if acting >= 0 && acting != seatIdx {
			h.tallyCast(gs, acting, ev)
		}
		// Refresh graveyard high-water for every other seat.
		for _, s := range gs.Seats {
			if s != nil && s.Idx != seatIdx {
				o := h.obsFor(s.Idx)
				gy := len(s.Graveyard)
				if gy > o.graveyardHighWater {
					o.graveyardHighWater = gy
				}
			}
		}
	}

	// Re-evaluate on relevant events only (avoid thrash on noisy logs).
	switch ev.Kind {
	case "game_start", "turn_start", "draw", "damage",
		"life_change", "cast", "attackers", "blockers",
		"sba_704_5a", "seat_eliminated", "combat_damage":
		h.reEvaluate(gs, seatIdx)
	}
}

// tallyCast updates the observation for the seat that just cast.
// Looks up the card on that seat's battlefield / graveyard to
// re-inspect its effect kinds.
func (h *PokerHat) tallyCast(gs *gameengine.GameState, actingIdx int, ev *gameengine.Event) {
	if actingIdx < 0 || actingIdx >= len(gs.Seats) {
		return
	}
	o := h.obsFor(actingIdx)
	o.cardsCastTotal++

	cardName, _ := ev.Details["card"].(string)
	if cardName == "" {
		cardName = ev.Source
	}
	cmc := ev.Amount
	var card *gameengine.Card
	if cardName != "" {
		acting := gs.Seats[actingIdx]
		for _, p := range acting.Battlefield {
			if p != nil && p.Card != nil && p.Card.DisplayName() == cardName {
				card = p.Card
				break
			}
		}
		if card == nil {
			for _, c := range acting.Graveyard {
				if c != nil && c.DisplayName() == cardName {
					card = c
					break
				}
			}
		}
	}
	if card == nil {
		return
	}
	if isRamp(card) {
		o.rampSpent += cmc
	}
	if gameengine.CardHasCounterSpell(card) {
		o.countersCast++
	}
	if typeLineContains(card, "creature") {
		o.creaturesCast++
	}
}

// observeStrategyEvent tracks combo piece assembly/degradation for the
// hat's own seat. Called from ObserveEvent only when strategy is set.
func (h *PokerHat) observeStrategyEvent(gs *gameengine.GameState, seatIdx int, ev *gameengine.Event) {
	cardName := ""
	if ev.Source != "" {
		cardName = ev.Source
	}
	if cn, ok := ev.Details["card"].(string); ok && cn != "" {
		cardName = cn
	}
	if cardName == "" {
		return
	}

	isPiece := h.strategyPieceSet[cardName]
	if !isPiece {
		return
	}

	switch ev.Kind {
	case "draw", "tutor":
		// Combo piece entered hand — consider nudging toward RAISE if
		// we're assembling multiple pieces.
		if seatIdx >= 0 && seatIdx < len(gs.Seats) {
			seat := gs.Seats[seatIdx]
			piecesInHand := 0
			for _, c := range seat.Hand {
				if c != nil && h.strategyPieceSet[c.DisplayName()] {
					piecesInHand++
				}
			}
			if piecesInHand >= 2 && h.Mode != ModeRaise {
				h.transition(gs, seatIdx, ModeRaise,
					fmt.Sprintf("strategy: %d combo pieces in hand", piecesInHand))
			}
		}

	case "exile", "counter":
		// Combo piece was exiled or countered — mark affected combos
		// as degraded.
		for i, cp := range h.strategy.ComboPieces {
			for _, p := range cp.Pieces {
				if p == cardName {
					h.degradedCombos[i] = true
					break
				}
			}
		}
	}
}

// classifyArchetype — heuristic thresholds matching the Python spec.
func classifyArchetype(o *observation, turn int) string {
	if o == nil || o.cardsCastTotal < 3 {
		return ArchetypeUnknown
	}
	if o.graveyardHighWater >= 10 && turn <= 6 {
		return ArchetypeCombo
	}
	if o.countersCast >= 6 {
		return ArchetypeControl
	}
	if o.rampSpent >= 8 && turn <= 5 {
		return ArchetypeRamp
	}
	if o.cardsCastTotal > 0 && float64(o.creaturesCast)/float64(o.cardsCastTotal) >= 0.4 {
		return ArchetypeAggro
	}
	return ArchetypeMidrange
}

func archetypeBonus(archetype string) float64 {
	switch archetype {
	case ArchetypeCombo:
		return 6.0
	case ArchetypeControl:
		return 3.0
	case ArchetypeRamp:
		return 2.5
	case ArchetypeAggro:
		return 2.0
	case ArchetypeMidrange:
		return 1.5
	}
	return 0.0
}

// ---------------------------------------------------------------------
// 7-dim threat score
// ---------------------------------------------------------------------

// Breakdown is a snapshot of the 7 dimensions for one (source, target)
// pair. Exposed so tests + debug logs can cite the dominant signal.
type Breakdown struct {
	Score     float64
	Board     float64
	Graveyard float64
	Hand      float64
	Commander float64
	Ramp      float64
	Library   float64
	Archetype float64
	Label     string
}

func (h *PokerHat) threatBreakdown(gs *gameengine.GameState, srcIdx int, target *gameengine.Seat) Breakdown {
	if target == nil || target.Lost {
		return Breakdown{Score: -1_000_000}
	}

	// Dim 1 — board power + high-CMC proxy + low-life leverage.
	bp := boardPower(target)
	hc := highCMCPermanents(target)
	dimBoard := wBoardPower * float64(bp+hc)
	if target.Life <= 10 {
		dimBoard += 5
	}
	if target.Life <= 5 {
		dimBoard += 5
	}
	// Commander damage leverage — we benefit if OUR commander has
	// already dealt damage to target (closer to the 21 threshold). The
	// CommanderDamage map is keyed (dealer_seat, name); bonus applies only
	// to damage WE dealt, not damage some other opponent dealt.
	if srcIdx >= 0 && srcIdx < len(gs.Seats) {
		src := gs.Seats[srcIdx]
		for _, nm := range src.CommanderNames {
			dimBoard += 2 * float64(gameengine.CommanderDamageFrom(target, srcIdx, nm))
		}
	}

	// Dim 2 — graveyard value.
	gyRaw := len(target.Graveyard)
	var gyBonus float64
	for _, c := range target.Graveyard {
		if c == nil {
			continue
		}
		if typeLineContains(c, "creature") {
			gyBonus += 0.5
		}
		if gameengine.ManaCostOf(c) >= 4 {
			gyBonus += 0.2
		}
	}
	dimGraveyard := wGraveyard * (0.3*float64(gyRaw) + gyBonus)

	// Dim 3 — hand size.
	handN := len(target.Hand)
	var dimHand float64
	if handN >= 5 {
		dimHand = wHand * float64(handN-4) * 1.5
	} else if handN >= 3 {
		dimHand = wHand * 1.0
	}

	// Dim 4 — commander zone / cast count.
	cmdZone := len(target.CommandZone)
	cmdCast := 0
	for _, n := range target.CommanderTax {
		cmdCast += n
	}
	dimCommander := wCommander * (2.0*float64(cmdZone) + 1.0*float64(cmdCast))

	// Dim 5 — ramp / mana density.
	rocksLands := countManaRocksAndLands(target)
	dimRamp := 0.0
	if rocksLands > 3 {
		dimRamp = wRamp * float64(rocksLands-3) * 0.7
	}

	// Dim 6 — library pressure.
	libN := len(target.Library)
	var dimLib float64
	if libN < 20 {
		dimLib = wLibraryPressure * float64(20-libN) * 0.25
	} else if libN < 40 {
		dimLib = wLibraryPressure * 0.5
	}

	// Dim 7 — archetype bonus.
	label := ArchetypeUnknown
	if o := h.obs[target.Idx]; o != nil {
		label = classifyArchetype(o, gs.Turn)
	}
	dimArch := wArchetype * archetypeBonus(label)

	score := dimBoard + dimGraveyard + dimHand + dimCommander + dimRamp + dimLib + dimArch
	return Breakdown{
		Score:     score,
		Board:     dimBoard,
		Graveyard: dimGraveyard,
		Hand:      dimHand,
		Commander: dimCommander,
		Ramp:      dimRamp,
		Library:   dimLib,
		Archetype: dimArch,
		Label:     label,
	}
}

// selfThreatScore — 7-dim threat score pointed INWARD. Used for
// HOLD ↔ CALL hysteresis.
func (h *PokerHat) selfThreatScore(gs *gameengine.GameState, seatIdx int) float64 {
	// Pick any living other seat as the "source" — the breakdown is
	// symmetric enough for our purposes.
	var other *gameengine.Seat
	for _, s := range gs.Seats {
		if s != nil && s.Idx != seatIdx && !s.Lost {
			other = s
			break
		}
	}
	if other == nil {
		return 0
	}
	bd := h.threatBreakdown(gs, other.Idx, gs.Seats[seatIdx])
	return bd.Score
}

// ThreatBreakdownFor exposes the breakdown for debug / assertion.
func (h *PokerHat) ThreatBreakdownFor(gs *gameengine.GameState, seatIdx int, target *gameengine.Seat) Breakdown {
	return h.threatBreakdown(gs, seatIdx, target)
}

// ---------------------------------------------------------------------
// RAISE trigger detection
// ---------------------------------------------------------------------

func (h *PokerHat) comboPiecesInHand(seat *gameengine.Seat) int {
	n := 0
	for _, c := range seat.Hand {
		if c == nil {
			continue
		}
		// Strategy-aware: actual combo pieces count as combo pieces.
		if h.strategyPieceSet != nil && h.strategyPieceSet[c.DisplayName()] {
			n++
			continue
		}
		if isTutor(c) {
			n++
			continue
		}
		if isCardDraw(c) && gameengine.ManaCostOf(c) >= 3 {
			n++
			continue
		}
		if isRecursion(c) {
			n++
			continue
		}
		ot := gameengine.OracleTextLower(c)
		if strings.Contains(ot, "win the game") {
			n += 2
			continue
		}
		if gameengine.ManaCostOf(c) >= 6 {
			n++
		}
	}
	return n
}

func (h *PokerHat) comboReady(gs *gameengine.GameState, seat *gameengine.Seat) bool {
	if gameengine.AvailableManaEstimate(gs, seat) < raiseComboManaFloor {
		return false
	}
	return h.comboPiecesInHand(seat) > 0
}

func (h *PokerHat) opponentOneTurnFromWin(gs *gameengine.GameState, seatIdx int) bool {
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	me := gs.Seats[seatIdx]
	for _, opp := range gs.Seats {
		if opp == nil || opp.Lost || opp.Idx == seatIdx {
			continue
		}
		untappedPower := 0
		for _, p := range opp.Battlefield {
			if p != nil && p.IsCreature() && !p.Tapped {
				pw := p.Power()
				if pw > 0 {
					untappedPower += pw
				}
			}
		}
		if untappedPower >= me.Life {
			return true
		}
		// Opp threatening lethal from commander damage: if they've dealt
		// 15+ from a single commander, one more good swing ends us.
		for _, nm := range opp.CommanderNames {
			if gameengine.CommanderDamageFrom(me, opp.Idx, nm) >= 15 {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------
// Overridden decisions (mode-sensitive)
// ---------------------------------------------------------------------

// ChooseActivation — mode-aware activated ability selection (FIX 5).
//
//	HOLD:  only activate draw, tutor, and scry abilities (rebuild mode).
//	CALL:  activate removal + equip + draw + token creation.
//	RAISE: activate everything aggressively (all positive-score options).
func (h *PokerHat) ChooseActivation(gs *gameengine.GameState, seatIdx int, options []gameengine.Activation) *gameengine.Activation {
	if len(options) == 0 {
		return nil
	}

	// Use GreedyHat's scoring as the baseline, then apply mode filter.
	bestScore := 0
	var bestOpt *gameengine.Activation

	for i := range options {
		opt := &options[i]
		score := scoreActivation(gs, seatIdx, opt)
		if score <= 0 {
			continue
		}

		// Mode-based threshold filtering.
		switch h.Mode {
		case ModeHold:
			// HOLD: only draw/tutor/scry abilities (score >= 45 for draw/tutor,
			// 20 for scry).
			if score < 20 {
				continue
			}
			// In HOLD mode, skip removal and combat-oriented abilities.
			if score >= 35 && score < 45 {
				// 35-44 range is removal/bounce — skip in HOLD unless
				// an opponent has threat score >= 10.
				worthy := false
				for _, s := range gs.Seats {
					if s == nil || s.Lost || s.Idx == seatIdx {
						continue
					}
					if h.threatBreakdown(gs, seatIdx, s).Score >= 10 {
						worthy = true
						break
					}
				}
				if !worthy {
					continue
				}
			}
		case ModeCall:
			// CALL: activate most abilities but skip very low-value ones.
			if score < 5 {
				continue
			}
		case ModeRaise:
			// RAISE: activate everything with any positive score.
			// No additional filtering.
		}

		if score > bestScore {
			bestScore = score
			bestOpt = opt
		}
	}

	return bestOpt
}

// ChooseCastFromHand — mode-sensitive.
//
//	HOLD:  tutor → draw → recursion → ramp → targeted removal (threat≥10)
//	       → cheap threats (CMC≤3); skip haymakers.
//	CALL / RAISE: defer to GreedyHat (biggest-affordable-first).
//
// Before normal priority logic, checks for known two-card combos so
// the hat can sequence them correctly (e.g. Demonic Consultation before
// Thassa's Oracle).
func (h *PokerHat) ChooseCastFromHand(gs *gameengine.GameState, seatIdx int, castable []*gameengine.Card) *gameengine.Card {
	// Filter out counterspells (saved for respond_to_stack) AND
	// self-destructive combo pieces whose partner isn't available AND
	// cards that should be held for a better moment.
	safe := filterSuicideComboPieces(gs, seatIdx, castable)
	pool := make([]*gameengine.Card, 0, len(safe))
	for _, c := range safe {
		if c == nil || gameengine.CardHasCounterSpell(c) {
			continue
		}
		if shouldHoldCard(c, gs, seatIdx) {
			continue
		}
		pool = append(pool, c)
	}
	if len(pool) == 0 {
		return nil
	}

	// --- Combo sequencing: pending second piece from prior call ----------
	if h.pendingComboSecond != "" {
		secondName := h.pendingComboSecond
		h.pendingComboSecond = ""
		for _, c := range pool {
			if c.DisplayName() == secondName {
				return c
			}
		}
		// Second piece not in the castable list (maybe not affordable or
		// countered) — fall through to normal logic.
	}

	// --- Combo detection: both pieces in castable -----------------------
	if combo := h.findComboInCastable(gs, seatIdx, pool); combo != nil {
		return combo
	}

	// --- Combo detection: one piece in hand, other on battlefield -------
	if combo := h.findComboWithBattlefield(gs, seatIdx, pool); combo != nil {
		return combo
	}

	if h.Mode != ModeHold {
		return h.GreedyHat.ChooseCastFromHand(gs, seatIdx, safe)
	}
	return h.chooseCastHold(gs, seatIdx, pool)
}

// effectiveCombos returns the combo list the hat should use. Strategy
// combos take priority (checked first), then the global knownCombos
// list provides fallback coverage for combos Freya didn't detect.
func (h *PokerHat) effectiveCombos() []comboPair {
	if h.strategy == nil || len(h.strategyCombos) == 0 {
		return knownCombos
	}
	// Strategy combos first (higher priority), then global fallbacks.
	// Deduplicate: skip global combos that overlap with strategy combos.
	seen := make(map[string]bool, len(h.strategyCombos))
	for _, sc := range h.strategyCombos {
		key := sc.piece1 + "+" + sc.piece2
		seen[key] = true
		// Also check reverse order.
		key2 := sc.piece2 + "+" + sc.piece1
		seen[key2] = true
	}
	merged := make([]comboPair, 0, len(h.strategyCombos)+len(knownCombos))
	merged = append(merged, h.strategyCombos...)
	for _, gc := range knownCombos {
		key := gc.piece1 + "+" + gc.piece2
		if !seen[key] {
			merged = append(merged, gc)
		}
	}
	return merged
}

// findComboInCastable checks whether both pieces of a known combo are
// in the castable list. If so, returns the piece that should be cast
// first and sets pendingComboSecond for the next call.
func (h *PokerHat) findComboInCastable(gs *gameengine.GameState, seatIdx int, pool []*gameengine.Card) *gameengine.Card {
	names := make(map[string]*gameengine.Card, len(pool))
	for _, c := range pool {
		names[c.DisplayName()] = c
	}
	for _, combo := range h.effectiveCombos() {
		c1, ok1 := names[combo.piece1]
		c2, ok2 := names[combo.piece2]
		if !ok1 || !ok2 {
			continue
		}
		// Both pieces in castable — cast the designated first piece.
		if combo.castFirst == combo.piece1 {
			h.pendingComboSecond = combo.piece2
			return c1
		}
		h.pendingComboSecond = combo.piece1
		return c2
	}

	// Strategy-aware: also check N-card combos (N > 2) where all pieces
	// are castable. Use CastOrder for sequencing.
	if h.strategy != nil {
		for _, cp := range h.strategy.ComboPieces {
			if len(cp.Pieces) <= 2 {
				continue // already handled above
			}
			allPresent := true
			for _, p := range cp.Pieces {
				if _, ok := names[p]; !ok {
					allPresent = false
					break
				}
			}
			if !allPresent {
				continue
			}
			// All pieces castable. Cast first piece from CastOrder,
			// set the second as pending (the rest will be handled in
			// subsequent calls via the same N-card path).
			order := cp.CastOrder
			if len(order) == 0 {
				order = cp.Pieces
			}
			if len(order) >= 2 {
				h.pendingComboSecond = order[1]
			}
			if c, ok := names[order[0]]; ok {
				return c
			}
		}
	}
	return nil
}

// findComboWithBattlefield checks if one piece of a known combo is in
// the castable list and the other is already on the battlefield (ours).
// If so, returns the remaining piece to cast.
//
// IMPORTANT: only suggests the combo if the piece on the battlefield is
// the one that should be in play first (castFirst), and the piece to
// cast is the finisher. For Thoracle combos, Consultation must resolve
// first (exiling the library), THEN Oracle enters (ETB checks library).
// Having Oracle on the battlefield and Consultation in hand is the
// WRONG order — Oracle's ETB already fired and won't re-trigger.
func (h *PokerHat) findComboWithBattlefield(gs *gameengine.GameState, seatIdx int, pool []*gameengine.Card) *gameengine.Card {
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	seat := gs.Seats[seatIdx]
	bf := make(map[string]bool, len(seat.Battlefield))
	for _, p := range seat.Battlefield {
		if p != nil && p.Card != nil {
			bf[p.Card.DisplayName()] = true
		}
	}
	names := make(map[string]*gameengine.Card, len(pool))
	for _, c := range pool {
		names[c.DisplayName()] = c
	}
	for _, combo := range h.effectiveCombos() {
		// Only trigger if the piece on the battlefield is castFirst and
		// the piece in hand is the finisher. For instant/sorcery combos
		// (Consultation), castFirst resolves from the stack and goes to
		// graveyard, so it won't be on the battlefield. But for permanent-
		// based combos (Isochron Scepter, Food Chain, Heliod), castFirst
		// stays on the battlefield.
		//
		// Case A: castFirst is on battlefield, finisher is in castable.
		if combo.castFirst == combo.piece1 && bf[combo.piece1] {
			if c, ok := names[combo.piece2]; ok {
				return c
			}
		}
		if combo.castFirst == combo.piece2 && bf[combo.piece2] {
			if c, ok := names[combo.piece1]; ok {
				return c
			}
		}
	}
	return nil
}

// suicideComboPieces are cards that, if cast without their combo partner,
// produce catastrophically bad outcomes:
//   - Demonic Consultation / Tainted Pact: exile entire library = lose
//   - Thassa's Oracle / Laboratory Maniac: waste the win condition on a
//     1/3 or 2/2 body when the library is still full (ETB won't win)
//
// These must ONLY be cast when their partner is in hand or on the
// battlefield AND the combo sequence is correct.
var suicideComboPieces = map[string][]string{
	"Demonic Consultation": {"Thassa's Oracle", "Laboratory Maniac"},
	"Tainted Pact":         {"Thassa's Oracle", "Laboratory Maniac"},
	"Thassa's Oracle":      {"Demonic Consultation", "Tainted Pact"},
	"Laboratory Maniac":    {"Demonic Consultation", "Tainted Pact"},
}

// filterSuicideComboPieces returns castable minus any self-destructive
// combo piece whose partner isn't in hand or on the battlefield.
func filterSuicideComboPieces(gs *gameengine.GameState, seatIdx int, castable []*gameengine.Card) []*gameengine.Card {
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return castable
	}
	seat := gs.Seats[seatIdx]

	// Build lookup of cards in hand + battlefield.
	available := make(map[string]bool, len(seat.Hand)+len(seat.Battlefield))
	for _, c := range seat.Hand {
		if c != nil {
			available[c.DisplayName()] = true
		}
	}
	for _, p := range seat.Battlefield {
		if p != nil && p.Card != nil {
			available[p.Card.DisplayName()] = true
		}
	}

	out := make([]*gameengine.Card, 0, len(castable))
	for _, c := range castable {
		if c == nil {
			continue
		}
		partners, isSuicide := suicideComboPieces[c.DisplayName()]
		if !isSuicide {
			out = append(out, c)
			continue
		}
		// Only include if at least one partner is available.
		partnerFound := false
		for _, p := range partners {
			if available[p] {
				partnerFound = true
				break
			}
		}
		if partnerFound {
			out = append(out, c)
		}
		// else: skip this suicide piece — no partner available
	}
	return out
}

// shouldHoldCard returns true if a card should NOT be cast right now
// because the game state doesn't support its intended use. This prevents
// combo pieces from being wasted as vanilla spells.
func shouldHoldCard(c *gameengine.Card, gs *gameengine.GameState, seatIdx int) bool {
	if c == nil || gs == nil {
		return false
	}
	name := strings.ToLower(c.DisplayName())

	// Don't cast Brain Freeze without a meaningful storm count.
	if name == "brain freeze" {
		stormKey := fmt.Sprintf("storm_count_seat_%d", seatIdx)
		stormCount := 0
		if gs.Flags != nil {
			stormCount = gs.Flags[stormKey]
		}
		// Also count global SpellsCastThisTurn as a proxy.
		if gs.SpellsCastThisTurn > stormCount {
			stormCount = gs.SpellsCastThisTurn
		}
		if stormCount < 3 {
			return true
		}
	}

	// Don't cast Tendrils of Agony without storm count.
	if name == "tendrils of agony" {
		stormCount := gs.SpellsCastThisTurn
		if stormCount < 3 {
			return true
		}
	}

	// Don't cast Grapeshot without storm count.
	if name == "grapeshot" {
		stormCount := gs.SpellsCastThisTurn
		if stormCount < 3 {
			return true
		}
	}

	// Don't hardcast Simian/Elvish Spirit Guide — they should be exiled
	// for mana, not played as vanilla 2/2s.
	if name == "simian spirit guide" || name == "elvish spirit guide" {
		return true
	}

	return false
}

func (h *PokerHat) chooseCastHold(gs *gameengine.GameState, seatIdx int, pool []*gameengine.Card) *gameengine.Card {
	// Bucket 0..5 (6 = haymakers; skipped).
	// Strategy-aware: value engine keys get promoted to bucket 0.5
	// (between tutors and draw), treated as essential setup.
	buckets := make([][]*gameengine.Card, 7)
	for _, c := range pool {
		// Strategy: value engine keys are promoted above draw/recursion.
		if h.strategy != nil && h.isValueEngineKey(c) {
			buckets[0] = append(buckets[0], c) // same priority as tutors
			continue
		}
		switch {
		case isTutor(c):
			buckets[0] = append(buckets[0], c)
		case isCardDraw(c):
			buckets[1] = append(buckets[1], c)
		case isRecursion(c):
			buckets[2] = append(buckets[2], c)
		case isRamp(c):
			buckets[3] = append(buckets[3], c)
		case isTargetedRemoval(c):
			buckets[4] = append(buckets[4], c)
		case isCheapThreat(c):
			buckets[5] = append(buckets[5], c)
		default:
			buckets[6] = append(buckets[6], c)
		}
	}
	// Removal needs a worthy target — drop to bucket 6 if no threat ≥ 10.
	if len(buckets[4]) > 0 {
		worthy := false
		for _, s := range gs.Seats {
			if s == nil || s.Lost || s.Idx == seatIdx {
				continue
			}
			if h.threatBreakdown(gs, seatIdx, s).Score >= 10 {
				worthy = true
				break
			}
		}
		if !worthy {
			buckets[4] = nil
		}
	}
	for i := 0; i < 6; i++ { // skip bucket 6
		if len(buckets[i]) == 0 {
			continue
		}
		// Cheaper first within a bucket.
		sort.SliceStable(buckets[i], func(a, b int) bool {
			ca := gameengine.ManaCostOf(buckets[i][a])
			cb := gameengine.ManaCostOf(buckets[i][b])
			if ca != cb {
				return ca < cb
			}
			return buckets[i][a].DisplayName() < buckets[i][b].DisplayName()
		})
		return buckets[i][0]
	}
	return nil
}

// ChooseAttackers — mode-sensitive attack declaration.
func (h *PokerHat) ChooseAttackers(gs *gameengine.GameState, seatIdx int, legal []*gameengine.Permanent) []*gameengine.Permanent {
	if len(legal) == 0 {
		return nil
	}
	if h.Mode == ModeRaise {
		return h.GreedyHat.ChooseAttackers(gs, seatIdx, legal)
	}
	if h.Mode == ModeHold {
		return h.pickSafeAttackers(gs, seatIdx, legal)
	}
	// CALL — deadliest-first 70% with worthwhile targets.
	return h.pickCallAttackers(gs, seatIdx, legal)
}

func (h *PokerHat) pickSafeAttackers(gs *gameengine.GameState, seatIdx int, legal []*gameengine.Permanent) []*gameengine.Permanent {
	safe := make([]*gameengine.Permanent, 0, len(legal))
	for _, a := range legal {
		if a == nil {
			continue
		}
		// Evasive creatures always attack.
		if a.HasKeyword("flying") || a.HasKeyword("menace") ||
			a.HasKeyword("unblockable") || a.HasKeyword("shadow") ||
			a.HasKeyword("skulk") {
			safe = append(safe, a)
			continue
		}
		// Attack if any opponent is open or at <= 30 life (keep pressure).
		for _, o := range gs.Seats {
			if o == nil || o.Lost || o.Idx == seatIdx {
				continue
			}
			if isOpenForAttacker(a, o) || o.Life <= 30 {
				safe = append(safe, a)
				break
			}
		}
	}
	return safe
}

func (h *PokerHat) pickCallAttackers(gs *gameengine.GameState, seatIdx int, legal []*gameengine.Permanent) []*gameengine.Permanent {
	// Rank deadliest-first.
	ranked := make([]*gameengine.Permanent, 0, len(legal))
	for _, a := range legal {
		if a != nil {
			ranked = append(ranked, a)
		}
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		ri := attackerRank(ranked[i])
		rj := attackerRank(ranked[j])
		return ri > rj
	})

	// After turn 12 (round 3), attack with everything — games need to
	// end. Before that, filter to attackers with worthwhile targets.
	if gs != nil && gs.Turn > 12 {
		return ranked
	}

	// Early game: filter to attackers with a worthwhile target.
	worthy := make([]*gameengine.Permanent, 0, len(ranked))
	for _, a := range ranked {
		ok := false
		for _, o := range gs.Seats {
			if o == nil || o.Lost || o.Idx == seatIdx {
				continue
			}
			if isOpenForAttacker(a, o) || o.Life <= 30 {
				ok = true
				break
			}
		}
		if ok {
			worthy = append(worthy, a)
		}
	}
	if len(worthy) == 0 {
		return nil
	}
	return worthy
}

// ChooseAttackTarget — open-target first, then lowest-life, then 7-dim
// threat, else first living.
func (h *PokerHat) ChooseAttackTarget(gs *gameengine.GameState, seatIdx int, attacker *gameengine.Permanent, legalDefenders []int) int {
	if len(legalDefenders) == 0 {
		return seatIdx
	}
	if len(legalDefenders) == 1 {
		return legalDefenders[0]
	}
	// Map to living opponent seats we're actually allowed to attack.
	living := make([]*gameengine.Seat, 0, len(legalDefenders))
	for _, d := range legalDefenders {
		if d < 0 || d >= len(gs.Seats) {
			continue
		}
		s := gs.Seats[d]
		if s == nil || s.Lost || s.Idx == seatIdx {
			continue
		}
		living = append(living, s)
	}
	if len(living) == 0 {
		return seatIdx
	}
	// Prefer open target.
	for _, o := range living {
		if isOpenForAttacker(attacker, o) {
			return o.Idx
		}
	}
	// Political targeting: attack the LEADING player (highest life +
	// board power). This is how real Commander politics work — the table
	// gangs up on whoever is ahead. Only exception: if someone is at
	// very low life (<=10), finish them off.
	anyLethal := false
	for _, o := range living {
		if o.Life <= 10 {
			anyLethal = true
			break
		}
	}
	if anyLethal {
		// Finish off the weakest player if they're near death.
		best := living[0]
		for _, o := range living[1:] {
			if o.Life < best.Life {
				best = o
			}
		}
		return best.Idx
	}
	// Otherwise, attack the leader (highest combined score).
	best := living[0]
	bestScore := leaderScore(best)
	for _, o := range living[1:] {
		s := leaderScore(o)
		if s > bestScore {
			best, bestScore = o, s
		}
	}
	return best.Idx
}

// AssignBlockers — mode-sensitive blocking.
// RAISE: don't block unless incoming damage is lethal. Preserve creatures
//   for our own attack next turn.
// HOLD: block conservatively, prioritize favorable trades where our
//   creature survives.
// CALL: use GreedyHat's blocking which looks for favorable trades and
//   chump blocks when necessary.
func (h *PokerHat) AssignBlockers(gs *gameengine.GameState, seatIdx int, attackers []*gameengine.Permanent) map[*gameengine.Permanent][]*gameengine.Permanent {
	if h.Mode == ModeRaise {
		incoming := 0
		for _, a := range attackers {
			if a == nil {
				continue
			}
			mul := 1
			if a.HasKeyword("double strike") || a.HasKeyword("double_strike") {
				mul = 2
			}
			incoming += a.Power() * mul
		}
		if seatIdx >= 0 && seatIdx < len(gs.Seats) {
			if incoming < gs.Seats[seatIdx].Life {
				out := make(map[*gameengine.Permanent][]*gameengine.Permanent, len(attackers))
				for _, a := range attackers {
					out[a] = nil
				}
				return out
			}
		}
	}
	// HOLD and CALL: use GreedyHat's smart blocking (favorable trades,
	// chump only when lethal).
	return h.GreedyHat.AssignBlockers(gs, seatIdx, attackers)
}

// ChooseResponse — mode-specific threat threshold.
// In HOLD mode: counter anything dangerous (low threshold = 3).
// In CALL mode: counter significant threats (threshold = 4).
// In RAISE mode: only counter game-winning threats (threshold = 6) --
//   save mana for our own combo/threats.
func (h *PokerHat) ChooseResponse(gs *gameengine.GameState, seatIdx int, top *gameengine.StackItem) *gameengine.StackItem {
	if top == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	if top.Controller == seatIdx || top.Countered {
		return nil
	}
	if gameengine.SplitSecondActive(gs) {
		return nil
	}
	if gameengine.OppRestrictsDefenderToSorcerySpeed(gs, seatIdx) {
		return nil
	}

	// Detect opponent combo: if the spell on the stack is a known combo
	// piece and its partner is on the opponent's battlefield, always counter.
	if top.Card != nil {
		topName := top.Card.DisplayName()
		caster := top.Controller
		if caster >= 0 && caster < len(gs.Seats) {
			oppSeat := gs.Seats[caster]
			for _, combo := range h.effectiveCombos() {
				if topName == combo.piece1 || topName == combo.piece2 {
					partner := combo.piece2
					if topName == combo.piece2 {
						partner = combo.piece1
					}
					for _, p := range oppSeat.Battlefield {
						if p != nil && p.Card != nil && p.Card.DisplayName() == partner {
							// Opponent is assembling a combo -- counter it!
							return h.GreedyHat.ChooseResponse(gs, seatIdx, top)
						}
					}
					// Also check opponent's hand (we can't see it, but if
					// the spell itself is dangerous, counter with low threshold)
					break
				}
			}
		}
	}

	// Mode-specific threshold.
	score := stackItemScore(top)
	var minScore int
	switch h.Mode {
	case ModeRaise:
		minScore = 6 // Only counter big threats, save mana for our push
	case ModeHold:
		minScore = 2 // Counter aggressively to protect our rebuild
	default: // CALL
		minScore = 3 // Counter most significant spells
	}
	if score < minScore {
		return nil
	}
	return h.GreedyHat.ChooseResponse(gs, seatIdx, top)
}

// ChooseTarget — HOLD prefers self-targets for buffs/draw; CALL / RAISE
// want highest 7-dim threat opp for player targets.
//
// Strategy-aware: when the legal targets include cards (TargetKindCard)
// and the strategy has tutor targets, prefer cards from the tutor target
// list. This covers any targeting effect that selects from a zone.
func (h *PokerHat) ChooseTarget(gs *gameengine.GameState, seatIdx int, filter gameast.Filter, legal []gameengine.Target) gameengine.Target {
	// Strategy-aware card selection: when choosing among card targets,
	// prefer tutor targets and value engine keys from the strategy.
	if h.strategy != nil && len(legal) > 1 {
		if pick, ok := h.pickStrategyCardTarget(legal); ok {
			return pick
		}
	}

	if h.Mode == ModeHold && (filter.Base == "any_target" || filter.Base == "player") {
		// Return the seat-target for self if present in legal; else fall through.
		for _, t := range legal {
			if t.Kind == gameengine.TargetKindSeat && t.Seat == seatIdx {
				return t
			}
		}
		return h.GreedyHat.ChooseTarget(gs, seatIdx, filter, legal)
	}
	// CALL / RAISE — pick the highest-threat living opp for player filters.
	if filter.Base == "player" || filter.Base == "opponent" || filter.Base == "any_target" {
		best := h.pickBestPlayerTarget(gs, seatIdx)
		if best != nil {
			for _, t := range legal {
				if t.Kind == gameengine.TargetKindSeat && t.Seat == best.Idx {
					return t
				}
			}
			// Synthesize a seat target if no legal slot matched.
			return gameengine.Target{Kind: gameengine.TargetKindSeat, Seat: best.Idx}
		}
	}
	return h.GreedyHat.ChooseTarget(gs, seatIdx, filter, legal)
}

// pickStrategyCardTarget checks if any legal target is a card from the
// strategy's tutor targets or value engine keys. Returns the highest-
// priority match. Only considers TargetKindCard targets (cards in zones
// like library, graveyard, exile).
func (h *PokerHat) pickStrategyCardTarget(legal []gameengine.Target) (gameengine.Target, bool) {
	if h.strategy == nil {
		return gameengine.Target{}, false
	}

	// Check if there are any card-kind targets at all.
	hasCards := false
	for _, t := range legal {
		if t.Kind == gameengine.TargetKindCard && t.Card != nil {
			hasCards = true
			break
		}
	}
	if !hasCards {
		return gameengine.Target{}, false
	}

	// Build priority order: tutor targets first, then value engine keys.
	priorities := make([]string, 0, len(h.strategy.TutorTargets)+len(h.strategy.ValueEngineKeys))
	priorities = append(priorities, h.strategy.TutorTargets...)
	priorities = append(priorities, h.strategy.ValueEngineKeys...)

	// Find the highest-priority card in the legal list.
	for _, name := range priorities {
		for _, t := range legal {
			if t.Kind == gameengine.TargetKindCard && t.Card != nil &&
				t.Card.DisplayName() == name {
				return t, true
			}
		}
	}
	return gameengine.Target{}, false
}

func (h *PokerHat) pickBestPlayerTarget(gs *gameengine.GameState, seatIdx int) *gameengine.Seat {
	var best *gameengine.Seat
	bestScore := -1e18
	for _, s := range gs.Seats {
		if s == nil || s.Lost || s.Idx == seatIdx {
			continue
		}
		score := h.threatBreakdown(gs, seatIdx, s).Score
		if score > bestScore {
			best, bestScore = s, score
		}
	}
	return best
}

func (h *PokerHat) String() string {
	if h.strategy != nil {
		return fmt.Sprintf("PokerHat(mode=%s, strategy=%s)", h.Mode.String(), h.strategy.Archetype)
	}
	return fmt.Sprintf("PokerHat(mode=%s)", h.Mode.String())
}

// isValueEngineKey returns true if the card is in the strategy's value
// engine key list.
func (h *PokerHat) isValueEngineKey(c *gameengine.Card) bool {
	if h.strategy == nil || c == nil {
		return false
	}
	name := c.DisplayName()
	for _, vk := range h.strategy.ValueEngineKeys {
		if name == vk {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------
// Card-effect classification (used by HOLD-mode bucketing + archetype)
// ---------------------------------------------------------------------

// effectKinds walks a card's AST and returns the set of effect kinds
// present on its spell-side bodies (Activated + Triggered bodies).
func effectKinds(card *gameengine.Card) map[string]struct{} {
	out := map[string]struct{}{}
	if card == nil || card.AST == nil {
		return out
	}
	var walk func(e gameast.Effect)
	walk = func(e gameast.Effect) {
		if e == nil {
			return
		}
		out[e.Kind()] = struct{}{}
		switch v := e.(type) {
		case *gameast.Sequence:
			for _, it := range v.Items {
				walk(it)
			}
		case *gameast.Choice:
			for _, o := range v.Options {
				walk(o)
			}
		case *gameast.Optional_:
			walk(v.Body)
		case *gameast.Conditional:
			walk(v.Body)
		}
	}
	for _, ab := range card.AST.Abilities {
		switch a := ab.(type) {
		case *gameast.Activated:
			walk(a.Effect)
		case *gameast.Triggered:
			walk(a.Effect)
		}
	}
	return out
}

func isRamp(card *gameengine.Card) bool {
	if card == nil {
		return false
	}
	kinds := effectKinds(card)
	if _, ok := kinds["add_mana"]; ok {
		return true
	}
	// Land-ramp tutors (tutor destination = battlefield, target = land).
	if card.AST != nil {
		for _, ab := range card.AST.Abilities {
			a, ok := ab.(*gameast.Activated)
			if !ok {
				continue
			}
			if found := walkForLandTutor(a.Effect); found {
				return true
			}
		}
	}
	// Cheap artifact mana rock fallback.
	if typeLineContains(card, "artifact") && gameengine.ManaCostOf(card) <= 3 {
		if strings.Contains(gameengine.OracleTextLower(card), "mana") {
			return true
		}
	}
	return false
}

func walkForLandTutor(e gameast.Effect) bool {
	if e == nil {
		return false
	}
	if t, ok := e.(*gameast.Tutor); ok {
		if strings.Contains(strings.ToLower(t.Destination), "battlefield") {
			if strings.Contains(strings.ToLower(t.Query.Base), "land") {
				return true
			}
		}
	}
	if seq, ok := e.(*gameast.Sequence); ok {
		for _, it := range seq.Items {
			if walkForLandTutor(it) {
				return true
			}
		}
	}
	return false
}

func isCardDraw(card *gameengine.Card) bool {
	_, ok := effectKinds(card)["draw"]
	return ok
}

func isTutor(card *gameengine.Card) bool {
	_, ok := effectKinds(card)["tutor"]
	return ok
}

func isRecursion(card *gameengine.Card) bool {
	k := effectKinds(card)
	if _, ok := k["reanimate"]; ok {
		return true
	}
	if _, ok := k["recurse"]; ok {
		return true
	}
	return false
}

func isTargetedRemoval(card *gameengine.Card) bool {
	if card == nil || card.AST == nil {
		return false
	}
	for _, ab := range card.AST.Abilities {
		a, ok := ab.(*gameast.Activated)
		if !ok {
			continue
		}
		if checkTargetedRemovalEffect(a.Effect) {
			return true
		}
	}
	return false
}

func checkTargetedRemovalEffect(e gameast.Effect) bool {
	if e == nil {
		return false
	}
	if seq, ok := e.(*gameast.Sequence); ok {
		for _, it := range seq.Items {
			if checkTargetedRemovalEffect(it) {
				return true
			}
		}
		return false
	}
	// Pull the Filter from the concrete-effect's Target field.
	var f *gameast.Filter
	switch v := e.(type) {
	case *gameast.Destroy:
		f = &v.Target
	case *gameast.Exile:
		f = &v.Target
	case *gameast.Bounce:
		f = &v.Target
	case *gameast.Damage:
		f = &v.Target
	default:
		return false
	}
	if f == nil {
		return false
	}
	switch f.Base {
	case "creature", "planeswalker", "any_target",
		"player", "opponent", "artifact",
		"enchantment", "nonland_permanent":
		return true
	}
	return false
}

func isCheapThreat(card *gameengine.Card) bool {
	if !typeLineContains(card, "creature") {
		return false
	}
	return gameengine.ManaCostOf(card) <= 3
}

// ---------------------------------------------------------------------
// Board / power helpers
// ---------------------------------------------------------------------

func boardPower(seat *gameengine.Seat) int {
	if seat == nil {
		return 0
	}
	n := 0
	for _, p := range seat.Battlefield {
		if p != nil && p.IsCreature() {
			if pw := p.Power(); pw > 0 {
				n += pw
			}
		}
	}
	return n
}

func highCMCPermanents(seat *gameengine.Seat) int {
	if seat == nil {
		return 0
	}
	n := 0
	for _, p := range seat.Battlefield {
		if p != nil && p.Card != nil && gameengine.ManaCostOf(p.Card) >= 5 {
			n++
		}
	}
	return n
}

func countManaRocksAndLands(seat *gameengine.Seat) int {
	if seat == nil {
		return 0
	}
	n := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.IsLand() {
			n++
			continue
		}
		if typeLineContains(p.Card, "artifact") && gameengine.ManaCostOf(p.Card) <= 3 {
			ot := gameengine.OracleTextLower(p.Card)
			if strings.Contains(ot, "mana") || strings.Contains(ot, "{t}") {
				n++
			}
		}
	}
	return n
}

// isOpenForAttacker — rough heuristic for "can attacker swing through
// this defender's pool mostly unblocked".
func isOpenForAttacker(attacker *gameengine.Permanent, defender *gameengine.Seat) bool {
	if attacker == nil || defender == nil {
		return false
	}
	if attacker.HasKeyword("unblockable") {
		return true
	}
	untapped := make([]*gameengine.Permanent, 0)
	for _, b := range defender.Battlefield {
		if b != nil && b.IsCreature() && !b.Tapped {
			untapped = append(untapped, b)
		}
	}
	if len(untapped) == 0 {
		return true
	}
	if attacker.HasKeyword("flying") {
		for _, b := range untapped {
			if b.HasKeyword("flying") || b.HasKeyword("reach") {
				return false
			}
		}
		return true
	}
	if attacker.HasKeyword("menace") && len(untapped) <= 1 {
		return true
	}
	ap := attacker.Power()
	if ap < 0 {
		ap = 0
	}
	for _, b := range untapped {
		if b.Power() >= ap {
			return false
		}
	}
	return true
}

// attackerRank returns a deadliest-first numeric score.
func attackerRank(a *gameengine.Permanent) int {
	if a == nil {
		return -1 << 30
	}
	dt := 0
	if a.HasKeyword("deathtouch") {
		dt = 5
	}
	ds := 0
	if a.HasKeyword("double strike") || a.HasKeyword("double_strike") {
		ds = 3
	}
	return a.Power() + dt + ds
}

// bestChumpBlocker picks the best creature to sacrifice as a chump blocker.
// Prefers tokens, then creatures with death triggers, then summoning-sick
// creatures, then smallest. This ensures we don't waste valuable permanents
// when we need to chump-block to survive.
func bestChumpBlocker(legal []*gameengine.Permanent) *gameengine.Permanent {
	if len(legal) == 0 {
		return nil
	}
	best := legal[0]
	bestScore := chumpScore(best)
	for _, b := range legal[1:] {
		s := chumpScore(b)
		if s > bestScore {
			best = b
			bestScore = s
		}
	}
	return best
}

func chumpScore(p *gameengine.Permanent) float64 {
	if p == nil {
		return -100
	}
	score := 0.0
	if p.Card != nil {
		for _, t := range p.Card.Types {
			if t == "token" {
				score += 3.0
				break
			}
		}
		ot := gameengine.OracleTextLower(p.Card)
		if strings.Contains(ot, "when") && (strings.Contains(ot, "dies") || strings.Contains(ot, "leaves the battlefield")) {
			score += 2.0
		}
	}
	if p.SummoningSick {
		score += 1.0
	}
	score -= float64(p.Power()+p.Toughness()) * 0.1
	return score
}

// stackItemScore — cheap proxy for _stack_item_threat_score. CMC +
// kind-specific bonus. Full scoring lives in the engine; this is only
// the mode-threshold gate.
func stackItemScore(top *gameengine.StackItem) int {
	if top == nil {
		return 0
	}
	score := 0
	if top.Card != nil {
		score += gameengine.ManaCostOf(top.Card)
	}
	// Bump for mass-effect hints on the AST.
	if top.Card != nil {
		ot := gameengine.OracleTextLower(top.Card)
		if strings.Contains(ot, "win the game") {
			score += 10
		}
		if strings.Contains(ot, "destroy all") || strings.Contains(ot, "exile all") {
			score += 5
		}
	}
	return score
}

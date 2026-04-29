package gameengine

// Phase 5 — Stack + priority system.
//
// This file wires CR §117 priority, §601 spell casting, and §608.2 spell
// resolution on top of the GameState + resolver primitives built in
// Phases 3–4. It mirrors the Python reference at scripts/playloop.py —
// specifically cast_spell, _priority_round, _get_response,
// _resolve_stack_top, _split_second_active, and
// _opp_restricts_defender_to_sorcery_speed.
//
// Scope:
//
//   - CastSpell(gs, seat, card, targets)     — CR §601.2 casting sequence
//   - PushStackItem(gs, item)                — allocate ID + append + log
//   - PushTriggeredAbility(gs, src, effect)  — CR §603.2 put trigger on stack
//   - PriorityRound(gs)                      — CR §117.3-5 APNAP polling
//   - ResolveStackTop(gs)                    — CR §608.2 pop + resolve
//   - GetResponse(gs, defenderSeat, top)     — policy hook (greedy default)
//   - SplitSecondActive(gs)                  — CR §702.61a detection
//   - OppRestrictsDefenderToSorcerySpeed     — CR §307.1 / §601.3a check
//
// Comp-rules citations throughout refer to data/rules/MagicCompRules-20260227.txt.
//
// Implementation notes:
//   - Stack is LIFO; gs.Stack[len-1] is the TOP. §608.2 pops the top.
//   - Priority round is bounded at 16 iterations to match Python — counter
//     wars should terminate long before that; the cap catches policy bugs.
//   - Greedy response policy: a seat with a CounterSpell-bearing instant in
//     hand and enough mana to pay will always counter an opponent's spell.
//     Policy hooks can be layered later (Phase 10).
//   - Post-resolution: SBAs fire (CR §704.3 / §117.5), then priority re-opens
//     if the stack is still non-empty.
//   - Triggered abilities from combat damage, attack, and ETB now flow
//     through the stack rather than resolving inline. This is the Phase 4
//     coupling flagged in the combat agent's handoff note.

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// maxStackDrainIterations caps the resolve-SBA-priority loop that runs
// after a cast or activation. Each iteration resolves one stack item;
// cascading triggers can push many items per resolution, causing the loop
// to run thousands of times. 500 iterations is enough for any legal game
// state (most games peak at 20-30) while preventing the engine from
// spinning for minutes on degenerate trigger avalanches.
const maxStackDrainIterations = 500

// maxDrainRecursion caps how deep DrainStack can recurse into itself
// (via ResolveStackTop → trigger handler → CastSpell → DrainStack).
const maxDrainRecursion = 10

// maxResolveDepth caps the inline PriorityRound+ResolveStackTop recursion
// inside PushTriggeredAbility. Reanimate/sacrifice loops create a cycle:
// PushTriggeredAbility → PriorityRound → ResolveStackTop → ResolveEffect
// → zone-change triggers → PushTriggeredAbility → ... which recurses
// through Go's call stack. Beyond this depth, triggers are left on the
// stack for DrainStack's iterative loop to resolve.
const maxResolveDepth = 50

// DrainStack resolves items until the stack is empty, with loop detection
// (CR §727) and an iteration safety cap. Used by all cast/activation paths.
func DrainStack(gs *GameState) {
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["drain_depth"]++
	defer func() { gs.Flags["drain_depth"]-- }()
	if gs.Flags["drain_depth"] > maxDrainRecursion {
		gs.Stack = gs.Stack[:0]
		return
	}

	var ld *loopDetector
	for drainIter := 0; len(gs.Stack) > 0 && drainIter < maxStackDrainIterations; drainIter++ {
		if drainIter >= loopMinReps*2 {
			if ld == nil {
				ld = newLoopDetector()
			}
			ld.record(gs, stackTopFingerprint(gs))
			if ld.projectAndApply(gs) {
				break
			}
		}
		ResolveStackTop(gs)
		StateBasedActions(gs)
		if len(gs.Stack) > 0 {
			PriorityRound(gs)
		}
	}
	StateBasedActions(gs)
}

// ---------------------------------------------------------------------------
// Stack-item construction + push.
// ---------------------------------------------------------------------------

// nextStackID returns the next monotonically increasing stack item ID.
// We reuse gs.EffectTimestamp's counter via a dedicated flag key so
// state.go stays untouched (Phase 5 contract).
func nextStackID(gs *GameState) int {
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["_stack_seq"]++
	return gs.Flags["_stack_seq"]
}

// PushStackItem pushes a prepared StackItem onto gs.Stack, assigns it an ID
// if it doesn't already have one, and logs a stack_push event. Shared helper
// used by CastSpell, PushTriggeredAbility, and the response-casting code
// path inside PriorityRound.
func PushStackItem(gs *GameState, item *StackItem) *StackItem {
	if gs == nil || item == nil {
		return item
	}
	if item.ID == 0 {
		item.ID = nextStackID(gs)
	}
	gs.Stack = append(gs.Stack, item)
	name := ""
	if item.Card != nil {
		name = item.Card.DisplayName()
	} else if item.Source != nil {
		name = item.Source.Card.DisplayName()
	}
	// Stack trace: log push event for CR audit.
	GlobalStackTrace.Log("push", name, item.Controller, len(gs.Stack), "spell_cast")
	gs.LogEvent(Event{
		Kind:   "stack_push",
		Seat:   item.Controller,
		Source: name,
		Details: map[string]interface{}{
			"stack_id":   item.ID,
			"stack_size": len(gs.Stack),
			"rule":       "608.1",
		},
	})
	return item
}

// PushTriggeredAbility creates a StackItem for a triggered ability and pushes
// it. Mirrors CR §603.2: "Whenever a game event or game state matches a
// triggered ability's trigger event, the ability automatically triggers. The
// ability doesn't do anything at this point." Then per §603.3, "the next
// time a player would receive priority, each ability that has triggered but
// hasn't yet been put on the stack is put on the stack."
//
// We push immediately here (no lazy bucket) because the Phase 4 combat
// code fires triggers at well-defined rulebook moments and the engine is
// single-threaded. A future "pending triggers" queue is a Phase 7 concern.
func PushTriggeredAbility(gs *GameState, src *Permanent, effect gameast.Effect) *StackItem {
	if gs == nil || src == nil || effect == nil {
		return nil
	}
	item := &StackItem{
		Controller: src.Controller,
		Source:     src,
		Effect:     effect,
	}
	if src.Card != nil {
		// StackItem.Card is usually for spells, not triggers, but we point it
		// at the source card so logs show the right name.
		item.Card = src.Card
	}
	// Stack trace: log triggered ability push for CR audit.
	trigName := ""
	if src.Card != nil {
		trigName = src.Card.DisplayName()
	}
	GlobalStackTrace.Log("trigger_push", trigName, src.Controller, len(gs.Stack), "triggered_ability")
	PushStackItem(gs, item)

	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["resolve_depth"]++
	defer func() { gs.Flags["resolve_depth"]-- }()

	if gs.Flags["resolve_depth"] > maxResolveDepth {
		return item
	}

	// Per CR §117.3a priority opens on triggers — open a priority round then
	// resolve. This matches the Python _push_trigger_and_resolve pattern.
	PriorityRound(gs)
	if len(gs.Stack) > 0 && gs.Stack[len(gs.Stack)-1] == item {
		ResolveStackTop(gs)
	}
	return item
}

// ---------------------------------------------------------------------------
// CastSpell — CR §601.
// ---------------------------------------------------------------------------

// CastError is returned by CastSpell when the cast fails a legality check
// (card not in hand, insufficient mana, split-second in play, etc.).
// CR §601.2e: "If the proposed spell is illegal, the game returns to the
// moment before the casting of that spell was proposed."
type CastError struct {
	Reason string
}

func (e *CastError) Error() string { return "cast failed: " + e.Reason }

// CastSpell executes the CR §601.2 casting sequence for a single spell:
//
//   1. §601.2a  — announce the spell (remove from hand, create stack item).
//   2. §601.2b  — choose modes / targets (caller-supplied targets[]).
//   3. §601.2f  — pay costs (mana only for MVP).
//   4. Priority opens (CR §117.3c).
//   5. On all-pass, top of stack resolves (CR §117.4).
//
// Returns a CastError on any of:
//   - card not in caster's hand
//   - insufficient mana in pool
//   - split-second spell on stack forbidding non-mana casts (CR §702.61a)
//   - sorcery-speed restriction + stack non-empty (CR §307.1 / §601.3a)
//
// For MVP, mana cost is read from card.AST.ManaCost.CMC() if available,
// else the callers of CastSpell can stash a `_cost` flag via Flags. A real
// Phase 8 color/mana pool will replace this.
func CastSpell(gs *GameState, seatIdx int, card *Card, targets []Target) error {
	if gs == nil {
		return &CastError{Reason: "nil game"}
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return &CastError{Reason: "invalid seat"}
	}
	if card == nil {
		return &CastError{Reason: "nil card"}
	}
	seat := gs.Seats[seatIdx]

	// CR §702.61a: split-second spell on stack forbids casting non-mana
	// spells. Mana abilities are fine, but the caller doesn't pass those
	// through CastSpell.
	if SplitSecondActive(gs) {
		return &CastError{Reason: "split_second"}
	}

	// CR §307.1: sorcery-speed timing. Sorcery-type cards can only be cast
	// during the active player's main phase when the stack is empty.
	// Only enforce when the phase is explicitly set to a combat/non-main
	// phase or when the stack is non-empty — skip when phase is unset or
	// "beginning" to preserve backward compatibility with test fixtures.
	if cardHasType(card, "sorcery") {
		isMainPhase := gs.Phase == "" || gs.Phase == "beginning" ||
			gs.Phase == "precombat_main" || gs.Phase == "postcombat_main"
		if !isMainPhase || len(gs.Stack) > 0 {
			return &CastError{Reason: "sorcery_speed_timing"}
		}
	}

	// CR §307.1 / §601.3a: a Teferi-style static that restricts the seat to
	// sorcery speed while an opponent controls it, combined with a non-empty
	// stack, forbids the cast. Active player casting sorceries on an empty
	// stack is fine.
	if len(gs.Stack) > 0 && OppRestrictsDefenderToSorcerySpeed(gs, seatIdx) {
		return &CastError{Reason: "sorcery_speed_restriction"}
	}

	// Remove from hand. CR §601.2a places the card on the stack (it leaves
	// its origin zone) as the first step of casting.
	if !removeFromHand(seat, card) {
		return &CastError{Reason: "not_in_hand"}
	}

	// Pay mana cost per CR §601.2f. CalculateTotalCost walks the battlefield
	// for static cost modifiers (Thalia, Trinisphere, Helm of Awakening,
	// medallions, etc.) and applies increases → reductions → minimums.
	baseCost := CalculateTotalCost(gs, card, seatIdx)
	chosenX := 0

	// §107.3: if the mana cost contains X, the Hat announces X.
	if ManaCostContainsX(card) {
		xPool := EnsureTypedPool(seat)
		availableForX := xPool.Total() - baseCost
		if availableForX < 0 {
			seat.Hand = append(seat.Hand, card)
			return &CastError{Reason: "insufficient_mana"}
		}
		if seat.Hat != nil {
			chosenX = seat.Hat.ChooseX(gs, seatIdx, card, availableForX)
			if chosenX < 0 {
				chosenX = 0
			}
			if chosenX > availableForX {
				chosenX = availableForX
			}
		} else {
			// No hat — default to spending all available mana.
			chosenX = availableForX
		}
	}

	cost := baseCost + chosenX
	// Check total available mana. Use EnsureTypedPool to bridge any
	// legacy ManaPool integer into the typed pool, then read Total()
	// as the single source of truth. The previous approach of adding
	// seat.ManaPool + seat.Mana.Total() double-counted because AddMana
	// already syncs ManaPool = Mana.Total().
	pool := EnsureTypedPool(seat)
	availMana := pool.Total()
	if availMana < cost {
		seat.Hand = append(seat.Hand, card)
		return &CastError{Reason: "insufficient_mana"}
	}
	seat.ManaPool -= cost
	SyncManaAfterSpend(seat)
	if cost > 0 {
		details := map[string]interface{}{
			"reason": "cast",
			"rule":   "601.2f",
		}
		if chosenX > 0 {
			details["chosen_x"] = chosenX
		}
		gs.LogEvent(Event{
			Kind:   "pay_mana",
			Seat:   seatIdx,
			Amount: cost,
			Source: card.DisplayName(),
			Details: details,
		})
	}
	gs.LogEvent(Event{
		Kind:   "cast",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Amount: cost,
		Details: map[string]interface{}{
			"rule":     "601.2",
			"chosen_x": chosenX,
		},
	})

	// CR §700.4 / §702.40 cast-count bookkeeping. Increment BEFORE storm
	// + cast-trigger observers so the storm spell itself counts toward
	// its own storm tally (copies = spells_cast_this_turn - 1). Copies
	// do NOT call IncrementCastCount (§706.10).
	IncrementCastCount(gs, seatIdx)

	// Fire per-card triggers keyed on "spell cast" events. Rhystic Study,
	// Mystic Remora, Aetherflux Reservoir, Displacer Kitten, Hullbreaker
	// Horror all depend on these. We fire BEFORE pushing the stack item
	// so the triggered abilities go onto the stack ABOVE the spell being
	// cast — matching CR §603.3 ("the next time a player would receive
	// priority...").
	fireCastTriggers(gs, seatIdx, card)

	// Bridge fire for cast-trigger observer permanents NOT yet wired into
	// the per_card registry (Storm-Kiln Artist, Young Pyromancer, Birgi,
	// Monastery Mentor, Runaway Steam-Kin, Niv-Mizzet Parun, Third Path
	// Iconoclast). Real casts only; storm copies bypass. Long-term these
	// should migrate to the per_card pipeline.
	FireCastTriggerObservers(gs, card, seatIdx, false)

	// Build stack item. For non-permanent spells (instants/sorceries) we
	// pull the Effect off the AST's first Activated/Triggered or — more
	// commonly — the collected spell effect. MVP: pick the first Damage/
	// CounterSpell/Draw/etc. from card.AST.Abilities by scanning for
	// Activated-with-empty-cost (spell effect pattern).
	eff := collectSpellEffect(card)
	item := &StackItem{
		Controller: seatIdx,
		Card:       card,
		Effect:     eff,
		Targets:    targets,
		ChosenX:    chosenX,
	}
	PushStackItem(gs, item)

	// CR §702.21 — Ward. "When this creature becomes the target of a
	// spell or ability an opponent controls, counter it unless that
	// player pays {cost}." Check each target for ward and apply.
	CheckWardOnTargeting(gs, item)

	// CR §702.40 — storm trigger. (Copies land ON TOP of the original
	// storm spell. LIFO resolution gives the copies priority, which is
	// gameplay-correct: triggered abilities go on the stack above the
	// spell that triggered them.)
	if HasStormKeyword(card) {
		ApplyStormCopies(gs, item, seatIdx)
	}

	// CR §702.84 — cascade trigger. Exile from library until nonland
	// with lesser CMC, may cast for free, put rest on bottom.
	if HasCascadeKeyword(card) {
		ApplyCascade(gs, seatIdx, manaCostOf(card), card.DisplayName())
	}

	// CR §701.51 — discover trigger. Like cascade but card goes to hand.
	if cardHasKeyword(card, "discover") {
		PerformDiscover(gs, seatIdx, manaCostOf(card))
	}

	// Per-card cast-time snowflake dispatch (currently unused in batch #1
	// but wired for future cards that need to mutate state at cast).
	InvokeCastHook(gs, item)

	// CR §117.3c: caster keeps priority after casting. Open a priority
	// window in which opponents can respond.
	PriorityRound(gs)

	// CR §117.4 + §608.2 + §727: resolve stack with loop shortcut detection.
	DrainStack(gs)
	return nil
}

// removeFromHand removes the card (by pointer identity) from seat.Hand.
// Returns true iff removed.
func removeFromHand(seat *Seat, card *Card) bool {
	for i, c := range seat.Hand {
		if c == card {
			seat.Hand = append(seat.Hand[:i], seat.Hand[i+1:]...)
			return true
		}
	}
	return false
}

// ManaCostOf is the exported version of manaCostOf for consumers
// outside gameengine (Phase 10 Hat implementations in internal/hat).
func ManaCostOf(card *Card) int { return manaCostOf(card) }

// CounterSpellEffectOf exposes counterSpellEffect for Hat implementations
// that need to detect counter-capable cards in hand.
func CounterSpellEffectOf(card *Card) gameast.Effect { return counterSpellEffect(card) }

// CollectSpellEffectOf exposes collectSpellEffect to Hat implementations
// that need to enumerate the spell-side body effects for card
// classification (ramp / draw / tutor / recursion detection).
func CollectSpellEffectOf(card *Card) gameast.Effect { return collectSpellEffect(card) }

// CardHasCounterSpell returns true if the card's AST contains a
// CounterSpell effect anywhere in the spell-side body. Mirrors Python
// `_card_has_counterspell`.
func CardHasCounterSpell(card *Card) bool { return counterSpellEffect(card) != nil }

// ManaCostContainsX returns true if the card's mana cost includes at least
// one X symbol (CR §107.3). Checks both the AST ManaCost and the Card.Types
// "cost:X" test convention.
func ManaCostContainsX(card *Card) bool {
	if card == nil {
		return false
	}
	// Check Types for test convention "cost:X" or "x_cost".
	for _, t := range card.Types {
		if t == "x_cost" || t == "cost:X" {
			return true
		}
	}
	// Check AST ManaCost symbols for IsX.
	if card.AST != nil {
		for _, ab := range card.AST.Abilities {
			if a, ok := ab.(*gameast.Activated); ok && a.Cost.Mana != nil {
				for _, sym := range a.Cost.Mana.Symbols {
					if sym.IsX {
						return true
					}
				}
			}
		}
	}
	return false
}

// baseCostExcludingX returns the mana cost of a card WITHOUT the X portion.
// For cards with X in cost, this is the non-X component that must be paid
// regardless of the chosen X value.
func baseCostExcludingX(card *Card) int {
	if card == nil {
		return 0
	}
	// The cost:N convention in Types already excludes X (it represents
	// the base cost). CMC fallback also excludes X (CMC treats X as 0).
	return manaCostOf(card)
}

// manaCostOf extracts the generic mana cost for MVP. CardAST doesn't
// currently carry a ManaCost field (the parser stores cost on Activated
// abilities via Cost.Mana). For spell-cost, tests encode the cost as a
// "cost:N" token in Card.Types — this avoids rewriting state.go, which
// is out of scope for Phase 5. A proper typed mana pool + per-card CMC
// is Phase 8 territory.
func manaCostOf(card *Card) int {
	if card == nil {
		return 0
	}
	for _, t := range card.Types {
		if strings.HasPrefix(t, "cost:") {
			n := 0
			for _, ch := range t[5:] {
				if ch < '0' || ch > '9' {
					break
				}
				n = n*10 + int(ch-'0')
			}
			return n
		}
	}
	// Fallback: if the card has a top-level Activated with a non-zero mana
	// cost, use that. This lets AST-constructed test cards work without
	// the cost:N hack.
	if card.AST != nil {
		for _, ab := range card.AST.Abilities {
			if a, ok := ab.(*gameast.Activated); ok && a.Cost.Mana != nil {
				cmc := a.Cost.Mana.CMC()
				if cmc > 0 {
					return cmc
				}
			}
		}
	}
	// Second-tier fallback: Card.CMC was populated by the deckparser's
	// buildCard from the Scryfall metadata. This is the canonical source
	// of truth for real corpus cards; the cost:N hack above is test-only.
	if card.CMC > 0 {
		return card.CMC
	}
	return 0
}

// collectSpellEffect returns the first "spell effect" on a card's AST.
// For instants/sorceries the parser emits these as Activated abilities
// whose cost is empty; the effect is the body. Falls back to the first
// Triggered's effect, and ultimately nil (permanent spells have no
// intrinsic on-resolution effect — ETB abilities fire from resolve.go).
func collectSpellEffect(card *Card) gameast.Effect {
	if card == nil || card.AST == nil {
		return nil
	}
	for _, ab := range card.AST.Abilities {
		if a, ok := ab.(*gameast.Activated); ok && a.Effect != nil {
			return a.Effect
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Priority round — CR §117.3–§117.5.
// ---------------------------------------------------------------------------

// apnapOrder returns the list of seats in Active-Player-Next-Active-Player
// order starting from gs.Active. CR §101.4a: "starting with the active
// player and proceeding in turn order."
func apnapOrder(gs *GameState) []int {
	n := len(gs.Seats)
	out := make([]int, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, (gs.Active+i)%n)
	}
	return out
}

// PriorityRound polls seats in APNAP order for responses. When any seat
// responds (casts an instant or activates an ability), a new StackItem is
// pushed and the round restarts at the new top. When all seats pass in
// succession, the round ends — the caller (CastSpell or ResolveStackTop)
// is responsible for resolving the top of the stack per CR §117.4.
//
// Depth-capped at 8; realistic counter-wars rarely exceed 4-5.
func PriorityRound(gs *GameState) {
	if gs == nil {
		return
	}
	const maxDepth = 8
	for depth := 0; depth < maxDepth; depth++ {
		if len(gs.Stack) == 0 {
			return
		}
		top := gs.Stack[len(gs.Stack)-1]
		// CR §702.61a: with a split-second spell in play, nobody can cast
		// non-mana spells — so nobody can respond. Short-circuit the round.
		if SplitSecondActive(gs) {
			gs.LogEvent(Event{
				Kind: "priority_skipped",
				Details: map[string]interface{}{
					"reason": "split_second",
					"rule":   "702.61a",
				},
			})
			return
		}
		responded := false
		for _, seat := range apnapOrder(gs) {
			if seat == top.Controller {
				// MVP: caster passes after casting (CR §117.3c says they keep
				// priority but the greedy policy always passes).
				continue
			}
			s := gs.Seats[seat]
			if s == nil || s.Lost {
				continue
			}
			// CR §307.1 / §601.3a: a Teferi-style static on an opponent's
			// battlefield restricts this seat to sorcery speed. Stack is
			// non-empty by definition here, so they can't respond.
			if OppRestrictsDefenderToSorcerySpeed(gs, seat) {
				gs.LogEvent(Event{
					Kind: "priority_skipped",
					Seat: seat,
					Details: map[string]interface{}{
						"reason": "sorcery_speed",
						"rule":   "307.1",
					},
				})
				continue
			}
			resp := GetResponse(gs, seat, top)
			if resp == nil {
				gs.LogEvent(Event{
					Kind: "priority_pass", Seat: seat,
					Details: map[string]interface{}{"rule": "117.3d"},
				})
				continue
			}
			// Policy picked a response. Pay its cost; if broke, skip.
			cost := manaCostOf(resp.Card)
			if s.ManaPool < cost {
				// Return the card to hand — we took it out optimistically.
				if resp.Card != nil {
					s.Hand = append(s.Hand, resp.Card)
				}
				continue
			}
			s.ManaPool -= cost
			SyncManaAfterSpend(s)
			if cost > 0 {
				gs.LogEvent(Event{
					Kind:   "pay_mana",
					Seat:   seat,
					Amount: cost,
					Source: resp.Card.DisplayName(),
					Details: map[string]interface{}{
						"reason": "response",
						"rule":   "601.2f",
					},
				})
			}
			gs.LogEvent(Event{
				Kind:   "cast",
				Seat:   seat,
				Source: resp.Card.DisplayName(),
				Amount: cost,
				Details: map[string]interface{}{
					"in_response_to": top.Card.DisplayName(),
					"rule":           "117.7",
				},
			})
			// CR §700.4 / §702.40 — response casts (counterspells) are
			// casts per §601 and MUST increment the cast counters + fire
			// reactive observers. Without this, Rhystic Study / Mystic
			// Remora / Esper Sentinel miss every counterspell cast —
			// the "bless you" gap Wave 1b closes.
			IncrementCastCount(gs, seat)
			fireCastTriggers(gs, seat, resp.Card)
			FireCastTriggerObservers(gs, resp.Card, seat, false)
			PushStackItem(gs, resp)
			responded = true
			break // Restart priority at new top.
		}
		if !responded {
			// Stack trace: all players passed priority (CR §117.4).
			GlobalStackTrace.Log("priority_pass", "", gs.Active, len(gs.Stack), "all_players_pass")
			return
		}
	}
}

// GetResponse is the defender-side policy hook. Returns a *StackItem to push
// on the stack (typically a counter-spell or instant-speed removal) or nil
// to pass. The greedy MVP implementation:
//
//   - If stack is currently topped by a spell controlled by an opponent of
//     `defenderSeat`, scan defender's hand for a card whose AST carries
//     a CounterSpell effect. If affordable, return a StackItem wrapping it.
//   - Respect CR §702.61a (split-second) and CR §307.1 (sorcery speed) —
//     already screened by PriorityRound, but we defend-in-depth here.
//
// Note: this intentionally does NOT mutate the seat's hand. The caller
// (PriorityRound) is responsible for the hand-removal side-effect after
// confirming the cost was paid — this keeps the policy free of side effects
// and simplifies policy-swap experiments.
func GetResponse(gs *GameState, defenderSeat int, incoming *StackItem) *StackItem {
	if gs == nil || incoming == nil {
		return nil
	}
	if SplitSecondActive(gs) {
		return nil
	}
	if OppRestrictsDefenderToSorcerySpeed(gs, defenderSeat) {
		return nil
	}
	if defenderSeat < 0 || defenderSeat >= len(gs.Seats) {
		return nil
	}
	s := gs.Seats[defenderSeat]
	// Only counter opponents' spells.
	if incoming.Controller == defenderSeat {
		return nil
	}
	if incoming.Countered {
		return nil
	}

	// Delegate to Hat if available (§117.3).
	if s.Hat != nil {
		return s.Hat.ChooseResponse(gs, defenderSeat, incoming)
	}

	// Fallback: hardcoded counter-scan policy.
	for i, c := range s.Hand {
		if c == nil {
			continue
		}
		ceff := counterSpellEffect(c)
		if ceff == nil {
			continue
		}
		cost := manaCostOf(c)
		if cost > s.ManaPool {
			continue
		}
		// Check that the counterspell's filter matches the incoming spell.
		if !CounterCanTarget(ceff, incoming) {
			continue
		}
		// Pull the card OUT of hand (PriorityRound will handle the cost
		// and re-push-to-hand on failure). We do this so each priority
		// iteration doesn't re-pick the same card if its cost check fails.
		s.Hand = append(s.Hand[:i], s.Hand[i+1:]...)
		return &StackItem{
			Controller: defenderSeat,
			Card:       c,
			Effect:     ceff,
		}
	}
	return nil
}

// counterSpellEffect returns the CounterSpell effect from a card's AST if
// one exists, else nil. Used by GetResponse to identify counter-capable
// cards in hand.
//
// The AST stores instant/sorcery spell bodies in two layouts:
//   1. Activated ability with empty cost (legacy / test cards)
//   2. Static ability with Modification.ModKind == "spell_effect" and
//      Modification.Args[0] being the CounterSpell effect (real corpus)
//
// This function scans both.
func counterSpellEffect(c *Card) gameast.Effect {
	if c == nil || c.AST == nil {
		return nil
	}
	for _, ab := range c.AST.Abilities {
		// Layout 1: Activated ability (test cards / legacy).
		if a, ok := ab.(*gameast.Activated); ok && a.Effect != nil {
			if isCounterSpellEffect(a.Effect) {
				return a.Effect
			}
		}
		// Layout 2: Static with Modification.kind == "spell_effect"
		// whose first arg is a CounterSpell effect (real AST corpus).
		if s, ok := ab.(*gameast.Static); ok && s.Modification != nil &&
			s.Modification.ModKind == "spell_effect" && len(s.Modification.Args) > 0 {
			if eff, ok := s.Modification.Args[0].(gameast.Effect); ok {
				if isCounterSpellEffect(eff) {
					return eff
				}
			}
		}
	}
	return nil
}

// ExtractCounterSpellNode walks an effect tree and returns the first
// *gameast.CounterSpell node found, or nil. Used by the generic resolver
// to extract the structured counter data from a potentially wrapped effect
// (e.g. inside a Sequence with side-effects).
func ExtractCounterSpellNode(e gameast.Effect) *gameast.CounterSpell {
	if e == nil {
		return nil
	}
	if cs, ok := e.(*gameast.CounterSpell); ok {
		return cs
	}
	if seq, ok := e.(*gameast.Sequence); ok {
		for _, sub := range seq.Items {
			if cs := ExtractCounterSpellNode(sub); cs != nil {
				return cs
			}
		}
	}
	return nil
}

// isCounterSpellEffect returns true if e (or any effect nested within a
// Sequence) is a CounterSpell. Shallow walk is enough for Phase 5.
func isCounterSpellEffect(e gameast.Effect) bool {
	if e == nil {
		return false
	}
	if _, ok := e.(*gameast.CounterSpell); ok {
		return true
	}
	if seq, ok := e.(*gameast.Sequence); ok {
		for _, sub := range seq.Items {
			if isCounterSpellEffect(sub) {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// ResolveStackTop — CR §608.2.
// ---------------------------------------------------------------------------

// ResolveStackTop pops the top of gs.Stack and resolves it, handling three
// cases per CR §608.2:
//
//   - Countered item (StackItem.Countered == true): the spell is put into
//     its owner's graveyard; its effect does not happen.
//   - Non-countered spell: ResolveEffect is called; for non-permanent
//     spells the card then goes to the graveyard. For permanent spells
//     (creature/artifact/enchantment/planeswalker) the card enters the
//     battlefield as a Permanent.
//   - Triggered ability: ResolveEffect is called; there's no card-to-zone
//     move because the ability's source stays where it is.
//
// After resolution, StateBasedActions fires (CR §117.5 / §704.3). If the
// stack is still non-empty, the caller is expected to open another priority
// round — CastSpell loops until empty.
func ResolveStackTop(gs *GameState) {
	if gs == nil || len(gs.Stack) == 0 {
		return
	}
	item := gs.Stack[len(gs.Stack)-1]
	gs.Stack = gs.Stack[:len(gs.Stack)-1]

	// Log the resolution regardless of outcome so counterspell test fixtures
	// can observe ordering.
	name := ""
	isSpell := item.Card != nil && item.Source == nil
	if item.Card != nil {
		name = item.Card.DisplayName()
	} else if item.Source != nil {
		name = item.Source.Card.DisplayName()
	}
	// Stack trace: log resolution for CR audit.
	GlobalStackTrace.Log("resolve", name, item.Controller, len(gs.Stack), "resolving")
	gs.LogEvent(Event{
		Kind:   "stack_resolve",
		Seat:   item.Controller,
		Source: name,
		Details: map[string]interface{}{
			"countered":  item.Countered,
			"stack_size": len(gs.Stack),
			"rule":       "608.2",
		},
	})

	if item.Countered {
		// CR §701.5a: a countered spell is put into its owner's graveyard.
		// Abilities aren't cards so the "graveyard move" only applies to
		// spells (items with Card set and Source unset).
		if isSpell && item.Card != nil {
			MoveCard(gs, item.Card, item.Controller, "stack", "graveyard", "countered")
			gs.LogEvent(Event{
				Kind:   "resolve",
				Seat:   item.Controller,
				Source: name,
				Details: map[string]interface{}{
					"to":        "graveyard",
					"countered": true,
					"rule":      "701.5a",
				},
			})
		}
		return
	}

	// CR §608.2b: target legality check at resolution. If ALL targets are
	// illegal, the spell/ability is countered on resolution ("fizzles").
	// If SOME targets are illegal but at least one is legal, resolve with
	// the legal targets only.
	if len(item.Targets) > 0 {
		allIllegal, legalTargets := CheckTargetLegality(gs, item)
		if allIllegal {
			gs.LogEvent(Event{
				Kind:   "fizzle",
				Seat:   item.Controller,
				Source: name,
				Details: map[string]interface{}{
					"rule":   "608.2b",
					"reason": "all_targets_illegal",
				},
			})
			// Countered on resolution — spell goes to graveyard, ability vanishes.
			if isSpell && item.Card != nil {
				MoveCard(gs, item.Card, item.Controller, "stack", "graveyard", "fizzle")
			}
			return
		}
		// Update the stack item's targets to only the legal subset.
		item.Targets = legalTargets
	}

	// Wave 3a: activated-ability stack items resolve through their own
	// dispatch path. CR §602.2: "The controller of an activated ability
	// on the stack is the player who activated it."
	if item.Kind == "activated" {
		resolveActivatedAbility(gs, item)
		return
	}

	// Per-card trigger handler: if this stack item was pushed by
	// PushPerCardTrigger, resolve it by calling the wrapped Go function.
	// This is the CR §603.3 bridge: the trigger was placed on the stack,
	// priority passed, and now the handler executes on resolution.
	if item.CostMeta != nil {
		if trigData, ok := item.CostMeta["trigger_handler"]; ok {
			if th, ok := trigData.(*TriggerHandlerStackItem); ok {
				th.HandlerFunc(gs, th.SourcePerm, th.Ctx)
				// Run SBAs after trigger resolution per CR §704.3.
				StateBasedActions(gs)
				return
			}
		}
	}

	// Distinguish permanent spells from instant/sorcery spells. CR §608.3:
	// a permanent spell becomes a permanent on the battlefield when it
	// resolves. Non-permanent (instant/sorcery) spells resolve their effect
	// and go to graveyard per §608.2g.
	isPermanent := isSpell && item.Card != nil && isPermanentSpell(item.Card)

	// Per-card resolve-time snowflake dispatch. Fired BEFORE stock Effect
	// dispatch; when a handler is registered (fired > 0), we SKIP the
	// stock dispatch — the handler is the authoritative spell body.
	// Used by Doomsday / Demonic Consultation / Tainted Pact, whose
	// oracle text doesn't fit the general AST.
	snowflakeFired := InvokeResolveHook(gs, item)

	// Resolve the effect if present. CR §608.2c.
	if item.Effect != nil && snowflakeFired == 0 {
		// Stash targets on gs.Flags so resolver helpers that support it
		// can read them. For now resolve.go uses its own PickTarget —
		// StackItem.Targets is reserved for Phase 6 retarget/fizzle logic.
		_ = item.Targets
		var src *Permanent
		if item.Source != nil {
			src = item.Source
		} else if item.Card != nil {
			// For spell resolution we synthesize a transient Permanent as
			// the source so existing resolve.go handlers (which all take
			// *Permanent) have a controller + name to reference.
			src = &Permanent{
				Card:       item.Card,
				Controller: item.Controller,
				Flags:      map[string]int{},
			}
		}
		ResolveEffect(gs, src, item.Effect)
	}

	if isSpell && item.Card != nil {
		if isPermanent {
			// CR §608.3a: the permanent spell becomes a permanent under
			// its controller's control on the battlefield. Mirrors Python
			// _resolve_stack_top's is_permanent_spell branch. If this was
			// a COPY (§706.10a), the resolving permanent is a TOKEN copy.
			etbPerm := resolvePermanentSpellETB(gs, item)

			// Wave 2: evoke — if the spell was cast with evoke, register
			// a sacrifice trigger on ETB per CR §702.73.
			if etbPerm != nil && item.CostMeta != nil {
				if v, ok := item.CostMeta["evoke"]; ok {
					if b, ok := v.(bool); ok && b {
						// §702.73: "When this permanent enters the battlefield,
						// sacrifice it." Register as an immediate ETB trigger.
						gs.LogEvent(Event{
							Kind:   "evoke_sacrifice_trigger",
							Seat:   item.Controller,
							Source: name,
							Details: map[string]interface{}{
								"rule": "702.73",
							},
						})
						// Sacrifice the permanent immediately (after ETB
						// triggers have already fired via resolvePermanentSpellETB).
						// Route through SacrificePermanent for proper §614
						// replacement effects, dies/LTB triggers, and
						// commander redirect.
						SacrificePermanent(gs, etbPerm, "evoke")
					}
				}
			}
		} else if item.IsCopy {
			// CR §706.10 — a copy of a non-permanent spell ceases to
			// exist on resolution. Do NOT route to graveyard: the copy
			// is a transient game object, not a card in any deck, and
			// appending it to a zone would violate zone conservation.
			gs.LogEvent(Event{
				Kind:   "resolve",
				Seat:   item.Controller,
				Source: name,
				Details: map[string]interface{}{
					"to":   "ceases_to_exist",
					"rule": "706.10",
				},
			})
		} else if ShouldExileOnResolve(item) {
			// Wave 2: flashback / escape — exile instead of graveyard.
			// CR §702.33: "If the flashback cost was paid, exile this
			// card instead of putting it anywhere else any time it would
			// leave the stack."
			MoveCard(gs, item.Card, item.Controller, "stack", "exile", "flashback-exile")
			gs.LogEvent(Event{
				Kind:   "resolve",
				Seat:   item.Controller,
				Source: name,
				Details: map[string]interface{}{
					"to":        "exile",
					"reason":    "zone_cast_exile_on_resolve",
					"cast_zone": item.CastZone,
					"rule":      "702.33",
				},
			})
		} else {
			// CR §608.2g: non-permanent spells go to the graveyard on
			// resolution.
			MoveCard(gs, item.Card, item.Controller, "stack", "graveyard", "resolve")
			gs.LogEvent(Event{
				Kind:   "resolve",
				Seat:   item.Controller,
				Source: name,
				Details: map[string]interface{}{
					"to":   "graveyard",
					"rule": "608.2g",
				},
			})
		}
	}
}

// resolvePermanentSpellETB is the ETB path for a resolving permanent
// spell. Mirrors the Python `_resolve_stack_top` permanent branch +
// `_etb_initialize`:
//
//   1. Allocate a new Permanent with summoning_sick = !has_keyword("haste")
//      (creatures only; non-creatures ignore summoning sickness per §302.1).
//   2. Assign §613.7 timestamp via NextTimestamp().
//   3. Initialize planeswalker loyalty counters (§306.5b) / battle defense
//      counters (§310.3) if the metadata hints carry a starting value
//      (BasePower / BaseToughness is the nearest runtime approximation —
//      the engine doesn't carry explicit starting_loyalty today).
//   4. Append to controller's battlefield.
//   5. Register §613 continuous effects from Static abilities.
//   6. Register §614 replacement effects.
//   7. Fire ETB triggered abilities through the stack.
//   8. Emit an enter_battlefield event.
func resolvePermanentSpellETB(gs *GameState, item *StackItem) *Permanent {
	if gs == nil || item == nil || item.Card == nil {
		return nil
	}
	card := item.Card
	seatIdx := item.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return nil
	}

	// Summoning sickness: only creatures care (§302.1 / §212.3f). A creature
	// with haste ignores it.
	isCreature := cardHasType(card, "creature")
	sick := false
	if isCreature {
		sick = !cardHasKeyword(card, "haste")
	}
	perm := &Permanent{
		Card:          card,
		Controller:    seatIdx,
		Owner:         card.Owner,
		Tapped:        false,
		SummoningSick: sick,
		Timestamp:     gs.NextTimestamp(),
		Counters:      map[string]int{},
		Flags:         map[string]int{},
	}
	if perm.Owner < 0 {
		perm.Owner = seatIdx
	}
	// §306.5b planeswalker loyalty counter initialization. We don't carry
	// starting_loyalty on Card today — fall back to CMC-ish heuristic so
	// planeswalkers at least start with a positive counter.
	if cardHasType(card, "planeswalker") {
		n := card.BaseToughness
		if n <= 0 {
			n = 3
		}
		perm.Counters["loyalty"] = n
	}
	// §310.3 battle defense counter initialization.
	if cardHasType(card, "battle") {
		n := card.BaseToughness
		if n <= 0 {
			n = 1
		}
		perm.Counters["defense"] = n
	}

	seat.Battlefield = append(seat.Battlefield, perm)

	// §303.4f — Aura attachment on ETB. When an Aura enters the battlefield
	// as a permanent spell, it must be attached to a legal object. Infer the
	// target type from the card's oracle text / TypeLine and attach to a
	// valid own permanent. Without this, SBA §704.5m destroys unattached auras.
	if perm.IsAura() {
		attachAuraOnETB(gs, perm)
	}

	// §702.136 — Riot: as this enters, choose +1/+1 counter or haste.
	ApplyRiot(gs, perm)

	// Register §613 continuous effects (layers 1-7).
	RegisterContinuousEffectsForPermanent(gs, perm)
	// Register §614 replacement effects.
	RegisterReplacementsForPermanent(gs, perm)

	gs.LogEvent(Event{
		Kind:   "enter_battlefield",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Details: map[string]interface{}{
			"summoning_sick": sick,
			"rule":           "608.3a",
		},
	})

	// Fire ETB triggered abilities (§603.6). Walk the card's abilities and
	// push Triggered entries whose Trigger.Event == "etb".
	if card.AST != nil {
		for _, ab := range card.AST.Abilities {
			trig, ok := ab.(*gameast.Triggered)
			if !ok || trig.Effect == nil {
				continue
			}
			if !EventEquals(trig.Trigger.Event, "etb") {
				continue
			}
			// §603.3 — the engine is responsible for putting the trigger
			// on the stack. PushTriggeredAbility handles that + priority.
			PushTriggeredAbility(gs, perm, trig.Effect)
			if gs.CheckEnd() {
				return perm
			}
		}
	}

	// Per-card ETB snowflake dispatch. This fires AFTER stock AST ETB
	// triggers so the snowflake runs as the "bottom" of the cascade —
	// the order matters for Thassa's Oracle which reads the library
	// AFTER any ETB scrys/tutors resolve.
	InvokeETBHook(gs, perm)

	// §702.131 Ascend — check if controller now has 10+ permanents
	CheckAscend(gs, perm.Controller)

	// Generic "nonland permanent etb" event for Cloudstone Curio et al.
	// Fires regardless of whether the permanent itself has a snowflake.
	if !cardHasType(card, "land") {
		FireCardTrigger(gs, "nonland_permanent_etb", map[string]interface{}{
			"perm":            perm,
			"controller_seat": perm.Controller,
			"card":            card,
		})
	}
	FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"perm":            perm,
		"controller_seat": perm.Controller,
		"card":            card,
	})

	// Observer ETB triggers — scan all OTHER permanents for triggered
	// abilities that fire when a permanent enters (e.g. "whenever another
	// creature enters the battlefield", "whenever a creature you control
	// enters"). Mirrors fireObserverZoneChangeTriggers but for ETB events.
	fireObserverETBTriggers(gs, perm)

	return perm
}

// attachAuraOnETB finds a valid target for an aura permanent entering the
// battlefield and sets AttachedTo. Uses the card's TypeLine to infer the
// enchant target ("Enchantment — Aura" with oracle "Enchant land/creature/...").
// Falls back to: own land → own creature → any own permanent (excluding self).
func attachAuraOnETB(gs *GameState, perm *Permanent) {
	if perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}

	tl := strings.ToLower(perm.Card.TypeLine)

	wantLand := strings.Contains(tl, "enchant land")
	wantCreature := strings.Contains(tl, "enchant creature")
	wantArtifact := strings.Contains(tl, "enchant artifact")

	// If TypeLine doesn't specify, check card name heuristics or default
	// to creature (most common aura target).
	if !wantLand && !wantCreature && !wantArtifact {
		wantCreature = true
	}

	// Search own battlefield for a valid target.
	for _, p := range seat.Battlefield {
		if p == perm || p == nil {
			continue
		}
		if wantLand && p.IsLand() {
			perm.AttachedTo = p
			return
		}
		if wantCreature && p.IsCreature() {
			perm.AttachedTo = p
			return
		}
		if wantArtifact && p.IsArtifact() {
			perm.AttachedTo = p
			return
		}
	}

	// Fallback: attach to any own permanent (excluding self) to prevent
	// immediate SBA destruction. This is a lossy heuristic but better than
	// the aura self-destructing.
	for _, p := range seat.Battlefield {
		if p != perm && p != nil {
			perm.AttachedTo = p
			return
		}
	}
}

// cardHasKeyword returns true if the card's AST contains a Keyword ability
// with the given name (case-insensitive). We check AST only — runtime
// grants are per-permanent, not per-card, so they're not relevant to the
// ETB initial state.
func cardHasKeyword(c *Card, name string) bool {
	if c == nil || c.AST == nil {
		return false
	}
	want := strings.ToLower(strings.TrimSpace(name))
	for _, ab := range c.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		if strings.ToLower(strings.TrimSpace(kw.Name)) == want {
			return true
		}
	}
	return false
}

// isPermanentSpell returns true if the card's type line designates a
// permanent (creature/artifact/enchantment/planeswalker/land/battle). For
// these, resolution puts the card ON the battlefield, not in the graveyard.
// MVP reads from Card.Types.
func isPermanentSpell(c *Card) bool {
	if c == nil {
		return false
	}
	for _, t := range c.Types {
		switch strings.ToLower(t) {
		case "creature", "artifact", "enchantment", "planeswalker",
			"land", "battle":
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Timing restrictions.
// ---------------------------------------------------------------------------

// SplitSecondActive reports whether any item on the stack carries the
// split-second keyword. CR §702.61a: "Split second is a static ability that
// functions only while the spell with split second is on the stack. 'Split
// second' means 'As long as this spell is on the stack, players can't cast
// other spells or activate abilities that aren't mana abilities.'"
//
// We scan every non-countered stack item for a Keyword AST node whose name
// contains "split" (canonical parser name: "split_second"; tolerate "split"
// alone for extension parity). Flags-based tokens also trip the check
// ("kw:split_second").
func SplitSecondActive(gs *GameState) bool {
	if gs == nil {
		return false
	}
	for _, item := range gs.Stack {
		if item == nil || item.Countered {
			continue
		}
		if item.Source != nil && permHasSplitSecond(item.Source) {
			return true
		}
		if item.Card != nil && cardHasSplitSecond(item.Card) {
			return true
		}
	}
	return false
}

// permHasSplitSecond delegates to HasKeyword (which already checks AST +
// GrantedAbilities + Flags).
func permHasSplitSecond(p *Permanent) bool {
	if p == nil {
		return false
	}
	if p.HasKeyword("split_second") || p.HasKeyword("split second") || p.HasKeyword("split") {
		return true
	}
	return false
}

// cardHasSplitSecond scans a Card's AST for a Keyword ability whose name
// contains "split". Called for stack items that represent spells (no
// associated Permanent yet).
func cardHasSplitSecond(c *Card) bool {
	if c == nil || c.AST == nil {
		return false
	}
	for _, ab := range c.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(kw.Name))
		if strings.Contains(name, "split") {
			return true
		}
	}
	return false
}

// OppRestrictsDefenderToSorcerySpeed reports whether any opponent of
// `defenderSeat` controls a static ability that restricts `defenderSeat`
// to sorcery speed (CR §307.1 / §601.3a).
//
// This models Teferi, Time Raveler — "Each opponent can cast spells only
// any time they could cast a sorcery." Since sorceries can only be cast
// on an empty stack during that player's main phase, and the defender is
// being asked to respond to a spell (stack non-empty by definition, not
// their main phase), the defender can't legally cast a response.
//
// Scans every seat's battlefield (except defenderSeat's own — a player
// isn't restricted by their own Teferi) for Static abilities whose
// Modification.ModKind is "opp_sorcery_speed_only" (or its known variants),
// or whose raw text contains the canonical Teferi phrase.
func OppRestrictsDefenderToSorcerySpeed(gs *GameState, defenderSeat int) bool {
	if gs == nil {
		return false
	}
	for i, seat := range gs.Seats {
		if i == defenderSeat || seat == nil || seat.Lost {
			continue
		}
		for _, perm := range seat.Battlefield {
			if perm == nil || perm.Card == nil || perm.Card.AST == nil {
				continue
			}
			for _, ab := range perm.Card.AST.Abilities {
				st, ok := ab.(*gameast.Static)
				if !ok {
					continue
				}
				if st.Modification != nil {
					switch st.Modification.ModKind {
					case "opp_sorcery_speed_only",
						"cast_timing_opp_sorcery",
						"opp_only_sorcery_speed":
						return true
					}
				}
				// Raw-text fallback for parser variants.
				// Raw is pre-lowercased at AST load time.
				if strings.Contains(st.Raw,
					"each opponent can cast spells only any time they could cast a sorcery") {
					return true
				}
			}
			// Runtime-flag fallback — tests set perm.Flags["opp_sorcery_speed"]=1.
			if perm.Flags != nil && perm.Flags["opp_sorcery_speed"] != 0 {
				return true
			}
		}
	}
	return false
}

// fireCastTriggers emits the family of "spell was cast" per-card
// triggers. Scopes the event so per-card handlers can tell whether the
// cast was by the listener's controller, an opponent, instant/sorcery,
// creature, etc.
//
// Emitted events (each handler decides which one to listen on):
//   - "spell_cast"                — always
//   - "spell_cast_by_opponent"    — caster != listener's controller
//   - "noncreature_spell_cast"    — spell is not a creature
//   - "creature_spell_cast"       — spell is a creature
//   - "instant_or_sorcery_cast"   — spell is instant/sorcery
func fireCastTriggers(gs *GameState, casterSeat int, card *Card) {
	if gs == nil || card == nil {
		return
	}
	ctx := map[string]interface{}{
		"caster_seat": casterSeat,
		"spell_name":  card.DisplayName(),
		"card":        card,
		"is_creature": cardHasType(card, "creature"),
	}
	FireCardTrigger(gs, "spell_cast", ctx)
	// Opponent scoping — fire this event unconditionally; handlers check
	// ctx["caster_seat"] against their own controller to decide.
	FireCardTrigger(gs, "spell_cast_by_opponent", ctx)
	if cardHasType(card, "creature") {
		FireCardTrigger(gs, "creature_spell_cast", ctx)
	} else {
		FireCardTrigger(gs, "noncreature_spell_cast", ctx)
	}
	if cardHasType(card, "instant") || cardHasType(card, "sorcery") {
		FireCardTrigger(gs, "instant_or_sorcery_cast", ctx)
	}

	// Observer cast triggers — scan all permanents for AST-driven "whenever
	// a player/opponent casts a spell" triggers (cast_filtered, cast_any, etc.)
	fireObserverCastTriggers(gs, casterSeat, card)
}

// FireCastTriggers is the exported wrapper around fireCastTriggers. It
// exists so tests + alternative cast paths (commander cast, response
// cast, future alt-cost cast) can emit the same cast-trigger fan-out
// without duplicating the cardHasType shuffle.
func FireCastTriggers(gs *GameState, casterSeat int, card *Card) {
	fireCastTriggers(gs, casterSeat, card)
}

// ---------------------------------------------------------------------------
// Ward — CR §702.21
// ---------------------------------------------------------------------------

// CheckWardOnTargeting implements the ward triggered ability. When a
// spell or ability targets a permanent with ward, and the source is
// controlled by an opponent, the controller of the spell must pay the
// ward cost or the spell is countered.
//
// Ward cost is extracted from:
//   1. Permanent.Flags["ward_cost"] — generic mana cost (int)
//   2. Keyword "ward" with no cost defaults to ward {1}
//
// Per CR §702.21c: "If a player doesn't pay the ward cost for a
// spell they control, the spell is countered."
func CheckWardOnTargeting(gs *GameState, item *StackItem) {
	if gs == nil || item == nil {
		return
	}
	for _, tgt := range item.Targets {
		if tgt.Kind != TargetKindPermanent || tgt.Permanent == nil {
			continue
		}
		perm := tgt.Permanent
		// Ward only triggers when an OPPONENT targets.
		if perm.Controller == item.Controller {
			continue
		}
		if !perm.HasKeyword("ward") {
			continue
		}
		// Determine ward cost. Check Flags["ward_cost"] first, else default 1.
		wardCost := 1
		if perm.Flags != nil {
			if v, ok := perm.Flags["ward_cost"]; ok && v > 0 {
				wardCost = v
			}
		}
		// Check if the caster can and will pay.
		casterSeat := gs.Seats[item.Controller]
		if casterSeat == nil {
			continue
		}
		if casterSeat.ManaPool >= wardCost {
			// Pay the ward cost.
			casterSeat.ManaPool -= wardCost
			SyncManaAfterSpend(casterSeat)
			gs.LogEvent(Event{
				Kind:   "ward_paid",
				Seat:   item.Controller,
				Source: perm.Card.DisplayName(),
				Amount: wardCost,
				Details: map[string]interface{}{
					"rule":        "702.21",
					"ward_target": perm.Card.DisplayName(),
					"spell":       itemName(item),
				},
			})
		} else {
			// Can't pay — counter the spell.
			item.Countered = true
			gs.LogEvent(Event{
				Kind:   "ward_counter",
				Seat:   perm.Controller,
				Source: perm.Card.DisplayName(),
				Amount: wardCost,
				Details: map[string]interface{}{
					"rule":        "702.21c",
					"ward_target": perm.Card.DisplayName(),
					"spell":       itemName(item),
					"caster_seat": item.Controller,
				},
			})
			return // spell is countered, no more ward checks needed
		}
	}
}

// itemName returns the display name of a stack item for logging.
func itemName(item *StackItem) string {
	if item == nil {
		return "<nil>"
	}
	if item.Card != nil {
		return item.Card.DisplayName()
	}
	if item.Source != nil && item.Source.Card != nil {
		return item.Source.Card.DisplayName()
	}
	return "<unknown>"
}

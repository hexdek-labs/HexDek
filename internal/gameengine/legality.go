package gameengine

// legality.go — ride-along rules-legality validator (r62, owner design
// from 7174n1c).
//
// The post-hoc invariant census (invariants.go) checks that the RESULTING
// game state is consistent — it is structurally blind to actions that were
// illegal at the moment they were taken but leave a legal-looking board
// behind (a sorcery cast on an opponent's turn, a spell targeting something
// its filter forbids, a cost announced but never deducted). This validator
// observes each player action AT THE MOMENT IT HAPPENS and re-derives its
// legality independently of the engine's own gates — the whole point is to
// catch the cases where those gates are wrong or bypassed.
//
// Activation model:
//
//	gs.Legality == nil          → validator off (the default). Every hook
//	                              is a method on a possibly-nil receiver
//	                              that returns immediately — one nil check
//	                              per action, zero allocations, zero
//	                              behavior change.
//	gs.Legality = NewLegalityValidator(seed)
//	                            → ride-along on. Violations accumulate on
//	                              the validator (capped) and are also
//	                              emitted as "legality_violation" events.
//
// CloneForRollout constructs its clone field-by-field and deliberately
// does NOT copy this field: MCTS rollouts explore hypothetical (often
// deliberately silly) lines and must not pollute the violation stream —
// and the validator is not safe for the rollout goroutine fan-out.
//
// Observation lifecycle: the action site calls Begin* before paying any
// cost (capturing phase/active/stack-depth/pool, i.e. announcement-time
// state per CR §601.2 / §602.2), holds the returned observation in a
// local, and calls Finish* after the action is on the stack. Error
// returns in between simply abandon the observation — Finish pops the
// active-observation stack down to its own entry, so abandoned frames
// cannot mis-attribute later mana adds. AddMana/AddRestrictedMana report
// pool additions so cast-trigger mana (Birgi, Storm-Kiln) inside the
// announcement window doesn't read as an under-payment.
//
// Extensibility: checks are registered as LegalityCheck{Name, Fn} on the
// validator. NewLegalityValidator installs the phase-1 set (timing,
// targets, cost-paid); callers append their own with Register. A check
// receives the completed observation plus the live GameState and returns
// zero or more violations.

import (
	"fmt"
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// LegalityViolation is one observed-illegal action, with enough context
// to reproduce: re-run the seed, jump to the turn, watch the seat.
type LegalityViolation struct {
	Seed   int64  // engine RNG seed of the game (repro key)
	Turn   int    // gs.Turn at the moment of the action
	Seat   int    // acting seat
	Action string // "cast:<card>" or "activate:<card>#<abilityIdx>"
	Rule   string // CR citation, e.g. "307.1", "608.2c", "601.2f-h"
	Detail string // human-readable specifics
}

func (v LegalityViolation) String() string {
	return fmt.Sprintf("[legality %s] seed=%d turn=%d seat=%d %s: %s",
		v.Rule, v.Seed, v.Turn, v.Seat, v.Action, v.Detail)
}

// LegalityObservation captures one announced action: state snapshotted at
// announcement (Begin*) plus the outcome at completion (Finish*). Checks
// read this instead of re-deriving "what just happened" from the live
// state, which by Finish time already reflects the action.
type LegalityObservation struct {
	Kind       string // "cast" | "activate"
	Seat       int
	Card       *Card
	Perm       *Permanent // activation source (nil for casts)
	AbilityIdx int        // activations only

	// Announcement-time snapshot (Begin*).
	TurnAtAnnounce       int
	PhaseAtAnnounce      string
	ActiveAtAnnounce     int
	StackDepthAtAnnounce int
	PoolBefore           int
	BaseCostAtAnnounce   int  // casts: CalculateTotalCost before any payment
	AbilityManaCost      int  // activations: announced mana component
	ZoneCastGranted      bool // card had a ZoneCastGrants permission ("as though" timing may apply)

	// Completion snapshot (Finish*).
	Item                  *StackItem // nil for inline mana abilities
	PoolAfter             int
	ManaAddedDuringWindow int // pool additions credited to Seat between Begin and Finish
}

// ActionLabel renders the canonical Action string for violations.
func (o *LegalityObservation) ActionLabel() string {
	name := "<unknown>"
	if o.Card != nil {
		name = o.Card.DisplayName()
	}
	if o.Kind == "activate" {
		return fmt.Sprintf("activate:%s#%d", name, o.AbilityIdx)
	}
	return "cast:" + name
}

// LegalityCheck is one registered rule check.
type LegalityCheck struct {
	Name string
	Fn   func(gs *GameState, obs *LegalityObservation) []LegalityViolation
}

// LegalityValidator is the ride-along container. One per game, attached
// to gs.Legality by an opted-in runner (loki -legality, tests).
type LegalityValidator struct {
	Seed          int64
	Checks        []LegalityCheck
	Violations    []LegalityViolation
	MaxViolations int // hard cap on stored violations (memory bound)

	// active is the stack of in-flight observations (announcement begun,
	// not yet finished). Casting can nest observations only via re-entrant
	// engine paths; stack discipline plus pop-until-self in Finish keeps
	// abandoned frames (error returns between Begin and Finish) from
	// leaking or mis-attributing mana adds.
	active []*LegalityObservation
}

// NewLegalityValidator builds a validator with the phase-1 check set.
func NewLegalityValidator(seed int64) *LegalityValidator {
	v := &LegalityValidator{Seed: seed, MaxViolations: 2000}
	v.Register(LegalityCheck{Name: "timing", Fn: checkLegalityTiming})
	v.Register(LegalityCheck{Name: "targets", Fn: checkLegalityTargets})
	v.Register(LegalityCheck{Name: "cost-paid", Fn: checkLegalityCostPaid})
	return v
}

// Register appends a rule check. Safe to call any time before the game.
func (v *LegalityValidator) Register(c LegalityCheck) {
	if v == nil || c.Fn == nil {
		return
	}
	v.Checks = append(v.Checks, c)
}

func (v *LegalityValidator) record(gs *GameState, viol LegalityViolation) {
	if v == nil {
		return
	}
	if viol.Seed == 0 {
		viol.Seed = v.Seed
	}
	if len(v.Violations) < v.MaxViolations {
		v.Violations = append(v.Violations, viol)
	}
	if gs != nil {
		gs.LogEvent(Event{
			Kind:   "legality_violation",
			Seat:   viol.Seat,
			Source: viol.Action,
			Details: map[string]interface{}{
				"rule":   viol.Rule,
				"detail": viol.Detail,
				"seed":   viol.Seed,
			},
		})
	}
}

// ---------------------------------------------------------------------------
// Engine-facing hooks (all nil-receiver safe)
// ---------------------------------------------------------------------------

// BeginCast snapshots announcement-time state for a spell cast. Called by
// CastSpell after its own early gates, before any cost is paid. Returns
// nil when the validator is off.
func (v *LegalityValidator) BeginCast(gs *GameState, seatIdx int, card *Card) *LegalityObservation {
	if v == nil || gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	seat := gs.Seats[seatIdx]
	obs := &LegalityObservation{
		Kind:                 "cast",
		Seat:                 seatIdx,
		Card:                 card,
		TurnAtAnnounce:       gs.Turn,
		PhaseAtAnnounce:      gs.Phase,
		ActiveAtAnnounce:     gs.Active,
		StackDepthAtAnnounce: len(gs.Stack),
		PoolBefore:           EnsureTypedPool(seat).Total(),
		BaseCostAtAnnounce:   CalculateTotalCost(gs, card, seatIdx),
	}
	if card != nil && gs.ZoneCastGrants != nil {
		_, obs.ZoneCastGranted = gs.ZoneCastGrants[card]
	}
	v.pushActive(obs)
	return obs
}

// FinishCast completes a cast observation (the spell is on the stack with
// its CostMeta stamped; replicate/conspire/casualty and ward have not run
// yet) and runs every registered check.
func (v *LegalityValidator) FinishCast(gs *GameState, obs *LegalityObservation, item *StackItem) {
	if v == nil || obs == nil || gs == nil {
		return
	}
	v.popActiveThrough(obs)
	obs.Item = item
	if obs.Seat >= 0 && obs.Seat < len(gs.Seats) {
		obs.PoolAfter = EnsureTypedPool(gs.Seats[obs.Seat]).Total()
	}
	v.runChecks(gs, obs)
}

// BeginActivation snapshots announcement-time state for an activated
// ability. Called by ActivateAbility after the stax gate, before any cost
// is paid. ab may be nil (per_card-only activations).
func (v *LegalityValidator) BeginActivation(gs *GameState, seatIdx int, perm *Permanent, abilityIdx int, ab *gameast.Activated) *LegalityObservation {
	if v == nil || gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	seat := gs.Seats[seatIdx]
	obs := &LegalityObservation{
		Kind:                 "activate",
		Seat:                 seatIdx,
		Perm:                 perm,
		AbilityIdx:           abilityIdx,
		TurnAtAnnounce:       gs.Turn,
		PhaseAtAnnounce:      gs.Phase,
		ActiveAtAnnounce:     gs.Active,
		StackDepthAtAnnounce: len(gs.Stack),
		PoolBefore:           EnsureTypedPool(seat).Total(),
	}
	if perm != nil {
		obs.Card = perm.Card
	}
	if ab != nil && ab.Cost.Mana != nil {
		obs.AbilityManaCost = ab.Cost.Mana.CMC()
	}
	v.pushActive(obs)
	return obs
}

// FinishActivation completes an activation observation. item is nil for
// inline mana abilities (CR §605.3a).
func (v *LegalityValidator) FinishActivation(gs *GameState, obs *LegalityObservation, item *StackItem) {
	if v == nil || obs == nil || gs == nil {
		return
	}
	v.popActiveThrough(obs)
	obs.Item = item
	if obs.Seat >= 0 && obs.Seat < len(gs.Seats) {
		obs.PoolAfter = EnsureTypedPool(gs.Seats[obs.Seat]).Total()
	}
	v.runChecks(gs, obs)
}

// NoteManaAdd credits a pool addition to every in-flight observation for
// that seat, so mana produced DURING an announcement window (cast-trigger
// mana like Birgi, inline mana abilities inside an activation) doesn't
// read as an under-payment in the cost check. Called from AddMana /
// AddRestrictedMana behind the same nil guard.
func (v *LegalityValidator) NoteManaAdd(seatIdx, amount int) {
	if v == nil || amount <= 0 {
		return
	}
	for _, obs := range v.active {
		if obs != nil && obs.Seat == seatIdx {
			obs.ManaAddedDuringWindow += amount
		}
	}
}

func (v *LegalityValidator) pushActive(obs *LegalityObservation) {
	// Prune abandoned frames from earlier turns (error-return leftovers).
	if len(v.active) > 0 && obs != nil {
		kept := v.active[:0]
		for _, a := range v.active {
			if a != nil && a.TurnAtAnnounce == obs.TurnAtAnnounce {
				kept = append(kept, a)
			}
		}
		v.active = kept
	}
	v.active = append(v.active, obs)
}

// popActiveThrough removes obs and anything pushed above it (abandoned
// nested frames whose action error-returned before Finish).
func (v *LegalityValidator) popActiveThrough(obs *LegalityObservation) {
	for i := len(v.active) - 1; i >= 0; i-- {
		if v.active[i] == obs {
			v.active = v.active[:i]
			return
		}
	}
}

func (v *LegalityValidator) runChecks(gs *GameState, obs *LegalityObservation) {
	for _, c := range v.Checks {
		for _, viol := range c.Fn(gs, obs) {
			v.record(gs, viol)
		}
	}
}

// ---------------------------------------------------------------------------
// Phase-1 checks
// ---------------------------------------------------------------------------

// legalityMainPhase reports whether the announcement-time phase counts as
// a main phase. "" is allowed for fixture-built states; the live turn
// runner sets gs.Phase = "main". "beginning" is deliberately NOT included
// (CR §505.1 — upkeep/draw are not sorcery-speed windows), diverging from
// CastSpell's own gate, which is exactly the kind of divergence this
// validator exists to surface.
func legalityMainPhase(phase string) bool {
	return phase == "" || phase == "main" ||
		phase == "precombat_main" || phase == "postcombat_main"
}

// legalityCardIsInstantSpeed mirrors the turn runner's isInstantSpeed:
// instant type, "flash" leaked into Types (tokens/copies), or a Flash
// keyword on the AST.
func legalityCardIsInstantSpeed(c *Card) bool {
	if c == nil {
		return false
	}
	for _, t := range c.Types {
		if strings.EqualFold(t, "instant") || strings.EqualFold(t, "flash") {
			return true
		}
	}
	if c.AST != nil {
		for _, ab := range c.AST.Abilities {
			if kw, ok := ab.(*gameast.Keyword); ok && strings.EqualFold(kw.Name, "flash") {
				return true
			}
		}
	}
	return false
}

// checkLegalityTiming — CR §307.1 (sorceries) and §117.1a (casting a
// non-flash permanent spell follows sorcery timing): a sorcery-speed
// spell may only be cast by the ACTIVE player, during a main phase, with
// an empty stack. The engine's own CastSpell gate checks only literal
// sorceries and never checks that the caster is the active player — the
// two gaps behind the "illegal casts" class observed live while
// spectating.
//
// Exemptions: zone-cast grants (impulse play / Opposition Agent — many
// carry "as though it had flash"-class permissions the grant registry
// doesn't expose granularly yet; exempting all grants trades a small
// false-negative surface for zero false positives on that path).
func checkLegalityTiming(_ *GameState, obs *LegalityObservation) []LegalityViolation {
	if obs.Kind != "cast" || obs.Card == nil {
		return nil
	}
	if legalityCardIsInstantSpeed(obs.Card) {
		return nil
	}
	if obs.ZoneCastGranted {
		return nil
	}
	sorcery := false
	for _, t := range obs.Card.Types {
		if strings.EqualFold(t, "sorcery") {
			sorcery = true
			break
		}
	}
	rule := "117.1a"
	if sorcery {
		rule = "307.1"
	}
	var out []LegalityViolation
	if obs.Seat != obs.ActiveAtAnnounce {
		out = append(out, LegalityViolation{
			Turn: obs.TurnAtAnnounce, Seat: obs.Seat, Action: obs.ActionLabel(), Rule: rule,
			Detail: fmt.Sprintf("sorcery-speed spell cast by non-active seat (active=%d)", obs.ActiveAtAnnounce),
		})
	}
	if !legalityMainPhase(obs.PhaseAtAnnounce) {
		out = append(out, LegalityViolation{
			Turn: obs.TurnAtAnnounce, Seat: obs.Seat, Action: obs.ActionLabel(), Rule: rule,
			Detail: fmt.Sprintf("sorcery-speed spell cast outside a main phase (phase=%q)", obs.PhaseAtAnnounce),
		})
	}
	if obs.StackDepthAtAnnounce > 0 {
		out = append(out, LegalityViolation{
			Turn: obs.TurnAtAnnounce, Seat: obs.Seat, Action: obs.ActionLabel(), Rule: rule,
			Detail: fmt.Sprintf("sorcery-speed spell cast with %d item(s) on the stack", obs.StackDepthAtAnnounce),
		})
	}
	return out
}

// checkLegalityTargets — CR §608.2c / §601.2c. Re-validates the FINAL
// announced target list (including targets stamped by the r62
// announcement path after the engine's own caller-supplied-target
// validation already ran): every target must still exist and be legally
// targetable, and permanent targets must satisfy the effect's targeting
// filter when one is recoverable from the stack item. The engine's
// ValidateTargetsAtAnnouncement checks existence + protection but NOT
// filter satisfaction — "destroy target creature" pointed at a
// non-creature passes it.
func checkLegalityTargets(gs *GameState, obs *LegalityObservation) []LegalityViolation {
	if obs.Item == nil || len(obs.Item.Targets) == 0 {
		return nil
	}
	var out []LegalityViolation
	if err := ValidateTargetsAtAnnouncement(gs, obs.Seat, obs.Card, obs.Item.Targets, obs.Item); err != nil {
		out = append(out, LegalityViolation{
			Turn: obs.TurnAtAnnounce, Seat: obs.Seat, Action: obs.ActionLabel(), Rule: "608.2c",
			Detail: "announced target no longer legal: " + err.Error(),
		})
	}
	// Filter satisfaction for permanent targets, when the targeting
	// filter is recoverable from the item's effect shape.
	var filter gameast.Filter
	haveFilter := false
	if f, ok := announceTargetFilter(obs.Item); ok {
		filter, haveFilter = f, true
	} else if f, ok := requiredTargetFilter(obs.Item); ok {
		filter, haveFilter = f, true
	}
	if haveFilter {
		for _, t := range obs.Item.Targets {
			if t.Kind != TargetKindPermanent || t.Permanent == nil {
				continue
			}
			if !matchesPermanent(filter, t.Permanent) {
				name := "<unknown>"
				if t.Permanent.Card != nil {
					name = t.Permanent.Card.DisplayName()
				}
				out = append(out, LegalityViolation{
					Turn: obs.TurnAtAnnounce, Seat: obs.Seat, Action: obs.ActionLabel(), Rule: "601.2c",
					Detail: fmt.Sprintf("target %q does not satisfy the targeting restriction (filter base=%q)", name, filter.Base),
				})
			}
		}
	}
	return out
}

// checkLegalityCostPaid — CR §601.2f-h / §602.2: the announced total cost
// must equal what actually left the pool. Spent is measured as
// PoolBefore + ManaAddedDuringWindow - PoolAfter so cast-trigger mana
// (Birgi) and inline mana abilities don't skew the delta. Expected is
// reconstructed from the announcement snapshot plus the decisions stamped
// on the StackItem (X, kicker, buyback, alternative costs) — i.e. an
// INDEPENDENT reconstruction, so a double-deduction (the Chalice
// multikicker class) or an un-deducted announcement both flag.
func checkLegalityCostPaid(_ *GameState, obs *LegalityObservation) []LegalityViolation {
	spent := obs.PoolBefore + obs.ManaAddedDuringWindow - obs.PoolAfter

	expected := 0
	switch obs.Kind {
	case "activate":
		expected = obs.AbilityManaCost
	case "cast":
		expected = obs.BaseCostAtAnnounce
		if obs.Item != nil {
			cm := obs.Item.CostMeta
			// Alternative costs REPLACE the base (CR §601.2b / §118.9).
			if cm != nil {
				if b, _ := cm["overloaded"].(bool); b && obs.Card != nil {
					expected = OverloadCost(obs.Card)
				}
				if c, ok := cm["surge_cost"].(int); ok {
					expected = c
				}
				if c, ok := cm["spectacle_cost"].(int); ok {
					expected = c
				}
			}
			expected += obs.Item.ChosenX
			if cm != nil {
				if kicks, ok := cm["multikick_count"].(int); ok && kicks > 0 && obs.Card != nil {
					expected += kicks * KickerCost(obs.Card)
				}
				if bc, ok := cm["buyback_cost"].(int); ok {
					expected += bc
				}
			}
		}
	default:
		return nil
	}

	if spent == expected {
		return nil
	}
	kind := "under-paid (free or discounted without announcement)"
	if spent > expected {
		kind = "over-paid (double-deduction)"
	}
	return []LegalityViolation{{
		Turn: obs.TurnAtAnnounce, Seat: obs.Seat, Action: obs.ActionLabel(), Rule: "601.2f-h",
		Detail: fmt.Sprintf("%s: announced total %d but pool delta shows %d spent (before=%d after=%d added-in-window=%d)",
			kind, expected, spent, obs.PoolBefore, obs.PoolAfter, obs.ManaAddedDuringWindow),
	}}
}

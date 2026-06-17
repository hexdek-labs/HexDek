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
	"github.com/hexdek/hexdek/internal/judge"
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

// Canonical maps the repro-context-rich legality violation onto the one
// canonical vocabulary (consolidation step 4). The struct itself stays:
// it is the observation record (seed/turn/action are load-bearing for
// repro), and the canonical view is what flows through LogViolation.
func (v LegalityViolation) Canonical() judge.ValidationViolation {
	return judge.ValidationViolation{
		Surface:   judge.SurfaceLegality,
		Dimension: judge.DimensionLegality,
		Name:      v.Rule,
		Severity:  judge.SeverityCritical,
		Message:   v.Action + ": " + v.Detail,
		Seat:      v.Seat,
		Context: map[string]interface{}{
			"seed":   v.Seed,
			"turn":   v.Turn,
			"action": v.Action,
		},
	}
}

// LegalityObservation captures one announced action: state snapshotted at
// announcement (Begin*) plus the outcome at completion (Finish*). Checks
// read this instead of re-deriving "what just happened" from the live
// state, which by Finish time already reflects the action.
type LegalityObservation struct {
	Kind       string // "cast" | "activate" | "attack" | "block" | "etb" | "zone_change"
	Seat       int
	Card       *Card
	Perm       *Permanent // activation source / declared attacker / blocker's attacker target
	AbilityIdx int        // activations only

	// Ability is the announced Activated AST node (activations only) —
	// phase-2 checks read its TimingRestriction and loyalty cost.
	Ability *gameast.Activated

	// NoStackReason explains an activation that completed WITHOUT a
	// stack item (phase 3): "mana_ability" (CR 605.3a inline resolution)
	// or "fizzled_608.2b" (all targets illegal after costs — the
	// activation was countered by game rules). Set by the engine call
	// sites via SetNoStackReason; empty means the engine completed an
	// activation inline without declaring why.
	NoStackReason string

	// Zone-change observations (phase 3, Kind "zone_change").
	FromZone string
	ToZone   string

	// Combat declarations (phase 2). For Kind "attack", Perm is the
	// declared attacker. For Kind "block", Attacker is the attacked
	// creature and Blockers the full set assigned to it in this
	// declaration; BlockersWereBlocking[i] records whether Blockers[i]
	// was ALREADY marked blocking (another attacker) before this
	// assignment — the multi-block detection per CR 509.
	Attacker             *Permanent
	Blockers             []*Permanent
	BlockersWereBlocking []bool
	// BlockersPriorBlockCount[i] records how many attackers Blockers[i]
	// was ALREADY committed to before this assignment (snapshot at
	// observe time). The numeric companion to BlockersWereBlocking — it
	// lets the validator enforce the per-blocker block CAP (CR §509.1a:
	// one, unless "can block an additional N creatures" raises it to 1+N
	// or "can block any number of creatures" makes it unbounded).
	BlockersPriorBlockCount []int

	// Announcement-time snapshot (Begin*).
	TurnAtAnnounce       int
	PhaseAtAnnounce      string
	ActiveAtAnnounce     int
	StackDepthAtAnnounce int
	PoolBefore           int
	BaseCostAtAnnounce   int  // casts: CalculateTotalCost before any payment
	AbilityManaCost      int  // activations: announced mana component
	ZoneCastGranted      bool // card had a ZoneCastGrants permission ("as though" timing may apply)

	// ResolveFrameDepthAtAnnounce is gs.Flags["_resolve_frame_depth"] at
	// announcement: >0 means the action was initiated DURING the
	// resolution of another spell or ability (cascade-class permission
	// effects, per_card "you may cast" handlers) rather than from open
	// priority. Diagnostic for separating effect-driven mid-resolution
	// casts (CR §608.2c) from open-priority sequencing violations.
	ResolveFrameDepthAtAnnounce int
	// StackTopAtAnnounce names the item on top of the stack at
	// announcement (empty when the stack was empty) — attributes the
	// resident item in mid-stack-cast violations (this is how the r62
	// 302-hit cluster was pinned to unresolved commander spells).
	StackTopAtAnnounce string

	// Completion snapshot (Finish*).
	Item                  *StackItem // nil for inline mana abilities
	PoolAfter             int
	ManaAddedDuringWindow int // pool additions credited to Seat between Begin and Finish
	// Auxiliary (non-cost) pool deductions during the window: optional
	// trigger payments the seat legitimately makes mid-cast — extort
	// {W/B}, Rhystic Study {1}, Mystic Remora {4}, Smothering Tithe {2}
	// on an in-window draw, ward costs on targeting. These leave the
	// pool inside Begin..Finish but are NOT part of the spell's
	// announced total (CR 601.2f covers the spell's own cost only), so
	// the cost check must exclude them or every extort cast reads as a
	// +1 over-pay (judge r62 finding: seed 840043 seat 2 x8, extort
	// commander Sorin of House Markov).
	AuxManaSpentDuringWindow int
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
	if o.Kind == "attack" {
		if o.Perm != nil && o.Perm.Card != nil {
			name = o.Perm.Card.DisplayName()
		}
		return "attack:" + name
	}
	if o.Kind == "block" {
		atk := "<unknown>"
		if o.Attacker != nil && o.Attacker.Card != nil {
			atk = o.Attacker.Card.DisplayName()
		}
		return "block:" + atk
	}
	if o.Kind == "etb" {
		return "etb:" + name
	}
	if o.Kind == "zone_change" {
		return fmt.Sprintf("zone_change:%s(%s->%s)", name, o.FromZone, o.ToZone)
	}
	return "cast:" + name
}

// SetNoStackReason annotates why an activation completed without a
// stack item. Nil-safe so engine call sites can tag unconditionally.
func (o *LegalityObservation) SetNoStackReason(r string) {
	if o == nil {
		return
	}
	o.NoStackReason = r
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
	// Phase-2 set (r62 follow-up): combat declarations + ability timing.
	v.Register(LegalityCheck{Name: "attack-decl", Fn: checkLegalityAttackDecl})
	v.Register(LegalityCheck{Name: "block-decl", Fn: checkLegalityBlockDecl})
	v.Register(LegalityCheck{Name: "ability-timing", Fn: checkLegalityAbilityTiming})
	// Phase-3 set (r62 follow-up): mana-ability discipline + replacement
	// application sanity.
	v.Register(LegalityCheck{Name: "mana-ability", Fn: checkLegalityManaAbility})
	v.Register(LegalityCheck{Name: "replacement-etb", Fn: checkLegalityReplacementETB})
	v.Register(LegalityCheck{Name: "replacement-graveyard", Fn: checkLegalityReplacementGraveyard})
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
	// Consolidation step 4: every legality violation also flows through
	// the unified router at record time (origin) — drains/aggregators
	// (Loki) must NOT re-log.
	judge.LogViolation(viol.Canonical())
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
	obs.ResolveFrameDepthAtAnnounce = legalityResolveFrameDepth(gs)
	obs.StackTopAtAnnounce = legalityStackTopName(gs)
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
	v.creditNestedSpendToParents(obs)
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
	obs.ResolveFrameDepthAtAnnounce = legalityResolveFrameDepth(gs)
	obs.StackTopAtAnnounce = legalityStackTopName(gs)
	if perm != nil {
		obs.Card = perm.Card
	}
	if ab != nil && ab.Cost.Mana != nil {
		obs.AbilityManaCost = ab.Cost.Mana.CMC()
	}
	obs.Ability = ab
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
	v.creditNestedSpendToParents(obs)
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

// NoteManaSpend credits an auxiliary (non-cost) pool deduction to every
// in-flight observation for that seat — the spend-side mirror of
// NoteManaAdd. Called from the optional-trigger-payment sites (extort,
// Rhystic Study, Mystic Remora, Smothering Tithe, ward) behind the same
// nil guard, so a legitimate mid-cast payment doesn't read as the spell
// over-paying its announced total.
func (v *LegalityValidator) NoteManaSpend(seatIdx, amount int) {
	if v == nil || amount <= 0 {
		return
	}
	for _, obs := range v.active {
		if obs != nil && obs.Seat == seatIdx {
			obs.AuxManaSpentDuringWindow += amount
		}
	}
}

// creditNestedSpendToParents folds a just-completed observation's net pool
// consumption into the aux-spend total of every still-active observation for
// the same seat — i.e. its PARENT windows. Called from FinishCast /
// FinishActivation AFTER popActiveThrough(obs) has removed obs from v.active,
// so the inner observation never credits itself.
//
// Why this is needed: the engine lets a cast/activation's triggered abilities
// resolve INSIDE the announcing window (the PushPerCardTrigger bridge — see
// CastSpell's gs.ResolvingCards note). A trigger that grants priority can let
// the same seat cast a SECOND spell (e.g. an Urtet cast-trigger lands on the
// stack and the caster responds with a counterspell) before the outer spell's
// FinishCast snapshots PoolAfter. That nested cast's own cost left the pool, so
// the outer window's `PoolBefore - PoolAfter` delta over-counts by exactly the
// nested spell's cost and the §601.2f-h cost-paid check false-positives as an
// "over-paid (double-deduction)". NoteManaAdd / NoteManaSpend already mirror a
// child's added / aux mana onto the parents; this is the missing third leg —
// the child's own base cost. Crediting the child's measured net spend
// (PoolBefore + added - aux - PoolAfter) makes the parent delta neutral to
// everything the child did to the pool, and composes recursively for
// nested-within-nested casts.
func (v *LegalityValidator) creditNestedSpendToParents(obs *LegalityObservation) {
	if v == nil || obs == nil {
		return
	}
	net := obs.PoolBefore + obs.ManaAddedDuringWindow - obs.AuxManaSpentDuringWindow - obs.PoolAfter
	if net <= 0 {
		return // child netted zero or gained mana — added mirror already covers parents
	}
	for _, parent := range v.active {
		if parent != nil && parent.Seat == obs.Seat {
			parent.AuxManaSpentDuringWindow += net
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

// AbandonObservation discards an in-flight observation WITHOUT running
// checks — for action sites that opened a window and then determined no
// action happened at all (e.g. ApplyArtifactMana returning ok=false: the
// artifact was never tapped, nothing resolved inline, there is nothing
// to validate). Nil-receiver / nil-obs safe.
func (v *LegalityValidator) AbandonObservation(obs *LegalityObservation) {
	if v == nil || obs == nil {
		return
	}
	v.popActiveThrough(obs)
}

// ActiveObservations returns a snapshot of the in-flight observation
// frames (diagnostic/test accessor — frame leaks indicate a Begin
// without a matching Finish/Abandon).
func (v *LegalityValidator) ActiveObservations() []*LegalityObservation {
	if v == nil {
		return nil
	}
	return append([]*LegalityObservation(nil), v.active...)
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
			Detail: fmt.Sprintf("sorcery-speed spell cast with %d item(s) on the stack (resolve_frame_depth=%d top=%q)",
				obs.StackDepthAtAnnounce, obs.ResolveFrameDepthAtAnnounce, obs.StackTopAtAnnounce),
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
func checkLegalityCostPaid(gs *GameState, obs *LegalityObservation) []LegalityViolation {
	// A seat that LOST during the window is unmeasurable: markSeatLost
	// zeroes ManaPool and clears the typed pool on the loss transition
	// (sba.go), so the delta conflates the real payment with the loss
	// cleanup (CR §104.3 / §106.4). Two independently-diagnosed ground
	// truths, one per direction:
	//   - over-pay: judge round-5, seed 555 game 691 — Mageta's
	//     controller paid 4, then died mid-activation to Bloodchief
	//     Ascension triggering off Mageta's own discard cost; the SBA
	//     drained the remaining 4 → "announced 4, spent 8". (That the
	//     activation CONTINUES after its controller dies is a separate
	//     engine question, ranked in the round-5 report.)
	//   - cast-side: r63, seed 42 game 482 — "Knights" (memorabilia
	//     CMC-0 insert) cast with 1 floating; Ruric Thar's punisher
	//     resolved lethal inside fireCastTriggers, the SBA eliminated
	//     the caster, the cleared float read "announced 0, spent 1".
	if gs != nil && obs.Seat >= 0 && obs.Seat < len(gs.Seats) {
		if s := gs.Seats[obs.Seat]; s != nil && s.Lost {
			return nil
		}
	}
	spent := obs.PoolBefore + obs.ManaAddedDuringWindow - obs.AuxManaSpentDuringWindow - obs.PoolAfter

	expected := 0
	switch obs.Kind {
	case "activate":
		// AST-less inline mana resolutions (ApplyArtifactMana branches,
		// land taps — r62 follow-up #1) have no announced cost to
		// reconstruct: a Signet legitimately pays {1} inside its window
		// and a Lion's Eye Diamond discards a hand. The §605 behavioral
		// check still validates the mana_ability claim; cost discipline
		// for these sites is un-derivable without an AST and is skipped
		// rather than false-positived.
		if obs.Ability == nil && obs.NoStackReason == "mana_ability" {
			return nil
		}
		expected = obs.AbilityManaCost
	case "cast":
		expected = obs.BaseCostAtAnnounce
		if obs.Item != nil {
			cm := obs.Item.CostMeta
			// Alternative costs REPLACE the base (CR §601.2b / §118.9).
			if cm != nil {
				if b, _ := cm["overloaded"].(bool); b && obs.Card != nil {
					// Prefer the EFFECTIVE (modifier-adjusted) overload cost the
					// engine actually paid — stamped on the item, mirroring
					// surge_cost / spectacle_cost. Fall back to the raw printed
					// OverloadCost only for legacy items without the stamp. Using
					// the raw cost flagged a false §601.2f-h under/over-payment
					// whenever a cost modifier changed the overload cost.
					expected = OverloadCost(obs.Card)
					if c, ok := cm["overload_cost"].(int); ok {
						expected = c
					}
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
		Detail: fmt.Sprintf("%s: announced total %d but pool delta shows %d spent (before=%d after=%d added-in-window=%d aux-spent-in-window=%d)",
			kind, expected, spent, obs.PoolBefore, obs.PoolAfter, obs.ManaAddedDuringWindow, obs.AuxManaSpentDuringWindow),
	}}
}

// legalityResolveFrameDepth reads the §608.2c resolution-frame nesting
// counter (maintained by ResolveStackTop) at announcement time.
func legalityResolveFrameDepth(gs *GameState) int {
	if gs == nil || gs.Flags == nil {
		return 0
	}
	return gs.Flags["_resolve_frame_depth"]
}

// legalityStackTopName names the top stack item for diagnostics
// ("<kind>:<card or source name>"). Empty when the stack is empty.
func legalityStackTopName(gs *GameState) string {
	if gs == nil || len(gs.Stack) == 0 {
		return ""
	}
	it := gs.Stack[len(gs.Stack)-1]
	if it == nil {
		return "<nil-item>"
	}
	name := ""
	if it.Card != nil {
		name = it.Card.DisplayName()
	} else if it.Source != nil && it.Source.Card != nil {
		name = it.Source.Card.DisplayName() + " (ability)"
	}
	return it.Kind + ":" + name
}

// ---------------------------------------------------------------------------
// Phase-2 hooks — combat declarations (synchronous: no Begin/Finish span,
// the declaration is checked at the moment it becomes final)
// ---------------------------------------------------------------------------

// ObserveAttackDeclaration is called by DeclareAttackers once per
// finalized attacker, BEFORE the engine taps it for attacking — Tapped
// still reflects pre-declaration state. nil-receiver no-op when off.
func (v *LegalityValidator) ObserveAttackDeclaration(gs *GameState, seatIdx int, p *Permanent) {
	if v == nil || gs == nil || p == nil {
		return
	}
	obs := &LegalityObservation{
		Kind:                 "attack",
		Seat:                 seatIdx,
		Perm:                 p,
		Card:                 p.Card,
		TurnAtAnnounce:       gs.Turn,
		PhaseAtAnnounce:      gs.Phase,
		ActiveAtAnnounce:     gs.Active,
		StackDepthAtAnnounce: len(gs.Stack),
	}
	v.runChecks(gs, obs)
}

// ObserveBlockDeclaration is called by DeclareBlockers once per
// (attacker, assigned-blocker-set) pair, BEFORE flagBlocking is set on
// the assigned blockers — so a blocker already marked blocking was
// committed to a DIFFERENT attacker earlier in this declaration.
// nil-receiver no-op when off.
func (v *LegalityValidator) ObserveBlockDeclaration(gs *GameState, defenderSeat int, attacker *Permanent, blockers []*Permanent) {
	if v == nil || gs == nil || attacker == nil || len(blockers) == 0 {
		return
	}
	were := make([]bool, len(blockers))
	priorCount := make([]int, len(blockers))
	for i, b := range blockers {
		were[i] = b != nil && b.IsBlocking()
		priorCount[i] = blockCommitCountOf(b)
	}
	obs := &LegalityObservation{
		Kind:                    "block",
		Seat:                    defenderSeat,
		Attacker:                attacker,
		Blockers:                blockers,
		BlockersWereBlocking:    were,
		BlockersPriorBlockCount: priorCount,
		TurnAtAnnounce:          gs.Turn,
		PhaseAtAnnounce:         gs.Phase,
		ActiveAtAnnounce:        gs.Active,
		StackDepthAtAnnounce:    len(gs.Stack),
	}
	v.runChecks(gs, obs)
}

// ---------------------------------------------------------------------------
// Phase-2 checks
// ---------------------------------------------------------------------------

// checkLegalityAttackDecl — CR §508.1: a declared attacker must be an
// untapped creature its controller has controlled continuously since the
// turn began (or have haste, CR §302.6), must not have defender
// (§508.1a), and must exist (§702.26 phasing). The engine's own
// DeclareAttackers builds a legal pool via canAttack but then trusts the
// Hat's ChooseAttackers return UNVERIFIED — a policy returning a
// creature outside the legal pool is declared anyway. This check
// re-derives legality for what was actually declared.
func checkLegalityAttackDecl(gs *GameState, obs *LegalityObservation) []LegalityViolation {
	if obs.Kind != "attack" || obs.Perm == nil {
		return nil
	}
	p := obs.Perm
	var out []LegalityViolation
	add := func(rule, detail string) {
		out = append(out, LegalityViolation{
			Turn: obs.TurnAtAnnounce, Seat: obs.Seat, Action: obs.ActionLabel(),
			Rule: rule, Detail: detail,
		})
	}
	if !p.IsCreature() {
		add("508.1a", "declared attacker is not a creature")
		return out
	}
	if p.PhasedOut {
		add("702.26b", "declared attacker is phased out")
		return out
	}
	if p.Tapped {
		add("508.1c", "declared attacker is already tapped")
	}
	// Haste detection is deliberately permissive (printed/granted OR
	// layer-granted) so an anthem-haste creature never false-positives.
	if p.SummoningSick && !(p.HasKeyword("haste") || gs.HasKeywordOf(p, "haste")) {
		add("302.6", "declared attacker is summoning-sick without haste")
	}
	// Defender via the layer-aware query so a strip effect (Humility)
	// correctly legalizes the attack.
	if gs.HasKeywordOf(p, "defender") {
		add("508.1a", "declared attacker has defender")
	}
	if p.Flags != nil && p.Flags["detained"] == 1 {
		add("508.1a", "declared attacker is detained (can't attack)")
	}
	return out
}

// unlimitedBlockCap is the sentinel cap for "can block any number of
// creatures" — large enough that it is never the binding constraint in a
// real game, while staying well clear of integer overflow when compared.
const unlimitedBlockCap = 1 << 30

// multiBlockCap returns the maximum number of attackers a blocker may be
// committed to this combat (CR §509.1a). The default is ONE. Two effects
// raise it:
//
//   - "can block an additional N creatures" → 1 + N (N read from the
//     card's text — a digit or a number word, including hyphenated
//     compounds like "ninety-nine", so Hundred-Handed One reads 100, not
//     a hardcoded 2);
//   - "can block any number of creatures" → unbounded.
//
// An effect-granted "blocks_additional" flag (set by the
// block_additional_creature modification) adds its stored N on top, so a
// runtime grant stacks with any printed capability.
//
// NOTE: the printed-text match is condition-blind (e.g. Hundred-Handed
// One's "as long as it's monstrous" is not evaluated) — this mirrors the
// pre-existing substring approach and is the same simplification used
// throughout combat legality; gating on the conditional is a separate
// content task.
func multiBlockCap(b *Permanent) int {
	if b == nil {
		return 1
	}
	cap := 1
	ot := OracleTextLower(b.Card)
	switch {
	case strings.Contains(ot, "block any number"):
		return unlimitedBlockCap
	case strings.Contains(ot, "block an additional"):
		if n := parseAdditionalBlockCount(ot); n > 0 {
			cap = 1 + n
		}
	}
	// Effect-granted additional-block (block_additional_creature). The
	// flag stores N (the number of EXTRA attackers granted); default 1.
	if b.Flags != nil {
		if n := b.Flags["blocks_additional"]; n > 0 {
			cap += n
		}
	}
	return cap
}

// legalityCanMultiBlock reports whether a blocker may block more than one
// attacker at all — a boolean view over multiBlockCap, retained for the
// menace-pairing and same-pair call sites.
func legalityCanMultiBlock(b *Permanent) bool {
	return multiBlockCap(b) > 1
}

// parseAdditionalBlockCount extracts the N in "can block an additional N
// creatures". Returns 1 for the bare "an additional creature" form (no
// explicit count), 0 if the phrase is absent.
func parseAdditionalBlockCount(ot string) int {
	const marker = "block an additional"
	i := strings.Index(ot, marker)
	if i < 0 {
		return 0
	}
	rest := ot[i+len(marker):]
	if j := strings.Index(rest, "creature"); j >= 0 {
		rest = rest[:j]
	}
	rest = strings.TrimSpace(rest)
	if rest == "" {
		// "block an additional creature" — exactly one extra.
		return 1
	}
	if n, ok := blockNumberWord(rest); ok {
		return n
	}
	for _, tok := range strings.Fields(rest) {
		if v, ok := atoiSafe(strings.Trim(tok, ".,")); ok {
			return v
		}
	}
	// A number we cannot parse still clearly grants AT LEAST one extra.
	return 1
}

var blockNumberOnes = map[string]int{
	"a": 1, "an": 1, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
	"six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10, "eleven": 11,
	"twelve": 12, "thirteen": 13, "fourteen": 14, "fifteen": 15, "sixteen": 16,
	"seventeen": 17, "eighteen": 18, "nineteen": 19,
}

var blockNumberTens = map[string]int{
	"twenty": 20, "thirty": 30, "forty": 40, "fifty": 50,
	"sixty": 60, "seventy": 70, "eighty": 80, "ninety": 90,
}

// blockNumberWord parses an English number word up to 99, including
// hyphenated or space-separated tens-ones compounds ("ninety-nine" → 99).
func blockNumberWord(s string) (int, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "-", " ")
	total, matched := 0, false
	for _, p := range strings.Fields(s) {
		if v, ok := blockNumberTens[p]; ok {
			total += v
			matched = true
			continue
		}
		if v, ok := blockNumberOnes[p]; ok {
			total += v
			matched = true
			continue
		}
		break // stop at the first non-number token
	}
	if !matched {
		return 0, false
	}
	return total, true
}

// checkLegalityBlockDecl — CR §509.1: each assigned blocker must be an
// untapped creature (§509.1a), may block only one attacker unless its
// text allows more, and must satisfy the attacker's evasion/blocking
// restrictions (§509.1b — re-derived through canBlockGS: flying,
// horsemanship, fear/intimidate/shadow/skulk, landwalk, protection,
// unblockable). Menace (§702.110b) is a set-level requirement: a menace
// attacker blocked by exactly one creature is an illegal declaration.
// The engine's hat path (DeclareBlockers → AssignBlockers) applies the
// returned map with NO validation at all — this check is the only gate
// behind a policy bug there.
func checkLegalityBlockDecl(gs *GameState, obs *LegalityObservation) []LegalityViolation {
	if obs.Kind != "block" || obs.Attacker == nil || len(obs.Blockers) == 0 {
		return nil
	}
	var out []LegalityViolation
	add := func(rule, detail string) {
		out = append(out, LegalityViolation{
			Turn: obs.TurnAtAnnounce, Seat: obs.Seat, Action: obs.ActionLabel(),
			Rule: rule, Detail: detail,
		})
	}
	live := 0
	seen := map[*Permanent]bool{}
	for i, b := range obs.Blockers {
		if b == nil {
			continue
		}
		live++
		// CR §509.1 — the SAME blocker+attacker pair committed twice is
		// always illegal, even for a creature that may block multiple
		// attackers (you still can't block one attacker twice).
		if seen[b] {
			name := "<unknown>"
			if b.Card != nil {
				name = b.Card.DisplayName()
			}
			add("509.1", fmt.Sprintf("blocker %q committed twice in one assignment", name))
			continue
		}
		seen[b] = true
		name := "<unknown>"
		if b.Card != nil {
			name = b.Card.DisplayName()
		}
		if !b.IsCreature() {
			add("509.1a", fmt.Sprintf("blocker %q is not a creature", name))
			continue
		}
		if b.PhasedOut {
			add("702.26b", fmt.Sprintf("blocker %q is phased out", name))
			continue
		}
		if b.Tapped {
			add("509.1a", fmt.Sprintf("blocker %q is tapped", name))
			continue
		}
		// CR §509.1a — a blocker may be committed to at most multiBlockCap
		// attackers this combat. The prior-commit COUNT (not a bare bool)
		// is the binding test: a creature that may block an additional N
		// is illegal only once it would exceed 1+N total commitments.
		prior := 0
		if i < len(obs.BlockersPriorBlockCount) {
			prior = obs.BlockersPriorBlockCount[i]
		} else if i < len(obs.BlockersWereBlocking) && obs.BlockersWereBlocking[i] {
			prior = 1
		}
		if prior >= multiBlockCap(b) {
			add("509.1", fmt.Sprintf("blocker %q is already blocking the maximum number of attackers", name))
			continue
		}
		if !canBlockGS(gs, obs.Attacker, b) {
			add("509.1b", fmt.Sprintf("blocker %q cannot legally block this attacker (evasion/restriction unsatisfied)", name))
		}
	}
	// §702.110b — menace: can't be blocked except by two or more.
	if live == 1 && (obs.Attacker.HasKeyword("menace") || gs.HasKeywordOf(obs.Attacker, "menace")) {
		add("702.110b", "menace attacker blocked by exactly one creature")
	}
	return out
}

// checkLegalityAbilityTiming — CR §606.3 (loyalty abilities) and
// §602.5d ("activate only as a sorcery"): both restrict activation to
// the controller's own turn, a main phase, and an empty stack. The
// engine enforces the loyalty ONCE-PER-TURN limit (§606.3's second
// half) but has NO sorcery-speed gate on either family — an AI hat can
// (and does) fire loyalty abilities at instant speed mid-stack.
func checkLegalityAbilityTiming(_ *GameState, obs *LegalityObservation) []LegalityViolation {
	if obs.Kind != "activate" || obs.Ability == nil {
		return nil
	}
	rule := ""
	if strings.EqualFold(strings.TrimSpace(obs.Ability.TimingRestriction), "sorcery") {
		rule = "602.5d"
	}
	if obs.Perm != nil && obs.Perm.IsPlaneswalker() {
		if _, ok := LoyaltyCost(obs.Ability); ok {
			rule = "606.3"
		} else if obs.Ability.Cost.PayLife != nil && *obs.Ability.Cost.PayLife > 0 {
			// Legacy dataset shape: planeswalker minus costs parsed into
			// PayLife — ActivateAbility routes these to loyalty, so the
			// timing restriction applies identically.
			rule = "606.3"
		}
	}
	if rule == "" {
		return nil
	}
	var out []LegalityViolation
	add := func(detail string) {
		out = append(out, LegalityViolation{
			Turn: obs.TurnAtAnnounce, Seat: obs.Seat, Action: obs.ActionLabel(),
			Rule: rule, Detail: detail,
		})
	}
	if obs.Seat != obs.ActiveAtAnnounce {
		add(fmt.Sprintf("sorcery-speed ability activated by non-active seat (active=%d)", obs.ActiveAtAnnounce))
	}
	if !legalityMainPhase(obs.PhaseAtAnnounce) {
		add(fmt.Sprintf("sorcery-speed ability activated outside a main phase (phase=%q)", obs.PhaseAtAnnounce))
	}
	if obs.StackDepthAtAnnounce > 0 {
		add(fmt.Sprintf("sorcery-speed ability activated with %d item(s) on the stack (top=%q)",
			obs.StackDepthAtAnnounce, obs.StackTopAtAnnounce))
	}
	return out
}

// ---------------------------------------------------------------------------
// Phase-3 hooks
// ---------------------------------------------------------------------------

// ObserveETB is called at the end of both ETB cascades (the stack-cast
// inline cascade and FirePermanentETBTriggers) once the entering
// permanent's self-replacements have been applied — counters from
// "enters with N counters" statics are final here. nil-receiver no-op.
func (v *LegalityValidator) ObserveETB(gs *GameState, perm *Permanent) {
	if v == nil || gs == nil || perm == nil {
		return
	}
	obs := &LegalityObservation{
		Kind:                 "etb",
		Seat:                 perm.Controller,
		Perm:                 perm,
		Card:                 perm.Card,
		TurnAtAnnounce:       gs.Turn,
		PhaseAtAnnounce:      gs.Phase,
		ActiveAtAnnounce:     gs.Active,
		StackDepthAtAnnounce: len(gs.Stack),
	}
	v.runChecks(gs, obs)
}

// ObserveZoneChange is called from FireZoneChangeTriggers — the canonical
// post-move chokepoint — for every mover-routed zone change. Paths that
// append to zones directly without the mover are invisible here (a known
// engine fragmentation; this observes what routes through the front door).
// nil-receiver no-op.
func (v *LegalityValidator) ObserveZoneChange(gs *GameState, perm *Permanent, card *Card, fromZone, toZone string) {
	if v == nil || gs == nil {
		return
	}
	seat := -1
	if perm != nil {
		seat = perm.Controller
	} else if card != nil && card.Owner >= 0 {
		seat = card.Owner
	}
	obs := &LegalityObservation{
		Kind:                 "zone_change",
		Seat:                 seat,
		Perm:                 perm,
		Card:                 card,
		FromZone:             fromZone,
		ToZone:               toZone,
		TurnAtAnnounce:       gs.Turn,
		PhaseAtAnnounce:      gs.Phase,
		ActiveAtAnnounce:     gs.Active,
		StackDepthAtAnnounce: len(gs.Stack),
	}
	v.runChecks(gs, obs)
}

// ---------------------------------------------------------------------------
// Phase-3 checks
// ---------------------------------------------------------------------------

// checkLegalityManaAbility — CR §605.1a / §605.3a discipline. Mana
// abilities (produce mana, don't target, not loyalty) resolve inline and
// never use the stack; everything else MUST use the stack so opponents
// get responses. The check re-derives mana-ability-ness independently
// via IsManaAbility and compares it against what the engine actually did
// (stack item vs. none), using NoStackReason to distinguish legitimate
// no-item completions (608.2b fizzles) from inline resolutions.
func checkLegalityManaAbility(_ *GameState, obs *LegalityObservation) []LegalityViolation {
	if obs.Kind != "activate" || obs.Perm == nil {
		return nil
	}
	isMana := IsManaAbility(obs.Perm, obs.AbilityIdx)
	var out []LegalityViolation
	add := func(rule, detail string) {
		out = append(out, LegalityViolation{
			Turn: obs.TurnAtAnnounce, Seat: obs.Seat, Action: obs.ActionLabel(),
			Rule: rule, Detail: detail,
		})
	}
	if obs.Item == nil {
		switch obs.NoStackReason {
		case "fizzled_608.2b":
			// Countered by game rules — legitimately no stack item.
		case "mana_ability":
			if obs.Ability == nil {
				// AST-less inline resolution (ApplyArtifactMana branches,
				// land taps, per_card sites). No AST to re-derive from —
				// verify the claim BEHAVIORALLY: a real mana ability
				// produces mana. The window crediting (NoteManaAdd via
				// AddMana) gives us actual production for free; an inline
				// resolution that claimed mana_ability and produced
				// nothing denied opponents responses for no mana.
				if obs.ManaAddedDuringWindow <= 0 {
					add("605.1a", "ability resolved inline claiming mana ability but produced no mana in its window — opponents were denied responses")
				}
			} else if !isMana {
				// The engine resolved this inline claiming it's a mana
				// ability; independent AST re-derivation disagrees. Name
				// the disqualifier for the repro.
				reason := "produces no mana"
				if obs.Ability.Effect != nil {
					if effectProducesMana(obs.Ability.Effect) && effectTargets(obs.Ability.Effect) {
						reason = "it targets (CR 605.1a)"
					}
				}
				add("605.1a", "ability resolved inline as a mana ability but is not one ("+reason+") — opponents were denied responses")
			}
		default:
			// Unknown inline completion. r62 follow-up #1: the blanket
			// per_card exemption (obs.Ability == nil → silent) is GONE —
			// every engine site that completes an activation without a
			// stack item must declare why via SetNoStackReason
			// (mana_ability / fizzled_608.2b). An undeclared inline
			// completion is exactly the shape that denies opponents
			// responses without justification.
			if !isMana {
				add("605.3a", "activated ability completed without a stack item and without a declared reason — opponents were denied responses")
			}
		}
		return out
	}
	// Item present: a true mana ability must never be put on the stack.
	if isMana {
		add("605.3a", "mana ability was put on the stack (mana abilities don't use the stack)")
	}
	return out
}

// checkLegalityReplacementETB — CR §614.1c / §122.1g: a permanent whose
// own AST carries an "enters with N counters" self-replacement must
// actually have those counters once the ETB cascade settles. Conditional
// statics ("if kicked") are honored via evalCondition; variable counts
// route through resolveETBCounterCount (0 when unkicked — not a
// violation).
func checkLegalityReplacementETB(gs *GameState, obs *LegalityObservation) []LegalityViolation {
	if obs.Kind != "etb" || obs.Perm == nil || obs.Perm.Card == nil || obs.Perm.Card.AST == nil {
		return nil
	}
	p := obs.Perm
	// Face-down permanents have no abilities (CR §708.4).
	if p.Flags != nil && p.Flags["face_down"] != 0 {
		return nil
	}
	var out []LegalityViolation
	for _, ab := range p.Card.AST.Abilities {
		st, ok := ab.(*gameast.Static)
		if !ok || st.Modification == nil || st.Modification.ModKind != "etb_with_counters" {
			continue
		}
		if st.Condition != nil && !evalCondition(gs, p, st.Condition) {
			continue
		}
		want := resolveETBCounterCount(p, st.Modification.Args)
		if want <= 0 {
			continue
		}
		kind := "+1/+1"
		if len(st.Modification.Args) > 1 {
			if k, ok := st.Modification.Args[1].(string); ok && k != "" {
				kind = k
			}
		}
		got := 0
		if p.Counters != nil {
			got = p.Counters[kind]
		}
		// CR §616 / §122.1g — an active `would_put_counter` replacement can
		// legitimately change how many counters the permanent enters with, so
		// the raw printed count is no longer the expected final tally. The
		// canonical case (loki seed 41076465 / game 4107): an opponent's
		// Vorinclex, Monstrous Raider halves a Clockwork Beetle's 2 → 1 and a
		// Clockwork Avian's 4 → 2 as they enter — rules-correct, but the
		// strict `got < want` test false-flagged it. Doubling Season /
		// Hardened Scales / Branching Evolution INCREASE the count (got >=
		// want), so they never tripped this; only halve/cancel-class
		// replacements do. Skip the comparison when such a replacement applies
		// to this ETB placement. The District-Mascot bug this check targets
		// (counter never applied → got == 0) still fires in the overwhelmingly
		// common case where no counter-modifying replacement is in play.
		// CR §614.1d / §122.1g — the check verifies the self-replacement was
		// APPLIED (the counters were placed as the permanent entered), NOT that
		// they still PERSIST. Once placed, a counter can be legitimately removed
		// by a SEPARATE event before this observation closes — most commonly the
		// §704.5q +1/+1 / -1/-1 annihilation SBA (which per_card ETB handlers run
		// inline mid-cascade), but also -1/-1 self-removal abilities, wither/
		// infect, etc. ApplyStaticETBCounters records what it actually placed on
		// perm.Flags["_etb_placed:<kind>"]; if that meets the printed count the
		// replacement DID apply and a later shortfall is not a §614.1c miss. (The
		// §616 doubler/halver case where placement itself is modified to < want is
		// still covered by etbCounterReplacementApplies, #1075.)
		placed := 0
		if p.Flags != nil {
			placed = p.Flags["_etb_placed:"+kind]
		}
		if got < want && placed < want && !etbCounterReplacementApplies(gs, p, kind, want) {
			out = append(out, LegalityViolation{
				Turn: obs.TurnAtAnnounce, Seat: obs.Seat, Action: "etb:" + p.Card.DisplayName(),
				Rule:   "614.1c",
				Detail: fmt.Sprintf("enters-with-counters self-replacement not applied: want >=%d %q counter(s), have %d", want, kind, got),
			})
		}
	}
	return out
}

// etbCounterReplacementApplies reports whether any active `would_put_counter`
// replacement effect (Vorinclex, Monstrous Raider's halve/double arms,
// Doubling Season, Hardened Scales, Branching Evolution, …) would apply to a
// placement of `count` `kind` counters on `perm` as it enters. It probes the
// replacements' Applies predicates with a synthetic event matching the shape
// FirePutCounterEvent builds — Applies is side-effect-free (it only reads
// state), so this is a safe read-only check; the replacements' ApplyFn (which
// logs / mutates) is never invoked. Used by checkLegalityReplacementETB to
// avoid false-flagging a count the engine legitimately modified at §616.
func etbCounterReplacementApplies(gs *GameState, perm *Permanent, kind string, count int) bool {
	if gs == nil || perm == nil || len(gs.Replacements) == 0 {
		return false
	}
	probe := NewReplEvent("would_put_counter")
	probe.TargetPerm = perm
	probe.Source = perm // self-replacement: the entering permanent puts its own counters
	probe.Payload["counter_type"] = kind
	probe.SetCount(count)
	for _, r := range gs.Replacements {
		if r == nil || r.EventType != "would_put_counter" || r.Applies == nil {
			continue
		}
		if r.Applies(gs, probe) {
			return true
		}
	}
	return false
}

// checkLegalityReplacementGraveyard — CR §614.1a: a card arriving in a
// graveyard from the battlefield while a REGISTERED, APPLICABLE
// graveyard-redirect replacement exists (Rest in Peace, Leyline of the
// Void, Anafenza) means the §614 chain was bypassed — redirect-class
// effects change the destination, so an application would have prevented
// this arrival. The check re-derives applicability through the actual
// registry's Applies predicates (no name lists), probed with the same
// event shape FireDieEvent constructs. Replacements SOURCED by the
// arriving permanent itself are skipped (self-replacements often
// legitimately allow the graveyard leg of their effect).
func checkLegalityReplacementGraveyard(gs *GameState, obs *LegalityObservation) []LegalityViolation {
	if obs.Kind != "zone_change" || obs.FromZone != "battlefield" || obs.ToZone != "graveyard" {
		return nil
	}
	if obs.Perm == nil || len(gs.Replacements) == 0 {
		return nil
	}
	// Tokens cease to exist on zone change — their graveyard visit is
	// transient bookkeeping, not a §614 surface.
	if obs.Perm.IsToken() {
		return nil
	}
	probe := NewReplEvent("would_die")
	probe.TargetPerm = obs.Perm
	probe.TargetSeat = obs.Perm.Controller
	probe.Payload["to_zone"] = "graveyard"
	probeGY := NewReplEvent("would_be_put_into_graveyard")
	probeGY.TargetPerm = obs.Perm
	probeGY.TargetSeat = obs.Perm.Controller
	probeGY.Payload["to_zone"] = "graveyard"

	var out []LegalityViolation
	for _, re := range gs.Replacements {
		if re == nil || re.Applies == nil {
			continue
		}
		// Only declared zone-redirectors are auditable: a bookkeeping
		// replacement that rides would_die without altering the event
		// (Skullclamp's stamp, Solemn's dies-draw) legitimately
		// witnesses graveyard arrivals — flagging those was the r63
		// sweep's FP cluster (9 hits, all skullclamp:stamp).
		if !re.RedirectsZone {
			continue
		}
		if re.SourcePerm == obs.Perm {
			continue // self-replacement
		}
		var ev *ReplEvent
		switch re.EventType {
		case "would_die":
			ev = probe
		case "would_be_put_into_graveyard":
			ev = probeGY
		default:
			continue
		}
		applies := false
		func() {
			defer func() { _ = recover() }()
			applies = re.Applies(gs, ev)
		}()
		if applies {
			name := "<unknown>"
			if obs.Card != nil {
				name = obs.Card.DisplayName()
			}
			out = append(out, LegalityViolation{
				Turn: obs.TurnAtAnnounce, Seat: obs.Seat, Action: "zone_change:" + name,
				Rule: "614.1a",
				Detail: fmt.Sprintf("card reached the graveyard from the battlefield while replacement %q (event %s) was registered and applicable — §614 chain bypassed",
					re.HandlerID, re.EventType),
			})
			// One violation per arrival is enough signal.
			break
		}
	}
	return out
}

package gameengine

// Wave 3a/3b — Central activated-ability dispatcher with stax enforcement.
//
// Comp-rules citations:
//
//   CR §602.1   Activating activated abilities.
//   CR §602.1d  "Activating an activated ability creates an activated ability
//               on the stack and an activated ability on the stack is not a
//               spell." It goes on the stack and opponents receive priority
//               before it resolves.
//   CR §605     Mana abilities — exempted from the stack (resolve inline).
//   CR §602.1b  "A player may activate an activated ability any time he or
//               she has priority" unless a restriction applies.
//
// Stax enforcement — flag consumers:
//
//   Null Rod / Collector Ouphe (per_card.NullRodSuppresses):
//     "Activated abilities of artifacts can't be activated unless they are
//     mana abilities." Checked via gs.Flags["null_rod_count"] > 0.
//
//   Cursed Totem (per_card.CursedTotemSuppresses):
//     "Activated abilities of creatures can't be activated." Checked via
//     gs.Flags["cursed_totem_count"] > 0. Note: unlike Null Rod, Cursed
//     Totem does NOT exempt mana abilities.
//
//   Grand Abolisher (per_card.GrandAbolisherActive):
//     "Your opponents can't [...] activate abilities of artifacts, creatures,
//     or enchantments during your turn." Checked via
//     gs.Flags["grand_abolisher_active_seat_N"].
//
//   Drannith Magistrate (per_card.DrannithMagistrateRestrictsOpponent):
//     Restriction on casting from non-hand zones; enforced by CastFromZone
//     rather than activation dispatch.
//
//   Opposition Agent (per_card.OppositionAgentControlsSearch):
//     Restriction on library searches; enforced by search primitives
//     (resolveTutor) rather than activation dispatch.
//
// This file provides:
//
//   - ActivateAbility(gs, seatIdx, perm, abilityIdx, targets) error
//   - IsManaAbility(perm, abilityIdx) bool
//   - StaxCheck(gs, seatIdx, perm, abilityIdx) (suppressed bool, reason string)
//   - PushActivatedAbility(gs, seatIdx, perm, abilityIdx, effect, targets)

import (
	"fmt"
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// itoa is a tiny, allocation-free int-to-string conversion. Shared by
// activation.go, zone_cast.go, and resolve.go for building flag keys
// like "drannith_active_seat_0" without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [12]byte{}
	i := len(buf)
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// ---------------------------------------------------------------------------
// Mana-ability detection (CR §605).
// ---------------------------------------------------------------------------

// IsManaAbility returns true if the activated ability at `abilityIdx` on
// `perm` is a mana ability per CR §605.1a:
//
//	(1) it doesn't target,
//	(2) it could add mana to a player's mana pool, and
//	(3) it's not a loyalty ability.
//
// MVP detection: if the ability's Effect (or any sub-effect in a Sequence)
// is an AddMana, it's a mana ability unless the ability targets or is a
// loyalty ability. For per_card-registered abilities we use the whitelist
// approach from NullRodSuppresses (null_rod.go), but here we provide the
// general AST-based detector.
func IsManaAbility(perm *Permanent, abilityIdx int) bool {
	if perm == nil || perm.Card == nil || perm.Card.AST == nil {
		return false
	}
	abilities := perm.Card.AST.Abilities
	if abilityIdx < 0 || abilityIdx >= len(abilities) {
		return false
	}
	ab, ok := abilities[abilityIdx].(*gameast.Activated)
	if !ok || ab.Effect == nil {
		return false
	}
	// Loyalty abilities are never mana abilities (CR §605.1a).
	if ab.Cost.PayLife != nil && perm.IsPlaneswalker() {
		return false
	}
	// Check if the effect (or any sub-effect) produces mana.
	if !effectProducesMana(ab.Effect) {
		return false
	}
	// Check targeting — mana abilities don't target (CR §605.1a).
	if effectTargets(ab.Effect) {
		return false
	}
	return true
}

// effectProducesMana returns true if the effect (or any nested sub-effect)
// is or contains an AddMana effect.
func effectProducesMana(e gameast.Effect) bool {
	if e == nil {
		return false
	}
	switch v := e.(type) {
	case *gameast.AddMana:
		return true
	case *gameast.ModificationEffect:
		// Many mana abilities are represented as ModificationEffect
		// scaffold nodes rather than AddMana — count-scaled mana
		// ("Add {G} for each creature you control" → add_mana_per),
		// the catch-all add-mana text fallback (add_mana_effect), and
		// the plain add_mana modkind. These satisfy CR §605.1a (they
		// add mana, don't target) and so MUST resolve inline per
		// §605.3a rather than using the stack — otherwise Gaea's Cradle
		// / Cabal Coffers / filter lands can't be tapped mid-cost to
		// pay for a spell and could be illegally responded to.
		if manaProducingModKinds[v.ModKind] {
			return true
		}
		// A handful of count-scaled creature mana abilities mis-parse to
		// the catch-all `untyped_effect` ModKind with the count-scaled
		// payload smuggled into args[0] as "add_{<color>}_per:<basis>"
		// (Elvish Archdruid: "{T}: Add {G} for each Elf you control").
		// That is unambiguously a §605.1a mana ability — recognize the
		// exact emitted shape so it resolves inline (and produces mana,
		// via the matching resolveResidualByText arm) instead of being
		// pushed onto the stack as a non-mana activated ability.
		if v.ModKind == "untyped_effect" && reAddManaPerResidual.MatchString(modArgString(v.Args, 0)) {
			return true
		}
		return false
	case *gameast.Sequence:
		for _, sub := range v.Items {
			if effectProducesMana(sub) {
				return true
			}
		}
	case *gameast.Choice:
		// Modal mana abilities — "{T}: Add {W} or {U}" (pain lands, dual
		// lands, talismans, refuges, check lands, the bounce-land cycle,
		// …) — parse to a Choice whose options are each an AddMana. Per CR
		// §605.1a such an ability could put mana into a player's pool and
		// (since every mode is a pure AddMana) doesn't target, so it IS a
		// mana ability and MUST resolve inline per §605.3a — not be pushed
		// onto the stack where it could be illegally responded to and
		// couldn't be tapped mid-cost to pay for a spell. Require EVERY
		// option to produce mana so a modal ability with a non-mana mode
		// keeps its conservative stack-using classification (mirrors the
		// choose_color stance above). ~405 cards were mis-routed pre-fix.
		if len(v.Options) == 0 {
			return false
		}
		for _, opt := range v.Options {
			if !effectProducesMana(opt) {
				return false
			}
		}
		return true
	}
	return false
}

// manaProducingModKinds is the whitelist of ModificationEffect ModKinds
// that unambiguously add mana (and only mana). choose_color /
// choose_color_typed are deliberately excluded — "choose a color" is
// ambiguous (e.g. Meteor Crater's non-mana color pick), so those abilities
// keep their stack-using classification rather than risk mis-flagging a
// non-mana ability as a §605 mana ability.
var manaProducingModKinds = map[string]bool{
	"add_mana":        true,
	"add_mana_per":    true,
	"add_mana_effect": true,
}

// effectTargets returns true if the effect (or any nested sub-effect)
// references a targeted filter.
func effectTargets(e gameast.Effect) bool {
	if e == nil {
		return false
	}
	switch v := e.(type) {
	case *gameast.Damage:
		return v.Target.Targeted
	case *gameast.Destroy:
		return v.Target.Targeted
	case *gameast.Exile:
		return v.Target.Targeted
	case *gameast.Bounce:
		return v.Target.Targeted
	case *gameast.CounterSpell:
		return v.Target.Targeted
	case *gameast.Sequence:
		for _, sub := range v.Items {
			if effectTargets(sub) {
				return true
			}
		}
	case *gameast.Choice:
		// Recurse into modal options: a modal ability with a targeted mode
		// is NOT a mana ability (CR §605.1a) even if another mode adds
		// mana. (Defensive — effectProducesMana currently only flags a
		// Choice whose modes are all AddMana, which never target — but
		// keep the two checks consistent if that ever loosens.)
		for _, opt := range v.Options {
			if effectTargets(opt) {
				return true
			}
		}
	}
	return false
}

// applyManaAbilityRiders applies a rider that the parser emitted as a
// standalone Static sibling of an activated mana ability rather than
// folding it into the ability's own effect. The dominant case is the
// pain-land self_damage downside ("This land deals N damage to you"),
// which parses to a trailing Static{self_damage} disconnected from the
// "{T}: Add {W} or {U}" ability it modifies (Adarkar Wastes, Shivan Reef,
// Brushland, Tarnished Citadel, Grand Coliseum, Karplusan Forest, …).
// Because a Static ability is never resolved on its own, that downside was
// silently skipped — 25 pain lands tapped for mana with no life cost.
//
// The rider belongs to the PAINFUL tap — the colored/any-color mana
// ability. A coexisting free "{T}: Add {C}" tap deals no damage, so a
// pure-colorless-only AddMana ability is exempt UNLESS it is the card's
// only mana ability.
//
// Skipped when a per_card OnActivated handler owns the card: those
// handlers (Ancient Tomb) deal the rider damage themselves, so applying
// it here too would double it.
func applyManaAbilityRiders(gs *GameState, perm *Permanent, abilityIdx int) {
	if gs == nil || perm == nil || perm.Card == nil || perm.Card.AST == nil {
		return
	}
	if PerCardOwnsActivated(perm.Card.DisplayName()) {
		return
	}
	abilities := perm.Card.AST.Abilities
	if abilityIdx < 0 || abilityIdx >= len(abilities) {
		return
	}
	act, ok := abilities[abilityIdx].(*gameast.Activated)
	if !ok || act.Effect == nil {
		return
	}

	selfDmg := 0
	manaAbilityCount := 0
	for _, ab := range abilities {
		switch v := ab.(type) {
		case *gameast.Activated:
			if effectProducesMana(v.Effect) && !effectTargets(v.Effect) {
				manaAbilityCount++
			}
		case *gameast.Static:
			if v.Modification != nil && v.Modification.ModKind == "self_damage" {
				selfDmg = manaRiderAmount(v.Modification.Args)
			}
		}
	}
	if selfDmg <= 0 {
		return
	}
	// Only the painful tap deals damage: a colored/any-color ability, or
	// the card's sole mana ability (no free {C} alternative exists).
	if !manaEffectIsColoredOrAny(act.Effect) && manaAbilityCount > 1 {
		return
	}
	DealDamage(gs, perm.Controller, selfDmg, sourceName(perm))
}

// manaRiderAmount reads the damage amount from a self_damage Static's args
// (args[0]); defaults to 1 — the pain-land norm — when unspecified.
func manaRiderAmount(args []interface{}) int {
	if len(args) > 0 {
		if n, ok := asInt(args[0]); ok && n > 0 {
			return n
		}
	}
	return 1
}

// manaEffectIsColoredOrAny reports whether a mana effect produces colored
// or any-color mana (i.e. it is NOT a pure colorless-{C}-only producer).
func manaEffectIsColoredOrAny(e gameast.Effect) bool {
	switch v := e.(type) {
	case *gameast.AddMana:
		if v.AnyColorCount > 0 {
			return true
		}
		for _, sym := range v.Pool {
			for _, c := range sym.Color {
				if c != "C" {
					return true
				}
			}
		}
	case *gameast.Choice:
		for _, opt := range v.Options {
			if manaEffectIsColoredOrAny(opt) {
				return true
			}
		}
	case *gameast.Sequence:
		for _, sub := range v.Items {
			if manaEffectIsColoredOrAny(sub) {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Stax enforcement.
// ---------------------------------------------------------------------------

// ActivationSuppression describes why an activation was blocked.
type ActivationSuppression struct {
	Suppressed bool
	Reason     string // "null_rod", "cursed_totem", "grand_abolisher", "split_second"
}

// StaxCheck tests whether activating ability `abilityIdx` on `perm` by
// `seatIdx` is suppressed by any active stax piece. Returns the first
// applicable suppression, or {false, ""} if the activation is legal.
//
// Mana abilities are exempt from Null Rod (but NOT from Cursed Totem,
// which suppresses all creature activated abilities including mana).
func StaxCheck(gs *GameState, seatIdx int, perm *Permanent, abilityIdx int) ActivationSuppression {
	if gs == nil || perm == nil {
		return ActivationSuppression{}
	}

	isMana := IsManaAbility(perm, abilityIdx)

	// CR §613.1f — a permanent that has lost all abilities (Humility,
	// Lignify, Darksteel Mutation, Frogify, …) has no activated abilities
	// to activate, including MANA abilities. The activation path otherwise
	// reads raw Card.AST, which is unaware of the layer strip; this gate
	// enforces the removal. Scoped to the permanent's own printed AST
	// abilities — a strip nils every one of them, while abilities granted
	// by a later effect arrive through a different (non-AST-index) path.
	if perm.Card != nil && perm.Card.AST != nil &&
		abilityIdx >= 0 && abilityIdx < len(perm.Card.AST.Abilities) &&
		gs.HasLostAllAbilities(perm) {
		return ActivationSuppression{Suppressed: true, Reason: "abilities_removed"}
	}

	// CR §702.61a: split-second on the stack prevents activating
	// non-mana abilities.
	if !isMana && SplitSecondActive(gs) {
		return ActivationSuppression{Suppressed: true, Reason: "split_second"}
	}

	// Null Rod / Collector Ouphe: artifact non-mana activations.
	if perm.IsArtifact() && !isMana {
		if gs.Flags != nil && gs.Flags["null_rod_count"] > 0 {
			return ActivationSuppression{Suppressed: true, Reason: "null_rod"}
		}
	}

	// Cursed Totem: ALL creature activated abilities (including mana).
	if perm.IsCreature() {
		if gs.Flags != nil && gs.Flags["cursed_totem_count"] > 0 {
			return ActivationSuppression{Suppressed: true, Reason: "cursed_totem"}
		}
	}

	// Grand Abolisher: opponents can't activate abilities of artifacts,
	// creatures, or enchantments during the Abolisher-controller's turn.
	// The Abolisher's controller IS allowed to activate freely.
	if gs.Flags != nil && seatIdx != gs.Active {
		flagKey := "grand_abolisher_active_seat_" + itoa(gs.Active)
		if gs.Flags[flagKey] > 0 {
			if perm.IsArtifact() || perm.IsCreature() || perm.IsEnchantment() {
				return ActivationSuppression{Suppressed: true, Reason: "grand_abolisher"}
			}
		}
	}

	return ActivationSuppression{}
}

// ---------------------------------------------------------------------------
// ActivateAbility — the central entry point.
// ---------------------------------------------------------------------------

// ActivateAbility is the CR §602.1 entry point for activating an activated
// ability on a permanent. Non-mana abilities are pushed onto the stack as
// StackItem{Kind: "activated"} so opponents receive priority (CR §602.1d).
// Mana abilities resolve inline per CR §605.
//
// Steps:
//  1. Stax check — abort if suppressed.
//  2. Pay activation cost (tap + mana for MVP).
//  3. If mana ability: resolve inline via InvokeActivatedHook / ResolveEffect.
//  4. If non-mana: push StackItem onto stack, run priority round, then
//     resolve when all players pass.
//
// Returns an error if the activation is illegal.
func ActivateAbility(gs *GameState, seatIdx int, perm *Permanent, abilityIdx int, targets []Target) error {
	if gs == nil {
		return &CastError{Reason: "nil game"}
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return &CastError{Reason: "invalid seat"}
	}
	if perm == nil || perm.Card == nil {
		return &CastError{Reason: "nil permanent"}
	}

	// Predefined NON-mana artifact tokens (Clue/Food/Blood/Map) carry no AST,
	// so the AST dispatch below can't run their built-in sacrifice ability.
	// Route them to the canonical token-ability primitive. (Mana tokens —
	// Treasure/Gold/Powerstone — go through ApplyArtifactMana instead.)
	if predefinedTokenSacSubtype(perm) != "" {
		if ActivatePredefinedTokenAbility(gs, seatIdx, perm) {
			return nil
		}
		return &CastError{Reason: "cannot_activate_token_ability"}
	}

	// Validate ability index.
	var ab *gameast.Activated
	if perm.Card.AST != nil && abilityIdx >= 0 && abilityIdx < len(perm.Card.AST.Abilities) {
		if a, ok := perm.Card.AST.Abilities[abilityIdx].(*gameast.Activated); ok {
			ab = a
		}
	}
	// If we can't find the Activated ability on the AST, this might still
	// be a per_card-registered activation. Allow it through — the per_card
	// hook is the authoritative handler.

	// 0.5. Exhaust check — "activate each exhaust ability only once."
	// Exhaust abilities have TimingRestriction == "exhaust" in the AST.
	// Once used, perm.Flags["exhaust_used_<idx>"] is set permanently.
	if IsExhaustAbility(perm, abilityIdx) && IsExhausted(perm, abilityIdx) {
		gs.LogEvent(Event{
			Kind:   "exhaust_already_used",
			Seat:   seatIdx,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"ability_idx": abilityIdx,
				"rule":        "exhaust",
			},
		})
		return &CastError{Reason: "exhaust_already_used"}
	}

	// plannedSacrifice carries the activation cost's predicted sacrifice
	// victim from the announcement-time pick (step 0.7, where it is
	// excluded from the legal target set) to the payment site (step 2,
	// which sacrifices exactly this permanent). nil when the ability has
	// no sacrifice cost or no announce-time pick ran.
	var plannedSacrifice *Permanent

	// 0.7. Announcement-time target legality. CR §602.2b: targets for an
	// activated ability are chosen as the ability is activated. §115.2 +
	// the keyword targeting rules (§702.11 hexproof, §702.18 shroud,
	// §702.16 protection) gate them at this point. PickTarget already
	// enforces this when the AI builds the target list; ActivateAbility
	// is the central trust boundary — validate again so per-card / fuzz /
	// fixture callers can't bypass.
	if len(targets) > 0 {
		var srcCard *Card
		if perm != nil {
			srcCard = perm.Card
		}
		if err := ValidateTargetsAtAnnouncement(gs, seatIdx, srcCard, targets, nil); err != nil {
			return err
		}
	} else if ab != nil {
		// Announcement-time pick (r62, replacing the r61 PR-3 discard-the-
		// result probe). CR §601.2c / §602.2b: an ability that REQUIRES a
		// target can't be activated unless a legal target exists — reject
		// BEFORE paying any cost, exactly as before. For single-target
		// shapes we now pick the target for real (consulting the seat's Hat
		// per §608.2a) and stamp it on the activation so resolution honors
		// it; fixed-multi shapes ("two target creatures") keep the old
		// probe + lazy resolution. CONSERVATIVE: only the curated
		// required-target effect set gates here; everything else activates
		// as before.
		probe := &StackItem{Source: perm, Effect: ab.Effect}
		if filter, ok := requiredTargetFilter(probe); ok {
			// r62.1 — predict the activation cost's sacrifice victim BEFORE
			// picking targets, and exclude it from the legal set. A
			// sacrifice-self ability (Gremlin Mine / Crater Elemental /
			// Ingenuity Engine class) could otherwise announce the
			// about-to-be-sacrificed permanent as its own target — the
			// target is then guaranteed dead before the ability reaches the
			// stack (§608.2c legality finding, 5 hits in the r62 chaos run).
			// The predicted victim is remembered and reused at cost payment
			// so the exclusion and the actual sacrifice can never diverge
			// (FindSacrificeTarget consults the Hat; asking twice could
			// pick two different victims).
			if ab.Cost.Sacrifice != nil {
				plannedSacrifice = FindSacrificeTarget(gs, seatIdx, perm, ab.Cost.Sacrifice)
			}
			rejected := false
			var picked []Target
			if plannedSacrifice != nil {
				picked = AnnounceTargets(gs, perm, seatIdx, filter, plannedSacrifice)
			} else {
				picked = AnnounceTargets(gs, perm, seatIdx, filter)
			}
			if len(picked) > 0 {
				targets = picked
			} else {
				// AnnounceTargets declines out-of-scope shapes (fixed-multi
				// quantifiers, forced self/equipped references) as well as
				// genuinely-empty legal sets — fall back to the r61 PR-3
				// probe to tell those apart, preserving the old
				// "no legal target → reject before any cost" contract.
				rejected = len(PickTarget(gs, perm, filter)) == 0
			}
			if rejected {
				gs.LogEvent(Event{
					Kind:   "activation_rejected",
					Seat:   seatIdx,
					Source: perm.Card.DisplayName(),
					Details: map[string]interface{}{
						"ability_idx": abilityIdx,
						"reason":      "no_legal_target",
						"rule":        "602.2b",
					},
				})
				return &CastError{Reason: "no_legal_target"}
			}
		}
	}

	// 1. Stax check.
	supp := StaxCheck(gs, seatIdx, perm, abilityIdx)
	if supp.Suppressed {
		gs.LogEvent(Event{
			Kind:   "activation_suppressed",
			Seat:   seatIdx,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"ability_idx": abilityIdx,
				"reason":      supp.Reason,
				"rule":        "602.1b",
			},
		})
		return &CastError{Reason: "suppressed:" + supp.Reason}
	}

	// Ride-along legality validator (legality.go): snapshot announcement-
	// time state BEFORE any cost is paid. nil-receiver no-op when off.
	legalityObs := gs.Legality.BeginActivation(gs, seatIdx, perm, abilityIdx, ab)

	// 2. Pay activation cost (MVP: tap cost + mana cost).
	seat := gs.Seats[seatIdx]

	// Loyalty cost (CR §606.5) — paid first: it is all-or-nothing and
	// needs no rollback, unlike the tap/mana pair below. A planeswalker
	// loyalty ability adjusts the walker's loyalty counters and NEVER
	// the player's life; a minus ability is only legal when the walker
	// has at least that many counters. isLoyaltyAbility also drives the
	// §606.3 once-per-turn check and disables the PayLife branch below
	// (legacy datasets parsed minus costs into PayLife — that number is
	// a loyalty adjustment, not a life payment).
	isLoyaltyAbility := false
	loyaltyDelta := 0
	if perm.IsPlaneswalker() && ab != nil {
		if d, ok := LoyaltyCost(ab); ok {
			isLoyaltyAbility = true
			loyaltyDelta = d
		} else if ab.Cost.PayLife != nil && *ab.Cost.PayLife > 0 {
			isLoyaltyAbility = true
			loyaltyDelta = -*ab.Cost.PayLife
		}
	}
	if isLoyaltyAbility {
		// §606.3: a player may activate a loyalty ability of a permanent
		// only once each turn. The flag is set in step 2.5 below and
		// cleared at turn end (phases.go).
		if perm.Flags != nil && perm.Flags["loyalty_used_this_turn"] > 0 {
			return &CastError{Reason: "loyalty_already_used"}
		}
		if err := payLoyaltyCost(gs, seatIdx, perm, loyaltyDelta); err != nil {
			return err
		}
	}

	// paidSacrifice remembers the permanent sacrificed to pay this
	// activation's cost so resolution can hand it to the per_card handler
	// as ctx["sacrificed_perm"] — Lyzolda-class abilities ("Sacrifice a
	// creature: ... if the sacrificed creature was red/black ...") read
	// the victim's characteristics at resolution, and before r63 the
	// dispatcher dropped that information on the floor.
	var paidSacrifice *Permanent
	if ab != nil {
		// Remove-counter cost (CR §602.1b) — "Remove N <kind> counters from
		// this permanent: …". Enforced FIRST so an unaffordable activation
		// rejects before any other cost is paid (nothing to roll back). The
		// cost lives either in the structured Cost.RemoveCountersN/Knd fields
		// or, for the ~346 cards the parser leaves unstructured, as a raw
		// Cost.Extra string ("remove three spore counters from this creature")
		// — RemoveCounterCostSpec reads both. Until now neither form was paid,
		// so any such ability whose only cost was the counter removal (Elvish
		// Farmer's "remove three spore counters: create a Saproling", spore /
		// fungus engines, charge-counter sinks, …) was FREE and unboundedly
		// activatable → board-explosion non-termination (firespot r63).
		//
		// Skipped when a per_card handler owns this card's activated ability —
		// that handler is authoritative for the cost and already removes the
		// counters itself, so generic enforcement would double-remove. Mirrors
		// the mana-ability rider de-dup (PerCardOwnsActivated).
		if n, kind, ok := RemoveCounterCostSpec(ab.Cost); ok && !PerCardOwnsActivated(perm.Card.DisplayName()) {
			if perm.Counters == nil || perm.Counters[kind] < n {
				return &CastError{Reason: "insufficient_counters"}
			}
			perm.Counters[kind] -= n
			gs.LogEvent(Event{
				Kind:   "remove_counter",
				Seat:   seatIdx,
				Amount: n,
				Source: perm.Card.DisplayName(),
				Details: map[string]interface{}{
					"counter_kind": kind,
					"reason":       "activation_cost",
					"rule":         "602.1b",
				},
			})
		}
		// Tap cost.
		if ab.Cost.Tap {
			if perm.Tapped {
				return &CastError{Reason: "already_tapped"}
			}
			// Summoning-sick creatures can't use tap-symbol abilities (CR §302.6).
			if perm.SummoningSick && perm.IsCreature() {
				return &CastError{Reason: "summoning_sick"}
			}
			perm.Tapped = true
			gs.LogEvent(Event{
				Kind:   "tap",
				Seat:   seatIdx,
				Source: perm.Card.DisplayName(),
				Details: map[string]interface{}{
					"reason": "activation_cost",
					"rule":   "602.1",
				},
			})
			// Dispatch tap_event for cards like Magda, Brazen Outlaw
			// and Emmara, Soul of the Accord.
			FireCardTrigger(gs, "tap_event", map[string]interface{}{
				"seat": seatIdx,
				"perm": perm,
			})
			FireTapEventASTTriggers(gs, perm)
		}
		// Mana cost.
		if ab.Cost.Mana != nil {
			cost := ab.Cost.Mana.CMC()
			if seat.ManaPool < cost {
				// Untap if we tapped for cost but can't pay mana.
				if ab.Cost.Tap {
					perm.Tapped = false
				}
				return &CastError{Reason: "insufficient_mana"}
			}
			seat.ManaPool -= cost
			SyncManaAfterSpend(seat)
			if cost > 0 {
				gs.LogEvent(Event{
					Kind:   "pay_mana",
					Seat:   seatIdx,
					Amount: cost,
					Source: perm.Card.DisplayName(),
					Details: map[string]interface{}{
						"reason": "activation_cost",
						"rule":   "602.1",
					},
				})
			}
		}
		// Life cost. Skipped for loyalty abilities: a PayLife on a
		// planeswalker is a legacy encoding of the loyalty adjustment
		// (already paid in loyalty counters above), never a life cost.
		if ab.Cost.PayLife != nil && *ab.Cost.PayLife > 0 && !isLoyaltyAbility {
			lc := *ab.Cost.PayLife
			if seat.Life <= lc {
				if ab.Cost.Tap {
					perm.Tapped = false
				}
				return &CastError{Reason: "insufficient_life"}
			}
			LoseLife(gs, seatIdx, lc, perm.Card.DisplayName())
		}
		// Sacrifice cost (CR §602.1b) — "sacrifice [filter]" or sacrifice self.
		if ab.Cost.Sacrifice != nil {
			// r62.1 — reuse the victim predicted at announcement (see step
			// 0.7) so the announce-time exclusion and the paid cost agree;
			// fall back to a fresh pick for caller-supplied-target paths
			// (no prediction ran) or if the predicted victim left the
			// battlefield in between.
			victim := plannedSacrifice
			if victim == nil || !permanentOnBattlefield(gs, victim) {
				victim = FindSacrificeTarget(gs, seatIdx, perm, ab.Cost.Sacrifice)
			}
			if victim == nil {
				if ab.Cost.Tap {
					perm.Tapped = false
				}
				return &CastError{Reason: "no_sacrifice_target"}
			}
			SacrificePermanent(gs, victim, "activation_cost")
			paidSacrifice = victim
		}
		// Discard cost — discard N cards from hand. Routes through
		// DiscardCard so CR §702.34a Madness replacement, §702.187
		// Mayhem tracking, Necropotence skip-draw rerouting, the
		// card_discarded trigger (Liliana's Caress / Waste Not /
		// Tergrid), and Turn.Discarded stat all see the discard. Pre-
		// r60-normalize this path direct-spliced seat.Hand and
		// silently bypassed every one of those rules — every
		// activated ability with "discard a card" in its cost
		// (Compulsive Research-style activations, exhume-cost
		// abilities, etc.) was broken.
		if ab.Cost.Discard != nil && *ab.Cost.Discard > 0 {
			n := *ab.Cost.Discard
			if len(seat.Hand) < n {
				if ab.Cost.Tap {
					perm.Tapped = false
				}
				return &CastError{Reason: "insufficient_cards_to_discard"}
			}
			for i := 0; i < n; i++ {
				if len(seat.Hand) == 0 {
					break
				}
				idx := len(seat.Hand) - 1
				card := seat.Hand[idx]
				DiscardCard(gs, card, seatIdx)
				gs.LogEvent(Event{
					Kind:   "discard",
					Seat:   seatIdx,
					Source: perm.Card.DisplayName(),
					Details: map[string]interface{}{
						"card":   card.DisplayName(),
						"reason": "activation_cost",
						"rule":   "602.1",
					},
				})
			}
		}
		// Discard-your-hand cost (CR §602.1b). The parser emits this as a
		// free-text "discard your hand" entry in Cost.Extra rather than a
		// numeric Cost.Discard, so the structured Discard block above never
		// saw it — every "discard your hand, …: …" ability was being
		// activated for free. This bites the mana abilities Lion's Eye
		// Diamond and Diamond Lion ("discard your hand, Sacrifice this:
		// Add three mana of any one color") — they're §605.1a mana
		// abilities that resolve inline (AddMana), but until now did so
		// without paying the hand. Discarding the whole hand is always
		// affordable (zero cards is a legal "discard your hand"), so no
		// affordability gate / tap-rollback is needed. Routes through
		// DiscardCard so Madness/Mayhem/Tergrid/card_discarded triggers and
		// Turn.Discarded all observe it, exactly like the numeric path.
		for _, extra := range ab.Cost.Extra {
			if !strings.EqualFold(strings.TrimSpace(extra), "discard your hand") {
				continue
			}
			for n := len(seat.Hand); n > 0 && len(seat.Hand) > 0; n-- {
				card := seat.Hand[len(seat.Hand)-1]
				DiscardCard(gs, card, seatIdx)
				gs.LogEvent(Event{
					Kind:   "discard",
					Seat:   seatIdx,
					Source: perm.Card.DisplayName(),
					Details: map[string]interface{}{
						"card":   card.DisplayName(),
						"reason": "activation_cost",
						"rule":   "602.1b",
					},
				})
			}
			break
		}
		// Exile-self cost (Channel and similar).
		if ab.Cost.ExileSelf {
			removePermanentFromBattlefield(gs, perm)
			seat.Exile = append(seat.Exile, perm.Card)
			gs.LogEvent(Event{
				Kind:   "exile",
				Seat:   seatIdx,
				Source: perm.Card.DisplayName(),
				Details: map[string]interface{}{
					"reason": "activation_cost",
					"rule":   "602.1",
				},
			})
		}
	}

	// 2.5. Planeswalker loyalty — §606.3: only one loyalty ability per turn.
	// All activated abilities on planeswalkers are loyalty abilities.
	if perm.IsPlaneswalker() {
		if perm.Flags == nil {
			perm.Flags = map[string]int{}
		}
		perm.Flags["loyalty_used_this_turn"] = 1
	}

	// 3. Determine mana ability status.
	isMana := IsManaAbility(perm, abilityIdx)

	// Resolve the effect.
	var eff gameast.Effect
	if ab != nil {
		eff = ab.Effect
	}

	if isMana {
		// Mana abilities resolve inline per CR §605.3a — they don't use
		// the stack and can't be responded to.
		gs.LogEvent(Event{
			Kind:   "activate_mana_ability",
			Seat:   seatIdx,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"ability_idx": abilityIdx,
				"rule":        "605.3a",
			},
		})
		// Mana abilities still mint an AB AbilityInstance for lineage
		// (any token / copy mints inside the effect resolution stamp it
		// as their EnablerInstanceID). EnablerInstanceID stays empty —
		// activated mana abilities have no triggering event.
		manaAB := NewAbilityInstance(gs, perm, seatIdx,
			fmt.Sprintf("act:%d", abilityIdx), "", nil)
		pushIIDEnabler(gs, manaAB.InstanceID)
		// Try per_card hook first; fall back to AST effect.
		manaCtx := map[string]interface{}{
			"controller": seatIdx,
			"targets":    targets,
		}
		if paidSacrifice != nil {
			manaCtx["sacrificed_perm"] = paidSacrifice
		}
		InvokeActivatedHook(gs, perm, abilityIdx, manaCtx)
		if eff != nil {
			ResolveEffect(gs, perm, eff)
		}
		// Apply any rider the parser split off into a standalone Static
		// sibling (pain-land self_damage downside, …) — see CR §605.1a:
		// the rider is part of the same mana ability and resolves inline
		// with it.
		applyManaAbilityRiders(gs, perm, abilityIdx)
		popIIDEnabler(gs)
		// Exhaust: mark used after inline resolution (mana-ability path).
		if IsExhaustAbility(perm, abilityIdx) {
			MarkExhausted(perm, abilityIdx)
		}
		// Ride-along legality validator: inline mana ability complete
		// (no stack item per CR §605.3a). nil-receiver no-op when off.
		// The reason tag lets the phase-3 mana-ability check re-derive
		// CR 605.1a independently and flag inline resolutions of
		// abilities that are NOT mana abilities.
		legalityObs.SetNoStackReason("mana_ability")
		gs.Legality.FinishActivation(gs, legalityObs, nil)
		return nil
	}

	// 3.9 (r62.1) — post-cost target re-validation, mirroring the
	// resolution-time §608.2b gate. Paying the activation cost can remove
	// an announced target (a sacrifice cost's death triggers, or the
	// sacrifice victim itself on caller-supplied-target paths). Per CR
	// §608.2b an ability whose targets have ALL become illegal is
	// countered; costs stay paid. Doing the check HERE — before the item
	// is placed on the stack — keeps an illegal target from ever being
	// announced on a stack item (the §608.2c legality-validator finding);
	// partially-illegal lists are trimmed to the legal subset exactly as
	// the resolution gate would.
	if len(targets) > 0 {
		gateProbe := &StackItem{
			Kind:       "activated",
			Controller: seatIdx,
			Source:     perm,
			Card:       perm.Card,
			Effect:     eff,
			Targets:    targets,
			AbilityIdx: abilityIdx,
		}
		allIllegal, legalTargets := CheckTargetLegality(gs, gateProbe)
		if allIllegal {
			gs.LogEvent(Event{
				Kind:   "activation_fizzle",
				Seat:   seatIdx,
				Source: perm.Card.DisplayName(),
				Details: map[string]interface{}{
					"ability_idx": abilityIdx,
					"reason":      "all_targets_illegal_after_costs",
					"rule":        "608.2b",
				},
			})
			legalityObs.SetNoStackReason("fizzled_608.2b")
			gs.Legality.FinishActivation(gs, legalityObs, nil)
			return nil
		}
		targets = legalTargets
	}

	// 4. Non-mana ability: push onto stack (CR §602.1d).
	gs.LogEvent(Event{
		Kind:   "activate_ability",
		Seat:   seatIdx,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"ability_idx": abilityIdx,
			"rule":        "602.1d",
		},
	})

	item := &StackItem{
		Kind:       "activated",
		Controller: seatIdx,
		Source:     perm,
		Card:       perm.Card,
		Effect:     eff,
		Targets:    targets,
		AbilityIdx: abilityIdx,
	}
	if paidSacrifice != nil {
		item.CostMeta = map[string]interface{}{"sacrificed_perm": paidSacrifice}
	}
	// Mint an AB AbilityInstance for this stack item. Activated abilities
	// have no triggering event so EnablerInstanceID stays empty per the
	// §4.3 schema. TriggerMetadata stays nil; AbilityID encodes the
	// activated-ability index.
	item.Ability = NewAbilityInstance(gs, perm, seatIdx,
		fmt.Sprintf("act:%d", abilityIdx), "", nil)
	PushStackItem(gs, item)

	// Ride-along legality validator: the activation is announced and on
	// the stack with its costs paid. nil-receiver no-op when off.
	gs.Legality.FinishActivation(gs, legalityObs, item)

	// CR §702.21 — Ward triggers on abilities too, not just spells.
	CheckWardOnTargeting(gs, item)

	// Priority round — opponents may respond (CR §602.2d).
	PriorityRound(gs)

	// CR §117.4 + §608.2 + §727: resolve stack with loop shortcut detection.
	DrainStack(gs)

	return nil
}

// ---------------------------------------------------------------------------
// ResolveActivatedAbility — handler for Kind=="activated" in ResolveStackTop.
// ---------------------------------------------------------------------------

// resolveActivatedAbility handles resolution of an activated-ability stack
// item. Called from ResolveStackTop when item.Kind == "activated".
func resolveActivatedAbility(gs *GameState, item *StackItem) {
	if gs == nil || item == nil {
		return
	}
	name := ""
	if item.Source != nil && item.Source.Card != nil {
		name = item.Source.Card.DisplayName()
	} else if item.Card != nil {
		name = item.Card.DisplayName()
	}

	// Try per_card snowflake hook first — if a handler is registered for
	// this card, it's authoritative.
	resolveCtx := map[string]interface{}{
		"controller": item.Controller,
		"targets":    item.Targets,
		"from_stack": true,
	}
	if item.CostMeta != nil {
		if v, ok := item.CostMeta["sacrificed_perm"]; ok && v != nil {
			resolveCtx["sacrificed_perm"] = v
		}
	}
	InvokeActivatedHook(gs, item.Source, item.AbilityIdx, resolveCtx)

	// If an AST effect is present, resolve it as well.
	if item.Effect != nil {
		src := item.Source
		if src == nil && item.Card != nil {
			// Mirror the spell-resolution synthetic-Permanent in
			// ResolveStackTop: handlers that key off src.Owner (e.g.
			// shuffle_into_owner_library) need a non-zero Owner so the
			// effect routes to the card's actual owner, not seat 0.
			src = &Permanent{
				Card:       item.Card,
				Controller: item.Controller,
				Owner:      item.Card.Owner,
				Flags:      map[string]int{},
			}
		}
		ResolveEffect(gs, src, item.Effect)
	}

	// Exhaust — mark the ability as permanently used if it's an exhaust
	// ability. This must happen AFTER resolution so the effect fires, but
	// the flag prevents all future activations.
	if item.Source != nil && IsExhaustAbility(item.Source, item.AbilityIdx) {
		MarkExhausted(item.Source, item.AbilityIdx)
	}

	gs.LogEvent(Event{
		Kind:   "activated_ability_resolved",
		Seat:   item.Controller,
		Source: name,
		Details: map[string]interface{}{
			"ability_idx": item.AbilityIdx,
			"rule":        "602.2",
		},
	})
}

// FindSacrificeTarget finds a permanent to sacrifice matching the filter.
// Returns nil if no valid target exists. Used by both buildActivationOptions
// (legality check) and ActivateAbility (cost payment).
//
// When the seat's Hat implements SacrificeChooser AND the filter has more
// than one matching candidate, the Hat picks; otherwise the first match
// (deterministic) is returned. "Self" sacrifices always resolve to the
// source permanent — there is no choice to make.
func FindSacrificeTarget(gs *GameState, seatIdx int, source *Permanent, filter *gameast.Filter) *Permanent {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) || filter == nil {
		return nil
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return nil
	}
	base := strings.ToLower(filter.Base)
	isSelf := base == "self" || base == "this" || base == "~"
	if isSelf {
		for _, p := range seat.Battlefield {
			if p == source {
				return p
			}
		}
		return nil
	}
	var candidates []*Permanent
	for _, p := range seat.Battlefield {
		if p == nil {
			continue
		}
		if matchesSacrificeFilter(p, filter) {
			candidates = append(candidates, p)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	if len(candidates) == 1 {
		return candidates[0]
	}
	if seat.Hat != nil {
		if chooser, ok := seat.Hat.(SacrificeChooser); ok {
			if pick := chooser.ChooseSacrifice(gs, seatIdx, source, "activation_cost", candidates); pick != nil {
				return pick
			}
		}
	}
	return candidates[0]
}

func matchesSacrificeFilter(p *Permanent, filter *gameast.Filter) bool {
	if p == nil || p.Card == nil || filter == nil {
		return false
	}
	base := strings.ToLower(filter.Base)
	switch {
	case base == "creature", base == "a creature":
		return p.IsCreature()
	case base == "artifact", base == "an artifact":
		return p.IsArtifact()
	case base == "land", base == "a land":
		return p.IsLand()
	case base == "enchantment", base == "an enchantment":
		return p.IsEnchantment()
	case base == "permanent", base == "a permanent":
		return true
	default:
		return true
	}
}

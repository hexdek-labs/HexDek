package gameengine

// announce_targets.go — r62 announcement-time target selection.
//
// Comp-rules citations:
//
//   §601.2c  Targets for a spell are chosen at announcement.
//   §602.2b  Targets for an activated ability are chosen on activation.
//   §608.2a  "Each target is chosen" — the Hat protocol binds this to
//            Hat.ChooseTarget (internal/gameengine/hat.go:23), which the
//            engine never called before r62: all targeting went through
//            the engine-internal PickTarget policy at resolution time.
//
// AnnounceTargets is the single seam that fixes that disconnect. It:
//
//   1. Enumerates the LEGAL target set for a required single-target filter
//      (same legality predicates PickTarget uses: control/type match plus
//      shroud / hexproof / protection when targeted).
//   2. Consults seat.Hat.ChooseTarget when a Hat is present — activating
//      the hat-side targeting intelligence (Yggdrasil's ~270-line scorer +
//      the r61 PR-8 beneficial-effect branch) that was previously dead code.
//   3. Falls back to the engine policy pick (byte-identical to the old
//      lazy resolution-time pick) when no Hat is present or the Hat
//      returns something outside the legal set.
//   4. Fires the "targeted" per-card trigger for the chosen permanent —
//      the same trigger the lazy pick used to fire at resolution, now at
//      its CR-correct announcement timing. Resolution-time consumption
//      (consumeAnnouncedTargets in targets.go) is silent, so total
//      trigger counts stay stable.
//
// The results are stamped onto StackItem.Targets by CastSpell /
// ActivateAbility, which makes the announcement machinery that keys off
// item.Targets — ValidateTargetsAtAnnouncement, CheckWardOnTargeting,
// FireHeroicTriggers, and the populated-path §608.2b resolution gate —
// operate on real data for AI-driven casts for the first time.
//
// CONSERVATIVE by design: only required SINGLE-target filters (quantifier
// "" / "one") are announced. Multi-target ("n"/"up_to_n"), fan-out
// ("each"), optional, and wrapper shapes keep the lazy resolution-time
// path unchanged.

import (
	"github.com/hexdek/hexdek/internal/gameast"
)

// announceTargetFilter extracts the filter AnnounceTargets should pick for.
// It accepts everything the §608.2b fizzle gate recognizes
// (requiredTargetFilter's curated harmful/neutral leaf set) PLUS the
// clearly-beneficial single-target leaves (Buff, GrantAbility) — those are
// deliberately NOT in the fizzle gate's set (a missing pump target should
// not over-fizzle shapes that self-target on empty), but they are exactly
// the spells whose targeting polarity the Hat must own (r61 PR-8: pump
// spells were buffing the strongest ENEMY creature).
func announceTargetFilter(item *StackItem) (gameast.Filter, bool) {
	if f, ok := requiredTargetFilter(item); ok {
		return f, true
	}
	if item == nil || item.Effect == nil {
		return gameast.Filter{}, false
	}
	switch e := item.Effect.(type) {
	case *gameast.Buff:
		if isRequiredSingleTargetFilter(e.Target) {
			return e.Target, true
		}
	case *gameast.GrantAbility:
		if isRequiredSingleTargetFilter(e.Target) {
			return e.Target, true
		}
	}
	return gameast.Filter{}, false
}

// AnnounceTargets picks announcement-time targets for a required
// single-target filter on behalf of `controller`. Returns nil when the
// filter shape is out of scope (multi-target / fan-out) or no legal target
// exists — the caller leaves StackItem.Targets empty and the lazy
// resolution path (including the r61 PR-3 fizzle gate) applies unchanged.
//
// `src` is the targeting source for protection checks: the activating
// permanent for abilities, or a transient Permanent wrapping the spell
// card for casts (same shape the §608.2b lazy gate synthesizes).
//
// `exclude` (r62.1) removes specific permanents from the legal set before
// the Hat/policy choose. ActivateAbility passes the activation cost's
// planned sacrifice victim: a sacrifice-self ability (Gremlin Mine /
// Crater Elemental class) must never announce the about-to-be-sacrificed
// permanent as its own target — CR-legal, but the target is guaranteed
// dead before the ability reaches the stack, so the announcement is a
// wasted activation flagged by the §608.2c legality validator.
func AnnounceTargets(gs *GameState, src *Permanent, controller int, f gameast.Filter, exclude ...*Permanent) []Target {
	if gs == nil || controller < 0 || controller >= len(gs.Seats) {
		return nil
	}
	seat := gs.Seats[controller]
	if seat == nil {
		return nil
	}
	switch f.Quantifier {
	case "", "one":
	default:
		return nil // multi-target shapes keep the lazy path
	}

	// A cast/activation can be announced MID-resolution of an outer item
	// (cascade, per-card handlers, hat-driven trigger casts). Mask the
	// outer frame's announced-target stash for the duration of this
	// announcement so the policy fallbacks below (which route through
	// PickTarget-family code) can never consume the OUTER spell's targets
	// as this spell's announcement.
	prevAnnounced := gs.announcedTargets
	gs.announcedTargets = nil
	defer func() { gs.announcedTargets = prevAnnounced }()

	// Forced references (self / equipped / enchanted / pronoun) aren't
	// choices — resolution resolves the reference identically with no
	// player input — and announcing them is actively risky: a cast's src
	// is a transient off-battlefield Permanent whose stamped self-target
	// would fail the §608.2b battlefield-presence check and fizzle the
	// spell. Leave these shapes to the lazy path.
	switch f.Base {
	case "equipped_creature", "equipped creature",
		"enchanted_creature", "enchanted creature",
		"enchanted_permanent", "enchanted_land", "enchanted",
		"self", "it", "this",
		"that_creature", "that creature",
		"this_creature", "this creature",
		"that_thing", "that", "them",
		"pronoun", "pronoun_it":
		return nil
	}

	// Player-only filters ("target opponent", "target player"). The legal
	// set is enumerable; the policy fallback is the existing
	// pickPlayerTarget (which fires no triggers).
	if isPlayerFilter(f) {
		legal := legalSinglePlayerTargets(gs, f, controller)
		if len(legal) == 0 {
			return nil
		}
		choice := legal[0]
		if pol := pickPlayerTarget(gs, f, controller); len(pol) == 1 {
			choice = pol[0]
		}
		if seat.Hat != nil && len(legal) > 1 {
			if t := seat.Hat.ChooseTarget(gs, controller, f, legal); targetInLegalSet(t, legal) {
				choice = t
			}
		}
		return []Target{choice}
	}

	// "Any target" (creature, player, or planeswalker — Lightning Bolt
	// class). Legal set = living opponents + targetable creatures and
	// planeswalkers. Policy fallback mirrors pickPermanentTarget's
	// any-target branch (first living opponent's face).
	if isAnyTargetShape(f) {
		legal := dropExcludedTargets(legalAnyTargets(gs, controller, src), exclude)
		if len(legal) == 0 {
			return nil
		}
		var choice Target
		if pol := pickPermanentTarget(gs, f, controller, src); len(pol) == 1 &&
			(len(exclude) == 0 || targetInLegalSet(pol[0], legal) || pol[0].Kind == TargetKindSeat) {
			choice = pol[0]
		} else {
			choice = legal[0]
		}
		if seat.Hat != nil && len(legal) > 1 {
			if t := seat.Hat.ChooseTarget(gs, controller, f, legal); targetInLegalSet(t, legal) {
				choice = t
				fireAnnouncedTargetedTrigger(gs, controller, choice)
			}
		}
		return []Target{choice}
	}

	// Permanent filters — the main surface. Enumerate, consult the Hat,
	// fall back to the engine policy pick, fire the "targeted" trigger
	// for the final choice (matching the old lazy pick's behavior).
	legal := dropExcludedTargets(legalSinglePermanentTargets(gs, f, controller, src), exclude)
	if len(legal) == 0 {
		return nil
	}
	choice := policyPickSinglePermanent(legal, controller)
	if seat.Hat != nil && len(legal) > 1 {
		if t := seat.Hat.ChooseTarget(gs, controller, f, legal); targetInLegalSet(t, legal) {
			choice = t
		}
	}
	if f.Targeted {
		fireAnnouncedTargetedTrigger(gs, controller, choice)
	}
	return []Target{choice}
}

// legalSinglePlayerTargets enumerates the seats a single-target player
// filter may choose: living, in-game seats, excluding the controller for
// opponent-scoped bases. "you"/"controller" bases resolve to exactly the
// controller (no choice to make).
func legalSinglePlayerTargets(gs *GameState, f gameast.Filter, controller int) []Target {
	switch f.Base {
	case "you", "controller":
		return []Target{{Kind: TargetKindSeat, Seat: controller}}
	}
	isOpp := isOpponentBase(f.Base) || f.Base == "that_player"
	out := []Target{}
	for i, s := range gs.Seats {
		if s == nil || s.Lost || s.LeftGame {
			continue
		}
		if isOpp && i == controller {
			continue
		}
		out = append(out, Target{Kind: TargetKindSeat, Seat: i})
	}
	return out
}

// legalAnyTargets enumerates the "any target" legal set: each living
// opponent seat plus every targetable creature and planeswalker on any
// battlefield (CR §115.4 "any target" = creature, player, planeswalker,
// or battle; battles aren't enumerated here — the engine's policy pick
// never chose them either).
func legalAnyTargets(gs *GameState, controller int, src *Permanent) []Target {
	var srcCard *Card
	if src != nil {
		srcCard = src.Card
	}
	out := []Target{}
	for _, opp := range gs.Opponents(controller) {
		s := gs.Seats[opp]
		if s == nil || s.Lost || s.LeftGame {
			continue
		}
		out = append(out, Target{Kind: TargetKindSeat, Seat: opp})
	}
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil {
				continue
			}
			if !p.IsCreature() && !p.IsPlaneswalker() {
				continue
			}
			if !CanBeTargetedByCombat(p, controller, srcCard) {
				continue
			}
			out = append(out, Target{Kind: TargetKindPermanent, Permanent: p, Seat: p.Controller})
		}
	}
	return out
}

// targetInLegalSet reports whether the Hat's returned target is one of the
// enumerated legal targets (pointer identity for permanents, index for
// seats). A zero Target or an out-of-set pick falls back to engine policy.
func targetInLegalSet(t Target, legal []Target) bool {
	switch t.Kind {
	case TargetKindPermanent:
		if t.Permanent == nil {
			return false
		}
		for _, l := range legal {
			if l.Kind == TargetKindPermanent && l.Permanent == t.Permanent {
				return true
			}
		}
	case TargetKindSeat:
		for _, l := range legal {
			if l.Kind == TargetKindSeat && l.Seat == t.Seat {
				return true
			}
		}
	}
	return false
}

// fireAnnouncedTargetedTrigger emits the "targeted" per-card trigger for a
// permanent chosen at announcement — the same event the lazy resolution
// pick used to fire (pickPermanentTarget), relocated to its CR-correct
// announcement timing. Seat targets fire nothing, matching the old path.
func fireAnnouncedTargetedTrigger(gs *GameState, srcSeat int, t Target) {
	if t.Kind != TargetKindPermanent || t.Permanent == nil || t.Permanent.Card == nil {
		return
	}
	FireCardTrigger(gs, "targeted", map[string]interface{}{
		"seat":   t.Permanent.Controller,
		"card":   t.Permanent.Card.DisplayName(),
		"source": srcSeat,
	})
}

// dropExcludedTargets filters permanent targets whose Permanent appears in
// the exclusion list (r62.1 — see AnnounceTargets). Seat targets pass
// through untouched. No-op (returns the input slice) when the exclusion
// list is empty.
func dropExcludedTargets(legal []Target, exclude []*Permanent) []Target {
	if len(exclude) == 0 || len(legal) == 0 {
		return legal
	}
	out := legal[:0]
	for _, t := range legal {
		drop := false
		if t.Kind == TargetKindPermanent && t.Permanent != nil {
			for _, ex := range exclude {
				if ex != nil && t.Permanent == ex {
					drop = true
					break
				}
			}
		}
		if !drop {
			out = append(out, t)
		}
	}
	return out
}

package outcome

// widen4.go — OUTCOME interpreter phase 4 (r63): the coverage push
// toward the full corpus. Same independence contract as every phase:
// expected deltas derive from the AST + the BoardSpec the harness
// itself built — NEVER from engine resolution code.
//
// New kinds this phase (and the board/observation extensions that make
// them observable — library lands, stocked graveyards, a stack spell,
// per-seat mana pools, live-stack count):
//
//	TUTOR         search library → hand/battlefield/graveyard: zone
//	              count deltas; which matching card is picked is
//	              policy, the zone totals are rules.
//	BOUNCE        battlefield → owner's hand / library: zone deltas.
//	              Harmful single-target bounce hits the opponent
//	              (PickTarget policy, pinned by unit test) — self-
//	              bounce ("return this to its owner's hand") hits the
//	              source.
//	MASS REMOVAL  destroy/exile all <base>: every scaffold permanent of
//	              the base dies on BOTH sides (board wipes).
//	SACRIFICE     actor-resolved: controller / each opponent / each
//	              player loses a query-matching permanent of their own.
//	COUNTERSPELL  live-stack delta: a resolved counter marks the target
//	              Countered (graveyard routing happens at stack pop,
//	              which the harness never does) — observable as the
//	              non-countered stack count dropping by one.
//	SCRY/LOOK/    zero-delta point assertions: library REORDERING must
//	REVEAL/SHUFFLE not change any observed count (catches scry
//	              implementations that draw/mill by mistake).
//	SURVEIL       disjunction over 0..N cards routed to the graveyard
//	              (the split is policy, each split is a legal outcome).
//	DAMAGE        "any target" disjunction (player life OR creature
//	              marked damage), "each player", and sublethal mass
//	              creature damage.
//	COUNTER FAN-  "put N counters on each creature [you control]":
//	OUT           counter and P/T sums scale by the scaffold count.
//	LOSE LIFE     policy-targeted single player ("target player loses
//	              N") — harmful, hits the opponent.
//	REANIMATE/    stocked-graveyard zone deltas (gy → battlefield /
//	RECURSE       hand); the stocked cards are known 4/4 creatures so
//	              the P/T sums are deterministic.
//	FIGHT         marked-damage aggregate: 4/4s fight → +8 marked
//	              (+4 for one-sided bites) regardless of which pair the
//	              policy picks.
//	ADD MANA      per-seat pool delta from the symbol list.

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// treeContainsCounterSpell reports whether the effect tree holds a
// CounterSpell leaf (the harness stocks a stack spell only then).
func treeContainsCounterSpell(eff gameast.Effect) bool {
	switch e := eff.(type) {
	case *gameast.CounterSpell:
		return true
	case *gameast.Sequence:
		for _, it := range e.Items {
			if treeContainsCounterSpell(it) {
				return true
			}
		}
	case *gameast.Choice:
		for _, o := range e.Options {
			if treeContainsCounterSpell(o) {
				return true
			}
		}
	case *gameast.Optional_:
		if e.Body != nil {
			return treeContainsCounterSpell(e.Body)
		}
	case *gameast.Conditional:
		return (e.Body != nil && treeContainsCounterSpell(e.Body)) ||
			(e.ElseBody != nil && treeContainsCounterSpell(e.ElseBody))
	}
	return false
}

// cleanFilter reports whether a filter carries no restrictions beyond
// base/quantifier/control flags (subtype, color, mana-value, and
// free-text adjectives may not match scaffold permanents — skip).
func cleanFilter(f gameast.Filter) bool {
	return len(f.Extra) == 0 && len(f.CreatureTypes) == 0 &&
		len(f.ColorFilter) == 0 && len(f.ColorExclude) == 0 &&
		f.ManaValueOp == "" && !f.NonToken
}

// scaffoldPermCount returns the (ownCount, oppCount, tappedOwn,
// tappedOpp, isCreatureBase) census for a base type on the harness
// board, INCLUDING the source on the controller's side. ok=false for
// bases the scaffold can't enumerate.
func scaffoldPermCount(spec BoardSpec, base string) (own, opp, tappedOwn, tappedOpp int, creature bool, ok bool) {
	srcOwn := 1 // the source permanent always sits on the controller's battlefield
	switch base {
	case "creature":
		own = spec.OwnCreatures
		if spec.SrcIsCreature {
			own += srcOwn
		}
		return own, spec.OppCreatures, 0, 0, true, true
	case "artifact":
		if !spec.SrcIsCreature {
			own += srcOwn
		}
		return own, spec.OppArtifacts + spec.OppTappedArtifacts, 0, spec.OppTappedArtifacts, false, true
	case "enchantment":
		return 0, spec.OppEnchants, 0, 0, false, true
	case "land":
		return spec.OwnTappedLands, spec.OppLands, spec.OwnTappedLands, 0, false, true
	case "permanent", "nonland permanent", "nonland_permanent":
		ownAll := spec.OwnCreatures + spec.OwnTappedLands + 1 // + source
		oppAll := spec.OppCreatures + spec.OppArtifacts + spec.OppTappedArtifacts +
			spec.OppEnchants + spec.OppLands
		if base != "permanent" {
			ownAll -= spec.OwnTappedLands
			oppAll -= spec.OppLands
			return ownAll, oppAll, 0, spec.OppTappedArtifacts, false, true
		}
		return ownAll, oppAll, spec.OwnTappedLands, spec.OppTappedArtifacts, false, true
	}
	return 0, 0, 0, 0, false, false
}

// expandSetValuedLeaf intercepts leaves whose expectation is inherently
// set-valued (disjunctive) before the scalar leaf dispatch. Returns
// (deltas, handled, ok).
func expandSetValuedLeaf(spec BoardSpec, eff gameast.Effect, prefixes []*Delta) ([]*Delta, bool, bool) {
	switch e := eff.(type) {
	case *gameast.Damage:
		// "any target": the policy may hit a player (life) or a creature
		// (marked damage) — both are legal outcomes.
		base := normBase(e.Target.Base)
		anyTarget := base == "any" || base == "any_target" || base == "any target" ||
			// Alternate parser emission for the same oracle text
			// ("deals N damage to any target"), r63 phase-5 census:
			// {base:"target", quantifier:"any"}.
			(base == "target" && e.Target.Quantifier == "any")
		if !anyTarget {
			return nil, false, false
		}
		n, ok := amountVal(spec, e.Amount)
		if !ok || n <= 0 || e.Divided || !cleanFilter(e.Target) {
			return nil, true, false
		}
		var out []*Delta
		for _, pre := range prefixes {
			hitOpp := pre.clone()
			hitOpp.LifeBySeat[spec.Opponent] -= n
			out = append(out, hitOpp)
			if spec.OppCreatures+spec.OwnCreatures > 0 || spec.SrcIsCreature {
				hitCreature := pre.clone()
				hitCreature.MarkedDamage += n
				out = append(out, hitCreature)
			}
		}
		if len(out) == 0 || len(out) > maxDisjuncts {
			return nil, true, false
		}
		return out, true, true

	case *gameast.Surveil:
		// Surveil N: policy routes 0..N of the looked-at cards to the
		// graveyard; every split is a legal outcome.
		n, ok := amountVal(spec, e.Count)
		if !ok || n <= 0 || n > 3 || n > spec.LibrarySize {
			return nil, true, false
		}
		var out []*Delta
		for _, pre := range prefixes {
			for toGY := 0; toGY <= n; toGY++ {
				b := pre.clone()
				b.LibraryBySeat[spec.Controller] -= toGY
				b.GraveyardBySeat[spec.Controller] += toGY
				out = append(out, b)
			}
		}
		if len(out) > maxDisjuncts {
			return nil, true, false
		}
		return out, true, true

	case *gameast.Tutor:
		if !e.Optional {
			return nil, false, false // mandatory tutors: scalar leaf path
		}
		// "you may search …": declined ∪ taken.
		taken := cloneAll(prefixes)
		for _, d := range taken {
			if !leafTutor(spec, e, d) {
				return nil, true, false
			}
		}
		out := append(cloneAll(prefixes), taken...)
		if len(out) > maxDisjuncts {
			return nil, true, false
		}
		return out, true, true
	}
	return nil, false, false
}

// leafTutor folds a mandatory (or taken-optional) tutor into d.
// Query whitelist: plain land-class and any-card searches whose match
// is guaranteed on the stocked library.
func leafTutor(spec BoardSpec, e *gameast.Tutor, d *Delta) bool {
	// Phase-5 widened gates: subtype/positive-color queries provably
	// match nothing in the stocked library (subtype-less colorless
	// fillers + Wastes basics) — a legal fail-to-find, zero delta.
	// Mana-value filters are skipped (CMC-0 fillers match <= shapes).
	if e.Query.ManaValueOp != "" || len(e.Query.Extra) > 0 {
		return false
	}
	guaranteedMiss := false
	if len(e.Query.CreatureTypes) > 0 {
		for _, sub := range e.Query.CreatureTypes {
			if strings.EqualFold(sub, "wastes") || strings.EqualFold(sub, "basic") {
				return false // the stocked basics might match: skip
			}
		}
		guaranteedMiss = true
	}
	if len(e.Query.ColorFilter) > 0 {
		guaranteedMiss = true
	}
	n := 1
	if v, ok := e.Count.IntVal(); ok && v > 0 {
		n = v
	}
	var pool int
	switch normBase(e.Query.Base) {
	case "basic land_card", "basic_land_card", "basic land", "land_card", "land card", "land":
		pool = spec.LibraryLands
	case "card", "card_card":
		pool = spec.LibrarySize
	case "creature_card", "creature card", "creature":
		pool = spec.LibrarySize - spec.LibraryLands
	case "artifact_card", "artifact", "enchantment_card", "enchantment",
		"instant_card", "instant", "sorcery_card", "sorcery",
		"planeswalker_card", "planeswalker", "battle_card", "battle":
		pool = 0 // nothing of these types is stocked: guaranteed fail-to-find
	default:
		return false
	}
	if guaranteedMiss {
		pool = 0
	}
	if pool == 0 {
		return true // legal fail-to-find: zero delta (shuffle only reorders)
	}
	if n > pool {
		// A mandatory search finds what it can: the engine moves every
		// match it has (min(count, pool)).
		n = pool
	}
	switch e.Destination {
	case "hand", "":
		d.LibraryBySeat[spec.Controller] -= n
		d.HandBySeat[spec.Controller] += n
	case "battlefield":
		d.LibraryBySeat[spec.Controller] -= n
		d.BattlefieldBySeat[spec.Controller] += n
	case "battlefield_tapped":
		d.LibraryBySeat[spec.Controller] -= n
		d.BattlefieldBySeat[spec.Controller] += n
		d.Tapped += n
	case "graveyard":
		d.LibraryBySeat[spec.Controller] -= n
		d.GraveyardBySeat[spec.Controller] += n
	case "top_of_library":
		// Reorder only — zero count delta.
	default:
		return false
	}
	if e.Rest != "" {
		return false // "put the rest into …" shapes move more cards
	}
	return true
}

// leafPhase4 extends the leaf dispatch with the phase-4 kinds. Returns
// (handled, ok): handled=false → fall through to earlier phases.
func leafPhase4(spec BoardSpec, eff gameast.Effect, d *Delta) (bool, bool) {
	switch e := eff.(type) {
	case *gameast.Tutor:
		if e.Optional {
			return true, false // set-valued; handled at expand level
		}
		return true, leafTutor(spec, e, d)

	case *gameast.Bounce:
		if leafBounce(spec, e, d) {
			return true, true
		}
		// Fall through: phase 5 handles own-bounce, zero-candidate
		// adjectives, and color-filtered shapes.
		return false, false

	case *gameast.Destroy:
		if e.Target.Quantifier != "each" && e.Target.Quantifier != "all" {
			return false, false // single-target: phase-1 path
		}
		return true, leafMassRemoval(spec, e.Target, "graveyard", d)

	case *gameast.Exile:
		if e.Target.Quantifier != "each" && e.Target.Quantifier != "all" {
			return false, false
		}
		if e.Until != "" {
			return true, false
		}
		return true, leafMassRemoval(spec, e.Target, "exile", d)

	case *gameast.Sacrifice:
		return true, leafSacrifice(spec, e, d)

	case *gameast.CounterSpell:
		// The harness stocked one live opponent spell. A resolved
		// counter marks it Countered in place (graveyard routing is the
		// stack pop's job, which never happens here): live stack -1.
		// "Unless its controller pays N": the opponent's pool is empty
		// on the scaffold, so the counter always lands.
		if spec.StackSpells < 1 {
			return true, false
		}
		base := normBase(e.Target.Base)
		if base != "spell" && base != "instant" && base != "instant_or_sorcery" {
			// Ability-countering (Stifle class) falls through to the
			// phase-6 fizzle arm.
			return false, false
		}
		if !cleanFilter(e.Target) {
			// Adjective-filtered counters ("noncreature spell") fall
			// through to the phase-7 arms.
			return false, false
		}
		d.LiveStack--
		return true, true

	case *gameast.Scry:
		n, ok := amountVal(spec, e.Count)
		if !ok || n <= 0 {
			return true, false
		}
		return true, true // reorder only: zero delta expected

	case *gameast.LookAt:
		return true, true // zero delta

	case *gameast.Reveal:
		return true, true // zero delta

	case *gameast.Shuffle:
		return true, true // zero delta

	case *gameast.Fight:
		return true, leafFight(spec, e, d)

	case *gameast.AddMana:
		total := e.AnyColorCount
		for _, sym := range e.Pool {
			if sym.IsX {
				return true, false
			}
			if sym.Generic > 0 {
				total += sym.Generic
			} else {
				total++
			}
		}
		if total <= 0 {
			return true, false
		}
		d.PoolBySeat[spec.Controller] += total
		return true, true

	case *gameast.Reanimate:
		return true, leafReanimate(spec, e, d)

	case *gameast.Recurse:
		if !cleanFilter(e.Query) || spec.GraveyardCreatures < 1 {
			return true, false
		}
		if e.FromZone != "" && e.FromZone != "your_graveyard" {
			return true, false
		}
		base := normBase(e.Query.Base)
		if base != "creature_card" && base != "creature card" && base != "creature" && base != "card" {
			return true, false
		}
		if e.Destination != "" && e.Destination != "hand" {
			return true, false
		}
		d.GraveyardBySeat[spec.Controller]--
		d.HandBySeat[spec.Controller]++
		return true, true

	case *gameast.LoseLife:
		// Policy-targeted single player: harmful → the opponent. (Self,
		// each-opponent, and each-player shapes stay on the phase-1 path.)
		n, ok := amountVal(spec, e.Amount)
		if !ok || n <= 0 {
			return false, false
		}
		base := normBase(e.Target.Base)
		if (base == "player" || base == "target_player" || base == "target player" || base == "opponent") &&
			e.Target.Quantifier != "each" && e.Target.Quantifier != "all" && e.Target.Targeted {
			d.LifeBySeat[spec.Opponent] -= n
			return true, true
		}
		return false, false

	case *gameast.Damage:
		// Mass shapes the phase-1 accumulator skips: "each player" and
		// sublethal "each/all creature" fan-outs.
		n, ok := amountVal(spec, e.Amount)
		if !ok || n <= 0 || e.Divided || !cleanFilter(e.Target) {
			return false, false
		}
		base := normBase(e.Target.Base)
		q := e.Target.Quantifier
		if base == "player" && (q == "each" || q == "all" || q == "each_player") {
			d.LifeBySeat[spec.Controller] -= n
			d.LifeBySeat[spec.Opponent] -= n
			return true, true
		}
		if base == "creature" && (q == "each" || q == "all") && n < scaffoldCreaturePT {
			if len(d.BattlefieldBySeat) > 0 {
				return true, false // board evolved earlier in the sequence
			}
			// Sublethal only: the harness never runs SBAs mid-check, but
			// staying under the 4/4 toughness keeps the expectation valid
			// even if a resolver sweeps lethals itself.
			own, opp, _, _, _, _ := scaffoldPermCount(spec, "creature")
			count := own + opp
			if e.Target.YouControl {
				count = own
			} else if e.Target.OpponentControls {
				count = opp
			}
			if count == 0 {
				return true, false
			}
			d.MarkedDamage += n * count
			return true, true
		}
		return false, false

	case *gameast.CounterMod:
		// Fan-out: "put N <kind> counters on each creature [you control]".
		if e.Op != "put" && e.Op != "" {
			return false, false
		}
		q := e.Target.Quantifier
		if q != "each" && q != "all" {
			return false, false // single-target: phase-1 path
		}
		if normBase(e.Target.Base) != "creature" || !cleanFilter(e.Target) {
			return true, false
		}
		n, ok := amountVal(spec, e.Count)
		if !ok || n <= 0 {
			return true, false
		}
		if len(d.BattlefieldBySeat) > 0 {
			// An earlier clause in this sequence already added/removed
			// permanents (e.g. "create four tokens, then put a counter
			// on each creature you control") — the fan-out count has
			// evolved past the static scaffold. Out of scope, never
			// guessed (first audit: The Crystal's Chosen over-claim).
			return true, false
		}
		own, opp, _, _, _, _ := scaffoldPermCount(spec, "creature")
		count := own + opp
		if e.Target.YouControl {
			count = own
		} else if e.Target.OpponentControls {
			count = opp
		}
		if count == 0 {
			return true, false
		}
		kind := e.CounterKind
		if kind == "" {
			kind = "+1/+1"
		}
		ptShift := 0
		switch kind {
		case "+1/+1":
			ptShift = n * count
		case "-1/-1":
			ptShift = -n * count
		}
		d.CountersByKind[kind] += n * count
		d.PowerSum += ptShift
		d.ToughSum += ptShift
		return true, true
	}
	return false, false
}

// leafBounce folds a bounce into d. Harmful single-target bounce picks
// the opponent's permanent (policy pinned by unit test); self-bounce
// returns the source to the controller's hand.
func leafBounce(spec BoardSpec, e *gameast.Bounce, d *Delta) bool {
	base := normBase(e.Target.Base)
	dest := e.To
	if dest == "" {
		dest = "owners_hand"
	}

	// Self-bounce: "return this to its owner's hand".
	if base == "self" || base == "this" || base == "it" {
		if dest != "owners_hand" {
			return false
		}
		d.BattlefieldBySeat[spec.Controller]--
		d.HandBySeat[spec.Controller]++
		if spec.SrcIsCreature {
			d.PowerSum -= scaffoldCreaturePT
			d.ToughSum -= scaffoldCreaturePT
		}
		return true
	}

	if e.Target.Quantifier == "each" || e.Target.Quantifier == "all" ||
		e.Target.Quantifier == "up_to_n" || e.Target.Quantifier == "n" {
		return false // mass / multi bounce: later phase
	}
	if !cleanFilter(e.Target) || e.Target.YouControl {
		return false
	}
	// Opponent candidate pools per base (mirrors singleRemovalExpectation:
	// harmful targeting picks the opponent).
	var candidates, tapped int
	creature := false
	switch base {
	case "creature":
		candidates, creature = spec.OppCreatures, true
	case "artifact":
		candidates, tapped = spec.OppArtifacts+spec.OppTappedArtifacts, 0
	case "enchantment":
		candidates = spec.OppEnchants
	case "land":
		candidates = spec.OppLands
	default:
		// "permanent" is set-valued (creature pick changes P/T sums).
		return false
	}
	_ = tapped
	if candidates == 0 {
		return false
	}
	d.BattlefieldBySeat[spec.Opponent]--
	switch dest {
	case "owners_hand":
		d.HandBySeat[spec.Opponent]++
	case "top_of_library", "bottom_of_library":
		d.LibraryBySeat[spec.Opponent]++
	default:
		return false
	}
	if creature {
		d.PowerSum -= scaffoldCreaturePT
		d.ToughSum -= scaffoldCreaturePT
	}
	return true
}

// leafMassRemoval folds "destroy/exile all <base>" into d. destZone is
// "graveyard" or "exile".
func leafMassRemoval(spec BoardSpec, f gameast.Filter, destZone string, d *Delta) bool {
	if !cleanFilter(f) {
		return false
	}
	if d.MarkedDamage != 0 || len(d.BattlefieldBySeat) > 0 {
		// Earlier clauses marked damage (which leaves with the swept
		// bodies — Swift Demise class) or changed the board census.
		// The static scaffold counts no longer hold: out of scope.
		return false
	}
	base := normBase(f.Base)
	if absentBases[base] {
		// "destroy all forests" / "exile all walls": the scaffold
		// provably holds none — the sweep must be a no-op.
		return true
	}
	own, opp, tappedOwn, tappedOpp, creature, ok := scaffoldPermCount(spec, base)
	if !ok {
		return false
	}
	if f.YouControl {
		opp, tappedOpp = 0, 0
	} else if f.OpponentControls {
		own, tappedOwn = 0, 0
	}
	if own+opp == 0 {
		return true // nothing matches: zero delta is the assertion
	}
	d.BattlefieldBySeat[spec.Controller] -= own
	d.BattlefieldBySeat[spec.Opponent] -= opp
	switch destZone {
	case "graveyard":
		d.GraveyardBySeat[spec.Controller] += own
		d.GraveyardBySeat[spec.Opponent] += opp
	case "exile":
		d.ExileBySeat[spec.Controller] += own
		d.ExileBySeat[spec.Opponent] += opp
	default:
		return false
	}
	d.Tapped -= tappedOwn + tappedOpp
	if creature {
		d.PowerSum -= scaffoldCreaturePT * (own + opp)
		d.ToughSum -= scaffoldCreaturePT * (own + opp)
	}
	if base == "permanent" {
		// The permanent wipe takes the creatures' bodies with it.
		cOwn, cOpp, _, _, _, _ := scaffoldPermCount(spec, "creature")
		d.PowerSum -= scaffoldCreaturePT * (cOwn + cOpp)
		d.ToughSum -= scaffoldCreaturePT * (cOwn + cOpp)
	}
	return true
}

// leafSacrifice folds an actor-resolved sacrifice into d.
func leafSacrifice(spec BoardSpec, e *gameast.Sacrifice, d *Delta) bool {
	if !cleanFilter(e.Query) {
		return false
	}
	base := normBase(e.Query.Base)
	if base == "self" || base == "this" || base == "it" {
		// Self-sacrifice rider — the source goes away.
		d.BattlefieldBySeat[spec.Controller]--
		d.GraveyardBySeat[spec.Controller]++
		if spec.SrcIsCreature {
			d.PowerSum -= scaffoldCreaturePT
			d.ToughSum -= scaffoldCreaturePT
		}
		return true
	}
	// Only the creature base is pick-independent on the scaffold: every
	// creature is an untapped 4/4, so WHICH one the policy sacrifices
	// cannot change the aggregate. Wider bases ("permanent",
	// "nonland permanent", "artifact", "land") are set-valued across
	// tapped/creature/non-creature bodies — first audit showed the
	// engine legitimately picking the tapped land where this
	// interpreter pinned an untapped pick (Crack the Earth class, 7
	// over-claims). Out of scope rather than guessed.
	if base != "creature" {
		return false
	}
	n := 1
	if e.Query.Count != nil {
		if v, ok := e.Query.Count.IntVal(); ok && v > 0 {
			n = v
		}
	}
	own, opp, _, _, _, ok := scaffoldPermCount(spec, base)
	if !ok {
		return false
	}
	sacSide := func(seat, pool int) {
		k := n
		if k > pool {
			k = pool // a player without enough permanents sacrifices what they have
		}
		d.BattlefieldBySeat[seat] -= k
		d.GraveyardBySeat[seat] += k
		d.PowerSum -= scaffoldCreaturePT * k
		d.ToughSum -= scaffoldCreaturePT * k
	}
	switch e.Actor {
	case "", "controller", "you":
		sacSide(spec.Controller, own)
		return true
	case "each_opponent", "target_player", "target_opponent", "opponent":
		sacSide(spec.Opponent, opp)
		return true
	case "each_player":
		sacSide(spec.Controller, own)
		sacSide(spec.Opponent, opp)
		return true
	}
	return false
}

// leafFight folds a fight/bite into d: every scaffold creature is a
// 4/4, so the marked-damage aggregate is pick-independent.
func leafFight(spec BoardSpec, e *gameast.Fight, d *Delta) bool {
	if !cleanFilter(e.A) || !cleanFilter(e.B) {
		return false
	}
	if normBase(e.A.Base) != "creature" || normBase(e.B.Base) != "creature" {
		return false
	}
	// Need at least two creature bodies in total for a real fight (a
	// single candidate degenerates to self-fight, still 4+4 marked, but
	// requires at least one candidate for each pick).
	total := spec.OppCreatures + spec.OwnCreatures
	if spec.SrcIsCreature {
		total++
	}
	if total < 1 {
		return false
	}
	if e.OneSided {
		d.MarkedDamage += scaffoldCreaturePT
	} else {
		d.MarkedDamage += 2 * scaffoldCreaturePT
	}
	return true
}

// leafReanimate folds a graveyard → battlefield return into d. The
// stocked graveyard cards are known 4/4 creatures.
func leafReanimate(spec BoardSpec, e *gameast.Reanimate, d *Delta) bool {
	if !cleanFilter(e.Query) || spec.GraveyardCreatures < 1 {
		return false
	}
	if e.FromZone != "" && e.FromZone != "your_graveyard" {
		return false
	}
	if e.Controller != "" && e.Controller != "you" {
		return false
	}
	if len(e.WithModifications) > 0 {
		return false // riders (extra counters, tapped …) need per-mod modeling
	}
	base := normBase(e.Query.Base)
	if base != "creature_card" && base != "creature card" && base != "creature" && base != "card" {
		return false
	}
	switch e.Destination {
	case "", "battlefield":
		// no extra delta
	case "battlefield_tapped":
		d.Tapped++
	default:
		return false
	}
	d.GraveyardBySeat[spec.Controller]--
	d.BattlefieldBySeat[spec.Controller]++
	d.PowerSum += scaffoldCreaturePT
	d.ToughSum += scaffoldCreaturePT
	return true
}

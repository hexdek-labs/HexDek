package hat

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// beneficial_target_r61.go — r61 PR-8 Fix B.
//
// ChooseTarget's permanent-targeting branch historically scored EVERY
// legal permanent as a REMOVAL target (skip own creatures, rank enemies
// by power/threat). That is correct for harmful effects (damage / destroy
// / exile / bounce / tap / -X/-X / steal), but WRONG for a single-target
// BENEFICIAL spell (pump / +1/+1 / regeneration / hexproof / indestructible
// / protection / a protective aura) whose filter admits both own and
// opponent creatures — the AI would point the buff at the strongest ENEMY
// creature, buffing/protecting the opponent.
//
// This file adds a CONSERVATIVE beneficial-vs-harmful classifier so the
// ChooseTarget permanent branch can restrict to (and rank among) the AI's
// OWN creatures when — and ONLY when — the resolving effect is clearly
// beneficial. When the intent is harmful OR unknown, ChooseTarget keeps
// its existing removal behavior byte-for-byte (a false "harmful" is the
// status quo; a false "beneficial" would break the PR-7 reactive-removal
// reliance on enemy targeting, so we err toward harmful).

// targetIntent classifies whether the resolving effect the hat is about to
// target is beneficial to the creature it lands on.
type targetIntent int

const (
	// intentUnknown — could not classify (no resolving effect found, a
	// mixed/wrapper shape, or an effect kind outside the curated set).
	// ChooseTarget keeps existing (harmful/removal) behavior.
	intentUnknown targetIntent = iota
	// intentHarmful — damage / destroy / exile / bounce / tap / steal /
	// -X/-X. ChooseTarget keeps existing removal behavior.
	intentHarmful
	// intentBeneficial — pump / +1/+1 / regen / protective keyword grant /
	// damage prevention / untap. ChooseTarget restricts to own creatures.
	intentBeneficial
)

// beneficialGrantKeywords are ability grants you generally WANT on your own
// creature (offense + protection). Detected on GrantAbility.AbilityName and
// on protective ModificationEffect ModKinds.
var beneficialGrantKeywords = map[string]bool{
	"flying":         true,
	"trample":        true,
	"lifelink":       true,
	"vigilance":      true,
	"first strike":   true,
	"double strike":  true,
	"deathtouch":     true,
	"haste":          true,
	"menace":         true,
	"reach":          true,
	"hexproof":       true,
	"shroud":         true,
	"indestructible": true,
	"protection":     true,
	"ward":           true,
	"unblockable":    true,
	"flash":          true,
	"regeneration":   true,
	"regenerate":     true,
}

// harmfulGrantKeywords are ability grants you'd put on an OPPONENT's
// creature (downside keywords). If a grant matches one of these, the
// effect is NOT treated as beneficial even if other text reads positive.
var harmfulGrantKeywords = map[string]bool{
	"defender":    true,
	"can't block": true,
	"cant block":  true,
}

// classifyTargetIntent walks an effect tree and returns a single intent.
//
// Conservative composition for wrapper nodes (Sequence / Choice /
// Optional / Conditional):
//   - any HARMFUL leaf → the whole effect is HARMFUL (keep removal).
//   - else if at least one BENEFICIAL leaf and no harmful leaf → BENEFICIAL.
//   - else → UNKNOWN.
//
// This guarantees a mixed "buff your creature AND deal damage to theirs"
// spell is never mis-classified as beneficial (it stays harmful → removal),
// and a pure single-target buff is correctly beneficial.
func classifyTargetIntent(e gameast.Effect) targetIntent {
	sawBeneficial, sawHarmful := false, false
	var walk func(gameast.Effect)
	walk = func(ef gameast.Effect) {
		if ef == nil {
			return
		}
		switch n := ef.(type) {
		case *gameast.Sequence:
			for _, it := range n.Items {
				walk(it)
			}
		case *gameast.Choice:
			for _, o := range n.Options {
				walk(o)
			}
		case *gameast.Optional_:
			walk(n.Body)
		case *gameast.Conditional:
			walk(n.Body)
			walk(n.ElseBody)
		default:
			switch leafBeneficialOrHarmful(ef) {
			case intentBeneficial:
				sawBeneficial = true
			case intentHarmful:
				sawHarmful = true
			}
		}
	}
	walk(e)

	switch {
	case sawHarmful:
		return intentHarmful
	case sawBeneficial:
		return intentBeneficial
	default:
		return intentUnknown
	}
}

// leafBeneficialOrHarmful classifies a single (non-wrapper) effect leaf.
func leafBeneficialOrHarmful(e gameast.Effect) targetIntent {
	switch n := e.(type) {
	// --- Clearly HARMFUL (keep existing removal targeting) -------------
	case *gameast.Damage:
		return intentHarmful
	case *gameast.Destroy:
		return intentHarmful
	case *gameast.Exile:
		return intentHarmful
	case *gameast.Bounce:
		return intentHarmful
	case *gameast.TapEffect:
		return intentHarmful
	case *gameast.GainControl:
		return intentHarmful
	case *gameast.Sacrifice:
		return intentHarmful
	case *gameast.Fight:
		return intentHarmful

	// --- Clearly BENEFICIAL --------------------------------------------
	case *gameast.Buff:
		// Pure pump only. A negative power/toughness (a -X/-X "buff") is a
		// removal/shrink effect → harmful. Zero/zero is a no-op → unknown.
		if n.Power < 0 || n.Toughness < 0 {
			return intentHarmful
		}
		if n.Power > 0 || n.Toughness > 0 {
			return intentBeneficial
		}
		return intentUnknown
	case *gameast.CounterMod:
		kind := strings.ToLower(strings.TrimSpace(n.CounterKind))
		op := strings.ToLower(strings.TrimSpace(n.Op))
		// +1/+1 counters put on a creature = beneficial. -1/-1 counters, or
		// removing +1/+1 counters, = harmful.
		if strings.Contains(kind, "-1/-1") {
			return intentHarmful
		}
		if strings.Contains(kind, "+1/+1") {
			if op == "remove" {
				return intentHarmful
			}
			if op == "" || op == "put" || op == "add" {
				return intentBeneficial
			}
		}
		return intentUnknown
	case *gameast.GrantAbility:
		name := strings.ToLower(strings.TrimSpace(n.AbilityName))
		if name == "" {
			return intentUnknown
		}
		for kw := range harmfulGrantKeywords {
			if strings.Contains(name, kw) {
				return intentHarmful
			}
		}
		for kw := range beneficialGrantKeywords {
			if strings.Contains(name, kw) {
				return intentBeneficial
			}
		}
		return intentUnknown
	case *gameast.Prevent:
		// Damage prevention is protective.
		return intentBeneficial
	case *gameast.UntapEffect:
		// Untapping a creature you want active is beneficial; the engine
		// scopes it to the chosen target, so on an own-or-opponent filter
		// we'd rather untap our own.
		return intentBeneficial
	case *gameast.ModificationEffect:
		mk := strings.ToLower(strings.TrimSpace(n.ModKind))
		switch {
		case strings.Contains(mk, "regenerat"),
			strings.Contains(mk, "indestructible"),
			strings.Contains(mk, "hexproof"),
			strings.Contains(mk, "shroud"),
			strings.Contains(mk, "protection"),
			strings.Contains(mk, "prevent_damage"),
			strings.Contains(mk, "shield"):
			return intentBeneficial
		case strings.Contains(mk, "goad"),
			strings.Contains(mk, "phase_out"),
			strings.Contains(mk, "stun"),
			strings.Contains(mk, "tap"):
			return intentHarmful
		}
		return intentUnknown
	}
	return intentUnknown
}

// resolvingTargetIntent infers the intent of the effect currently being
// targeted by seatIdx, by inspecting the stack. Standard targeting happens
// at cast time (CR §601.2c) while the spell/ability is on the stack, so the
// topmost stack item the hat controls is the one choosing targets.
//
// Returns intentUnknown when no own stack item is found (the safe default —
// ChooseTarget keeps removal behavior). filter is used to prefer an item
// whose effect plausibly targets the same shape, but is not required.
func (h *YggdrasilHat) resolvingTargetIntent(gs *gameengine.GameState, seatIdx int, filter gameast.Filter) targetIntent {
	if gs == nil {
		return intentUnknown
	}
	// Walk the stack top-down; the resolving/just-cast item is the top.
	for i := len(gs.Stack) - 1; i >= 0; i-- {
		item := gs.Stack[i]
		if item == nil || item.Controller != seatIdx {
			continue
		}
		eff := item.Effect
		if eff == nil && item.Card != nil {
			eff = gameengine.CollectSpellEffectOf(item.Card)
		}
		if eff == nil {
			continue
		}
		intent := classifyTargetIntent(eff)
		if intent != intentUnknown {
			return intent
		}
		// First own item with a typed effect governs; stop scanning so an
		// older unrelated own item can't flip the read.
		return intentUnknown
	}
	return intentUnknown
}

// bestBeneficialOwnTarget picks the AI's own creature that a beneficial
// effect (pump / protection / regen) helps most. Heuristic: the biggest
// attacker (highest power) is usually the one a pump wants to push through,
// and protection is most valuable on the most-threatening own creature.
// Ties break toward the lower-toughness creature (more fragile → wants the
// save). Returns the matching Target, or a zero Target if none of the legal
// targets are own creatures.
func (h *YggdrasilHat) bestBeneficialOwnTarget(gs *gameengine.GameState, seatIdx int, legal []gameengine.Target) (gameengine.Target, bool) {
	var best gameengine.Target
	bestScore := -1 << 30
	found := false
	for _, t := range legal {
		if t.Kind != gameengine.TargetKindPermanent || t.Permanent == nil {
			continue
		}
		p := t.Permanent
		if p.Controller != seatIdx {
			continue
		}
		// Prefer biggest power; lower toughness breaks ties (fragile wants
		// the buff/save). Score keeps power dominant.
		score := gs.PowerOf(p)*4 - p.Toughness()
		if !found || score > bestScore {
			best = t
			bestScore = score
			found = true
		}
	}
	return best, found
}

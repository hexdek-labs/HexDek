package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Batch AL — copy / clone family. Five high-impact cards that stress the
// spell-copy machinery the Loki 100K sweep surfaced via the Naru Meha +
// Panharmonicon mandatory-loop draw cluster:
//
//   - Clone                       — vanilla 3U "ETB as a copy of any creature"
//   - Phyrexian Metamorph         — 3+phy ETB as copy of any artifact OR creature
//   - Copy Enchantment            — 2U "ETB as a copy of any enchantment"
//   - Sakashima of a Thousand Faces (upgrade) — full DeepCopy + legend suppression
//   - Spitting Image              — sorcery, token copy of target creature + retrace
//
// All four ETB-as-copy cards share `cloneVariantETB`, parametrized by a
// type predicate and an optional rider (extra type tags, flags). Spitting
// Image gets its own OnResolve since it mints a token rather than entering
// itself.

func registerBatchALClonesR60(r *Registry) {
	r.OnETB("Clone", cloneETB)
	r.OnETB("Phyrexian Metamorph", phyrexianMetamorphETB)
	r.OnETB("Copy Enchantment", copyEnchantmentETB)
	r.OnResolve("Spitting Image", spittingImageResolve)
}

// clonePredicate decides whether a battlefield permanent is a legal copy
// source for a given variant.
type clonePredicate func(*gameengine.Permanent) bool

// cloneRider applies post-copy modifications (extra type tag flags, sac
// riders, etc.) after the Card pointer has been swapped.
type cloneRider func(*gameengine.Permanent)

// cloneVariantETB performs the canonical "enter the battlefield as a copy
// of any X" replacement: scan all battlefields for a permanent matching
// allow, deep-copy its Card onto perm, fire the copied card's ETB
// handlers so chained effects (Dockside treasure, Mulldrifter card draw,
// etc.) trigger on the copy, then run rider. Mirrors Phantasmal Image's
// pattern minus the sac-on-target rider.
//
// When no copy source exists, the variant enters as itself — a 0/0 with
// no abilities for Clone-family vanilla bodies, which SBA 704.5f kills
// on the next pass. We DON'T no-op; downstream invariants need the ETB
// event regardless of whether a copy target was available.
func cloneVariantETB(gs *gameengine.GameState, perm *gameengine.Permanent, slug string, allow clonePredicate, rider cloneRider) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	target := findCloneTarget(gs, perm, allow)
	if target == nil {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":      perm.Controller,
			"copy":      "none",
			"enters_as": "self_no_legal_target",
		})
		if rider != nil {
			rider(perm)
		}
		return
	}
	perm.OriginalCard = perm.Card
	// InstanceID Phase 5: preserve perm's InstanceID across the in-place
	// copy so identity doesn't duplicate with target.Card. Per §706.2.
	if cp := gameengine.BecomeCopyOfCard(gs, perm, target.Card); cp != nil {
		perm.Card = cp
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"copied": target.Card.DisplayName(),
	})
	if rider != nil {
		rider(perm)
	}
	fireETBOnCopy(gs, perm)
}

func findCloneTarget(gs *gameengine.GameState, perm *gameengine.Permanent, allow clonePredicate) *gameengine.Permanent {
	// Prefer same-controller targets (combo intent: clone our own ETB
	// engine), then fall back to opponents.
	if s := gs.Seats[perm.Controller]; s != nil {
		if t := scanCloneTargets(s.Battlefield, perm, allow); t != nil {
			return t
		}
	}
	for i, s := range gs.Seats {
		if s == nil || i == perm.Controller {
			continue
		}
		if t := scanCloneTargets(s.Battlefield, perm, allow); t != nil {
			return t
		}
	}
	return nil
}

func scanCloneTargets(bf []*gameengine.Permanent, self *gameengine.Permanent, allow clonePredicate) *gameengine.Permanent {
	for _, p := range bf {
		if p == nil || p == self || p.Card == nil {
			continue
		}
		if !allow(p) {
			continue
		}
		return p
	}
	return nil
}

// ---------------------------------------------------------------------
// Clone — "You may have Clone enter as a copy of any creature on the
// battlefield." Vanilla 3U clone with no rider.
// ---------------------------------------------------------------------

func cloneETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	cloneVariantETB(gs, perm, "clone_etb", func(p *gameengine.Permanent) bool {
		return p.IsCreature()
	}, nil)
}

// ---------------------------------------------------------------------
// Phyrexian Metamorph — "You may have this enter as a copy of any
// artifact or creature on the battlefield, except it's an artifact in
// addition to its other types."
//
// The "artifact in addition" rider is type-layering territory. We stamp
// perm.Flags["phy_metamorph_artifact_addendum"] so downstream type
// queries (affinity, metalcraft, indestructible-artifact triggers, etc.)
// can detect the addendum. Full §613 layer-4 type-merge is engine work.
// ---------------------------------------------------------------------

func phyrexianMetamorphETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	cloneVariantETB(gs, perm, "phyrexian_metamorph_etb",
		func(p *gameengine.Permanent) bool {
			return p.IsArtifact() || p.IsCreature()
		},
		func(p *gameengine.Permanent) {
			p.Flags["phy_metamorph_artifact_addendum"] = 1
			emitPartial(gs, "phyrexian_metamorph_etb", "Phyrexian Metamorph",
				"artifact_type_addendum_set_via_flag_full_layer4_merge_needs_engine_support")
		})
}

// ---------------------------------------------------------------------
// Copy Enchantment — "You may have Copy Enchantment enter as a copy of
// any enchantment on the battlefield."
//
// For Auras this is fiddly — a copy of an Aura must enter attached, and
// the controller chooses what to attach it to (CR §706.10). We stamp
// the copied Aura with a partial signal; the engine's Aura-attach path
// can pick up the rider later.
// ---------------------------------------------------------------------

func copyEnchantmentETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	cloneVariantETB(gs, perm, "copy_enchantment_etb",
		func(p *gameengine.Permanent) bool {
			return p.IsEnchantment()
		},
		func(p *gameengine.Permanent) {
			// Detect Aura subtype on the copied Card.
			if p.Card != nil && cardHasSubtype(p.Card, "aura") {
				p.Flags["copy_enchantment_needs_aura_attach"] = 1
				emitPartial(gs, "copy_enchantment_etb", "Copy Enchantment",
					"aura_copy_needs_attach_target_choice_per_CR_706_10")
			}
		})
}

// ---------------------------------------------------------------------
// Spitting Image — Sorcery. "Create a token that's a copy of target
// creature. Retrace (You may cast this card from your graveyard by
// discarding a land card as an additional cost.)"
//
// On resolve, mint a token copy of the chosen creature under the
// caster's control. The token enters with summoning sickness and
// inherits the source's Card via DeepCopy + "token" type tag (mirrors
// resolveCreateTokenCopy at resolve.go:1959).
//
// Retrace is a separate alt-cost machinery; we emit a parser_gap rather
// than reimplementing the cast-from-graveyard pathway here.
// ---------------------------------------------------------------------

func spittingImageResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "spitting_image_resolve"
	if gs == nil || item == nil {
		return
	}
	controller := item.Controller
	// Find a target creature: prefer item.Targets, fall back to any
	// creature on the battlefield (the resolver's auto-target policy
	// matches the engine-wide convention in resolveCreateTokenCopy).
	var src *gameengine.Permanent
	for _, t := range item.Targets {
		if t.Kind == gameengine.TargetKindPermanent && t.Permanent != nil && t.Permanent.IsCreature() {
			src = t.Permanent
			break
		}
	}
	if src == nil {
		for _, s := range gs.Seats {
			if s == nil {
				continue
			}
			for _, p := range s.Battlefield {
				if p == nil || !p.IsCreature() {
					continue
				}
				src = p
				break
			}
			if src != nil {
				break
			}
		}
	}
	if src == nil {
		emit(gs, slug, "Spitting Image", map[string]interface{}{
			"seat": controller,
			"note": "no_target_creature_no_token",
		})
		return
	}
	// InstanceID Phase 5: token-as-copy chokepoint — clears the
	// inherited InstanceID + mints a fresh TK ID. Without this the
	// token would carry src.Card.InstanceID and the CardIdentity
	// invariant would fire (same ID in two zones).
	card := gameengine.MintTokenAsCopyOf(gs, src.Card, controller, "")
	if card == nil {
		return
	}
	token := &gameengine.Permanent{
		Card:                   card,
		Controller:             controller,
		CopiedTargetInstanceID: src.Card.InstanceID,
		Owner:         controller,
		SummoningSick: true,
		Timestamp:     gs.NextTimestamp(),
		Counters:      map[string]int{},
		Flags:         map[string]int{},
	}
	if seat := gs.Seats[controller]; seat != nil {
		seat.Battlefield = append(seat.Battlefield, token)
	}
	emit(gs, slug, "Spitting Image", map[string]interface{}{
		"seat":     controller,
		"token_of": src.Card.DisplayName(),
	})
	emitPartial(gs, slug, "Spitting Image",
		"retrace_alt_cost_cast_from_graveyard_discard_land_needs_alt_cost_primitive")
	// Fire ETB on the token so chained ETB effects (Lord of Extinction
	// graveyard count, Mulldrifter card draw, etc.) trigger.
	fireETBOnCopy(gs, token)
}

// ---------------------------------------------------------------------
// Sakashima of a Thousand Faces — UPGRADE.
//
// Pre-upgrade impl in custom_sakashima_of_a_thousand_faces.go did only
// P/T copy. CR §706.2 says "enter as a copy" means the new permanent
// copies every printable characteristic of the original. The upgrade
// path here calls sakashimaCopyETB() first (preserving the original
// emit + the existing test's P/T-copy assertion), then does the full
// DeepCopy + legend-rule-suppression rider.
//
// Wire-up note: the existing handler is registered in registry.go via
// registerSakashimaOfAThousandFacesCustom(). We don't add a second
// registration; instead we modify sakashimaCopyETB itself in the
// custom_* file to delegate to this upgrade after its P/T pass. Keeping
// the upgrade logic colocated with the other batch AL clones makes the
// "all clones share machinery" intent legible.
// ---------------------------------------------------------------------

func sakashimaFullCopyUpgrade(gs *gameengine.GameState, perm *gameengine.Permanent, target *gameengine.Permanent) {
	const slug = "sakashima_full_copy_upgrade"
	if gs == nil || perm == nil || target == nil || target.Card == nil {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	// Full DeepCopy preserves abilities/types/subtypes, not just P/T.
	// InstanceID Phase 5: BecomeCopyOfCard preserves perm.Card.InstanceID
	// across the copy + stamps CopiedTargetInstanceID + CopiableSnapshot.
	perm.OriginalCard = perm.Card
	if cp := gameengine.BecomeCopyOfCard(gs, perm, target.Card); cp != nil {
		perm.Card = cp
	}
	// Legend-rule suppression: "The 'legend rule' doesn't apply to
	// permanents you control." Sakashima-controller can have two of any
	// legendary. We stamp the flag; the SBA 704.5j pass consults it via
	// the existing legend-rule arbitration helpers.
	perm.Flags["legend_rule_suppressed"] = 1
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"copied": target.Card.DisplayName(),
		"rider":  "legend_rule_suppressed",
	})
	fireETBOnCopy(gs, perm)
}

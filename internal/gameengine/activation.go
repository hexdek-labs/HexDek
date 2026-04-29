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
//   (1) it doesn't target,
//   (2) it could add mana to a player's mana pool, and
//   (3) it's not a loyalty ability.
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
	case *gameast.Sequence:
		for _, sub := range v.Items {
			if effectProducesMana(sub) {
				return true
			}
		}
	}
	return false
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
//   1. Stax check — abort if suppressed.
//   2. Pay activation cost (tap + mana for MVP).
//   3. If mana ability: resolve inline via InvokeActivatedHook / ResolveEffect.
//   4. If non-mana: push StackItem onto stack, run priority round, then
//      resolve when all players pass.
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

	// 2. Pay activation cost (MVP: tap cost + mana cost).
	seat := gs.Seats[seatIdx]
	if ab != nil {
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
		// Life cost.
		if ab.Cost.PayLife != nil && *ab.Cost.PayLife > 0 {
			lc := *ab.Cost.PayLife
			if seat.Life <= lc {
				if ab.Cost.Tap {
					perm.Tapped = false
				}
				return &CastError{Reason: "insufficient_life"}
			}
			seat.Life -= lc
			gs.LogEvent(Event{
				Kind:   "pay_life",
				Seat:   seatIdx,
				Amount: lc,
				Source: perm.Card.DisplayName(),
				Details: map[string]interface{}{
					"reason": "activation_cost",
					"rule":   "602.1",
				},
			})
		}
		// Sacrifice cost (CR §602.1b) — "sacrifice [filter]" or sacrifice self.
		if ab.Cost.Sacrifice != nil {
			victim := FindSacrificeTarget(gs, seatIdx, perm, ab.Cost.Sacrifice)
			if victim == nil {
				if ab.Cost.Tap {
					perm.Tapped = false
				}
				return &CastError{Reason: "no_sacrifice_target"}
			}
			SacrificePermanent(gs, victim, "activation_cost")
		}
		// Discard cost — discard N cards from hand.
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
				seat.Hand = seat.Hand[:idx]
				seat.Graveyard = append(seat.Graveyard, card)
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
		// Try per_card hook first; fall back to AST effect.
		InvokeActivatedHook(gs, perm, abilityIdx, map[string]interface{}{
			"controller": seatIdx,
			"targets":    targets,
		})
		if eff != nil {
			ResolveEffect(gs, perm, eff)
		}
		// Exhaust: mark used after inline resolution (mana-ability path).
		if IsExhaustAbility(perm, abilityIdx) {
			MarkExhausted(perm, abilityIdx)
		}
		return nil
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
	PushStackItem(gs, item)

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
	InvokeActivatedHook(gs, item.Source, item.AbilityIdx, map[string]interface{}{
		"controller": item.Controller,
		"targets":    item.Targets,
		"from_stack": true,
	})

	// If an AST effect is present, resolve it as well.
	if item.Effect != nil {
		src := item.Source
		if src == nil && item.Card != nil {
			src = &Permanent{
				Card:       item.Card,
				Controller: item.Controller,
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
	for _, p := range seat.Battlefield {
		if p == nil {
			continue
		}
		if isSelf {
			if p == source {
				return p
			}
			continue
		}
		if matchesSacrificeFilter(p, filter) {
			return p
		}
	}
	return nil
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

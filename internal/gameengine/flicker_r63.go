package gameengine

import (
	"regexp"
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// flicker_r63.go — generic FLICKER / BLINK (exile-and-immediately-return).
//
// Mechanic audit r63: Cloudshift / Ephemerate / Flicker / Blur / Acrobatic
// Maneuver / Eldrazi Confluence … parse to a bare gameast.Exile node whose
// "then return that card to the battlefield" clause lives only in the parent
// ability's raw text (the Exile AST struct has no return field). resolveExile
// therefore just ExilePermanent'd the target and NEVER returned it — ~52
// "exile then return to the battlefield" spells permanently exiled their own
// creature. This file adds the missing return as a NEW object per CR §400.7.
//
// New-object semantics (CR §400.7, §111.7, §704.5d):
//   - new timestamp, summoning sick (unless haste), empty counters/flags,
//     damage cleared (fresh Permanent), auras/equipment dropped (detachAll).
//   - ETB triggers fire on the return (FirePermanentETBTriggers).
//   - a TOKEN ceases to exist and does NOT return (§111.7 / §704.5d).
//   - a DFC/MDFC returns on its FRONT face (EnsureBattlefieldFrontFace).
//   - returns under the controlSeat the caller specifies ("under your
//     control" vs "under its owner's control").

// FlickerPermanent exiles target and immediately returns it to the
// battlefield as a brand-new object under controlSeat's control. Returns the
// new Permanent, or nil when the card does not return: a token (ceased), a
// non-permanent card type, or a card whose owner has left the game (§800.4a).
func FlickerPermanent(gs *GameState, target *Permanent, controlSeat int) *Permanent {
	if gs == nil || target == nil || target.Card == nil {
		return nil
	}
	card := target.Card
	isToken := target.IsToken()
	if controlSeat < 0 || controlSeat >= len(gs.Seats) || gs.Seats[controlSeat] == nil {
		controlSeat = card.Owner
	}

	// Leave the battlefield (→ exile). Fire LTB zone-change triggers while
	// the leaving permanent's identity is still intact, then run the
	// canonical removal (unregister §613/§614 hooks, detach auras/equipment,
	// cease the card if it's a token and clear its InstanceID).
	FireZoneChangeTriggers(gs, target, card, "battlefield", "exile")
	ExpireSourceGrants(gs, target.Timestamp)
	removePermanentFromBattlefield(gs, target)

	// §111.7 / §704.5d — a token that has left the battlefield ceases to
	// exist; it cannot return. (removePermanentFromBattlefield already
	// marked its InstanceID ceased.)
	if isToken {
		gs.LogEvent(Event{
			Kind:   "flicker_token_ceased",
			Seat:   controlSeat,
			Source: card.DisplayName(),
			Details: map[string]interface{}{"rule": "111.7"},
		})
		return nil
	}

	// CR §712.13 — a flickered double-faced card returns front face up.
	EnsureBattlefieldFrontFace(card)
	// CR §304.4 / §307.1 — never wrap a non-permanent in a Permanent.
	if !CardCanEnterBattlefield(card) {
		return nil
	}
	// CR §800.4a — a card owned by a departed player left with them.
	if card.Owner >= 0 && card.Owner < len(gs.Seats) &&
		gs.Seats[card.Owner] != nil && gs.Seats[card.Owner].LeftGame {
		return nil
	}
	EnforceBattlefieldUniqueInstanceID(gs, card, controlSeat)

	sick := false
	if cardHasTypeName(card, "creature") {
		sick = !CardHasKeyword(card, "haste")
	}
	newPerm := &Permanent{
		Card:          card,
		Controller:    controlSeat,
		Owner:         card.Owner,
		SummoningSick: sick,
		Timestamp:     gs.NextTimestamp(),
		Counters:      map[string]int{},
		Flags:         map[string]int{},
	}
	gs.Seats[controlSeat].Battlefield = append(gs.Seats[controlSeat].Battlefield, newPerm)
	RegisterReplacementsForPermanent(gs, newPerm)
	gs.LogEvent(Event{
		Kind:   "flicker",
		Seat:   controlSeat,
		Target: card.Owner,
		Source: card.DisplayName(),
		Details: map[string]interface{}{
			"returned_as_new_object": true,
			"rule":                   "400.7",
		},
	})
	FirePermanentETBTriggers(gs, newPerm)
	return newPerm
}

// cardHasTypeName is a small case-insensitive Card type check (mirrors the
// per_card helper) — kept local so the flicker helper stays self-contained.
func cardHasTypeName(c *Card, want string) bool {
	if c == nil {
		return false
	}
	for _, t := range c.Types {
		if strings.EqualFold(t, want) {
			return true
		}
	}
	return false
}

// reFlickerImmediate matches an "exile … then return … to the battlefield"
// clause that returns the EXILED object immediately (no end-step delay).
// Gated tightly so only genuine immediate-flicker spells change behavior:
// requires exile→return→battlefield in one clause and the return pronoun to
// reference the exiled object (it / that card / those cards / them / that
// permanent / that creature).
var reFlickerImmediate = regexp.MustCompile(`(?i)exile[^.]*?then return (?:it|them|that card|those cards|that permanent|that creature)[^.]*?to the battlefield`)

// reFlickerYourControl distinguishes "under your control" (keep control,
// e.g. Cloudshift) from the default "under its owner's control".
var reFlickerYourControl = regexp.MustCompile(`(?i)return[^.]*?to the battlefield under your control`)

// exileFlickerSpec inspects the source spell/permanent's AST for an
// immediate-flicker clause. Returns (isFlicker, underYourControl). Used by
// resolveExile to route an Exile effect through FlickerPermanent instead of a
// plain permanent exile. Delayed ("at the beginning of the next end step")
// returns are deliberately excluded — they are a separate (also-broken) shape
// noted in the audit, not handled here.
func exileFlickerSpec(src *Permanent) (bool, bool) {
	if src == nil || src.Card == nil || src.Card.AST == nil {
		return false, false
	}
	for _, ab := range src.Card.AST.Abilities {
		raw := abilityRaw(ab)
		if raw == "" {
			continue
		}
		low := strings.ToLower(raw)
		if strings.Contains(low, "end step") || strings.Contains(low, "beginning of") {
			continue // delayed-return variant — out of scope here
		}
		if reFlickerImmediate.MatchString(raw) {
			return true, reFlickerYourControl.MatchString(raw)
		}
	}
	return false, false
}

// exileDelayedFlickerSpec inspects the source's AST for a DELAYED-return
// flicker — "exile … return … to the battlefield at the beginning of the next
// end step" (Flickerwisp, Otherworldly Journey, Long Road Home). Returns
// (isDelayed, underYourControl, returnsWithPlusOne). Before r63 these parsed to
// a bare Exile node and the permanent was exiled FOREVER (the return clause was
// dropped). resolveExile now exiles the target and registers a delayed trigger
// to return it as a NEW object at the next end step.
func exileDelayedFlickerSpec(src *Permanent) (bool, bool, bool) {
	if src == nil || src.Card == nil || src.Card.AST == nil {
		return false, false, false
	}
	for _, ab := range src.Card.AST.Abilities {
		raw := abilityRaw(ab)
		if raw == "" {
			continue
		}
		low := strings.ToLower(raw)
		if !strings.Contains(low, "to the battlefield") {
			continue
		}
		if !strings.Contains(low, "return") {
			continue
		}
		if !strings.Contains(low, "next end step") {
			continue
		}
		yourControl := strings.Contains(low, "under your control")
		plusOne := strings.Contains(low, "+1/+1 counter")
		return true, yourControl, plusOne
	}
	return false, false, false
}

// RegisterDelayedFlickerReturn schedules a flickered card (already in its
// owner's exile) to return to the battlefield as a new object at the next end
// step (CR §603.7 delayed trigger). controlSeat is where it returns; addPlusOne
// stamps a single +1/+1 counter (Otherworldly Journey / Long Road Home).
func RegisterDelayedFlickerReturn(gs *GameState, card *Card, controlSeat int, addPlusOne bool, srcName string) {
	if gs == nil || card == nil {
		return
	}
	gs.RegisterDelayedTrigger(&DelayedTrigger{
		TriggerAt:       "next_end_step",
		ControllerSeat:  controlSeat,
		SourceCardName:  srcName,
		SourceTimestamp: gs.NextTimestamp(),
		OneShot:         true,
		EffectFn: func(gs *GameState) {
			returnDelayedFlicker(gs, card, controlSeat, addPlusOne, srcName)
		},
	})
}

// returnDelayedFlicker is the delayed-trigger body: pull the card out of exile
// and re-enter it as a brand-new object. If the card is no longer in exile (it
// moved, or a token ceased), nothing returns — the linkage is to the exiled
// card, and a card that left exile by other means is not returned (CR §603.7).
func returnDelayedFlicker(gs *GameState, card *Card, controlSeat int, addPlusOne bool, srcName string) {
	if gs == nil || card == nil {
		return
	}
	// The card returns from its OWNER's exile (CR §406 exile is owner-scoped).
	if card.Owner < 0 || card.Owner >= len(gs.Seats) || gs.Seats[card.Owner] == nil {
		return
	}
	ownerExile := gs.Seats[card.Owner].Exile
	idx := -1
	for i, c := range ownerExile {
		if c == card {
			idx = i
			break
		}
	}
	if idx < 0 {
		return // no longer in exile — do not return (§603.7).
	}
	if controlSeat < 0 || controlSeat >= len(gs.Seats) || gs.Seats[controlSeat] == nil {
		controlSeat = card.Owner
	}
	// §800.4a — owner left the game: the card left with them.
	if gs.Seats[card.Owner].LeftGame {
		return
	}
	EnsureBattlefieldFrontFace(card)
	if !CardCanEnterBattlefield(card) {
		return
	}
	// Remove from exile.
	gs.Seats[card.Owner].Exile = append(ownerExile[:idx], ownerExile[idx+1:]...)
	EnforceBattlefieldUniqueInstanceID(gs, card, controlSeat)
	sick := false
	if cardHasTypeName(card, "creature") {
		sick = !CardHasKeyword(card, "haste")
	}
	newPerm := &Permanent{
		Card:          card,
		Controller:    controlSeat,
		Owner:         card.Owner,
		SummoningSick: sick,
		Timestamp:     gs.NextTimestamp(),
		Counters:      map[string]int{},
		Flags:         map[string]int{},
	}
	// §122.1 — the +1/+1 counter is part of the returning event; stamp it
	// BEFORE the ETB cascade so ETB-counter observers (Hardened Scales,
	// Doubling Season) see the entering permanent in its final state.
	if addPlusOne {
		newPerm.Counters["+1/+1"] = 1
	}
	gs.Seats[controlSeat].Battlefield = append(gs.Seats[controlSeat].Battlefield, newPerm)
	RegisterReplacementsForPermanent(gs, newPerm)
	gs.LogEvent(Event{
		Kind:   "flicker_return",
		Seat:   controlSeat,
		Target: card.Owner,
		Source: srcName,
		Details: map[string]interface{}{
			"returned_card":          card.DisplayName(),
			"returned_as_new_object": true,
			"with_plus_one":          addPlusOne,
			"rule":                   "603.7",
		},
	})
	FirePermanentETBTriggers(gs, newPerm)
}

// abilityRaw extracts the .Raw clause text from any AST ability node.
func abilityRaw(ab gameast.Ability) string {
	switch a := ab.(type) {
	case *gameast.Static:
		return a.Raw
	case *gameast.Triggered:
		return a.Raw
	case *gameast.Activated:
		return a.Raw
	}
	return ""
}

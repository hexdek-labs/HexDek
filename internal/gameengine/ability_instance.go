package gameengine

import (
	"github.com/hexdek/hexdek/internal/gameengine/instanceid"
)

// AbilityInstance is the InstanceID-bearing object representing a single
// triggered or activated ability that has been put on (or queued for) the
// stack per CR §603 / §602.
//
// Per docs/instanceid-system-v2-r60.md §4.3, each AB instance carries:
//
//   - InstanceID         — AB-provenance ID minted at push time
//   - SourceInstanceID   — InstanceID of the Permanent that owns the ability
//   - EnablerInstanceID  — the triggering event identifier (empty for
//     activated abilities, populated for triggers — typically the same
//     SourceInstanceID for self-triggers, or the InstanceID of the event
//     source for observer triggers)
//   - AbilityID          — discriminator for multi-ability cards (matches
//     StackItem.AbilityIdx for activated, a string slug for triggered)
//   - Controller         — seat that controls the ability per §603.7d
//   - PushedAt           — game-clock timestamp at stack push time
//
// TriggerMetadata is the per-instance bag for trigger-time captured state
// — storm count, X value, "ascension active", etc. Per the design v2
// schema-discipline open question, common keys are documented inline:
//
//   - "storm_count"   int    — captured for storm copies (§702.40)
//   - "x_value"       int    — captured for X-spells (§107.3)
//   - "ascension"     bool   — captured city's-blessing state at trigger
//   - "target_perm"   string — InstanceID of a captured target permanent
//
// Delayed triggers (§603.7c) reserve DelayedUntil / DelayedCreatedAt /
// DelayedExpiresAt. Phase 2 leaves the delayed pool stubbed; full wiring
// arrives in Phase 8.
type AbilityInstance struct {
	InstanceID        string
	SourceInstanceID  string
	EnablerInstanceID string
	AbilityID         string
	Controller        int
	PushedAt          int

	TriggerMetadata map[string]any

	DelayedUntil     *DelayedCondition
	DelayedCreatedAt int
	DelayedExpiresAt int
}

// DelayedCondition describes when an AbilityInstance sitting in
// gs.DelayedAbilityInstances becomes eligible to fire, per CR §603.7c
// and docs/instanceid-system-v2-r60.md §11.
//
// Phase 8 schema:
//
//   - EventType   — canonical event slug ("begin_upkeep", "begin_end_step",
//     "creature_dies", "spell_cast", "turn_end", "untap"). The pool walker
//     dispatches on this first; entries with EventType=="" never match.
//   - EventFilter — optional predicate evaluated against the firing event's
//     payload. Returning true allows the trigger to fire. Nil = no extra
//     filter beyond EventType.
//   - OneShot     — true (the common case) means the entry is removed from
//     the pool after the first firing. False = stays in pool until
//     DelayedExpiresAt or removed by other cleanup.
//   - Recurring   — non-nil for repeating delayed triggers (e.g. Suspend's
//     "at the beginning of upkeep, remove a time counter"). The walker uses
//     this to decide whether to keep the entry in the pool after firing,
//     and to advance the schedule for any next-only matchers.
type DelayedCondition struct {
	EventType   string
	EventFilter func(ev *Event) bool
	OneShot     bool
	Recurring   *Recurrence
}

// Recurrence describes how often a recurring delayed trigger fires. Per
// design v2 §11 the typical patterns are "every upkeep until a counter
// goes to zero" (Suspend) and "until your next upkeep" (madness-shape
// pending costs). Until is the predicate that decides when the recurrence
// ends — when Until returns true after a firing, the entry is removed
// from the pool.
type Recurrence struct {
	Until func(gs *GameState, ab *AbilityInstance) bool
}

// NewAbilityInstance constructs an AbilityInstance with a freshly minted
// AB-provenance InstanceID. Returns a partially populated instance even
// when minting fails (legacy / nil-minter paths) so callers can attach it
// to a StackItem without nil-checks; downstream code treats an empty
// InstanceID as legacy.
//
// abilityID is a free-form discriminator — callers use "trig:<event>" for
// triggered abilities and "act:<idx>" for activated abilities.
func NewAbilityInstance(
	gs *GameState,
	src *Permanent,
	controller int,
	abilityID string,
	enablerID string,
	meta map[string]any,
) *AbilityInstance {
	ab := &AbilityInstance{
		AbilityID:         abilityID,
		EnablerInstanceID: enablerID,
		Controller:        controller,
		TriggerMetadata:   meta,
	}
	if src != nil && src.Card != nil {
		ab.SourceInstanceID = src.Card.InstanceID
	}
	if gs != nil {
		ab.PushedAt = gs.EffectTimestamp
	}
	if gs == nil || gs.IIDMinter == nil {
		return ab
	}
	seat := controller
	if seat < 0 || seat >= gs.IIDMinter.SeatCount() {
		return ab
	}
	// Color + CMC for an AB instance are taken from the source permanent
	// per §603.10 (the triggered/activated ability has the same
	// characteristics as its source). When the source has no Card, fall
	// back to colorless / CMC 0 — the ID is still useful as a unique
	// stack-item label.
	color := "C"
	cmc := 0
	if src != nil && src.Card != nil {
		color = instanceid.CanonicalColor(src.Card.Colors)
		if src.Card.CMC > 0 {
			cmc = src.Card.CMC
		}
	}
	id := gs.IIDMinter.Mint(seat, instanceid.ProvAB, instanceid.Visible, color, cmc)
	if id != "" {
		ab.InstanceID = id
		RecordMintedInstanceID(gs, id)
	}
	return ab
}

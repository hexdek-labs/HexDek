package gameengine

// InstanceID Phase 5 — copy + replacement-effect handlers per
// docs/instanceid-system-v2-r60.md §5 (CopyMechanism registry) and §6
// (Replacement-effect layer).
//
// This file holds the type definitions, audit-trail helpers, and the
// MintTokenAsCopyOf chokepoint. Per-card override handlers live in
// instanceid_phase5_overrides.go. The CopyMechanisms slice + companion
// fields are declared on Permanent in state.go via the Phase 5 block.

// CopyTrigger names the kind of game event that activates a copy
// mechanism. Per design §5 + Probe A.
type CopyTrigger int

const (
	CopyTriggerETB CopyTrigger = iota
	CopyTriggerUpkeep
	CopyTriggerActivated
	CopyTriggerEventCondition
	CopyTriggerEOT
	CopyTriggerCombatBegin
	CopyTriggerOther
)

// String renders CopyTrigger for logs / invariant errors.
func (t CopyTrigger) String() string {
	switch t {
	case CopyTriggerETB:
		return "ETB"
	case CopyTriggerUpkeep:
		return "Upkeep"
	case CopyTriggerActivated:
		return "Activated"
	case CopyTriggerEventCondition:
		return "EventCondition"
	case CopyTriggerEOT:
		return "EOT"
	case CopyTriggerCombatBegin:
		return "CombatBegin"
	}
	return "Other"
}

// CopyDuration names how long a CopyMechanism's effect persists.
type CopyDuration int

const (
	CopyDurationPermanent CopyDuration = iota
	CopyDurationUntilEOT
	CopyDurationUntilNextCopy
	CopyDurationUntilNextUpkeep
	CopyDurationUntilNextEvent
)

// String renders CopyDuration for logs / invariant errors.
func (d CopyDuration) String() string {
	switch d {
	case CopyDurationUntilEOT:
		return "UntilEOT"
	case CopyDurationUntilNextCopy:
		return "UntilNextCopy"
	case CopyDurationUntilNextUpkeep:
		return "UntilNextUpkeep"
	case CopyDurationUntilNextEvent:
		return "UntilNextEvent"
	}
	return "Permanent"
}

// CopyTarget distinguishes copy-the-target-permanent from
// mint-a-new-token-as-copy-of-target per design §5 model-breaker row 5.
type CopyTarget int

const (
	CopyTargetSelf           CopyTarget = iota // this permanent becomes the copy
	CopyTargetOtherPermanent                   // a different existing permanent is copied
	CopyTargetNewToken                         // a fresh token is minted as a copy
)

// CopyRestriction filters legal copy targets per §706.3 (only permanents
// you could legally affect with the originating effect can be copied).
type CopyRestriction struct {
	TypeFilter     []string // e.g. ["artifact","creature","enchantment"]
	ControllerSeat int      // -1 = any controller
	SourceZone     string   // empty = battlefield (default); set for §5 row 2 Ertai-Meddling exile-snapshot
}

// CopyMechanism is one independently-triggered copy capability on a
// permanent. Per Probe A, cards like Mirage Mirror carry two entries:
// one upkeep-permanent, one activated-temporary.
type CopyMechanism struct {
	TriggerSource   CopyTrigger
	Duration        CopyDuration
	Target          CopyTarget
	Restriction     *CopyRestriction
	PerCardOverride string // names a handler in the Phase 5 overrides registry
}

// CopyEvent is an append-only audit log entry written each time a
// CopyMechanism fires. Replay tooling + Heimdall consume CopyHistory.
type CopyEvent struct {
	AtGameTick int
	SourceID   string // InstanceID of the permanent being copied
	EnablerID  string // AbilityInstance.InstanceID that triggered the copy
	Mechanism  int    // index into Permanent.CopyMechanisms
}

// CopiableCharacteristics is the §706.2 snapshot frozen at copy time.
// Captures the printed values the copy uses; layered effects on the copy
// stack on top of this snapshot via Layer 1.
type CopiableCharacteristics struct {
	Name       string
	Types      []string
	Subtypes   []string
	Supertypes []string
	Colors     []string
	Power      int
	Toughness  int
	CMC        int
	Keywords   []string
	// SourceInstanceID is the originating InstanceID at snapshot time.
	SourceInstanceID string
}

// ReplacementSpec describes a replacement effect a permanent PROVIDES
// (Doubling Season, Mondrak, Anointed Procession). Phase 5 surfaces this
// on Permanent so the §616 chain can be walked statically by the audit
// helper without re-querying gs.Replacements every time.
type ReplacementSpec struct {
	Name      string // human-readable handler label
	EventType string // ReplEvent.Type this provider listens on
	Category  string // CategorySelfReplacement / CategoryOther / etc.
	HandlerID string // unique key — matches gs.Replacements entry
}

// ReplacementOp names the kind of mutation a replacement applies.
type ReplacementOp int

const (
	ReplacementOpDouble ReplacementOp = iota
	ReplacementOpHalve
	ReplacementOpRedirect
	ReplacementOpZoneSubstitute
	ReplacementOpSkip
)

// String renders ReplacementOp for logs / invariant errors.
func (op ReplacementOp) String() string {
	switch op {
	case ReplacementOpHalve:
		return "Halve"
	case ReplacementOpRedirect:
		return "Redirect"
	case ReplacementOpZoneSubstitute:
		return "ZoneSubstitute"
	case ReplacementOpSkip:
		return "Skip"
	}
	return "Double"
}

// ReplacementRef is one entry in the EffectsApplied chain on a
// TokenMintEvent (or other Phase 5 audit-trail event). Records which
// permanent provided the replacement, what mutation it performed, and
// when within the chain.
type ReplacementRef struct {
	Source        string // InstanceID of the Permanent that provided the replacement
	SourceName    string // human-readable label for logs
	Modification  ReplacementOp
	HandlerID     string // matches ReplacementEffect.HandlerID
	AppliedAtTick int
	CountBefore   int
	CountAfter    int
}

// TokenMintEvent is the Phase 5 audit trail for a single token-mint
// resolution. One TokenMintEvent per FireCreateTokenEvent call. Records
// the §616 replacement chain that brought the final count from
// BaseCount to FinalCount, plus the InstanceIDs of every minted token.
//
// Per design v2 §6, this enables the Sai+Mondrak+Anointed+DS walkthrough:
// EffectsApplied lists [Mondrak, Anointed, DoublingSeason] in order;
// FinalCount = 8; MintedTokenIDs has 8 distinct entries all sharing
// EnablerInstanceID == the Sai-trigger AB instance.
type TokenMintEvent struct {
	SourceInstanceID    string // AbilityInstance that minted (e.g. Sai's trigger)
	SourceName          string
	TargetSeat          int
	BaseCharacteristics CopiableCharacteristics
	EffectsApplied      []ReplacementRef
	BaseCount           int
	FinalCount          int
	MintedTokenIDs      []string
	AtGameTick          int
}

// RecordTokenMintEvent appends to gs.TokenMintEvents. Nil-safe and a
// no-op when the slice cap has been intentionally disabled (the cap
// keeps the log bounded during long games — Phase 5 sets it at 256 to
// avoid pathological growth in test fuzzers).
func RecordTokenMintEvent(gs *GameState, ev TokenMintEvent) {
	if gs == nil {
		return
	}
	const cap = 256
	gs.TokenMintEvents = append(gs.TokenMintEvents, ev)
	if len(gs.TokenMintEvents) > cap {
		// Drop oldest half — keeps the tail bounded for replays.
		gs.TokenMintEvents = gs.TokenMintEvents[len(gs.TokenMintEvents)-cap/2:]
	}
}

// AddCopyMechanism appends to perm.CopyMechanisms. Stable per-card
// convention: handlers register their mechanisms at ETB so the audit
// trail + invariant checks see them before any trigger fires.
func AddCopyMechanism(perm *Permanent, m CopyMechanism) {
	if perm == nil {
		return
	}
	perm.CopyMechanisms = append(perm.CopyMechanisms, m)
}

// RecordCopyEvent appends to perm.CopyHistory. Called from each
// CopyMechanism's actuator when the copy actually fires.
func RecordCopyEvent(gs *GameState, perm *Permanent, ev CopyEvent) {
	if gs == nil || perm == nil {
		return
	}
	if ev.AtGameTick == 0 {
		ev.AtGameTick = gs.EffectTimestamp
	}
	perm.CopyHistory = append(perm.CopyHistory, ev)
}

// BecomeCopyOfCard implements the Phase 5 in-place copy semantics for
// permanent-becomes-copy effects (Clone, Spark Double, Phantasmal Image's
// ETB rider, Mirage Mirror's permanent-duration arm, Sakashima's ETB-as-
// copy). Per CR §706.2 + design §4 mint path 3, the copying permanent
// KEEPS ITS OWN IDENTITY — only the printed characteristics flip. So
// the right pointer-swap pattern is:
//
//  1. DeepCopy the source Card (gives a fresh *Card carrying source ID).
//  2. Overwrite the inherited InstanceID with the copying permanent's
//     ORIGINAL ID so identity persists across the copy.
//  3. Stamp CopiedTargetInstanceID + CopiableSnapshot on the Permanent.
//  4. Assign the rewritten Card pointer.
//
// Without step (2), the DeepCopy'd card carries the source's InstanceID
// — two zones now hold that ID and the CardIdentity invariant fires.
// This is one of the two duplication root causes Phase 4 surfaced.
//
// Returns the rewritten *Card so callers can assign it; returns nil
// when sourceCard or perm is nil.
func BecomeCopyOfCard(gs *GameState, perm *Permanent, sourceCard *Card) *Card {
	if perm == nil || sourceCard == nil {
		return nil
	}
	originalID := ""
	if perm.Card != nil {
		originalID = perm.Card.InstanceID
	}
	cp := sourceCard.DeepCopy()
	if cp == nil {
		return nil
	}
	cp.InstanceID = originalID
	// Lineage stays on the perm-side (CopiedTargetInstanceID +
	// CopiableSnapshot) — don't pollute the Card-side lineage fields
	// since they belong to the copy SOURCE's history, not the
	// copying permanent's.
	cp.SourceInstanceID = ""
	cp.EnablerInstanceID = ""
	cp.EnablerHistory = nil
	if perm.Card != nil {
		cp.Owner = perm.Card.Owner
	}
	// Snapshot the §706.2 copiable values.
	perm.CopiableSnapshot = &CopiableCharacteristics{
		Name:             sourceCard.DisplayName(),
		Types:            append([]string(nil), sourceCard.Types...),
		Colors:           append([]string(nil), sourceCard.Colors...),
		Power:            sourceCard.BasePower,
		Toughness:        sourceCard.BaseToughness,
		CMC:              sourceCard.CMC,
		SourceInstanceID: sourceCard.InstanceID,
	}
	perm.CopiedTargetInstanceID = sourceCard.InstanceID
	if gs != nil {
		RecordCopyEvent(gs, perm, CopyEvent{
			AtGameTick: gs.EffectTimestamp,
			SourceID:   sourceCard.InstanceID,
			EnablerID:  currentMintEnablerID(gs),
			Mechanism:  -1, // -1 = direct (non-mechanism-driven) copy
		})
	}
	return cp
}

// MintTokenAsCopyOf is the Phase 5 chokepoint for "create a token that's
// a copy of X" — per design §4 mint path 2. It DeepCopys the source
// Card, CLEARS the inherited InstanceID (preventing the same-ID
// duplication bug surfaced by Phase 4's gated census), and mints a fresh
// TK-provenance InstanceID with SourceInstanceID pointing at the
// original. Returns the fresh token Card; caller wraps it in a Permanent.
//
// Closes the 2.9M mint-coverage gap surfaced by PR #755: token-as-copy
// paths (Saheeli Decimator, Helm of the Host, Phantasmal Image,
// Spark Double, etc.) previously DeepCopy'd the source Card and the
// token kept the same InstanceID — fabrication detection saw the same
// ID in two zones.
//
// nil-safe; returns nil when sourceCard is nil.
func MintTokenAsCopyOf(gs *GameState, sourceCard *Card, controllerSeat int, enablerID string) *Card {
	if sourceCard == nil {
		return nil
	}
	tok := sourceCard.DeepCopy()
	if tok == nil {
		return nil
	}
	// Capture the original ID for lineage BEFORE clearing.
	srcID := tok.InstanceID
	// Reset every per-instance lineage field — the token is a new object.
	tok.InstanceID = ""
	tok.Provenance = 0
	tok.SourceInstanceID = ""
	tok.EnablerInstanceID = ""
	tok.EnablerHistory = nil
	tok.Owner = controllerSeat
	// Ensure "token" type is present.
	hasToken := false
	for _, t := range tok.Types {
		if t == "token" {
			hasToken = true
			break
		}
	}
	if !hasToken {
		tok.Types = append([]string{"token"}, tok.Types...)
	}
	MintTokenInstanceID(gs, tok, srcID, enablerID)
	return tok
}

// RecordReplacementApplied appends a ReplacementRef to a slice. Helper
// used by FireEvent's audit shim so per-event chain walkers can read
// back the full §616 application order. The ev parameter is the live
// ReplEvent (Phase 5 reads ev.Type / ev.Count for chain reconstruction).
func RecordReplacementApplied(
	gs *GameState,
	chain *[]ReplacementRef,
	src *Permanent,
	handlerID, sourceName string,
	op ReplacementOp,
	before, after int,
) {
	if chain == nil {
		return
	}
	ref := ReplacementRef{
		HandlerID:    handlerID,
		SourceName:   sourceName,
		Modification: op,
		CountBefore:  before,
		CountAfter:   after,
	}
	if gs != nil {
		ref.AppliedAtTick = gs.EffectTimestamp
	}
	if src != nil && src.Card != nil {
		ref.Source = src.Card.InstanceID
	}
	*chain = append(*chain, ref)
}

package gameengine

import (
	"github.com/hexdek/hexdek/internal/gameengine/instanceid"
)

// MintOGInstanceID stamps an OG-provenance InstanceID on c if it
// doesn't already have one. Used by SetupCommanderGame to mint IDs at
// deck-construction time for every library + commander card per
// docs/instanceid-system-v2-r60.md §4.1.
//
// Idempotent: if c.InstanceID is already non-empty (caller pre-minted,
// or the same Card was processed twice via a deck-load retry path), the
// helper no-ops to avoid double-consuming a seq counter.
//
// Front-face encoding per CR §712.6c: DFC / MDFC cards encode their
// FRONT face color + printed CMC. Card.Colors and Card.CMC already
// reflect the front face (the corpus loader sets them that way), so no
// special-casing is needed at this layer.
//
// Backwards-compat: when gs.IIDMinter is nil (struct-literal GameState
// from older tests), the helper no-ops silently — Card.InstanceID stays
// empty, and the engine treats that as legacy mode.
func MintOGInstanceID(gs *GameState, c *Card) {
	if gs == nil || c == nil {
		return
	}
	if c.InstanceID != "" {
		return
	}
	if gs.IIDMinter == nil {
		return
	}
	seat := c.Owner
	if seat < 0 || seat >= gs.IIDMinter.SeatCount() {
		return
	}
	color := instanceid.CanonicalColor(c.Colors)
	cmc := c.CMC
	if cmc < 0 {
		cmc = 0
	}
	id := gs.IIDMinter.Mint(seat, instanceid.ProvOG, instanceid.Visible, color, cmc)
	if id == "" {
		return
	}
	c.InstanceID = id
	c.Provenance = instanceid.ProvOG
	c.Visibility = instanceid.Visible
	c.ActiveFace = instanceid.Front
	// SourceInstanceID, EnablerInstanceID, EnablerHistory stay zero —
	// OG cards have no lineage by §4.1.
}

// currentMintEnablerID returns the top frame of gs.IIDEnablerStack — the
// InstanceID of the AbilityInstance whose resolution is currently
// minting child objects (tokens, copies). Returns "" when nothing is
// resolving; the empty string is a valid enabler for setup-time mints
// and the Phase 1 legacy path.
func currentMintEnablerID(gs *GameState) string {
	if gs == nil {
		return ""
	}
	n := len(gs.IIDEnablerStack)
	if n == 0 {
		return ""
	}
	return gs.IIDEnablerStack[n-1]
}

// pushIIDEnabler pushes an InstanceID onto gs.IIDEnablerStack. Empty
// strings are still pushed so the pop logic stays symmetric — they
// contribute a "no enabler" frame which is correct for spells (Card on
// stack, no Ability instance) resolving from cast.
func pushIIDEnabler(gs *GameState, id string) {
	if gs == nil {
		return
	}
	gs.IIDEnablerStack = append(gs.IIDEnablerStack, id)
}

// popIIDEnabler pops the top of gs.IIDEnablerStack. No-op when empty.
func popIIDEnabler(gs *GameState) {
	if gs == nil {
		return
	}
	n := len(gs.IIDEnablerStack)
	if n == 0 {
		return
	}
	gs.IIDEnablerStack = gs.IIDEnablerStack[:n-1]
}

// MintTokenInstanceID stamps a TK-provenance InstanceID on a token Card
// at mint time. Phase 2 chokepoint for all token-creation paths per
// docs/instanceid-system-v2-r60.md §4 mint path 1 (spell-copy → token)
// and path 2 (direct permanent copy → token).
//
// sourceID is the InstanceID of the spell or permanent being copied (may
// be empty for "fresh mint" tokens like Sai's Thopters that copy nothing
// — those still get TK provenance, just no SourceInstanceID).
// enablerID is the AbilityInstance.InstanceID of the trigger / activation
// that caused the mint (Sai's "whenever you cast an artifact spell"
// trigger instance, Mondrak's doubling-replacement instance, etc.).
// Per §3 format invariants, EnablerInstanceID is REQUIRED for TK.
//
// Idempotent: if c.InstanceID is already non-empty, the helper no-ops.
// Backwards-compat: nil minter → silent no-op.
func MintTokenInstanceID(gs *GameState, c *Card, sourceID, enablerID string) {
	if gs == nil || c == nil {
		return
	}
	if c.InstanceID != "" {
		return
	}
	if gs.IIDMinter == nil {
		return
	}
	seat := c.Owner
	if seat < 0 || seat >= gs.IIDMinter.SeatCount() {
		return
	}
	color := instanceid.CanonicalColor(c.Colors)
	cmc := c.CMC
	if cmc < 0 {
		cmc = 0
	}
	id := gs.IIDMinter.Mint(seat, instanceid.ProvTK, instanceid.Visible, color, cmc)
	if id == "" {
		return
	}
	c.InstanceID = id
	c.Provenance = instanceid.ProvTK
	c.Visibility = instanceid.Visible
	c.ActiveFace = instanceid.Front
	c.SourceInstanceID = sourceID
	c.EnablerInstanceID = enablerID
	if enablerID != "" {
		c.EnablerHistory = append(c.EnablerHistory, enablerID)
	}
}

// EnsureTokenInstanceID is the defensive auto-stamp catch-all: when a
// token Permanent reaches FirePermanentETBTriggers without an
// InstanceID, this helper mints a TK ID using the card name as a
// best-effort source label and an empty enabler. Per_card token mints
// that don't yet call MintTokenInstanceID explicitly still get covered
// by this safety net, preserving the Phase 1 "empty ID = legacy"
// invariant while moving the runtime toward universal ID coverage.
//
// Non-tokens are left untouched — only Permanents whose Card has the
// "token" type tag receive the stamp.
func EnsureTokenInstanceID(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	if perm.Card.InstanceID != "" {
		return
	}
	isToken := false
	for _, t := range perm.Card.Types {
		if t == "token" {
			isToken = true
			break
		}
	}
	if !isToken {
		return
	}
	MintTokenInstanceID(gs, perm.Card, "", "")
}

// MintCopyInstanceID stamps a CP-provenance InstanceID on a Card that
// represents a spell or permanent COPY remaining as a Card object on the
// stack (storm/Fork/Twincast/gravestorm copies). Per §3 format
// invariants, BOTH SourceInstanceID and EnablerInstanceID are REQUIRED
// for CP — callers must supply a non-empty sourceID.
//
// Idempotent + nil-safe like the other mint helpers.
func MintCopyInstanceID(gs *GameState, c *Card, sourceID, enablerID string) {
	if gs == nil || c == nil {
		return
	}
	if c.InstanceID != "" {
		return
	}
	if gs.IIDMinter == nil {
		return
	}
	seat := c.Owner
	if seat < 0 || seat >= gs.IIDMinter.SeatCount() {
		return
	}
	color := instanceid.CanonicalColor(c.Colors)
	cmc := c.CMC
	if cmc < 0 {
		cmc = 0
	}
	id := gs.IIDMinter.Mint(seat, instanceid.ProvCP, instanceid.Visible, color, cmc)
	if id == "" {
		return
	}
	c.InstanceID = id
	c.Provenance = instanceid.ProvCP
	c.Visibility = instanceid.Visible
	c.ActiveFace = instanceid.Front
	c.SourceInstanceID = sourceID
	c.EnablerInstanceID = enablerID
	if enablerID != "" {
		c.EnablerHistory = append(c.EnablerHistory, enablerID)
	}
}

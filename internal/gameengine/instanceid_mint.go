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

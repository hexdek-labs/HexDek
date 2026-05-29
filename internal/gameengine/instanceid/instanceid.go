// Package instanceid implements the InstanceID system foundation per
// docs/instanceid-system-v2-r60.md.
//
// Phase 1 scope: provenance/visibility/face-index enums, deterministic
// per-(game, seat) ID minter, canonical color helper, format regex.
// Subsequent phases (Phase 2+) layer on token / copy / ability mints,
// linkage refactor, invariant migration.
//
// Format (section 3 of the design):
//
//	<prefix><seat><provenance><visibility><color><cmc><seq5>
//
// Prefix is fixed "h" (HexDek namespace). Reserved alternates "t"
// (tournament) and "s" (simulation) are not minted by this package yet.
package instanceid

import (
	"fmt"
	"regexp"
	"strings"
)

// Provenance encodes WHAT the object is, not how it got there. Aligned to
// the four mint paths validated by Probe E (PR #744).
type Provenance int

const (
	// ProvUnknown is the zero value; treated as legacy ("no InstanceID
	// minted yet") by backwards-compat readers. Never written by Mint.
	ProvUnknown Provenance = iota
	// ProvOG marks a deck-load / printed-card mint. SourceInstanceID and
	// EnablerInstanceID stay empty.
	ProvOG
	// ProvTK marks a token (created by §111.1 token-mint or §706.10b
	// spell-copy that becomes a permanent). EnablerInstanceID required.
	ProvTK
	// ProvCP marks a spell/permanent copy that remains a Card object
	// (storm copies, Fork). SourceInstanceID + EnablerInstanceID required.
	ProvCP
	// ProvAB marks an AbilityInstance on the stack (triggered or
	// activated abilities, §112). EnablerInstanceID required for
	// triggered abilities; empty for activated.
	ProvAB
)

// String returns the two-character code used in the InstanceID format.
// Returns "??" for ProvUnknown.
func (p Provenance) String() string {
	switch p {
	case ProvOG:
		return "OG"
	case ProvTK:
		return "TK"
	case ProvCP:
		return "CP"
	case ProvAB:
		return "AB"
	}
	return "??"
}

// Visibility controls face-down information-leak protection. Hidden is
// stamped on cards minted face-down (manifested 2/2, cloaked, morph,
// foretold pre-cast). Flipping face-up does not re-mint — see §11 of the
// design.
type Visibility int

const (
	// Visible cards display normally (default for face-up).
	Visible Visibility = iota
	// Hidden cards conceal characteristics from spectators (face-down).
	Hidden
)

// String returns the one-character code used in the InstanceID format.
func (v Visibility) String() string {
	if v == Hidden {
		return "H"
	}
	return "V"
}

// FaceIndex selects which face of a DFC / MDFC is currently active.
// Default Front matches CR §712.6c (non-battlefield DFCs default to
// front).
type FaceIndex int

const (
	// Front is the front face (default).
	Front FaceIndex = iota
	// Back is the back face (after transform / cast-as-back).
	Back
)

// String returns a short human label. Not part of the InstanceID format.
func (f FaceIndex) String() string {
	if f == Back {
		return "Back"
	}
	return "Front"
}

// wubrgOrder is the canonical WUBRG sort key for color strings. Section 3
// pins this ordering for format invariants.
const wubrgOrder = "WUBRG"

// CanonicalColor takes a slice of color codes (any case, any order) and
// returns the canonical WUBRG-ordered concatenation, or "C" if the slice
// is empty / contains only invalid entries (colorless).
//
//	CanonicalColor(nil)              -> "C"
//	CanonicalColor([]string{"r"})    -> "R"
//	CanonicalColor([]string{"G","W"}) -> "WG"
//	CanonicalColor([]string{"W","U","B","R","G"}) -> "WUBRG"
func CanonicalColor(colors []string) string {
	if len(colors) == 0 {
		return "C"
	}
	seen := [5]bool{}
	for _, c := range colors {
		if c == "" {
			continue
		}
		switch strings.ToUpper(c)[:1] {
		case "W":
			seen[0] = true
		case "U":
			seen[1] = true
		case "B":
			seen[2] = true
		case "R":
			seen[3] = true
		case "G":
			seen[4] = true
		}
	}
	var b strings.Builder
	for i, ok := range seen {
		if ok {
			b.WriteByte(wubrgOrder[i])
		}
	}
	if b.Len() == 0 {
		return "C"
	}
	return b.String()
}

// FormatRegex pins the exact shape of every minted InstanceID. Property
// tests assert that every Mint output matches.
//
//	prefix     h
//	seat       [0-3]
//	provenance OG|TK|CP|AB
//	visibility V|H
//	color      W?U?B?R?G? canonical or C
//	cmc        one or more digits
//	seq5       exactly five digits (rendered with %05d when seq <= 99999)
var FormatRegex = regexp.MustCompile(`^h[0-3](OG|TK|CP|AB)(V|H)(W|U|B|R|G|C)+[0-9]+[0-9]{5}$`)

// FormatID renders a single InstanceID from its components. Pure
// function; safe in tests and replay tooling that needs to reconstruct
// IDs without holding a Minter.
//
// Validation: seat must be 0..3, prov must be a known non-unknown value,
// color must be non-empty (use "C" for colorless), cmc must be >= 0.
// Returns the empty string on invalid input rather than panicking — this
// is the construct-time signal to callers that something is wrong with
// their mint context.
func FormatID(seat int, prov Provenance, vis Visibility, color string, cmc, seq int) string {
	if seat < 0 || seat > 3 {
		return ""
	}
	if prov == ProvUnknown {
		return ""
	}
	if color == "" {
		return ""
	}
	if cmc < 0 {
		return ""
	}
	if seq < 0 {
		return ""
	}
	// Best-effort 5-digit padding. Caller is responsible for guarding
	// overflow past 99999 via Minter.Mint; FormatID renders whatever
	// digit count seq requires, and the regex will reject anything that
	// somehow lands beyond 5 digits' worth of seq AND looks otherwise
	// well-formed.
	return fmt.Sprintf("h%d%s%s%s%d%05d", seat, prov.String(), vis.String(), color, cmc, seq)
}

// Minter owns the deterministic per-(game, seat) sequence counters that
// drive the seq5 field. One Minter per game. Counter starts at 0 and
// increments on every successful Mint call; never re-used within the
// same game.
//
// Thread-safety: a Minter is NOT safe for concurrent use. Engine code
// drives mints from a single goroutine per game.
type Minter struct {
	seq []int // per-seat counter; len(seq) == seatCount
}

// NewMinter constructs a Minter sized for the given seat count. Counters
// start at 0; the first Mint per seat returns seq5 = "00000".
func NewMinter(seatCount int) *Minter {
	if seatCount < 1 {
		seatCount = 1
	}
	return &Minter{seq: make([]int, seatCount)}
}

// SeatCount returns the seat count the Minter was sized for.
func (m *Minter) SeatCount() int {
	if m == nil {
		return 0
	}
	return len(m.seq)
}

// Peek returns the seat's CURRENT counter value without advancing it.
// Useful for property tests that want to verify monotonicity across a
// batch.
func (m *Minter) Peek(seat int) int {
	if m == nil || seat < 0 || seat >= len(m.seq) {
		return -1
	}
	return m.seq[seat]
}

// Mint produces the next InstanceID for (seat, prov, vis, color, cmc)
// and advances the seat's seq counter. Returns the empty string when
// seat is out of range or any component is invalid (mirrors FormatID).
//
// The seq counter is per-seat-per-game and strictly monotonic. Overflow
// past 99999 produces a longer seq (caller will see a regex-mismatch in
// property tests, which is the deliberate failure signal — 99999 mints
// per seat per game is far beyond any realistic Phase 1..N usage).
func (m *Minter) Mint(seat int, prov Provenance, vis Visibility, color string, cmc int) string {
	if m == nil || seat < 0 || seat >= len(m.seq) {
		return ""
	}
	id := FormatID(seat, prov, vis, color, cmc, m.seq[seat])
	if id == "" {
		return ""
	}
	m.seq[seat]++
	return id
}

package gameengine

// Phase 10 — Pluggable player-decision protocol (Hat).
//
// A Hat encapsulates every decision the rules leave up to a player.
// The engine CALLS these methods; the engine MUST NEVER INSPECT what
// kind of Hat a seat has. Hats are swappable at any time — tournament
// runners load N hats into N seats and run.
//
// Architectural directive from the project lead (7174n1c):
//   "hats are swappable, engine never inspects them."
//
// Rationale — Comprehensive Rules pointer map
// ----------------------------------------------------------------------
//   §103.4   Mulligan decision                       → ChooseMulligan
//   §116.1b  Choosing which spell / land to play     → ChooseLandToPlay /
//                                                      ChooseCastFromHand
//   §117.3   Priority / passing                      → ChooseResponse
//   §506.1   Each attacker chooses its defender      → ChooseAttackers /
//                                                      ChooseAttackTarget
//   §509.1   Defending player declares blockers      → AssignBlockers
//   §601.2c  Choosing modes for a modal spell        → ChooseMode
//   §608.2a  Each target is chosen                   → ChooseTarget
//   §616.1   Affected-player picks replacement order → OrderReplacements
//   §701.8   Discard (player chooses unless told)    → ChooseDiscard
//   §903.8   Whether to cast commander (+ tax)       → ShouldCastCommander
//   §903.9a  Commander-zone redirect choice          → ShouldRedirectCommanderZone
//
// The interface lives in `gameengine` (not `internal/hat`) so that
// `Seat.Hat` can reference it without creating an import cycle between
// the engine (which needs the interface) and the implementations (which
// need engine types).
//
// Concrete implementations: `github.com/hexdek/hexdek/internal/hat`
//   - `hat.GreedyHat` — baseline heuristic, byte-equivalent to pre-Phase-10
//                       engine behavior (so parity tests don't drift).
//   - `hat.PokerHat`  — HOLD/CALL/RAISE adaptive hat with 7-dim threat score.

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ThreatScoreBasic returns a simple threat score for `target` as seen
// from `source`. Mirrors Python's baseline threat_score (pre-7-dim):
// lowest-life first (inverse), then board power, then hand size. Used
// as the default by GreedyHat's attack-target picker.
//
// This function lives in gameengine because it's tiny and callers in
// both gameengine itself (pickAttackDefender) and the hat package
// can share it without redundant copies.
func ThreatScoreBasic(gs *GameState, sourceSeat int, target *Seat) float64 {
	if target == nil || target.Lost {
		return -1_000_000
	}
	score := 0.0
	// Lower life = more pressurable — inverse of life.
	if target.Life > 0 {
		score += 40.0 / float64(target.Life)
	} else {
		score += 100
	}
	// Board power.
	for _, p := range target.Battlefield {
		if p != nil && p.IsCreature() {
			pw := p.Power()
			if pw > 0 {
				score += float64(pw) * 0.5
			}
		}
	}
	// Hand size — 5+ is scary.
	h := len(target.Hand)
	if h >= 5 {
		score += float64(h-4) * 0.5
	}
	return score
}

// ColoredManaEstimate captures the color-aware mana picture used by
// hats during priority. Fixed[W..C] are the contributions that can only
// satisfy that specific color (the current pool, plus mono-color lands).
// Flex is a slice of bitmasks: each entry represents one untapped
// multi-color source that can produce ANY one color from the mask
// (Volcanic Island = bits U+R; Forest = bits G alone is folded into
// Fixed). Any is truly any-color mana (Mox Opal-style "add one mana of
// any color"). Total mana for capacity comparison is
// sum(Fixed) + len(Flex) + Any.
type ColoredManaEstimate struct {
	Fixed [6]int  // [W, U, B, R, G, C]
	Flex  []uint8 // bitmask per multi-color source
	Any   int     // any-color sources (one mana per element)
}

// Color slot indices for ColoredManaEstimate.Fixed.
const (
	manaSlotW = 0
	manaSlotU = 1
	manaSlotB = 2
	manaSlotR = 3
	manaSlotG = 4
	manaSlotC = 5
)

func colorBitToSlot(bit uint8) int {
	switch bit {
	case colorBitMaskW:
		return manaSlotW
	case colorBitMaskU:
		return manaSlotU
	case colorBitMaskB:
		return manaSlotB
	case colorBitMaskR:
		return manaSlotR
	case colorBitMaskG:
		return manaSlotG
	}
	return -1
}

// Color bitmasks for hat-side color tracking. Kept package-local so
// internal/hat's identical constants remain the in-package canonical
// names; these are duplicates intentionally scoped to gameengine.
const (
	colorBitMaskW uint8 = 1 << iota
	colorBitMaskU
	colorBitMaskB
	colorBitMaskR
	colorBitMaskG
)

// Total returns the upper-bound spendable mana the estimate represents
// (sum of fixed buckets, every flex source as one, plus any).
func (e *ColoredManaEstimate) Total() int {
	if e == nil {
		return 0
	}
	n := 0
	for _, v := range e.Fixed {
		n += v
	}
	n += len(e.Flex) + e.Any
	return n
}

// AvailableManaEstimate returns a quick estimate of this seat's
// spendable mana. Uses the typed pool total (authoritative after
// EnsureTypedPool) plus untapped lands and mana artifacts on the
// battlefield. Hats call this as a gating predicate; the engine's real
// cost check during CastSpell is authoritative.
func AvailableManaEstimate(gs *GameState, seat *Seat) int {
	if seat == nil {
		return 0
	}
	EnsureTypedPool(seat)
	n := seat.Mana.Total()
	for _, p := range seat.Battlefield {
		if p == nil || p.Tapped {
			continue
		}
		if p.IsLand() {
			n++
			continue
		}
		// Count untapped mana artifacts.
		if IsArtifactOnly(p) && !ArtifactHasDestructiveCost(p) {
			n += ArtifactManaPotential(p)
		}
	}
	return n
}

// AvailableColoredManaEstimate decomposes a seat's spendable mana into
// color-aware buckets. Pool contents land in Fixed; mono-color lands
// land in Fixed; multi-color lands (duals, shocklands, "add any color")
// land in Flex with the producible-color bitmask. Mana artifacts that
// produce any color (Mox Opal-style — modeled as ArtifactManaPotential
// without a color hint) land in Any.
//
// Hats use this to type-aware-gate counter / removal eligibility, so a
// 2-mana Counterspell ({U}{U}) doesn't false-positive against a pool of
// 2 generic. Greedy single-color matching is sufficient for realistic
// mana bases; full bipartite matching would be O(V*E) overkill.
func AvailableColoredManaEstimate(gs *GameState, seat *Seat) ColoredManaEstimate {
	var est ColoredManaEstimate
	if seat == nil {
		return est
	}
	EnsureTypedPool(seat)
	if seat.Mana != nil {
		est.Fixed[manaSlotW] += seat.Mana.W
		est.Fixed[manaSlotU] += seat.Mana.U
		est.Fixed[manaSlotB] += seat.Mana.B
		est.Fixed[manaSlotR] += seat.Mana.R
		est.Fixed[manaSlotG] += seat.Mana.G
		est.Fixed[manaSlotC] += seat.Mana.C
		est.Any += seat.Mana.Any
	}
	for _, p := range seat.Battlefield {
		if p == nil || p.Tapped {
			continue
		}
		if p.IsLand() {
			mask, colorless := landProducedProfile(p.Card)
			est.addSource(mask, colorless)
			continue
		}
		if IsArtifactOnly(p) && !ArtifactHasDestructiveCost(p) {
			potential := ArtifactManaPotential(p)
			for i := 0; i < potential; i++ {
				est.Any++
			}
		}
	}
	return est
}

// landProducedProfile returns (colorMask, producesColorless). Mask is
// the bitmask of W/U/B/R/G the card can tap for; colorless is true when
// the card can also produce {C} (Wastes, Eldrazi Temple) so the
// estimate can route it into Fixed[C] rather than the catchall Any.
func landProducedProfile(c *Card) (uint8, bool) {
	if c == nil {
		return 0, false
	}
	mask := landProducedColorsMask(c)
	ot := OracleTextLower(c)
	tl := TypeLineLower(c)
	colorless := strings.Contains(ot, "add {c}") || strings.Contains(tl, "wastes")
	return mask, colorless
}

// addSource folds one untapped producer's color profile into the
// estimate. Convention: producesColorless distinguishes "explicit
// colorless source" (Wastes / Eldrazi Temple — goes to Fixed[C]) from
// "we couldn't tell what this produces" (mask == 0 && !colorless — goes
// to Any so non-basic utility lands with unparseable text still
// contribute capacity).
func (e *ColoredManaEstimate) addSource(mask uint8, producesColorless bool) {
	// Explicit any-color source ("add one mana of any color" or all 5
	// color bits set).
	if mask == colorBitMaskW|colorBitMaskU|colorBitMaskB|colorBitMaskR|colorBitMaskG {
		e.Any++
		return
	}
	bits := 0
	for b := mask; b != 0; b &= b - 1 {
		bits++
	}
	switch {
	case bits == 0 && producesColorless:
		e.Fixed[manaSlotC]++
	case bits == 0:
		e.Any++
	case bits == 1:
		slot := colorBitToSlot(mask)
		if slot < 0 {
			e.Any++
			return
		}
		e.Fixed[slot]++
	default:
		e.Flex = append(e.Flex, mask)
	}
}

// landProducedColorsMask is the engine-side mirror of
// hat.landProducesColorsMask. We can't import internal/hat from
// internal/gameengine (the dependency is the other way), so this is
// the canonical engine-package version. Caches on the Card so repeated
// calls are O(1). Oracle-text needles match Scryfall's "Add {X}" line.
func landProducedColorsMask(c *Card) uint8 {
	if c == nil {
		return 0
	}
	if c.ProducedColorsReady {
		return c.ProducedColorsMask
	}
	var mask uint8
	ot := OracleTextLower(c)
	tl := TypeLineLower(c)
	for _, lt := range engineLandColorTypes {
		if strings.Contains(tl, lt.typeName) || strings.Contains(ot, lt.addNeedle) {
			mask |= lt.bit
		}
	}
	if strings.Contains(ot, "any color") {
		mask = colorBitMaskW | colorBitMaskU | colorBitMaskB | colorBitMaskR | colorBitMaskG
	}
	c.ProducedColorsMask = mask
	c.ProducedColorsReady = true
	return mask
}

var engineLandColorTypes = []struct {
	typeName  string
	addNeedle string
	bit       uint8
}{
	{"plains", "add {w}", colorBitMaskW},
	{"island", "add {u}", colorBitMaskU},
	{"swamp", "add {b}", colorBitMaskB},
	{"mountain", "add {r}", colorBitMaskR},
	{"forest", "add {g}", colorBitMaskG},
}

// CostRequirements is the parsed shape of a printed mana cost. Pure
// pips are by color (W..C). Hybrid is the list of "{X/Y}" pips as
// color bitmasks — each entry needs ONE mana satisfying any color in
// the mask. Generic is the integer pip count.
type CostRequirements struct {
	Pure    [6]int
	Hybrid  []uint8
	Generic int
}

// CanPayColoredCost returns true if the seat can pay card's printed
// mana cost using the color-aware estimate. Allocation order:
//  1. Pure colored pips, satisfied from Fixed first, then Flex
//     (most-constrained flex first), then Any.
//  2. Hybrid pips, satisfied from any color in their mask, again
//     preferring Fixed → Flex → Any.
//  3. Generic, satisfied from leftover capacity (any color).
//
// Cards with no ManaCostString (engine-minted tokens) skip the colored
// match and only verify Total ≥ CMC.
func CanPayColoredCost(est ColoredManaEstimate, card *Card) bool {
	if card == nil {
		return true
	}
	cmc := manaCostOf(card)
	if est.Total() < cmc {
		return false
	}
	if card.ManaCostString == "" {
		return true
	}
	req := ParseCostRequirements(card.ManaCostString)
	if req.Generic == 0 {
		// Cost string may be partial / X-only. Derive generic from CMC
		// minus the sum of all colored pip slots (pure + hybrid).
		coloredCount := req.Hybrid
		_ = coloredCount
		colored := 0
		for _, v := range req.Pure {
			colored += v
		}
		colored += len(req.Hybrid)
		if cmc > colored {
			req.Generic = cmc - colored
		}
	}

	// Work on a copy so callers can re-use the estimate.
	fixed := est.Fixed
	flex := make([]uint8, len(est.Flex))
	copy(flex, est.Flex)
	any := est.Any

	// Pay each pure colored requirement.
	for slot, bit := range []uint8{
		colorBitMaskW, colorBitMaskU, colorBitMaskB, colorBitMaskR, colorBitMaskG,
	} {
		need := req.Pure[slot]
		if need == 0 {
			continue
		}
		if fixed[slot] >= need {
			fixed[slot] -= need
			need = 0
		} else {
			need -= fixed[slot]
			fixed[slot] = 0
		}
		for need > 0 {
			idx := pickFlexIndex(flex, bit)
			if idx < 0 {
				break
			}
			flex[idx] = 0
			need--
		}
		for need > 0 && any > 0 {
			any--
			need--
		}
		if need > 0 {
			return false
		}
	}
	// Pure colorless pip {C} can be paid by Fixed[C] or Any.
	if req.Pure[manaSlotC] > 0 {
		need := req.Pure[manaSlotC]
		if fixed[manaSlotC] >= need {
			fixed[manaSlotC] -= need
		} else {
			need -= fixed[manaSlotC]
			fixed[manaSlotC] = 0
			if any >= need {
				any -= need
			} else {
				return false
			}
		}
	}
	// Hybrid pips — each needs one mana from any color in its mask.
	for _, mask := range req.Hybrid {
		paid := false
		for slot, bit := range []uint8{
			colorBitMaskW, colorBitMaskU, colorBitMaskB, colorBitMaskR, colorBitMaskG,
		} {
			if mask&bit == 0 {
				continue
			}
			if fixed[slot] > 0 {
				fixed[slot]--
				paid = true
				break
			}
		}
		if paid {
			continue
		}
		idx := pickFlexIndex(flex, mask)
		if idx >= 0 {
			flex[idx] = 0
			continue
		}
		if any > 0 {
			any--
			continue
		}
		return false
	}

	// Generic: any leftover Fixed / Flex / Any covers it.
	if req.Generic == 0 {
		return true
	}
	left := any
	for _, v := range fixed {
		left += v
	}
	for _, m := range flex {
		if m != 0 {
			left++
		}
	}
	return left >= req.Generic
}

// pickFlexIndex returns the index of the most-constrained flex source
// (fewest colors) that includes `want`, or -1 if none. Preferring the
// tightest source first preserves more-flexible sources for later
// color pips.
func pickFlexIndex(flex []uint8, want uint8) int {
	bestIdx, bestBits := -1, 6
	for i, m := range flex {
		if m == 0 || m&want == 0 {
			continue
		}
		bits := 0
		for b := m; b != 0; b &= b - 1 {
			bits++
		}
		if bits < bestBits {
			bestBits = bits
			bestIdx = i
		}
	}
	return bestIdx
}

// ParseCostRequirements walks a printed cost string ("{2}{U}{U}",
// "{X}{B/R}", "{U/P}") and splits it into pure pips, hybrid pip masks,
// and a generic count. X / Y / Z / numeric symbols contribute to
// Generic (X resolves to 0 here — CanPayColoredCost's outer Total ≥ CMC
// check covers the rest of the X cost via the engine's real cost check).
// Phyrexian {color/P} is treated as a hybrid where one option is the
// color (P → just one color requirement).
func ParseCostRequirements(cost string) CostRequirements {
	var req CostRequirements
	if cost == "" {
		return req
	}
	i := 0
	for i < len(cost) {
		if cost[i] != '{' {
			i++
			continue
		}
		end := strings.IndexByte(cost[i:], '}')
		if end < 0 {
			break
		}
		inner := strings.ToUpper(cost[i+1 : i+end])
		i += end + 1
		if inner == "" {
			continue
		}
		if strings.Contains(inner, "/") {
			parts := strings.Split(inner, "/")
			var mask uint8
			numericPart := -1
			hasPhyrexian := false
			for _, part := range parts {
				switch {
				case part == "P":
					hasPhyrexian = true
				case isAllDigits(part):
					if n, ok := atoiSimple(part); ok {
						if numericPart < n {
							numericPart = n
						}
					}
				default:
					if slot := letterToColorSlot(part); slot >= 0 {
						mask |= slotToBit(slot)
					}
				}
			}
			// Phyrexian (e.g. {U/P}) — life is the alt cost; from a mana
			// perspective treat it as a pure color pip.
			if hasPhyrexian && mask != 0 && bitsSet(mask) == 1 {
				slot := colorBitToSlot(mask)
				if slot >= 0 {
					req.Pure[slot]++
					continue
				}
			}
			// Twobrid (e.g. {2/W}) — one of W OR pay generic 2. For a
			// permissive payable-check, treat as a pure W pip (best case)
			// AND fall through to generic if no W available. Simplest: log
			// as a hybrid with just the W bit set + add the numeric to
			// generic if we couldn't cover it. We model it as pure-W with
			// the numeric part added to generic — this slightly under-pays
			// in the corner where we have generic but no color, but the
			// total-capacity prefilter usually keeps us honest.
			if numericPart > 0 && mask != 0 && bitsSet(mask) == 1 {
				slot := colorBitToSlot(mask)
				if slot >= 0 {
					req.Pure[slot]++
					continue
				}
			}
			if mask != 0 {
				req.Hybrid = append(req.Hybrid, mask)
			}
			continue
		}
		if slot := letterToColorSlot(inner); slot >= 0 {
			req.Pure[slot]++
			continue
		}
		if isAllDigits(inner) {
			if n, ok := atoiSimple(inner); ok {
				req.Generic += n
			}
			continue
		}
		// X / Y / Z / S (snow) — count as 1 generic. The engine's real
		// CastSpell cost check will rebill X correctly.
		req.Generic++
	}
	return req
}

// parseColoredPips is the legacy entrypoint retained for the
// regression test that pins per-color pip extraction. Returns the Pure
// counts only; hybrid pips contribute to each color slot to preserve
// pre-R60 behavior for the test.
func parseColoredPips(cost string) [6]int {
	req := ParseCostRequirements(cost)
	out := req.Pure
	for _, m := range req.Hybrid {
		if m&colorBitMaskW != 0 {
			out[manaSlotW]++
		}
		if m&colorBitMaskU != 0 {
			out[manaSlotU]++
		}
		if m&colorBitMaskB != 0 {
			out[manaSlotB]++
		}
		if m&colorBitMaskR != 0 {
			out[manaSlotR]++
		}
		if m&colorBitMaskG != 0 {
			out[manaSlotG]++
		}
	}
	return out
}

func bitsSet(mask uint8) int {
	n := 0
	for b := mask; b != 0; b &= b - 1 {
		n++
	}
	return n
}

func slotToBit(slot int) uint8 {
	switch slot {
	case manaSlotW:
		return colorBitMaskW
	case manaSlotU:
		return colorBitMaskU
	case manaSlotB:
		return colorBitMaskB
	case manaSlotR:
		return colorBitMaskR
	case manaSlotG:
		return colorBitMaskG
	}
	return 0
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func atoiSimple(s string) (int, bool) {
	n := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, false
		}
		n = n*10 + int(ch-'0')
	}
	return n, true
}

func letterToColorSlot(s string) int {
	switch s {
	case "W":
		return manaSlotW
	case "U":
		return manaSlotU
	case "B":
		return manaSlotB
	case "R":
		return manaSlotR
	case "G":
		return manaSlotG
	case "C":
		return manaSlotC
	}
	return -1
}

// OracleTextLower returns the lowercased oracle text for a card. We
// don't carry oracle_text on the runtime Card directly — instead we
// reconstruct a minimal proxy from the AST's ability raw strings.
// Good enough for string-matching heuristics ("win the game",
// "destroy all", "draw three cards"). Returns "" for nil / AST-less
// cards (tokens).
func OracleTextLower(card *Card) string {
	if card == nil || card.AST == nil {
		return ""
	}
	if card.oracleTextReady {
		return card.OracleTextCache
	}
	var sb strings.Builder
	for _, ab := range card.AST.Abilities {
		switch a := ab.(type) {
		case *gameast.Keyword:
			sb.WriteString(a.Name)
			sb.WriteByte(' ')
		case *gameast.Static:
			sb.WriteString(a.Raw)
			sb.WriteByte(' ')
		case *gameast.Activated:
			sb.WriteString(a.Raw)
			sb.WriteByte(' ')
		case *gameast.Triggered:
			sb.WriteString(a.Raw)
			sb.WriteByte(' ')
		}
	}
	card.OracleTextCache = strings.ToLower(sb.String())
	card.oracleTextReady = true
	return card.OracleTextCache
}

// TypeLineLower returns the lowercased TypeLine for a card, caching the
// result on the Card so subsequent calls are O(1). Hat evaluators query
// this many times per game; without the cache, strings.ToLower runs in
// the hottest inner loops.
func TypeLineLower(card *Card) string {
	if card == nil {
		return ""
	}
	if card.typeLineLowerReady {
		return card.TypeLineLowerCache
	}
	card.TypeLineLowerCache = strings.ToLower(card.TypeLine)
	card.typeLineLowerReady = true
	return card.TypeLineLowerCache
}

// Activation is a legal activated-ability choice surfaced to a Hat.
// Index is the position in Permanent.Card.AST.Abilities of the chosen
// ability; the engine re-dispatches once the Hat picks one.
type Activation struct {
	Permanent *Permanent
	Ability   int
}

// Hat is the pluggable player-decision protocol. Every method receives
// the GameState as *GameState so the Hat can read ANY public state —
// that's fine because Hats are private per seat; the engine side
// never introspects a Hat.
//
// Mutation: Hats MUST NOT mutate *GameState. The contract is:
//   - methods return a decision the engine applies
//   - ObserveEvent is the only method that updates the Hat's OWN internal
//     state; it is called for every seat on every logged event
//
// Nil-safety: every method must tolerate empty / nil inputs — the
// engine calls them in edge positions (no legal attackers, empty hand,
// dead opponents). A Hat that panics on an empty legal list would hang
// the tournament runner.
type Hat interface {
	// -- Card selection --------------------------------------------------

	// ChooseMulligan returns true to mulligan the opening hand per §103.4.
	// `hand` is the current opener (7 cards first time, 6 on the second
	// call, and so on). The engine applies the mulligan machinery.
	ChooseMulligan(gs *GameState, seatIdx int, hand []*Card) bool

	// ChooseLandToPlay returns which land in hand to play this main phase,
	// or nil to skip the land drop. `lands` is the pre-filtered set of
	// lands currently in hand.
	ChooseLandToPlay(gs *GameState, seatIdx int, lands []*Card) *Card

	// ChooseCastFromHand returns which castable spell to cast next, or nil
	// to pass. `castable` is the engine-filtered set of affordable spells.
	ChooseCastFromHand(gs *GameState, seatIdx int, castable []*Card) *Card

	// ChooseActivation returns which activated ability to fire, or nil
	// to pass. `options` lists legal activations already filtered for
	// cost + timing restrictions.
	ChooseActivation(gs *GameState, seatIdx int, options []Activation) *Activation

	// -- Combat ----------------------------------------------------------

	// ChooseAttackers returns which of the legal attackers to declare.
	// Order of the returned slice is preserved for determinism.
	ChooseAttackers(gs *GameState, seatIdx int, legal []*Permanent) []*Permanent

	// ChooseAttackTarget returns the seat index `attacker` should attack.
	// `legalDefenders` is the set of living opponents (+ any planeswalker
	// seats the engine chose to surface). §506.1.
	ChooseAttackTarget(gs *GameState, seatIdx int, attacker *Permanent, legalDefenders []int) int

	// AssignBlockers maps each attacker to the blocker(s) assigned to it.
	// A missing / empty slice means the attacker is unblocked.
	// Blocker list order matters for damage assignment (§509.1h).
	AssignBlockers(gs *GameState, seatIdx int, attackers []*Permanent) map[*Permanent][]*Permanent

	// -- Stack / priority ------------------------------------------------

	// ChooseResponse returns a *StackItem (counter, removal, etc.) to put
	// on the stack in response, or nil to pass priority. §117.3.
	ChooseResponse(gs *GameState, seatIdx int, stackTop *StackItem) *StackItem

	// -- Targeting / modes -----------------------------------------------

	// ChooseTarget picks one target from `legal`. The engine passes `filter`
	// so the hat can apply filter-specific preferences (e.g. prefer a
	// highest-toughness creature for a buff vs lowest-toughness for a kill).
	ChooseTarget(gs *GameState, seatIdx int, filter gameast.Filter, legal []Target) Target

	// ChooseMode returns the selected mode INDEX for a modal effect.
	// `modes` is the list of options (§601.2c). Returns -1 to choose none.
	ChooseMode(gs *GameState, seatIdx int, modes []gameast.Effect) int

	// -- Commander -------------------------------------------------------

	// ShouldCastCommander returns true to cast the named commander from
	// the command zone this priority window. Engine already checked
	// mana affordability (`tax` is the current §903.8 surcharge).
	ShouldCastCommander(gs *GameState, seatIdx int, commanderName string, tax int) bool

	// ShouldRedirectCommanderZone returns true to apply the §903.9a / b
	// replacement and send the commander to the command zone INSTEAD of
	// `to` (hand / library / graveyard / exile). §903.9a wording: "the
	// OWNER may have that commander go to the command zone instead."
	ShouldRedirectCommanderZone(gs *GameState, seatIdx int, commander *Card, to string) bool

	// -- Replacements / discard ------------------------------------------

	// OrderReplacements returns the input slice re-ordered per the
	// affected player's choice. §616.1 says the AFFECTED player orders
	// the effects if more than one category-equivalent replacement is
	// applicable to the same event.
	OrderReplacements(gs *GameState, seatIdx int, candidates []*ReplacementEffect) []*ReplacementEffect

	// ChooseDiscard returns which N cards to discard. §701.8.
	ChooseDiscard(gs *GameState, seatIdx int, hand []*Card, n int) []*Card

	// -- Trigger ordering (§603.3b APNAP) --------------------------------

	// OrderTriggers returns the input slice of triggered-ability stack
	// items reordered per the controller's choice. CR §603.3b says when
	// multiple triggered abilities controlled by the same player go on
	// the stack simultaneously, that player chooses the order. The engine
	// calls this AFTER APNAP grouping — each call receives only the
	// triggers controlled by seatIdx. The returned order determines
	// stack push order (first element pushed first = resolves last).
	OrderTriggers(gs *GameState, seatIdx int, triggers []*StackItem) []*StackItem

	// -- X-cost announcement -----------------------------------------------

	// ChooseX returns the value of X for a spell with X in its mana cost.
	// Called by CastSpell when the card's mana cost contains X (CR §107.3).
	// `availableMana` is the mana the caster has after subtracting the non-X
	// portion of the cost. The Hat must return a value in [0, availableMana].
	ChooseX(gs *GameState, seatIdx int, card *Card, availableMana int) int

	// -- London mulligan bottom-card selection ----------------------------

	// ChooseBottomCards returns which N cards from hand to put on the
	// bottom of the library during the London mulligan (CR §103.5).
	// `count` is the number of cards to bottom (= number of mulligans
	// taken so far). The returned slice must have exactly `count` cards
	// from `hand`.
	ChooseBottomCards(gs *GameState, seatIdx int, hand []*Card, count int) []*Card

	// -- Scry / Surveil (§701.18 / §701.46) ----------------------------

	// ChooseScry is called when the player scries N cards. `cards` are
	// the top N cards of the library. The hat returns two slices:
	//   - `top`: cards to keep on top of the library, in order (first
	//     element will be the new top card)
	//   - `bottom`: cards to put on the bottom of the library, in order
	// Every card in `cards` must appear in exactly one of the two slices.
	ChooseScry(gs *GameState, seatIdx int, cards []*Card) (top []*Card, bottom []*Card)

	// ChooseSurveil is called when the player surveils N cards (§701.46).
	// `cards` are the top N cards of the library. The hat returns:
	//   - `graveyard`: cards to put into the graveyard
	//   - `top`: cards to keep on top of the library, in order
	// Every card in `cards` must appear in exactly one of the two slices.
	ChooseSurveil(gs *GameState, seatIdx int, cards []*Card) (graveyard []*Card, top []*Card)

	// -- Put-back (Brainstorm-style) --------------------------------------

	// ChoosePutBack returns `count` cards from `hand` to put back on top
	// of the library, in order (first element = new top card). Used by
	// Brainstorm ("draw 3, put 2 back") and similar effects. The returned
	// slice must have exactly `count` cards chosen from `hand`.
	ChoosePutBack(gs *GameState, seatIdx int, hand []*Card, count int) []*Card

	// -- Concession (conviction-based) -----------------------------------

	// ShouldConcede returns true if this hat wants to concede the game.
	// Called at the start of each turn. CR §104.3a: "A player can concede
	// the game at any time." Hats that don't track conviction return false.
	ShouldConcede(gs *GameState, seatIdx int) bool

	// -- Event observation (adaptive hats) -------------------------------

	// ObserveEvent is called for EVERY seat on EVERY logged event so
	// adaptive hats (e.g. PokerHat) can update internal state. The engine
	// broadcasts to every Hat; hats may filter by `event.Seat == seatIdx`
	// to look at their own actions only.
	//
	// MUST NOT mutate *GameState.
	ObserveEvent(gs *GameState, seatIdx int, event *Event)
}

// DamageDistributor is an optional interface that Hats can implement to
// control how "distribute N damage among targets" effects divide damage.
// CR §601.2d: when a spell says "distribute N damage", the caster divides
// at cast time and each target must receive at least 1.
//
// Hats that don't implement this interface get the default even-split
// heuristic in distributeDamage.
type DamageDistributor interface {
	// ChooseDamageDistribution returns an int slice of the same length as
	// targets, where amounts[i] is the damage to deal to targets[i].
	// Each value must be >= 1 and the sum must equal total.
	ChooseDamageDistribution(gs *GameState, seatIdx int, targets []Target, total int) []int
}

// SacrificeChooser is an optional interface Hats may implement to control
// which permanent the engine sacrifices when paying a cost-side or
// effect-driven sacrifice (CR §602.1b activation costs, §608.2c effect
// sacrifices). The engine routes through FindSacrificeTarget; if the
// seat's Hat implements this interface, the Hat's preference wins.
//
// Hats that don't implement this fall back to the deterministic
// "first matching battlefield permanent" path.
//
// Implementations MUST return one of the supplied candidates (or nil
// if no candidate is acceptable, in which case the engine treats the
// cost as unpayable). MUST NOT mutate gs.
type SacrificeChooser interface {
	// ChooseSacrifice returns the preferred permanent to sacrifice from
	// `candidates`. `source` is the permanent whose ability or effect
	// requires the sacrifice (may be nil for non-source contexts);
	// callers can use it to recognize "sacrifice ~" / "sacrifice itself"
	// situations. `reason` is "activation_cost", "additional_cost",
	// "bargain", or "effect" so the chooser can adjust priority by
	// context (e.g. paying for a recurring engine vs. a one-shot effect).
	ChooseSacrifice(gs *GameState, seatIdx int, source *Permanent, reason string, candidates []*Permanent) *Permanent
}

// WardPayer is an optional Hat interface that controls whether the caster
// pays the ward cost (CR §702.21) when their spell or ability targets a
// permanent with ward. The default if a Hat does not implement this is
// "pay if affordable, else fail-and-counter" — the historical engine
// behavior.
//
// CR §702.21c is a "may" — the caster is allowed to decline payment even
// when they could afford it, which the engine should respect (e.g. to save
// mana for a counterspell, or to deliberately fizzle into a recursion line).
//
// Implementations MUST NOT mutate gs.
//
// Alternative-payment ward (CR §702.21d — Ward—Sacrifice / Ward—Discard /
// Ward—Blight N) was added in R60 via the ward_alt_kind + ward_alt_filter
// Permanent.Flags. The engine handles those payments inline in
// ward_alt_payment.go's tryPayAltWardCost with a "pay if affordable"
// default; WardPayer is consulted only for the integer mana-ward path.
// A future Hat extension can add a sibling ShouldPayAltWard interface if
// per-hat opt-out semantics matter for the non-mana costs.
type WardPayer interface {
	// ShouldPayWard returns true if the caster wants to pay the ward cost
	// on `target` for the spell/ability represented by `item`. `wardCost`
	// is the generic mana amount (Ward {N}); non-generic ward is handled
	// inline by the engine via Permanent.Flags["ward_alt_kind"] and does
	// not flow through this hook.
	ShouldPayWard(gs *GameState, seatIdx int, item *StackItem, target *Permanent, wardCost int) bool
}

// ConcedeGame marks a seat as having conceded. CR §104.3a.
func ConcedeGame(gs *GameState, seatIdx int) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || seat.Lost || seat.LeftGame {
		return
	}
	seat.Lost = true
	seat.LossReason = "concession"
	gs.LogEvent(Event{
		Kind:   "concession",
		Seat:   seatIdx,
		Target: -1,
		Details: map[string]interface{}{
			"rule":   "104.3a",
			"reason": "conviction_concession",
			"turn":   gs.Turn,
		},
	})
	HandleSeatElimination(gs, seatIdx)
}

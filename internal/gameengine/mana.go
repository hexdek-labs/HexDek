package gameengine

// CR §106 — Mana.
//
// The typed mana pool tracks mana by color (W/U/B/R/G), plus colorless
// C mana (§106.1b — distinct from generic; Sol Ring and Wastes produce
// {C}), plus an "any"-color bucket for sources that let the player
// choose at spend time (Moxes, Lotus Petal, Treasures), plus a list of
// restricted-mana entries (§106.4a — Food Chain's creature-only mana,
// Powerstone's noncreature-activation-only {C}, Cabal Coffers variants).
//
// BACKWARDS COMPATIBILITY:
//
//   Every historical call site treats `Seat.ManaPool` as a plain int:
//   `seat.ManaPool = 10`, `seat.ManaPool -= cost`, `seat.ManaPool++`.
//   We PRESERVE this API. The Seat struct keeps its `ManaPool int`
//   field. A NEW `Mana *ColoredManaPool` field sits alongside it; when
//   the typed pool is initialized (via EnsureTypedPool), we mirror its
//   total into ManaPool for legacy reads. Writes via ManaPool directly
//   invalidate the typed pool (the next read rebuilds from zeros).
//
//   New code should prefer the typed helpers (AddMana, PayManaCost,
//   DrainManaPool) and treat ManaPool as a read-mostly mirror.
//
// CR §106.4 mandates the pool empty at every phase/step boundary. The
// turn loop calls DrainAllPools from turn.go at every transition;
// exemption cards (Upwelling, Omnath) are honored via ManaExemption.

import (
	"strings"
)

// ColoredManaPool is the typed five-color+colorless+any+restricted pool.
type ColoredManaPool struct {
	W, U, B, R, G, C, Any int
	Restricted            []RestrictedMana
}

// RestrictedMana is one unit of mana carrying a spend-time restriction
// per CR §106.4a.
type RestrictedMana struct {
	Amount      int
	Color       string // "W"/"U"/.../"C" or "" for any-color
	Restriction string // e.g. "creature_spell_only", "noncreature_or_artifact_activation"
	Source      string // source card name for attribution
}

// Total returns the total mana in the pool across every bucket.
func (p *ColoredManaPool) Total() int {
	if p == nil {
		return 0
	}
	t := p.W + p.U + p.B + p.R + p.G + p.C + p.Any
	for _, r := range p.Restricted {
		t += r.Amount
	}
	return t
}

// Clear empties every bucket (used for phase-end drain and test resets).
func (p *ColoredManaPool) Clear() {
	if p == nil {
		return
	}
	p.W, p.U, p.B, p.R, p.G, p.C, p.Any = 0, 0, 0, 0, 0, 0, 0
	p.Restricted = nil
}

// ClearExcept empties every bucket whose color is NOT in exempt.
// "any" in exempt means "all colors retained" (Upwelling). Colors are
// single-character codes: "W"/"U"/"B"/"R"/"G"/"C".
func (p *ColoredManaPool) ClearExcept(exempt map[string]bool) {
	if p == nil {
		return
	}
	if exempt["any"] {
		return // Upwelling: nothing empties.
	}
	if !exempt["W"] {
		p.W = 0
	}
	if !exempt["U"] {
		p.U = 0
	}
	if !exempt["B"] {
		p.B = 0
	}
	if !exempt["R"] {
		p.R = 0
	}
	if !exempt["G"] {
		p.G = 0
	}
	if !exempt["C"] {
		p.C = 0
	}
	// "any" and restricted drain unless fully exempt (handled above).
	p.Any = 0
	if len(exempt) == 0 {
		p.Restricted = nil
	} else {
		// With partial exemption, restricted mana also drains — it
		// has no fixed color, so a color-specific exemption can't hold it.
		p.Restricted = nil
	}
}

// Add credits `amount` mana of `color` into the appropriate bucket.
// color: "W"/"U"/"B"/"R"/"G"/"C"/"any".
func (p *ColoredManaPool) Add(color string, amount int) {
	if p == nil || amount <= 0 {
		return
	}
	switch strings.ToUpper(color) {
	case "W":
		p.W += amount
	case "U":
		p.U += amount
	case "B":
		p.B += amount
	case "R":
		p.R += amount
	case "G":
		p.G += amount
	case "C":
		p.C += amount
	case "ANY", "*":
		p.Any += amount
	default:
		// Unknown color → any-color as safe fallback.
		p.Any += amount
	}
}

// AddRestricted appends a restricted-mana entry.
func (p *ColoredManaPool) AddRestricted(amount int, color, restriction, source string) {
	if p == nil || amount <= 0 {
		return
	}
	p.Restricted = append(p.Restricted, RestrictedMana{
		Amount: amount, Color: color, Restriction: restriction, Source: source,
	})
}

// CanPayGeneric tests whether `amount` generic mana is payable given
// the spell type (for restriction checks).
func (p *ColoredManaPool) CanPayGeneric(amount int, spellType string) bool {
	if p == nil {
		return amount <= 0
	}
	if amount <= 0 {
		return true
	}
	avail := p.W + p.U + p.B + p.R + p.G + p.C + p.Any
	for _, r := range p.Restricted {
		if RestrictionAllows(r.Restriction, spellType, false) {
			avail += r.Amount
		}
	}
	return avail >= amount
}

// CanPayColored tests whether `amount` pips of `color` are payable.
func (p *ColoredManaPool) CanPayColored(color string, amount int, spellType string) bool {
	if p == nil {
		return amount <= 0
	}
	if amount <= 0 {
		return true
	}
	colorU := strings.ToUpper(color)
	var colorVal int
	switch colorU {
	case "W":
		colorVal = p.W
	case "U":
		colorVal = p.U
	case "B":
		colorVal = p.B
	case "R":
		colorVal = p.R
	case "G":
		colorVal = p.G
	case "C":
		colorVal = p.C
	}
	avail := colorVal + p.Any
	for _, r := range p.Restricted {
		if r.Color != "" && r.Color != colorU {
			continue
		}
		if RestrictionAllows(r.Restriction, spellType, colorU == "C") {
			avail += r.Amount
		}
	}
	return avail >= amount
}

// RestrictionAllows returns true if restricted mana tagged with
// `restriction` may pay for a spell/ability of type `spellType`.
// spellType: "creature"/"noncreature"/"instant"/"sorcery"/"artifact"/
// "activated"/"generic". Unknown restriction → allow (conservative).
func RestrictionAllows(restriction, spellType string, colorless bool) bool {
	r := strings.ToLower(restriction)
	if r == "" {
		return true
	}
	switch r {
	case "creature_spell_only":
		return spellType == "creature"
	case "noncreature_or_artifact_activation",
		"non_creature_activation_only",
		"noncreature_activation_only":
		return spellType == "noncreature" || spellType == "activated" ||
			spellType == "instant" || spellType == "sorcery"
	case "artifact_only":
		return spellType == "artifact" || spellType == "activated"
	case "instant_or_sorcery_only":
		return spellType == "instant" || spellType == "sorcery"
	}
	return true
}

// --- Seat-level helpers --------------------------------------------------

// EnsureTypedPool ensures seat.Mana is non-nil and, if ManaPool > 0 but
// the typed pool is empty, seeds the typed pool from ManaPool as Any-
// color mana (legacy-style migration).
func EnsureTypedPool(seat *Seat) *ColoredManaPool {
	if seat == nil {
		return nil
	}
	if seat.Mana == nil {
		seat.Mana = &ColoredManaPool{}
	}
	// Legacy bridge: if the legacy ManaPool int has more mana than the
	// typed pool, credit the delta as any-color. This keeps pre-existing
	// call sites that do `seat.ManaPool = 10` working — the next typed
	// operation sees the mana.
	if seat.ManaPool > seat.Mana.Total() {
		seat.Mana.Any += seat.ManaPool - seat.Mana.Total()
	}
	return seat.Mana
}

// AddMana adds `amount` mana of `color` into the seat's typed pool and
// mirrors the total into the legacy ManaPool int. Emits an add_mana event.
func AddMana(gs *GameState, seat *Seat, color string, amount int, source string) {
	if seat == nil || amount <= 0 {
		return
	}
	p := EnsureTypedPool(seat)
	p.Add(color, amount)
	seat.ManaPool = p.Total()
	if gs != nil {
		gs.LogEvent(Event{
			Kind:   "add_mana",
			Seat:   seat.Idx,
			Source: source,
			Amount: amount,
			Details: map[string]interface{}{
				"color":      strings.ToUpper(color),
				"pool_after": seat.ManaPool,
			},
		})
	}
}

// AddRestrictedMana adds a restriction-tagged mana unit.
func AddRestrictedMana(gs *GameState, seat *Seat, amount int,
	color, restriction, source string) {
	if seat == nil || amount <= 0 {
		return
	}
	p := EnsureTypedPool(seat)
	p.AddRestricted(amount, color, restriction, source)
	seat.ManaPool = p.Total()
	if gs != nil {
		gs.LogEvent(Event{
			Kind:   "add_mana",
			Seat:   seat.Idx,
			Source: source,
			Amount: amount,
			Details: map[string]interface{}{
				"color":       color,
				"restriction": restriction,
				"pool_after":  seat.ManaPool,
			},
		})
	}
}

// PayGenericCost spends `amount` generic mana from the seat's pool,
// respecting restrictions. Returns true on success. Mutates ManaPool
// (both legacy int and typed pool). Uses a "any-first" spending order
// so colored buckets are preserved for colored costs.
func PayGenericCost(gs *GameState, seat *Seat, amount int,
	spellType, reason, cardName string) bool {
	if amount <= 0 {
		return true
	}
	if seat == nil {
		return false
	}
	p := EnsureTypedPool(seat)
	if !p.CanPayGeneric(amount, spellType) {
		// Fallback: if the legacy ManaPool int has enough AND the
		// typed pool has no tracked mana (meaning the seat was primed
		// via `seat.ManaPool = 10` without typed ops), debit from it
		// directly so old tests keep working. If the typed pool has
		// entries, we trust it — a typed pool that can't pay is a
		// genuine restriction failure, not a missing-info fallback.
		if p.Total() == 0 && seat.ManaPool >= amount {
			seat.ManaPool -= amount
			if gs != nil {
				gs.LogEvent(Event{
					Kind:   "pay_mana",
					Seat:   seat.Idx,
					Source: cardName,
					Amount: amount,
					Details: map[string]interface{}{
						"reason":     reason,
						"pool_after": seat.ManaPool,
						"legacy":     true,
					},
				})
			}
			return true
		}
		return false
	}
	breakdown := map[string]int{}
	for i := 0; i < amount; i++ {
		bucket := debitGenericPip(p, spellType)
		if bucket == "" {
			return false
		}
		breakdown[bucket]++
	}
	seat.ManaPool = p.Total()
	if gs != nil {
		gs.LogEvent(Event{
			Kind:   "pay_mana",
			Seat:   seat.Idx,
			Source: cardName,
			Amount: amount,
			Details: map[string]interface{}{
				"reason":     reason,
				"breakdown":  breakdown,
				"pool_after": seat.ManaPool,
			},
		})
	}
	return true
}

// debitGenericPip removes one generic pip from the pool, preferring
// any-first to preserve colors. Returns the bucket name spent.
func debitGenericPip(p *ColoredManaPool, spellType string) string {
	if p.Any > 0 {
		p.Any--
		return "any"
	}
	if p.C > 0 {
		p.C--
		return "C"
	}
	if p.W > 0 {
		p.W--
		return "W"
	}
	if p.U > 0 {
		p.U--
		return "U"
	}
	if p.B > 0 {
		p.B--
		return "B"
	}
	if p.R > 0 {
		p.R--
		return "R"
	}
	if p.G > 0 {
		p.G--
		return "G"
	}
	// Fall back to restricted buckets.
	for i := range p.Restricted {
		r := &p.Restricted[i]
		if r.Amount <= 0 {
			continue
		}
		if !RestrictionAllows(r.Restriction, spellType, false) {
			continue
		}
		r.Amount--
		name := "restricted:" + r.Restriction
		if r.Amount == 0 {
			p.Restricted = append(p.Restricted[:i], p.Restricted[i+1:]...)
		}
		return name
	}
	return ""
}

// PayColoredPip spends one pip of a specific color. Returns the bucket
// spent, or "" if unpayable.
func PayColoredPip(p *ColoredManaPool, color, spellType string) string {
	colorU := strings.ToUpper(color)
	switch colorU {
	case "W":
		if p.W > 0 {
			p.W--
			return "W"
		}
	case "U":
		if p.U > 0 {
			p.U--
			return "U"
		}
	case "B":
		if p.B > 0 {
			p.B--
			return "B"
		}
	case "R":
		if p.R > 0 {
			p.R--
			return "R"
		}
	case "G":
		if p.G > 0 {
			p.G--
			return "G"
		}
	case "C":
		if p.C > 0 {
			p.C--
			return "C"
		}
	}
	if p.Any > 0 {
		p.Any--
		return "any"
	}
	for i := range p.Restricted {
		r := &p.Restricted[i]
		if r.Amount <= 0 {
			continue
		}
		if r.Color != "" && r.Color != colorU {
			continue
		}
		if !RestrictionAllows(r.Restriction, spellType, colorU == "C") {
			continue
		}
		r.Amount--
		name := "restricted:" + r.Restriction
		if r.Amount == 0 {
			p.Restricted = append(p.Restricted[:i], p.Restricted[i+1:]...)
		}
		return name
	}
	return ""
}

// --- Phase-end drain -----------------------------------------------------

// PoolExemptColors — CR §106.4 / §613 layer 6. Returns the set of color
// codes whose mana is RETAINED at phase boundaries for this seat.
// Values: subset of {"W","U","B","R","G","C"}, or "any" → all retained
// (Upwelling).
func PoolExemptColors(gs *GameState, seat *Seat) map[string]bool {
	if gs == nil || seat == nil {
		return nil
	}
	exempt := map[string]bool{}
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, perm := range s.Battlefield {
			if perm == nil || perm.Card == nil {
				continue
			}
			name := perm.Card.DisplayName()
			if name == "Upwelling" {
				return map[string]bool{"any": true}
			}
			if name == "Omnath, Locus of Mana" {
				if perm.Controller == seat.Idx {
					exempt["G"] = true
				}
			}
		}
	}
	return exempt
}

// DrainAllPools — CR §106.4 empties each player's mana pool at every
// phase/step boundary, honoring exemption cards. Called from the turn
// loop at every phase transition.
func DrainAllPools(gs *GameState, prevPhase, prevStep string) {
	if gs == nil {
		return
	}
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		p := seat.Mana
		before := 0
		if p != nil {
			before = p.Total()
		}
		legacyBefore := seat.ManaPool
		if before == 0 && legacyBefore == 0 {
			continue
		}
		exempt := PoolExemptColors(gs, seat)
		if exempt["any"] {
			continue // Upwelling: skip drain.
		}
		if p != nil {
			p.ClearExcept(exempt)
			seat.ManaPool = p.Total()
		} else {
			// No typed pool yet. The legacy int pool has no color info,
			// so any exempt-cards only help if Upwelling is present
			// (handled above). Absent that, drain to zero.
			seat.ManaPool = 0
		}
		drained := legacyBefore - seat.ManaPool
		if drained != 0 {
			exemptList := []string{}
			for k := range exempt {
				exemptList = append(exemptList, k)
			}
			gs.LogEvent(Event{
				Kind:   "pool_drain",
				Seat:   seat.Idx,
				Amount: drained,
				Details: map[string]interface{}{
					"remaining":     seat.ManaPool,
					"exempt_colors": exemptList,
					"from_phase":    prevPhase,
					"from_step":     prevStep,
					"rule":          "106.4",
				},
			})
		}
	}
}

// AddManaFromPermanent is the CR §605 "tap for mana" entry point — use
// this instead of AddMana when the mana originates from a SPECIFIC
// permanent tapping (as opposed to a spell's Add effect or a generic
// grant). The function:
//
//  1. Credits the seat's pool via AddMana (same as before — no change
//     to the general pool mutation path).
//  2. Fires the per-card trigger "mana_added_from_permanent" so
//     static-ability handlers like Kinnan can react and add extra
//     mana in response. The ctx carries:
//     - "source_perm": *Permanent  — the permanent that produced the mana
//     - "color":       string      — the color added ("W"/"U"/"B"/"R"/"G"/"C"/"any")
//     - "amount":      int         — amount added
//     - "from_kinnan": bool        — set by Kinnan when IT is adding
//                                    the extra mana (anti-recursion)
//
// This seam is ADDITIVE. Existing AddMana call sites are unaffected
// until they opt in by calling AddManaFromPermanent instead. Mana-
// artifact tap paths (mana_artifacts.go) and creature-dork paths
// should migrate to this helper to enable Kinnan and similar effects
// (e.g. Zhur-Taa Druid's ping, future mana-reaction snowflakes).
//
// Why not rewire AddMana directly: AddMana is called from spell
// effects (resolveAddMana in resolve.go) — "Lotus Bloom's treasure
// clause," "Dark Ritual adds BBB" — those are NOT tap-for-mana
// events per CR §605.1, so Kinnan must not trigger on them.
// Explicitly scoping this to the permanent-taps-for-mana branch
// preserves the §605 boundary.
func AddManaFromPermanent(gs *GameState, seat *Seat, sourcePerm *Permanent,
	color string, amount int) {
	if gs == nil || seat == nil || amount <= 0 {
		return
	}
	sourceName := ""
	if sourcePerm != nil && sourcePerm.Card != nil {
		sourceName = sourcePerm.Card.DisplayName()
	}
	AddMana(gs, seat, color, amount, sourceName)
	// Fire the mana-augmentation trigger. Handlers on permanents with
	// "whenever you tap a nonland permanent for mana" static abilities
	// listen here.
	FireCardTrigger(gs, "mana_added_from_permanent", map[string]interface{}{
		"source_perm": sourcePerm,
		"color":       color,
		"amount":      amount,
		"seat":        seat.Idx,
		"from_kinnan": false,
	})
}

// SpendMana deducts mana from the typed pool and syncs the legacy ManaPool.
// This is the PROPER way to spend mana — replaces direct `seat.ManaPool -= cost`.
// Deducts from generic/any/colorless buckets first.
func SpendMana(seat *Seat, amount int) {
	if seat == nil || amount <= 0 {
		return
	}
	p := EnsureTypedPool(seat)
	// If typed pool is empty but legacy ManaPool has mana (test setup
	// pattern: seat.ManaPool = 10), deduct from legacy directly.
	if p.Total() == 0 && seat.ManaPool >= amount {
		seat.ManaPool -= amount
		return
	}
	remaining := amount
	// Drain from any first (most flexible)
	if remaining > 0 && p.Any > 0 {
		take := remaining
		if take > p.Any { take = p.Any }
		p.Any -= take
		remaining -= take
	}
	// Then colorless
	if remaining > 0 && p.C > 0 {
		take := remaining
		if take > p.C { take = p.C }
		p.C -= take
		remaining -= take
	}
	// Then colors in WUBRG order
	for _, color := range []*int{&p.R, &p.G, &p.B, &p.U, &p.W} {
		if remaining <= 0 { break }
		if *color > 0 {
			take := remaining
			if take > *color { take = *color }
			*color -= take
			remaining -= take
		}
	}
	// Drain restricted mana last
	for i := range p.Restricted {
		if remaining <= 0 { break }
		if p.Restricted[i].Amount > 0 {
			take := remaining
			if take > p.Restricted[i].Amount { take = p.Restricted[i].Amount }
			p.Restricted[i].Amount -= take
			remaining -= take
		}
	}
	// Sync legacy
	seat.ManaPool = p.Total()
}

// SyncManaAfterSpend reconciles the typed pool after a direct ManaPool deduction.
// Call this immediately after any `seat.ManaPool -= N` to keep pools in sync.
// Safe no-op if typed pool is nil, empty, or already in sync.
func SyncManaAfterSpend(seat *Seat) {
	if seat == nil || seat.Mana == nil {
		return
	}
	typed := seat.Mana.Total()
	if typed <= seat.ManaPool {
		return // already in sync or ManaPool has more (legacy seeded)
	}
	// Typed pool has more than ManaPool — drain the excess.
	deficit := typed - seat.ManaPool
	// Drain from any > C > colors (WUBRG order matches debitGenericPip).
	for deficit > 0 && seat.Mana.Any > 0 {
		seat.Mana.Any--
		deficit--
	}
	for deficit > 0 && seat.Mana.C > 0 {
		seat.Mana.C--
		deficit--
	}
	for deficit > 0 && seat.Mana.R > 0 {
		seat.Mana.R--
		deficit--
	}
	for deficit > 0 && seat.Mana.G > 0 {
		seat.Mana.G--
		deficit--
	}
	for deficit > 0 && seat.Mana.B > 0 {
		seat.Mana.B--
		deficit--
	}
	for deficit > 0 && seat.Mana.U > 0 {
		seat.Mana.U--
		deficit--
	}
	for deficit > 0 && seat.Mana.W > 0 {
		seat.Mana.W--
		deficit--
	}
}

// SyncManaAfterAdd reconciles the typed pool after a direct ManaPool addition
// (ManaPool += N or ManaPool++). Credits the delta as Any-color mana.
func SyncManaAfterAdd(seat *Seat, amount int) {
	if seat == nil || seat.Mana == nil || amount <= 0 {
		return
	}
	seat.Mana.Any += amount
}

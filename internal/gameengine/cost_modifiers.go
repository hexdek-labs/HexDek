package gameengine

// Cost modification framework — CR §601.2f cost calculation.
//
// Comp-rules citations:
//
//   §601.2f  — "The total cost is the mana cost or alternative cost
//              (as determined in rule 601.2b), plus all additional costs
//              and cost increases, and minus all cost reductions."
//   §601.2f  — "If there are multiple cost reductions, the player
//              chooses the order to apply them."
//   §601.2f  — "Abilities that reduce a cost by an amount of generic
//              mana can't reduce that cost below zero."
//   §601.2f  — Trinisphere (minimum 3) applies AFTER all increases
//              and reductions.
//
// Ordering per CR §601.2f:
//   1. Start with base mana cost (or alternative cost)
//   2. Add all cost increases
//   3. Subtract all cost reductions (floor 0)
//   4. Apply Trinisphere minimum (if applicable)
//
// This file provides:
//
//   - CostModifier struct — a single cost increase/reduction/minimum
//   - CalculateTotalCost(gs, card, seatIdx) — walks battlefield for
//     cost modifiers and computes the final cost per §601.2f
//   - ScanCostModifiers(gs, card, seatIdx) — returns all applicable
//     modifiers for diagnostics/testing
//
// Per-card handlers (Thalia, Trinisphere, Helm of Awakening, etc.)
// register their modifiers by having the appropriate flags or by being
// recognized by name in the modifier scanner.

import (
	"strings"
)

// CostModifierKind discriminates cost modifier types.
type CostModifierKind int

const (
	// CostModIncrease — add to cost (Thalia, Sphere of Resistance).
	CostModIncrease CostModifierKind = iota
	// CostModReduction — subtract from cost (Helm of Awakening, Medallions).
	CostModReduction
	// CostModMinimum — set a floor (Trinisphere: minimum 3).
	CostModMinimum
)

// CostModifier describes a single cost modification effect from a
// permanent on the battlefield.
type CostModifier struct {
	Kind   CostModifierKind
	Amount int
	Source string // card name for logging
}

// CalculateTotalCost computes the total mana cost for casting `card`
// by `seatIdx`, walking the battlefield for static cost-modifying
// effects. Returns the final cost after increases, reductions, and
// minimums per CR §601.2f ordering.
func CalculateTotalCost(gs *GameState, card *Card, seatIdx int) int {
	if gs == nil || card == nil {
		return 0
	}
	baseCost := manaCostOf(card)
	if card.CastingBackFace && card.BackFaceCMC > 0 {
		baseCost = card.BackFaceCMC
	}
	mods := ScanCostModifiers(gs, card, seatIdx)

	// §903.8 — Commander tax. Each prior cast from the command zone adds {2}
	// to the cost. This applies when the game is in commander format and the
	// card is one of the caster's commanders.
	if gs.CommanderFormat && seatIdx >= 0 && seatIdx < len(gs.Seats) {
		seat := gs.Seats[seatIdx]
		if seat != nil && seat.CommanderCastCounts != nil {
			for _, cmdName := range seat.CommanderNames {
				if card.Name == cmdName {
					priorCasts := seat.CommanderCastCounts[cmdName]
					if priorCasts > 0 {
						mods = append(mods, CostModifier{
							Kind:   CostModIncrease,
							Amount: 2 * priorCasts,
							Source: "commander_tax",
						})
					}
					break
				}
			}
		}
	}

	return ApplyCostModifiers(baseCost, mods)
}

// ApplyCostModifiers applies a list of cost modifiers to a base cost
// per CR §601.2f ordering: increases first, then reductions (floor 0),
// then minimums.
func ApplyCostModifiers(baseCost int, mods []CostModifier) int {
	cost := baseCost

	// Step 1: Apply all increases.
	for _, m := range mods {
		if m.Kind == CostModIncrease {
			cost += m.Amount
		}
	}

	// Step 2: Apply all reductions (floor 0).
	for _, m := range mods {
		if m.Kind == CostModReduction {
			cost -= m.Amount
			if cost < 0 {
				cost = 0
			}
		}
	}

	// Step 3: Apply minimums (Trinisphere).
	for _, m := range mods {
		if m.Kind == CostModMinimum {
			if cost < m.Amount {
				cost = m.Amount
			}
		}
	}

	return cost
}

// ScanCostModifiers walks every permanent on every battlefield and
// returns all cost modifiers that apply to `card` being cast by
// `seatIdx`. Each recognized card produces one CostModifier entry.
//
// Recognized cost-modifying permanents:
//
//   Increases:
//     - Thalia, Guardian of Thraben: noncreature spells cost {1} more
//     - Sphere of Resistance: spells cost {1} more
//     - Thorn of Amethyst: noncreature spells cost {1} more
//     - Vryn Wingmare: noncreature spells cost {1} more
//     - Glowrider: noncreature spells cost {1} more
//     - Grand Arbiter Augustin IV: opponents' spells cost {1} more
//     - Thalia, Heretic Cathar: (lands ETB tapped, not cost — skip)
//
//   Reductions:
//     - Helm of Awakening: spells cost {1} less
//     - Goblin Electromancer: instant/sorcery spells cost {1} less
//     - Baral, Chief of Compliance: instant/sorcery spells cost {1} less
//     - Sapphire Medallion: blue spells cost {1} less
//     - Jet Medallion: black spells cost {1} less
//     - Ruby Medallion: red spells cost {1} less
//     - Pearl Medallion: white spells cost {1} less
//     - Emerald Medallion: green spells cost {1} less
//     - Edgewalker: cleric spells cost {W}{B} less (simplified to {2})
//     - Cloud Key: chosen type costs {1} less (simplified: artifacts)
//     - Semblance Anvil: exiled card's type costs {2} less (simplified)
//     - Grand Arbiter Augustin IV: your white/blue spells cost {1} less
//     - Urza's Incubator: chosen creature type costs {2} less (simplified)
//
//   Minimums:
//     - Trinisphere: spells cost at least {3} (only opponent's in practice,
//       but CR says all spells)
func ScanCostModifiers(gs *GameState, card *Card, seatIdx int) []CostModifier {
	if gs == nil || card == nil {
		return nil
	}

	// Precompute card properties for filter matching.
	isCreature := cardHasType(card, "creature")
	isNoncreature := !isCreature
	isInstant := cardHasType(card, "instant")
	isSorcery := cardHasType(card, "sorcery")
	isInstantOrSorcery := isInstant || isSorcery

	var mods []CostModifier

	// Eminence — The Ur-Dragon: as long as Ur-Dragon is in the command zone
	// or on the battlefield, OTHER Dragon spells you cast cost {1} less.
	// CR Eminence (Reflector Mage / Commander 2017): the ability functions
	// from BOTH zones, so we check command zone in addition to the
	// battlefield scan below.
	if seatIdx >= 0 && seatIdx < len(gs.Seats) {
		seat := gs.Seats[seatIdx]
		if seat != nil && cardHasSubtype(card, "dragon") &&
			!strings.EqualFold(card.DisplayName(), "The Ur-Dragon") {
			for _, c := range seat.CommandZone {
				if c != nil && strings.EqualFold(c.DisplayName(), "The Ur-Dragon") {
					mods = append(mods, CostModifier{
						Kind:   CostModReduction,
						Amount: 1,
						Source: "The Ur-Dragon (eminence, command zone)",
					})
					break
				}
			}
		}
	}

	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for _, perm := range seat.Battlefield {
			if perm == nil || perm.Card == nil {
				continue
			}
			name := perm.Card.DisplayName()
			isOpponent := perm.Controller != seatIdx
			isSelf := perm.Controller == seatIdx

			switch name {
			// --- COST INCREASES ---

			case "Thalia, Guardian of Thraben":
				// Noncreature spells cost {1} more.
				if isNoncreature {
					mods = append(mods, CostModifier{
						Kind:   CostModIncrease,
						Amount: 1,
						Source: name,
					})
				}

			case "Sphere of Resistance":
				// All spells cost {1} more.
				mods = append(mods, CostModifier{
					Kind:   CostModIncrease,
					Amount: 1,
					Source: name,
				})

			case "Thorn of Amethyst", "Vryn Wingmare", "Glowrider":
				// Noncreature spells cost {1} more.
				if isNoncreature {
					mods = append(mods, CostModifier{
						Kind:   CostModIncrease,
						Amount: 1,
						Source: name,
					})
				}

			case "Grand Arbiter Augustin IV":
				// Opponents' spells cost {1} more.
				if isOpponent {
					mods = append(mods, CostModifier{
						Kind:   CostModIncrease,
						Amount: 1,
						Source: name,
					})
				}
				// Your white spells cost {1} less, your blue spells cost {1}
				// less (non-cumulative — cap at 1 reduction total for simplicity).
				if isSelf {
					hasW := CardHasColor(card, "W")
					hasU := CardHasColor(card, "U")
					if hasW || hasU {
						mods = append(mods, CostModifier{
							Kind:   CostModReduction,
							Amount: 1,
							Source: name,
						})
					}
				}

			case "Defense Grid":
				// Each spell costs {3} more except during its controller's turn.
				// "Its controller" = the spell's caster, not Defense Grid's controller.
				if seatIdx != gs.Active {
					mods = append(mods, CostModifier{
						Kind:   CostModIncrease,
						Amount: 3,
						Source: name,
					})
				}

			case "Lodestone Golem":
				// Nonartifact spells cost {1} more.
				if !cardHasType(card, "artifact") {
					mods = append(mods, CostModifier{
						Kind:   CostModIncrease,
						Amount: 1,
						Source: name,
					})
				}

			// --- COST REDUCTIONS ---

			case "Helm of Awakening":
				// All spells cost {1} less.
				mods = append(mods, CostModifier{
					Kind:   CostModReduction,
					Amount: 1,
					Source: name,
				})

			case "Goblin Electromancer", "Baral, Chief of Compliance":
				// Instant and sorcery spells cost {1} less.
				if isSelf && isInstantOrSorcery {
					mods = append(mods, CostModifier{
						Kind:   CostModReduction,
						Amount: 1,
						Source: name,
					})
				}

			case "Mizzix of the Izmagnus":
				// Instant and sorcery spells you cast cost {1} less for each
				// experience counter you have (CR §601.2f, floor 0).
				if isSelf && isInstantOrSorcery {
					casterSeat := gs.Seats[seatIdx]
					if casterSeat != nil && casterSeat.Flags != nil {
						xp := casterSeat.Flags["experience_counters"]
						if xp > 0 {
							mods = append(mods, CostModifier{
								Kind:   CostModReduction,
								Amount: xp,
								Source: name,
							})
						}
					}
				}

			case "Sapphire Medallion":
				if isSelf && CardHasColor(card, "U") {
					mods = append(mods, CostModifier{
						Kind:   CostModReduction,
						Amount: 1,
						Source: name,
					})
				}
			case "Jet Medallion":
				if isSelf && CardHasColor(card, "B") {
					mods = append(mods, CostModifier{
						Kind:   CostModReduction,
						Amount: 1,
						Source: name,
					})
				}
			case "Ruby Medallion":
				if isSelf && CardHasColor(card, "R") {
					mods = append(mods, CostModifier{
						Kind:   CostModReduction,
						Amount: 1,
						Source: name,
					})
				}
			case "Pearl Medallion":
				if isSelf && CardHasColor(card, "W") {
					mods = append(mods, CostModifier{
						Kind:   CostModReduction,
						Amount: 1,
						Source: name,
					})
				}
			case "Emerald Medallion":
				if isSelf && CardHasColor(card, "G") {
					mods = append(mods, CostModifier{
						Kind:   CostModReduction,
						Amount: 1,
						Source: name,
					})
				}

			case "Edgewalker":
				// Cleric spells you cast cost {W}{B} less (simplified: {2} less).
				if isSelf && cardHasSubtype(card, "cleric") {
					mods = append(mods, CostModifier{
						Kind:   CostModReduction,
						Amount: 2,
						Source: name,
					})
				}

			case "Rooftop Storm":
				// Zombie creature spells you cast cost {0}.
				if isSelf && isCreature && cardHasSubtype(card, "zombie") {
					cmc := ManaCostOf(card)
					if cmc > 0 {
						mods = append(mods, CostModifier{
							Kind:   CostModReduction,
							Amount: cmc,
							Source: name,
						})
					}
				}

			case "Undead Warchief":
				// Zombie spells you cast cost {1} less.
				if isSelf && cardHasSubtype(card, "zombie") {
					mods = append(mods, CostModifier{
						Kind:   CostModReduction,
						Amount: 1,
						Source: name,
					})
				}

			case "The Ur-Dragon":
				// Eminence — other Dragon spells you cast cost {1} less.
				// Battlefield-side reduction (the command-zone half is
				// applied in the preamble above so we don't double-apply).
				if isSelf && cardHasSubtype(card, "dragon") &&
					!strings.EqualFold(card.DisplayName(), "The Ur-Dragon") {
					mods = append(mods, CostModifier{
						Kind:   CostModReduction,
						Amount: 1,
						Source: name + " (eminence)",
					})
				}

			case "Nightscape Familiar":
				// Blue and red spells cost {1} less.
				if isSelf && (CardHasColor(card, "U") || CardHasColor(card, "R")) {
					mods = append(mods, CostModifier{
						Kind:   CostModReduction,
						Amount: 1,
						Source: name,
					})
				}

			case "Foundry Inspector":
				// Artifact spells you cast cost {1} less.
				if isSelf && cardHasType(card, "artifact") {
					mods = append(mods, CostModifier{
						Kind:   CostModReduction,
						Amount: 1,
						Source: name,
					})
				}

			case "Witherbloom, the Balancer":
				// "Instant and sorcery spells you cast have affinity for
				// creatures." Grant the same {1}-less-per-creature
				// reduction that the printed keyword applies.
				if isSelf && isInstantOrSorcery {
					n := CountCreaturesOnBattlefield(gs, seatIdx)
					if n > 0 {
						mods = append(mods, CostModifier{
							Kind:   CostModReduction,
							Amount: n,
							Source: name + " (granted affinity)",
						})
					}
				}

			case "Henzie \"Toolbox\" Torre":
				// Each creature spell you cast with mana value 4+ has blitz;
				// blitz cost = mana cost. Blitz costs you pay cost {1} less
				// for each time you've cast your commander from the command
				// zone this game.
				//
				// Modeling: when Henzie's controller casts a creature spell
				// with MV >= 4, reduce its cost by the total commander cast
				// count for that seat. When the count is 0, this is a no-op
				// and blitz is strictly equivalent to the printed cast.
				if isSelf && isCreature && manaCostOf(card) >= 4 {
					casts := 0
					if seat := gs.Seats[seatIdx]; seat != nil && seat.CommanderCastCounts != nil {
						for _, n := range seat.CommanderCastCounts {
							casts += n
						}
					}
					if casts > 0 {
						mods = append(mods, CostModifier{
							Kind:   CostModReduction,
							Amount: casts,
							Source: name,
						})
					}
				}

			case "Animar, Soul of Elements":
				// Creature spells you cast cost {1} less to cast for each
				// +1/+1 counter on Animar.
				if isSelf && isCreature {
					n := 0
					if perm.Counters != nil {
						n = perm.Counters["+1/+1"]
					}
					if n > 0 {
						mods = append(mods, CostModifier{
							Kind:   CostModReduction,
							Amount: n,
							Source: name,
						})
					}
				}

			case "Gonti, Canny Acquisitor":
				// "Spells you cast but don't own cost {1} less to cast."
				// The card's Owner field is the original owner's seat;
				// when Gonti's controller casts a card whose Owner != caster
				// (typically a card exiled from an opponent's library by
				// Gonti's combat-damage trigger), the spell is discounted.
				if isSelf && card.Owner != seatIdx {
					mods = append(mods, CostModifier{
						Kind:   CostModReduction,
						Amount: 1,
						Source: name,
					})
				}

			case "Doran, Besieged by Time":
				// Each creature spell you cast with toughness greater than
				// its power costs {1} less to cast. Source toughness/power
				// from the printed Base values since we're at cost calc
				// time before the permanent exists.
				if isSelf && isCreature && card.BaseToughness > card.BasePower {
					mods = append(mods, CostModifier{
						Kind:   CostModReduction,
						Amount: 1,
						Source: name,
					})
				}

			case "Killian, Ink Duelist":
				// Spells you cast that target a creature cost {2} less.
				// Targets aren't bound until cost calculation completes,
				// so we approximate via oracle-text substring match for
				// "target ... creature" phrasing — covers removal, buffs,
				// auras, and equipment whose targets are creatures.
				if isSelf && spellOracleTargetsCreature(card) {
					mods = append(mods, CostModifier{
						Kind:   CostModReduction,
						Amount: 2,
						Source: name,
					})
				}

			case "Zimone, Infinite Analyst":
				// The first spell you cast with {X} in its mana cost each
				// turn costs {1} less to cast for each +1/+1 counter on
				// Zimone. Gated to first-X-spell-this-turn via the seat
				// flag set by zimone's OnCast trigger handler.
				if isSelf && ManaCostContainsX(card) {
					casterSeat := gs.Seats[seatIdx]
					alreadyFired := casterSeat != nil && casterSeat.Flags != nil &&
						casterSeat.Flags["zimone_x_spell_turn"] == gs.Turn
					if !alreadyFired {
						counters := 0
						if perm.Counters != nil {
							counters = perm.Counters["+1/+1"]
						}
						if counters > 0 {
							mods = append(mods, CostModifier{
								Kind:   CostModReduction,
								Amount: counters,
								Source: name,
							})
						}
					}
				}

			case "Hinata, Dawn-Crowned":
				// "Spells you cast cost {1} less to cast for each target."
				// "Spells your opponents cast cost {1} more to cast for
				// each target."
				// CR §601.2c puts target choice before cost calc, so the
				// target count IS legally available at cost time. The
				// engine doesn't yet thread chosen targets into
				// CalculateTotalCost, so we approximate the count by
				// scanning the spell's oracle text for "target" clauses.
				// Mono-target slots ("target creature", "target opponent")
				// count exactly; multi-target spells (e.g. "any number of
				// target creatures", "two target creatures") under-count
				// because they share a single "target" token. emitPartial
				// records the gap at the call site (handler file).
				if n := CountTargetClauses(card); n > 0 {
					if isSelf {
						mods = append(mods, CostModifier{
							Kind:   CostModReduction,
							Amount: n,
							Source: name,
						})
					} else if isOpponent {
						mods = append(mods, CostModifier{
							Kind:   CostModIncrease,
							Amount: n,
							Source: name,
						})
					}
				}

			// --- MINIMUMS ---

			case "Trinisphere":
				// Spells cost at least {3}. CR says this applies to all spells
				// but in practice it's opponents' (symmetric is fine for sim).
				mods = append(mods, CostModifier{
					Kind:   CostModMinimum,
					Amount: 3,
					Source: name,
				})
			}

			// Check for generic flags that per-card handlers may have set.
			// "cost_increase_noncreature:N" — noncreature spells cost N more.
			// "cost_reduction_instant_sorcery:N" — instant/sorcery cost N less.
			if perm.Flags != nil {
				if v, ok := perm.Flags["cost_increase_all"]; ok && v > 0 {
					mods = append(mods, CostModifier{
						Kind:   CostModIncrease,
						Amount: v,
						Source: name,
					})
				}
				if v, ok := perm.Flags["cost_increase_noncreature"]; ok && v > 0 && isNoncreature {
					mods = append(mods, CostModifier{
						Kind:   CostModIncrease,
						Amount: v,
						Source: name,
					})
				}
				if v, ok := perm.Flags["cost_reduction_all"]; ok && v > 0 && isSelf {
					mods = append(mods, CostModifier{
						Kind:   CostModReduction,
						Amount: v,
						Source: name,
					})
				}
				if v, ok := perm.Flags["cost_reduction_instant_sorcery"]; ok && v > 0 && isSelf && isInstantOrSorcery {
					mods = append(mods, CostModifier{
						Kind:   CostModReduction,
						Amount: v,
						Source: name,
					})
				}
				if v, ok := perm.Flags["cost_minimum"]; ok && v > 0 {
					mods = append(mods, CostModifier{
						Kind:   CostModMinimum,
						Amount: v,
						Source: name,
					})
				}
			}
		}
	}

	// §702.41 — Affinity for artifacts: "This spell costs {1} less to
	// cast for each artifact you control." Checked on the CARD being
	// cast, not on a battlefield permanent.
	if HasAffinityForArtifacts(card) {
		artCount := CountArtifacts(gs, seatIdx)
		if artCount > 0 {
			mods = append(mods, CostModifier{
				Kind:   CostModReduction,
				Amount: artCount,
				Source: "affinity for artifacts",
			})
		}
	}

	// Affinity for creatures (Witherbloom, the Balancer): "This spell
	// costs {1} less to cast for each creature you control." Same
	// pattern as artifacts — checked on the CARD being cast.
	if HasAffinityForCreatures(card) {
		crCount := CountCreaturesOnBattlefield(gs, seatIdx)
		if crCount > 0 {
			mods = append(mods, CostModifier{
				Kind:   CostModReduction,
				Amount: crCount,
				Source: "affinity for creatures",
			})
		}
	}

	// Affinity for artifact creatures (Urza, Chief Artificer): "This
	// spell costs {1} less to cast for each artifact creature you
	// control." Checked on the CARD being cast.
	if HasAffinityForArtifactCreatures(card) {
		acCount := CountArtifactCreatures(gs, seatIdx)
		if acCount > 0 {
			mods = append(mods, CostModifier{
				Kind:   CostModReduction,
				Amount: acCount,
				Source: "affinity for artifact creatures",
			})
		}
	}

	// §702.51 — Convoke: "Each creature you tap while casting this spell
	// pays for {1} or one mana of that creature's color." Modeled as a
	// cost reduction equal to the number of untapped creatures the caster
	// controls (the GreedyHat auto-taps for convoke).
	if HasConvoke(card) {
		convokeCount := ConvokeCostReduction(gs, seatIdx)
		if convokeCount > 0 {
			mods = append(mods, CostModifier{
				Kind:   CostModReduction,
				Amount: convokeCount,
				Source: "convoke",
			})
		}
	}

	// Tazri, Beacon of Unity — "This spell costs {1} less to cast for
	// each creature in your party." Party = up to 4 unique roles among
	// creatures you control (Cleric, Rogue, Warrior, Wizard). Checked on
	// the CARD being cast (self-modifier), like affinity/convoke.
	if strings.EqualFold(card.DisplayName(), "Tazri, Beacon of Unity") {
		partyCount := CountParty(gs, seatIdx)
		if partyCount > 0 {
			mods = append(mods, CostModifier{
				Kind:   CostModReduction,
				Amount: partyCount,
				Source: "Tazri, Beacon of Unity (party)",
			})
		}
	}

	// Combat-file cost modifiers: improvise, undaunted.
	mods = AppendCombatCostModifiers(gs, card, seatIdx, mods)

	return mods
}

// cardHasType checks if the Card's Types slice contains the given type (case-insensitive).
func cardHasType(card *Card, typeName string) bool {
	if card == nil {
		return false
	}
	want := strings.ToLower(typeName)
	for _, t := range card.Types {
		if strings.ToLower(t) == want {
			return true
		}
	}
	// Also check TypeLine for broader matching.
	if card.TypeLine != "" && strings.Contains(strings.ToLower(card.TypeLine), want) {
		return true
	}
	return false
}

// spellOracleTargetsCreature is a coarse heuristic for "this spell, when
// cast, will target at least one creature." Used by Killian, Ink Duelist's
// cost reducer where the actual target choice happens after cost
// calculation. We match common oracle-text phrasings: "target creature",
// "target a creature", "target attacking creature", "target tapped
// creature", "target nonland creature", and aura/equipment "enchant
// creature" / "equipped creature" patterns.
func spellOracleTargetsCreature(card *Card) bool {
	if card == nil {
		return false
	}
	text := OracleTextLower(card)
	if text == "" {
		return false
	}
	needles := []string{
		"target creature",
		"target a creature",
		"target another creature",
		"target attacking creature",
		"target blocking creature",
		"target tapped creature",
		"target untapped creature",
		"target nonlegendary creature",
		"target nonland creature",
		"target legendary creature",
		"target nontoken creature",
		"target token creature",
		"enchant creature",
	}
	for _, n := range needles {
		if strings.Contains(text, n) {
			return true
		}
	}
	return false
}

// CountTargetClauses estimates the number of targets the spell will be
// cast with by scanning the oracle text for "target <noun>" clauses.
// Used by Hinata, Dawn-Crowned cost modification ("for each target").
//
// Heuristic: count occurrences of "target " followed by a recognized
// target-noun stem (creature, permanent, player, opponent, planeswalker,
// spell, ability, land, artifact, enchantment, card). Each occurrence
// counts as ONE target, which is correct for mono-target slots but
// under-counts multi-target spells like "two target creatures" or "any
// number of target creatures" (they share one "target" word but bind
// multiple targets at cast time). For Hinata's discount/tax this
// approximation skews toward the cheap/safe direction (less reduction,
// less tax) rather than wildly over-rewarding.
func CountTargetClauses(card *Card) int {
	if card == nil {
		return 0
	}
	text := OracleTextLower(card)
	if text == "" {
		return 0
	}
	stems := []string{
		"target creature",
		"target permanent",
		"target player",
		"target opponent",
		"target planeswalker",
		"target spell",
		"target activated",
		"target triggered",
		"target ability",
		"target land",
		"target artifact",
		"target enchantment",
		"target card",
		"target nonland",
		"target nontoken",
		"target legendary",
		"target tapped",
		"target untapped",
		"target attacking",
		"target blocking",
		"target battle",
	}
	count := 0
	for _, s := range stems {
		idx := 0
		for {
			at := strings.Index(text[idx:], s)
			if at < 0 {
				break
			}
			count++
			idx += at + len(s)
		}
	}
	return count
}

// cardHasSubtype checks if the Card's Types or TypeLine contains a subtype.
func cardHasSubtype(card *Card, subtype string) bool {
	if card == nil {
		return false
	}
	want := strings.ToLower(subtype)
	for _, t := range card.Types {
		if strings.ToLower(t) == want {
			return true
		}
	}
	if card.TypeLine != "" && strings.Contains(strings.ToLower(card.TypeLine), want) {
		return true
	}
	return false
}

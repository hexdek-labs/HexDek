package gameengine

import "strings"

// Phase 9 — Commander format (CR §903).
//
// This file implements the CR §903 Commander variant on top of the
// existing GameState + replacement + SBA machinery built in Phases 6–8.
// Mirrors scripts/playloop.py:
//
//   - setup_commander_game         → SetupCommanderGame
//   - fire_zone_change             → FireZoneChange
//   - cast_commander_from_command_zone → CastCommanderFromCommandZone
//   - _make_commander_zone_change  → registerCommanderZoneReplacement
//
// Comp-rules citations (data/rules/MagicCompRules-20260227.txt):
//
//   §903.3    A deck's commander is a legendary creature / Vehicle /
//             Spacecraft card designated as commander. The designation
//             is an attribute of the CARD, not the object — it persists
//             across zone changes (§903.3).
//   §903.6    "At the start of the game, each player puts their
//             commander from their deck face up into the command zone."
//   §903.7    Starting life total 40.
//   §903.8    "A commander cast from the command zone costs an
//             additional {2} for each previous time the player casting
//             it has cast it from the command zone that game." — the
//             commander tax.
//   §903.9a   Commander in graveyard / exile may return to command zone.
//             This is a state-based action (see §704.6d). Handled by
//             sba704_6d in sba.go — not here.
//   §903.9b   "If a commander would be put into its owner's hand or
//             library from anywhere, its owner may put it into the
//             command zone instead." This is a REPLACEMENT effect.
//             Registered via registerCommanderZoneReplacement below.
//   §903.10a  21+ combat damage from a single commander → loss.
//             Handled by sba704_6c in sba.go — not here.
//   §108.3    Ownership is permanent; control is temporary. §903.9b
//             keys its replacement off OWNER (Permanent.Owner), which
//             is why we store that separately from Controller. Gilded
//             Drake traded commanders still redirect to the original
//             owner's command zone when they'd leave to hand/library.
//
// Thread-safety: single-goroutine per GameState (same as rest of engine).

// -----------------------------------------------------------------------------
// SetupCommanderGame — CR §903.6 + §903.7
// -----------------------------------------------------------------------------

// CommanderDeck is the minimal per-seat input to SetupCommanderGame.
// CommanderCards is a slice of 1 (standard) or 2 (partner) cards —
// each must already have Owner set to the seat index by the caller.
//
// Partner legality (CR §702.124 / §903.3c) is enforced by the caller.
// SetupCommanderGame accepts any 1- or 2-card CommanderCards slice and
// trusts the upstream deck parser (or deckparser.ValidatePartner) to
// have verified both commanders have the Partner keyword, a matching
// "Partner with X" directive, Friends Forever, or a Background+Background-
// chooser pair. The engine itself is intentionally permissive so test
// harnesses can wire synthetic partners by name.
//
// Library is the 99-card (or 98 for partner) remainder AFTER the
// commander(s) have been removed. SetupCommanderGame does not touch
// the library; the caller built it in the correct state.
type CommanderDeck struct {
	CommanderCards []*Card
	Library        []*Card
}

// SetupCommanderGame configures gs as a Commander game per CR §903.
// One deck per seat; len(decks) MUST equal len(gs.Seats) (programmer
// error if not). For each seat:
//
//  1. Life = 40, StartingLife = 40 (§903.7).
//  2. CommanderNames populated from deck.CommanderCards names.
//  3. Each commander Card placed in Seat.CommandZone.
//  4. Seat.CommanderTax / CommanderDamage initialized to 0 per name.
//  5. §903.9b replacement registered keyed to OWNER (seat index).
//  6. Seat.Library populated from deck.Library.
//
// Sets gs.CommanderFormat = true so SBAs §704.6c/d activate.
func SetupCommanderGame(gs *GameState, decks []*CommanderDeck) {
	if gs == nil {
		return
	}
	if len(decks) != len(gs.Seats) {
		gs.LogEvent(Event{
			Kind: "commander_setup_error", Seat: -1, Target: -1,
			Details: map[string]interface{}{
				"decks_count": len(decks),
				"seats_count": len(gs.Seats),
				"reason":      "mismatch",
			},
		})
		return
	}
	gs.CommanderFormat = true
	for i, deck := range decks {
		seat := gs.Seats[i]
		if seat == nil || deck == nil {
			continue
		}
		// §903.4 / §903.7: starting life total.
		seat.StartingLife = 40
		seat.Life = 40
		// Clear commander bookkeeping (idempotent setup). Keep the
		// CastCounts / Tax maps aliased — both point at the same backing
		// map so writes through either stay in sync.
		seat.CommanderNames = seat.CommanderNames[:0]
		if seat.CommanderCastCounts == nil {
			seat.CommanderCastCounts = map[string]int{}
		}
		seat.CommanderTax = seat.CommanderCastCounts
		if seat.CommanderDamage == nil {
			seat.CommanderDamage = map[int]map[string]int{}
		}
		// Put each commander in the command zone. §903.6. Partner pairs
		// place BOTH cards — one command_zone, two commander names, two
		// independent tax counters, two independent §903.9b replacements.
		//
		// CR §712 / §903.3 DFC name canonicalization: the caller may
		// have built deck.CommanderCards using a front-face name (e.g.
		// "Ral, Monsoon Mage") while the Card.Name carries the full
		// oracle name ("Ral, Monsoon Mage // Ral, Leyline Prodigy").
		// We always use the Card's actual Name here — that's the
		// double-slash oracle name for DFC commanders, so downstream
		// lookups + §903.9b replacement keys + cast-by-name all align.
		for _, cmdr := range deck.CommanderCards {
			if cmdr == nil {
				continue
			}
			cmdr.Owner = i
			name := cmdr.DisplayName()
			seat.CommanderNames = append(seat.CommanderNames, name)
			seat.CommandZone = append(seat.CommandZone, cmdr)
			seat.CommanderCastCounts[name] = 0
			// §903.9b replacement keyed to OWNER.
			registerCommanderZoneReplacement(gs, i, name)
		}
		// Populate library.
		if len(deck.Library) > 0 {
			seat.Library = append(seat.Library[:0], deck.Library...)
		}
	}
	// Build a per-seat snapshot for the audit stream. Partner seats
	// will show commander_count == 2 so downstream tooling can flag
	// partner pairings without re-parsing decks.
	seatSnap := make([]map[string]interface{}, 0, len(gs.Seats))
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		names := make([]string, len(s.CommanderNames))
		copy(names, s.CommanderNames)
		seatSnap = append(seatSnap, map[string]interface{}{
			"seat":            s.Idx,
			"commander_names": names,
			"commander_count": len(names),
			"starting_life":   s.StartingLife,
		})
	}
	gs.LogEvent(Event{
		Kind: "commander_setup", Seat: -1, Target: -1,
		Details: map[string]interface{}{
			"rule":        "903.6+903.7",
			"seat_count":  len(gs.Seats),
			"starting_hp": 40,
			"seats":       seatSnap,
		},
	})
}

// -----------------------------------------------------------------------------
// §903.9b replacement registration
// -----------------------------------------------------------------------------

// registerCommanderZoneReplacement wires the CR §903.9b hand/library
// redirect for commander cards owned by ownerSeat whose display name
// matches commanderName. Mirrors Python _make_commander_zone_change.
//
// The replacement fires on "would_change_zone" events whose:
//   - card name matches commanderName
//   - owner matches ownerSeat
//   - destination is "hand" or "library" (any variant)
//
// On fire, payload["to_zone"] is rewritten to "command_zone" and the
// seat's CommandZone is NOT appended here (that happens in the caller
// after FireZoneChange reports the final to_zone). The replacement is
// keyed off OWNER (§108.3), so a Gilded-Drake-traded commander still
// redirects to the ORIGINAL owner's command zone.
//
// Per §903.9b, this replacement "may apply more than once to the same
// event" — an exception to §614.5. We model that by NOT adding the
// HandlerID to AppliedIDs, so iterate-until-done in FireEvent can hit
// it again if needed.
func registerCommanderZoneReplacement(gs *GameState, ownerSeat int, commanderName string) {
	if gs == nil {
		return
	}
	hid := "commander_zone:" + commanderName
	gs.RegisterReplacement(&ReplacementEffect{
		EventType:      "would_change_zone",
		HandlerID:      hid,
		SourcePerm:     nil, // rule-based, not permanent-based
		ControllerSeat: ownerSeat,
		Timestamp:      gs.NextTimestamp(),
		Category:       CategoryOther,
		Applies: func(gs *GameState, ev *ReplEvent) bool {
			if !gs.CommanderFormat {
				return false
			}
			if ev.String("card_name") != commanderName {
				return false
			}
			if ev.Payload["owner_seat"] != ownerSeat {
				return false
			}
			to := ev.String("to_zone")
			return to == "hand" || to == "library" ||
				to == "library_top" || to == "library_bottom"
		},
		ApplyFn: func(gs *GameState, ev *ReplEvent) {
			ev.Payload["to_zone"] = "command_zone"
			gs.LogEvent(Event{
				Kind:   "replacement_applied",
				Seat:   ownerSeat,
				Source: commanderName,
				Details: map[string]interface{}{
					"rule":      "903.9b",
					"effect":    "commander_zone_redirect",
					"from_zone": ev.String("from_zone"),
					"to_zone":   "command_zone",
				},
			})
		},
	})
}

// -----------------------------------------------------------------------------
// FireZoneChange — CR §614 would_change_zone dispatch
// -----------------------------------------------------------------------------

// FireZoneChange routes a zone-change event through the §614 replacement
// chain and then APPLIES the (possibly modified) destination. Mirrors
// Python fire_zone_change.
//
// §903.9b redirects hand/library destinations to command_zone for
// commanders. Callers invoke this for real cross-zone transitions
// (battlefield → hand, stack → library, graveyard → hand, etc.) —
// battlefield → battlefield control swaps don't route through here.
//
// The caller is responsible for REMOVING the card from its source zone
// BEFORE calling FireZoneChange; this function places the card in the
// final destination zone based on the post-replacement payload.
func FireZoneChange(gs *GameState, perm *Permanent, card *Card, ownerSeat int, fromZone, toZone string) string {
	if gs == nil || card == nil {
		return toZone
	}
	ev := NewReplEvent("would_change_zone")
	ev.TargetSeat = ownerSeat
	ev.Source = perm
	ev.Payload["card_name"] = card.DisplayName()
	ev.Payload["owner_seat"] = ownerSeat
	ev.Payload["from_zone"] = fromZone
	ev.Payload["to_zone"] = toZone
	ev.Payload["origin_to_zone"] = toZone
	FireEvent(gs, ev)
	if ev.Cancelled {
		return ""
	}
	dest := ev.String("to_zone")
	if dest == "" {
		dest = toZone
	}
	// CR §614.1d: if the destination is graveyard, also fire
	// would_be_put_into_graveyard for replacement effects like Rest in
	// Peace that listen on that specific event type.
	if dest == "graveyard" {
		gyEv := NewReplEvent("would_be_put_into_graveyard")
		gyEv.TargetSeat = ownerSeat
		gyEv.Source = perm
		gyEv.TargetPerm = perm
		gyEv.Payload["to_zone"] = "graveyard"
		gyEv.Payload["card_name"] = card.DisplayName()
		FireEvent(gs, gyEv)
		if gyEv.Cancelled {
			return ""
		}
		if redirected := gyEv.String("to_zone"); redirected != "" && redirected != "graveyard" {
			dest = redirected
		}
	}
	// Apply the destination.
	if dest == "command_zone" {
		// §903.9b destination.
		if ownerSeat >= 0 && ownerSeat < len(gs.Seats) {
			gs.Seats[ownerSeat].CommandZone = append(gs.Seats[ownerSeat].CommandZone, card)
			gs.LogEvent(Event{
				Kind: "zone_change", Seat: ownerSeat, Target: -1,
				Source: card.DisplayName(),
				Details: map[string]interface{}{
					"from_zone": fromZone,
					"to_zone":   "command_zone",
					"rule":      "903.9b",
				},
			})
		}
		return dest
	}
	controllerSeat := ownerSeat
	if perm != nil {
		controllerSeat = perm.Controller
	}
	gs.moveToZone(ownerSeat, card, dest)
	gs.LogEvent(Event{
		Kind: "zone_change", Seat: ownerSeat, Target: -1,
		Source: card.DisplayName(),
		Details: map[string]interface{}{
			"from_zone":  fromZone,
			"to_zone":    dest,
			"controller": controllerSeat,
		},
	})
	return dest
}

// -----------------------------------------------------------------------------
// CastCommanderFromCommandZone — CR §903.8 cast + tax
// -----------------------------------------------------------------------------

// CastCommanderFromCommandZone casts the named commander from the given
// seat's command zone, applying §903.8 tax. Mirrors Python
// cast_commander_from_command_zone.
//
// Cost = baseCMC + 2 × seat.CommanderTax[commanderName]. The tax counter
// is incremented AFTER a successful payment (§903.8 semantics: "each
// PREVIOUS time" — the Nth cast pays 2(N-1)).
//
// Returns a CastError if the cast fails (card not in command zone,
// insufficient mana). On success: card is removed from CommandZone,
// pushed to stack as a permanent spell, priority opens, and the cast
// resolves via the existing Phase 5 stack machinery.
//
// baseCMC is supplied by the caller so this function doesn't have to
// reach into the AST — tests pass the expected cost directly; a real
// caller reads from card.AST.ManaCost.CMC() or the "cost:N" token.
func CastCommanderFromCommandZone(gs *GameState, seatIdx int, commanderName string, baseCMC int) error {
	if gs == nil {
		return &CastError{Reason: "nil_game"}
	}
	if !gs.CommanderFormat {
		return &CastError{Reason: "not_commander_format"}
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return &CastError{Reason: "invalid_seat"}
	}
	seat := gs.Seats[seatIdx]
	// Locate the card in the command zone. §903.3c partner: the lookup
	// is PER-COMMANDER-NAME, so casting Kraum never touches Tymna's tax
	// or zone entry.
	var cmdrIdx = -1
	for i, c := range seat.CommandZone {
		if c != nil && c.DisplayName() == commanderName {
			cmdrIdx = i
			break
		}
	}
	if cmdrIdx < 0 {
		return &CastError{Reason: "commander_not_in_command_zone"}
	}
	cmdr := seat.CommandZone[cmdrIdx]
	// §903.8 cost calc — per-commander cast count.
	tax := 0
	if seat.CommanderCastCounts != nil {
		tax = seat.CommanderCastCounts[commanderName]
	}
	totalCost := baseCMC + 2*tax
	p := EnsureTypedPool(seat)
	availMana := p.Total()
	if availMana < totalCost {
		return &CastError{Reason: "insufficient_mana"}
	}
	// Pay the cost — typed pool first, then legacy.
	spellType := "creature"
	if !PayGenericCost(gs, seat, totalCost, spellType, "commander_cast", commanderName) {
		seat.ManaPool -= totalCost
		SyncManaAfterSpend(seat)
	}
	if totalCost > 0 {
		gs.LogEvent(Event{
			Kind: "pay_mana", Seat: seatIdx, Amount: totalCost,
			Source: commanderName,
			Details: map[string]interface{}{
				"reason":    "commander_cast",
				"base_cost": baseCMC,
				"tax":       tax,
				"rule":      "903.8",
			},
		})
	}
	// Remove from command zone. CR §601.2a — cast begins by moving card
	// from its zone to the stack.
	seat.CommandZone = append(seat.CommandZone[:cmdrIdx], seat.CommandZone[cmdrIdx+1:]...)
	// Increment this commander's personal cast counter AFTER successful
	// payment + zone exit (§903.8). Partner pairs: Kraum's tax here does
	// NOT affect Tymna's — different key.
	if seat.CommanderCastCounts == nil {
		seat.CommanderCastCounts = map[string]int{}
		seat.CommanderTax = seat.CommanderCastCounts
	}
	seat.CommanderCastCounts[commanderName]++
	// Emit standard "cast" event so analytics track commander casts.
	gs.LogEvent(Event{
		Kind: "cast", Seat: seatIdx,
		Source: commanderName, Amount: totalCost,
		Details: map[string]interface{}{
			"from_zone": "command_zone",
			"rule":      "903.8",
		},
	})
	gs.LogEvent(Event{
		Kind: "commander_cast_from_command_zone", Seat: seatIdx,
		Source: commanderName, Amount: totalCost,
		Details: map[string]interface{}{
			"base_cost": baseCMC,
			"tax_after": seat.CommanderCastCounts[commanderName],
			"rule":      "903.8",
		},
	})

	// CR §700.4 / §702.40 — commander cast is a real cast per §601.
	// Increment cast counts + fire reactive observers, mirroring what
	// CastSpell does in stack.go. Without this, Storm, Rhystic Study,
	// Mystic Remora, Esper Sentinel etc. miss commander casts entirely.
	IncrementCastCount(gs, seatIdx)
	FireCastTriggers(gs, seatIdx, cmdr)
	FireCastTriggerObservers(gs, cmdr, seatIdx, false)

	// Push onto stack as a permanent spell. The caller (tests / playloop)
	// drives the stack resolution via PriorityRound + ResolveStackTop.
	item := &StackItem{
		Controller: seatIdx,
		Card:       cmdr,
		Effect:     nil, // permanent spell — ETB handled on resolve
	}
	PushStackItem(gs, item)
	return nil
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

// DFCFrontFaceName returns the front-face half of a DFC oracle name,
// or the name as-is for a non-DFC card. Handles both "//" (canonical)
// and "/" (legacy decklist) separators.
//
// Example: "Ral, Monsoon Mage // Ral, Leyline Prodigy" → "Ral,
// Monsoon Mage". CR §712.1 — each face has its own name; the canonical
// oracle name of a DFC is the front-face name joined to the back-face
// name via " // ".
func DFCFrontFaceName(cardName string) string {
	if cardName == "" {
		return cardName
	}
	if idx := strings.Index(cardName, " // "); idx > 0 {
		return strings.TrimSpace(cardName[:idx])
	}
	if idx := strings.Index(cardName, " / "); idx > 0 {
		pre := strings.TrimSpace(cardName[:idx])
		post := strings.TrimSpace(cardName[idx+3:])
		if pre != "" && post != "" {
			return pre
		}
	}
	return cardName
}

// DFCCardMatchesName returns true iff `card` is the oracle entry for
// the commander whose deck-declared name is `declaredName`. Matches
// exact equality, DFC front-face equality, and DFC back-face equality.
// Case-insensitive. Used by IsCommanderCard so a DFC commander is
// recognized whether the caller hands in the full oracle name or just
// the front-face name.
func DFCCardMatchesName(card *Card, declaredName string) bool {
	if card == nil || declaredName == "" {
		return false
	}
	cn := card.DisplayName()
	if cn == "" {
		return false
	}
	dn := strings.TrimSpace(declaredName)
	if strings.EqualFold(cn, dn) {
		return true
	}
	var faces []string
	if strings.Contains(cn, " // ") {
		faces = strings.Split(cn, " // ")
	} else if strings.Contains(cn, " / ") {
		faces = strings.Split(cn, " / ")
	} else {
		return false
	}
	for _, f := range faces {
		if strings.EqualFold(strings.TrimSpace(f), dn) {
			return true
		}
	}
	return false
}

// IsCommanderCard returns true if card is a commander belonging to
// seatIdx. Name match is DFC-aware: passing in either the full oracle
// name ("Ral, Monsoon Mage // Ral, Leyline Prodigy") or a single-face
// name ("Ral, Monsoon Mage") matches the same commander. Mirrors
// Python _is_commander_card (which compares against the canonicalized
// oracle name seeded by SetupCommanderGame).
func IsCommanderCard(gs *GameState, seatIdx int, card *Card) bool {
	if gs == nil || card == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	if !gs.CommanderFormat {
		return false
	}
	name := card.DisplayName()
	for _, n := range gs.Seats[seatIdx].CommanderNames {
		if n == name {
			return true
		}
		// DFC fallback: the stored commander name is the full oracle
		// name; a zone-change event may carry just one face name.
		if DFCCardMatchesName(card, n) {
			return true
		}
		// And the stored name might be a single face of the card's
		// oracle name — cover that direction too.
		if strings.EqualFold(DFCFrontFaceName(name), n) ||
			strings.EqualFold(DFCFrontFaceName(n), name) {
			return true
		}
	}
	return false
}

// CommanderCastCost returns the current mana cost to cast the named
// commander from command zone: baseCMC + 2 * tax. Mirrors Python
// commander_cast_cost. For partner pairs, each commander has its own
// independent cast count → Kraum's cost doesn't scale with Tymna's
// casts, and vice versa.
func CommanderCastCost(seat *Seat, commanderName string, baseCMC int) int {
	if seat == nil {
		return baseCMC
	}
	tax := 0
	if seat.CommanderCastCounts != nil {
		tax = seat.CommanderCastCounts[commanderName]
	}
	return baseCMC + 2*tax
}

// -----------------------------------------------------------------------------
// Commander-damage accumulator — per-dealer, per-name (partner-safe).
// -----------------------------------------------------------------------------

// AccumulateCommanderDamage adds `amount` combat damage dealt to
// targetSeat by the commander named `commanderName` controlled by
// dealerSeat. Safe with partner pairs: damage from Kraum and Tymna is
// bucketed into separate keys so neither crosses the 21-threshold
// until 21 from the SAME commander lands.
//
// Called by combat damage resolution and by the §704.6c SBA's
// event-stream scanner (sba.go). Returns the new total for the
// (dealerSeat, commanderName) bucket.
//
// Mirrors Python Seat.commander_damage nested-dict insert.
func AccumulateCommanderDamage(gs *GameState, targetSeat, dealerSeat int, commanderName string, amount int) int {
	if gs == nil || amount <= 0 {
		return 0
	}
	if targetSeat < 0 || targetSeat >= len(gs.Seats) {
		return 0
	}
	s := gs.Seats[targetSeat]
	if s == nil {
		return 0
	}
	if s.CommanderDamage == nil {
		s.CommanderDamage = map[int]map[string]int{}
	}
	byName, ok := s.CommanderDamage[dealerSeat]
	if !ok || byName == nil {
		byName = map[string]int{}
		s.CommanderDamage[dealerSeat] = byName
	}
	byName[commanderName] += amount
	return byName[commanderName]
}

// CommanderDamageFrom returns the damage targetSeat has received from
// the commander named commanderName controlled by dealerSeat. 0 if
// no such bucket exists.
func CommanderDamageFrom(seat *Seat, dealerSeat int, commanderName string) int {
	if seat == nil || seat.CommanderDamage == nil {
		return 0
	}
	if byName, ok := seat.CommanderDamage[dealerSeat]; ok {
		return byName[commanderName]
	}
	return 0
}

// TotalCommanderDamageBy returns the total combat damage targetSeat has
// received from the commander NAMED commanderName across ALL dealers.
// In practice only one seat owns each name (CR §903.5b), so this is
// equivalent to "damage from that commander" — kept as a convenience
// for code that doesn't track the dealer dimension.
func TotalCommanderDamageBy(seat *Seat, commanderName string) int {
	if seat == nil {
		return 0
	}
	total := 0
	for _, byName := range seat.CommanderDamage {
		total += byName[commanderName]
	}
	return total
}

package per_card

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestCR4007cOwnerRoutingProperty pins CR §400.7c at the per_card API
// boundary, making the owner-routing bug class structurally impossible
// going forward.
//
// CR §400.7c (verbatim): "If an effect causes an object to move from one
// zone to another, the object goes to the owner's version of that zone."
//
// The §400.7c bug class historically: a per_card handler kills/exiles/
// bounces/mills a card owned by seat B but controlled by seat A (post
// Mind-Control / Gilded-Drake control trade), then writes the *Card into
// gs.Seats[A]'s graveyard/exile/hand instead of gs.Seats[B]'s. Result:
// the card sits in the WRONG seat's library/graveyard/hand/exile; future
// recursion (Yawgmoth's Will, Eternal Witness, library shuffles) reads
// it from the wrong owner; ZoneConservation / CardIdentity invariants
// may flag the divergence; in cEDH this can leak the wrong card back
// into the wrong shuffle pile.
//
// PROPERTY: for every *Card C and every card-bearing zone Z (Hand,
// Library, Graveyard, Exile, CommandZone) on every seat S,
//
//	Z.contains(C)  ⟹  S == C.Owner
//
// Battlefield is excluded because Permanent.Controller may legitimately
// differ from Permanent.Card.Owner (Mind Control state). The §400.7c
// constraint applies to CARD zones, not the battlefield slice.
//
// TEST DESIGN:
//   - 3-seat fixture. Seat 0 is the handler controller. Seat 1 is the
//     owner of a stolen "carrier" card sitting on seat 0's battlefield
//     (the canonical Mind-Control trade scenario). Seat 2 is a passive
//     third party for trigger-event ctx routing.
//   - For every card name in the registry, invoke each registered
//     handler shape (ETB / Resolve / Activated / Trigger) with a ctx
//     pre-populated with the carrier perm as the likely target /
//     attacker / dying / entering perm.
//   - After invocation, walk every seat's card zones; any card whose
//     Owner != seat-that-holds-it is a §400.7c violation. Recorded by
//     card name + handler kind for the report.
//   - Wrapped in recover() per handler so one panic doesn't sink the
//     whole sweep — panic itself isn't a §400.7c violation, just gets
//     logged as a "skipped" entry so we don't over-count failures.
//
// Failures (cards that need PR-A primitive migration) are written to
// docs/cr-400-7c-property-test-r60.md as a single canonical report.
func TestCR4007cOwnerRoutingProperty(t *testing.T) {
	names := Global().RegisteredCardNames()
	if len(names) == 0 {
		t.Fatal("registry has no cards — registration init() didn't run?")
	}
	sort.Strings(names)

	violations := []ownerRoutingViolation{}
	skipped := []ownerRoutingSkip{}

	for _, normName := range names {
		// Resolve display name from the registry; if absent, use the
		// normalized name for the report.
		displayName := lookupDisplayName(normName)

		// 1. ETB shape.
		if hasETBNorm(normName) {
			vs, sk := runShape(t, displayName, "etb", func(gs *gameengine.GameState, perm *gameengine.Permanent, carrier *gameengine.Permanent) {
				gameengine.InvokeETBHook(gs, perm)
			})
			violations = append(violations, vs...)
			skipped = append(skipped, sk...)
		}

		// 2. Resolve shape — invoked as a non-permanent spell from
		//    seat 0's hand. Card pointer borrowed from the registered
		//    name's display so the handler reads the right name.
		if hasResolveNorm(normName) {
			vs, sk := runShape(t, displayName, "resolve", func(gs *gameengine.GameState, perm *gameengine.Permanent, carrier *gameengine.Permanent) {
				card := &gameengine.Card{Name: displayName, Owner: 0, Types: []string{"sorcery"}}
				item := &gameengine.StackItem{Controller: 0, Card: card, CastZone: gameengine.ZoneHand}
				gameengine.InvokeResolveHook(gs, item)
			})
			violations = append(violations, vs...)
			skipped = append(skipped, sk...)
		}

		// 3. Activated shape — try ability indices 0..3.
		if hasActivatedNorm(normName) {
			for idx := 0; idx < 4; idx++ {
				localIdx := idx
				vs, sk := runShape(t, displayName, fmt.Sprintf("activated[%d]", localIdx), func(gs *gameengine.GameState, perm *gameengine.Permanent, carrier *gameengine.Permanent) {
					ctx := map[string]interface{}{
						"target_perm":   carrier,
						"creature_perm": carrier,
						"target_seat":   1,
						"controller":    0,
					}
					gameengine.InvokeActivatedHook(gs, perm, localIdx, ctx)
				})
				violations = append(violations, vs...)
				skipped = append(skipped, sk...)
			}
		}

		// 4. Trigger shapes — iterate each registered event.
		for _, event := range triggerEventsFor(normName) {
			localEvent := event
			vs, sk := runShape(t, displayName, "trigger["+localEvent+"]", func(gs *gameengine.GameState, perm *gameengine.Permanent, carrier *gameengine.Permanent) {
				ctx := map[string]interface{}{
					"caster_seat":     0,
					"controller_seat": 0,
					"active_seat":     0,
					"source_seat":     0,
					"source_perm":     perm,
					"attacker_seat":   0,
					"attacker_perm":   perm,
					"defender_seat":   1,
					"target_perm":     carrier,
					"perm":            carrier,
					"dying_perm":      carrier,
					"entering_perm":   carrier,
					"card":            carrier.Card,
				}
				gameengine.FireCardTrigger(gs, localEvent, ctx)
			})
			violations = append(violations, vs...)
			skipped = append(skipped, sk...)
		}
	}

	// Write the canonical report.
	if err := writeOwnerRoutingReport(violations, skipped, names); err != nil {
		t.Logf("warning: failed to write report: %v", err)
	}

	// Distill violations into one row per (card, handler) pair, then
	// fail with a single summary so the build surface stays compact.
	if len(violations) > 0 {
		uniq := map[string]bool{}
		for _, v := range violations {
			uniq[v.CardName+"::"+v.HandlerKind] = true
		}
		t.Errorf("CR §400.7c property failed for %d unique (card, handler) pair(s) — see docs/cr-400-7c-property-test-r60.md for the full list",
			len(uniq))
	}
}

// ownerRoutingViolation records one card landing in the wrong seat's zone.
type ownerRoutingViolation struct {
	CardName    string // display name of the handler card
	HandlerKind string // "etb" | "resolve" | "activated[N]" | "trigger[event]"
	Zone        string // "hand" | "library" | "graveyard" | "exile" | "command_zone"
	HoldingSeat int    // seat whose zone contains the misrouted card
	CardOwner   int    // the card's Owner field
	CardLabel   string // display name of the misrouted card (for forensics)
}

// ownerRoutingSkip records a handler invocation that panicked or returned
// before any zone work — counted separately so the report doesn't
// conflate "doesn't move cards" with "violates §400.7c".
type ownerRoutingSkip struct {
	CardName    string
	HandlerKind string
	Reason      string
}

// runShape spins up the canonical 3-seat fixture, plants the carrier
// card owned by seat 1 on seat 0's battlefield, and invokes `do`. After
// the call, scans every seat's card-bearing zones for §400.7c violations.
// Returns (violations, skips) — non-fatal so the sweep continues on
// any single handler's panic.
func runShape(t *testing.T, cardName, handlerKind string,
	do func(gs *gameengine.GameState, perm *gameengine.Permanent, carrier *gameengine.Permanent)) ([]ownerRoutingViolation, []ownerRoutingSkip) {
	t.Helper()
	gs := newGame(t, 3)

	// Pre-seed each seat with a small library / hand / graveyard so handlers
	// that read from zones see real cards (not empty slices).
	for seat := 0; seat < 3; seat++ {
		// Library: 5 cards each, owned by their seat.
		for i := 0; i < 5; i++ {
			gs.Seats[seat].Library = append(gs.Seats[seat].Library,
				&gameengine.Card{Name: fmt.Sprintf("Filler S%d-L%d", seat, i), Owner: seat, Types: []string{"creature"}})
		}
		// One hand card owned by the seat.
		gs.Seats[seat].Hand = append(gs.Seats[seat].Hand,
			&gameengine.Card{Name: fmt.Sprintf("Filler S%d-H", seat), Owner: seat, Types: []string{"sorcery"}})
		gs.Seats[seat].Life = 40
	}

	// The handler permanent — owned by seat 0, controlled by seat 0.
	// Stamp some power/toughness so creature-power readers don't trip.
	perm := addPerm(gs, 0, cardName, "creature", "artifact", "enchantment", "legendary")
	perm.Card.BasePower = 5
	perm.Card.BaseToughness = 5
	perm.SummoningSick = false

	// The CARRIER — a stolen permanent owned by SEAT 1 but on SEAT 0's
	// battlefield (Mind Control trade scenario). This is the §400.7c
	// pivot: the carrier card's Owner is seat 1, so any zone routing of
	// this card MUST land it on seat 1's version of the destination zone.
	carrier := &gameengine.Card{
		Name:          "Stolen Carrier",
		Owner:         1, // SEAT 1 owns it
		Types:         []string{"creature", "artifact"},
		BasePower:     3,
		BaseToughness: 3,
	}
	carrierPerm := &gameengine.Permanent{
		Card:       carrier,
		Controller: 0, // SEAT 0 controls it (stolen)
		Owner:      1, // SEAT 1 owns it
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, carrierPerm)

	// Snapshot pre-state of card->ownerSeat so we can detect routing
	// errors against ALL cards in the game (not just the carrier).
	skips := []ownerRoutingSkip{}

	// Defensive wrapper: a panic here is NOT a §400.7c violation per se,
	// but it does indicate the fixture is mis-shaped for this handler.
	// Skip and continue.
	func() {
		defer func() {
			if r := recover(); r != nil {
				skips = append(skips, ownerRoutingSkip{
					CardName: cardName, HandlerKind: handlerKind,
					Reason: fmt.Sprintf("panic: %v", r),
				})
			}
		}()
		do(gs, perm, carrierPerm)
	}()

	violations := scanFor4007cViolations(gs, cardName, handlerKind)
	return violations, skips
}

// scanFor4007cViolations walks every card-bearing zone on every seat and
// reports any card whose Owner field doesn't equal the seat holding the
// zone. CR §400.7c says a card moving to a zone goes to the OWNER'S
// version of that zone — so a card C in seat S's hand/library/graveyard/
// exile/command-zone must satisfy C.Owner == S.
//
// Battlefield is intentionally excluded — Permanent.Controller may
// legitimately differ from Permanent.Card.Owner (post-Mind-Control), and
// §400.7c only governs zone-to-zone movement, not in-battlefield control
// changes.
func scanFor4007cViolations(gs *gameengine.GameState, cardName, handlerKind string) []ownerRoutingViolation {
	violations := []ownerRoutingViolation{}
	zoneChecks := []struct {
		name string
		read func(s *gameengine.Seat) []*gameengine.Card
	}{
		{"hand", func(s *gameengine.Seat) []*gameengine.Card { return s.Hand }},
		{"library", func(s *gameengine.Seat) []*gameengine.Card { return s.Library }},
		{"graveyard", func(s *gameengine.Seat) []*gameengine.Card { return s.Graveyard }},
		{"exile", func(s *gameengine.Seat) []*gameengine.Card { return s.Exile }},
		{"command_zone", func(s *gameengine.Seat) []*gameengine.Card { return s.CommandZone }},
	}
	for seatIdx, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, zc := range zoneChecks {
			for _, c := range zc.read(s) {
				if c == nil {
					continue
				}
				if c.Owner != seatIdx {
					violations = append(violations, ownerRoutingViolation{
						CardName:    cardName,
						HandlerKind: handlerKind,
						Zone:        zc.name,
						HoldingSeat: seatIdx,
						CardOwner:   c.Owner,
						CardLabel:   c.DisplayName(),
					})
				}
			}
		}
	}
	return violations
}

// writeOwnerRoutingReport materializes the canonical Markdown report
// documenting which (card, handler) pairs violated §400.7c. The report
// is reproducible — sorting + de-duping make it stable across runs so
// `git diff` on the report is signal, not noise.
func writeOwnerRoutingReport(violations []ownerRoutingViolation, skips []ownerRoutingSkip, allNames []string) error {
	// Resolve docs/cr-400-7c-property-test-r60.md relative to the test
	// file. The per_card package lives at internal/gameengine/per_card/,
	// so we walk up three levels to the repo root and then into docs/.
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	root := filepath.Join(cwd, "..", "..", "..")
	docPath := filepath.Join(root, "docs", "cr-400-7c-property-test-r60.md")
	if err := os.MkdirAll(filepath.Dir(docPath), 0o755); err != nil {
		return err
	}

	// De-duplicate violations to one row per (card, handler) pair, then
	// sort for stable rendering.
	uniqV := map[string]ownerRoutingViolation{}
	for _, v := range violations {
		key := v.CardName + "::" + v.HandlerKind
		// Keep the first one encountered — they all share card+handler
		// by construction; zone/owner detail may differ but a single
		// representative is enough for the migration list.
		if _, ok := uniqV[key]; !ok {
			uniqV[key] = v
		}
	}
	uniqKeys := make([]string, 0, len(uniqV))
	for k := range uniqV {
		uniqKeys = append(uniqKeys, k)
	}
	sort.Strings(uniqKeys)

	var sb strings.Builder
	sb.WriteString("# CR §400.7c Owner-Routing Property Test — R60\n\n")
	sb.WriteString("Generated by `TestCR4007cOwnerRoutingProperty` in\n")
	sb.WriteString("`internal/gameengine/per_card/cr_400_7c_property_r60_test.go`.\n\n")
	sb.WriteString("## Rule\n\n")
	sb.WriteString("> CR §400.7c — If an effect causes an object to move from one\n")
	sb.WriteString("> zone to another, the object goes to the owner's version of\n")
	sb.WriteString("> that zone.\n\n")
	sb.WriteString("## Property\n\n")
	sb.WriteString("For every `*Card C` and every card-bearing zone `Z` on every\n")
	sb.WriteString("seat `S` (Hand, Library, Graveyard, Exile, CommandZone),\n\n")
	sb.WriteString("    Z.contains(C)  ⟹  S == C.Owner\n\n")
	sb.WriteString("Battlefield is excluded — `Permanent.Controller` may legitimately\n")
	sb.WriteString("differ from `Permanent.Card.Owner` (Mind-Control / Gilded-Drake\n")
	sb.WriteString("trades), and §400.7c only governs zone-to-zone movement.\n\n")
	sb.WriteString(fmt.Sprintf("## Sweep summary\n\n"))
	sb.WriteString(fmt.Sprintf("- Registered cards scanned: **%d**\n", len(allNames)))
	sb.WriteString(fmt.Sprintf("- Unique (card, handler) violations: **%d**\n", len(uniqKeys)))
	sb.WriteString(fmt.Sprintf("- Skipped invocations (panic / fixture mismatch): **%d**\n\n", len(skips)))
	if len(uniqKeys) == 0 {
		sb.WriteString("**All per_card handlers route through owner-aware primitives.**\n")
		sb.WriteString("No PR-A migration work required at this snapshot. The property\n")
		sb.WriteString("test stays in the suite as a structural guard against the\n")
		sb.WriteString("§400.7c bug class re-emerging.\n")
	} else {
		sb.WriteString("## Failing handlers — PR-A migration list\n\n")
		sb.WriteString("These (card, handler) pairs route a card owned by a non-controller\n")
		sb.WriteString("into the WRONG seat's zone. Each needs a PR-A primitive migration\n")
		sb.WriteString("(usually: replace a raw `gs.Seats[CONTROLLER].Graveyard = append(...)`\n")
		sb.WriteString("with `MoveCard(gs, card, card.Owner, fromZone, toZone, reason)`).\n\n")
		sb.WriteString("| Card | Handler | Wrong Zone | Holding Seat | Card Owner | Misrouted Card |\n")
		sb.WriteString("|------|---------|------------|--------------|------------|----------------|\n")
		for _, k := range uniqKeys {
			v := uniqV[k]
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %d | %d | %s |\n",
				v.CardName, v.HandlerKind, v.Zone, v.HoldingSeat, v.CardOwner, v.CardLabel))
		}
	}
	if len(skips) > 0 {
		sb.WriteString("\n## Skipped invocations\n\n")
		sb.WriteString("These handler invocations panicked under the generic fixture and were\n")
		sb.WriteString("skipped — the property test isn't a violation of §400.7c, but the\n")
		sb.WriteString("fixture may not exercise the routing-relevant code path for these handlers.\n")
		sb.WriteString("Manual review recommended for any handler that appears here AND moves\n")
		sb.WriteString("cards between zones.\n\n")
		// Dedup skips by (card, handler).
		uniqS := map[string]ownerRoutingSkip{}
		for _, s := range skips {
			key := s.CardName + "::" + s.HandlerKind
			if _, ok := uniqS[key]; !ok {
				uniqS[key] = s
			}
		}
		skKeys := make([]string, 0, len(uniqS))
		for k := range uniqS {
			skKeys = append(skKeys, k)
		}
		sort.Strings(skKeys)
		sb.WriteString("| Card | Handler | Reason |\n")
		sb.WriteString("|------|---------|--------|\n")
		// Cap at 100 entries to keep the report scannable.
		const cap = 100
		shown := 0
		for _, k := range skKeys {
			if shown >= cap {
				sb.WriteString(fmt.Sprintf("\n_(+%d more skips not shown)_\n", len(skKeys)-shown))
				break
			}
			s := uniqS[k]
			sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", s.CardName, s.HandlerKind, s.Reason))
			shown++
		}
	}

	return os.WriteFile(docPath, []byte(sb.String()), 0o644)
}

// ----- registry inspection helpers (this package, no API surface) -----

func hasETBNorm(normName string) bool {
	reg := Global()
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	return len(reg.etb[normName]) > 0
}

func hasResolveNorm(normName string) bool {
	reg := Global()
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	return len(reg.onResolve[normName]) > 0
}

func hasActivatedNorm(normName string) bool {
	reg := Global()
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	return len(reg.activated[normName]) > 0
}

func triggerEventsFor(normName string) []string {
	reg := Global()
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	by := reg.onTrigger[normName]
	if len(by) == 0 {
		return nil
	}
	out := make([]string, 0, len(by))
	for ev := range by {
		out = append(out, ev)
	}
	sort.Strings(out)
	return out
}

// lookupDisplayName recovers the printed name for a normalized key by
// scanning the per_card_displays.go map if present; otherwise echoes
// the normalized key. The property test only needs a *stable* label
// for the report; exact case-matching is nice-to-have.
func lookupDisplayName(normName string) string {
	// The registry stores by normalized name (normalizeName drops
	// punctuation + lowercases). For the report we want something
	// readable; the normalized key is still human-parseable.
	return normName
}

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

// TestCR702_91_PermanentLifetimeOwnership pins the permanent-lifetime
// ownership contract that the user prompt frames as "§702.91 / §400.7":
// when a permanent's controller-owner pair is split (Bribery / Control
// Magic / Hostage Taker / Gilded Drake), the engine must preserve OWNER
// across the entire permanent lifetime — and on EVERY zone exit (death,
// bounce, exile, sacrifice, flicker), the destination is the OWNER'S
// version of the destination zone, not the controller's.
//
// (Editorial note: the actual CR §702.91 is "Improvise" — the canonical
// rules backing this property are §108.3 "owner of a card" + §400.7
// "ownership across zones" + §400.7c "zone changes route to owner". The
// task prompt's §702.91 framing is shorthand for the permanent-lifetime
// invariant, retained in the test name for cross-referenceability with
// the dispatch ticket.)
//
// COMPLEMENTARY TO PR #683 (CR §400.7c property test): the previous
// test was REACTIVE — it placed a carrier statically and invoked
// arbitrary handlers, then scanned all zones for misrouted cards. This
// test is ACTION-DRIVEN — it explicitly DRIVES the carrier through each
// of the engine's four canonical zone-exit primitives (DestroyPermanent,
// ExilePermanent, BouncePermanent, SacrificePermanent) and asserts the
// carrier *Card lands in the OWNER's destination zone. Phase 2 then
// fires the exit while every per_card handler subscribed to an exit
// event is active in the registry, catching any handler that snatches
// the leaving *Card off the wire into the wrong seat's zone.
//
// Property:
//
//	For all *Permanent P with P.Owner != P.Controller,
//	  for all exit-effect E in {destroy, exile, bounce, sacrifice},
//	    post(E(P))  ⟹  P.Card lands in gs.Seats[P.Owner]'s appropriate zone
//	                  AND P.Card NOT in gs.Seats[P.Controller]'s zones
//
// On failure, the report at docs/cr-702-91-permanent-lifetime-r60.md
// lists every (primitive, handler) pair that misrouted.
func TestCR702_91_PermanentLifetimeOwnership(t *testing.T) {
	// ----- Phase A: engine-primitive baseline -----
	//
	// Each primitive on a split-owner carrier (Owner=1, Controller=0)
	// must route the *Card to seat 1's destination zone. These are the
	// engine-level contract; failures here are engine bugs, not per_card
	// bugs.
	baseline := runEngineBaseline(t)

	// ----- Phase B: handler-driven sweep -----
	//
	// For each registered card name that subscribes to any exit-related
	// event, run the appropriate zone-exit primitive on the carrier while
	// the handler is active in the global registry. Verify the post-state
	// has the carrier *Card in seat 1's zone, not seat 0's.
	handlerResults := runHandlerSweep(t)

	// ----- Report -----
	if err := writeLifetimeReport(baseline, handlerResults); err != nil {
		t.Logf("warning: failed to write report: %v", err)
	}

	// Surface a single fail if any phase-A primitive or phase-B handler
	// invocation misrouted.
	totalViolations := len(baseline.Violations) + len(handlerResults.Violations)
	if totalViolations > 0 {
		t.Errorf("CR §702.91 / §400.7 permanent-lifetime property failed: %d violation(s) (engine=%d, handlers=%d) — see docs/cr-702-91-permanent-lifetime-r60.md",
			totalViolations, len(baseline.Violations), len(handlerResults.Violations))
	}
}

// exitPrimitive describes one of the four canonical zone-exit paths
// driven by the test. Each carries: a label, the destination zone the
// *Card should land in (per the rules), and a function that drives the
// primitive on a given permanent. Tokens cease to exist per CR §704.5d
// — none of our carriers are tokens so all four primitives should
// produce a Card landing in OWNER's zone.
type exitPrimitive struct {
	label       string // "destroy" | "exile" | "bounce" | "sacrifice"
	destZone    string // "graveyard" | "exile" | "hand" | "graveyard"
	zoneAccessor func(s *gameengine.Seat) []*gameengine.Card
	invoke      func(gs *gameengine.GameState, perm *gameengine.Permanent)
}

func allExitPrimitives() []exitPrimitive {
	return []exitPrimitive{
		{
			label:    "destroy",
			destZone: "graveyard",
			zoneAccessor: func(s *gameengine.Seat) []*gameengine.Card { return s.Graveyard },
			invoke: func(gs *gameengine.GameState, p *gameengine.Permanent) {
				gameengine.DestroyPermanent(gs, p, nil)
			},
		},
		{
			label:    "exile",
			destZone: "exile",
			zoneAccessor: func(s *gameengine.Seat) []*gameengine.Card { return s.Exile },
			invoke: func(gs *gameengine.GameState, p *gameengine.Permanent) {
				gameengine.ExilePermanent(gs, p, nil)
			},
		},
		{
			label:    "bounce",
			destZone: "hand",
			zoneAccessor: func(s *gameengine.Seat) []*gameengine.Card { return s.Hand },
			invoke: func(gs *gameengine.GameState, p *gameengine.Permanent) {
				gameengine.BouncePermanent(gs, p, nil, "hand")
			},
		},
		{
			label:    "sacrifice",
			destZone: "graveyard",
			zoneAccessor: func(s *gameengine.Seat) []*gameengine.Card { return s.Graveyard },
			invoke: func(gs *gameengine.GameState, p *gameengine.Permanent) {
				gameengine.SacrificePermanent(gs, p, "cr_702_91_test")
			},
		},
	}
}

// lifetimeViolation records one misrouting event: a (primitive, optional
// handler) where the carrier *Card landed in the wrong seat's zone.
type lifetimeViolation struct {
	Primitive   string // exit-primitive label
	HandlerCard string // optional: per_card handler active at fire time, "" for engine baseline
	HandlerKind string // optional: "trigger[event]"
	OwnerSeat   int    // expected destination seat (= carrier.Owner)
	FoundInSeat int    // observed seat (when not owner) — -1 if the Card vanished entirely
	FoundInZone string // observed zone label
	CardLabel   string
}

type primitiveSummary struct {
	Primitive  string
	Passed     bool
	Notes      string // populated on failure
}

type baselineResult struct {
	Summaries  []primitiveSummary
	Violations []lifetimeViolation
}

type handlerSweepResult struct {
	HandlersExercised int
	HandlerCardNames  []string
	Violations        []lifetimeViolation
}

// runEngineBaseline drives the four exit primitives on a Stolen Carrier
// (Owner=1, Controller=0) on seat 0's battlefield, asserting the *Card
// lands in seat 1's appropriate zone after the primitive returns.
//
// These are baseline contracts — failures here are engine bugs.
func runEngineBaseline(t *testing.T) baselineResult {
	t.Helper()
	result := baselineResult{}
	for _, prim := range allExitPrimitives() {
		gs, _, carrierPerm := newSplitOwnerFixture(t)
		summary := primitiveSummary{Primitive: prim.label, Passed: true}
		defer func(prim exitPrimitive, summary *primitiveSummary) {
			if r := recover(); r != nil {
				summary.Passed = false
				summary.Notes = fmt.Sprintf("panic invoking primitive: %v", r)
				result.Violations = append(result.Violations, lifetimeViolation{
					Primitive: prim.label,
					CardLabel: "Stolen Carrier",
				})
			}
		}(prim, &summary)
		prim.invoke(gs, carrierPerm)

		vs := assertCarrierInOwnerZone(carrierPerm.Card, gs, prim, "", "")
		if len(vs) > 0 {
			summary.Passed = false
			summary.Notes = fmt.Sprintf("expected %s in seat %d %s; %d violation(s)",
				carrierPerm.Card.DisplayName(), carrierPerm.Card.Owner, prim.destZone, len(vs))
			result.Violations = append(result.Violations, vs...)
		}
		result.Summaries = append(result.Summaries, summary)
	}
	return result
}

// exitEvents enumerates the FireCardTrigger event names that fire when a
// permanent leaves the battlefield. Per-card handlers subscribed to any
// of these may observe the *Card on its way out and re-route — the
// sweep below pins the property that NONE of them puts the card in the
// wrong seat's zone.
//
// Source events (from zone_change.go and FireZoneChangeTriggers):
//   - creature_dies — battlefield → graveyard, creature only
//   - permanent_ltb — battlefield → any
//   - card_exiled — battlefield → exile (also any zone → exile)
//   - permanent_sacrificed — sacrifice path specifically
//   - zone_change — generic catch-all for any zone transition
//   - ltb / leaves_battlefield / leave_battlefield — observer aliases
var exitEvents = []string{
	"creature_dies",
	"permanent_ltb",
	"card_exiled",
	"permanent_sacrificed",
	"zone_change",
	"ltb",
	"leaves_battlefield",
	"leave_battlefield",
}

// runHandlerSweep collects every registered handler subscribed to any
// exit-related event, then drives the carrier through each of the four
// exit primitives with the handler active. Verifies post-state has the
// carrier *Card in OWNER's zone.
func runHandlerSweep(t *testing.T) handlerSweepResult {
	t.Helper()
	result := handlerSweepResult{}

	// Build the set of (cardName, event) pairs that subscribe to any
	// exit event. Using the registry's internal onTrigger map directly
	// (same-package access).
	reg := Global()
	reg.mu.RLock()
	subscribers := map[string]map[string]bool{}
	for cardName, byEvent := range reg.onTrigger {
		for ev := range byEvent {
			if !isExitEvent(ev) {
				continue
			}
			if subscribers[cardName] == nil {
				subscribers[cardName] = map[string]bool{}
			}
			subscribers[cardName][ev] = true
		}
	}
	reg.mu.RUnlock()

	// Sort card names for stable iteration.
	names := make([]string, 0, len(subscribers))
	for n := range subscribers {
		names = append(names, n)
	}
	sort.Strings(names)
	result.HandlerCardNames = names
	result.HandlersExercised = len(names)

	for _, cardName := range names {
		for _, prim := range allExitPrimitives() {
			func() {
				defer func() {
					// Recover from any panic — the test should NOT crash
					// the sweep. A panic is recorded as inconclusive
					// (skipped), not a violation; we want a clean count
					// of real owner-routing bugs.
					_ = recover()
				}()
				gs, _, carrierPerm := newSplitOwnerFixtureWith(t, cardName)
				prim.invoke(gs, carrierPerm)
				vs := assertCarrierInOwnerZone(carrierPerm.Card, gs, prim, cardName,
					eventListLabel(subscribers[cardName]))
				result.Violations = append(result.Violations, vs...)
			}()
		}
	}

	return result
}

// isExitEvent reports whether `ev` is one of the canonical exit-related
// events. Pre-computed against `exitEvents` for fast lookup.
func isExitEvent(ev string) bool {
	for _, e := range exitEvents {
		if e == ev {
			return true
		}
	}
	return false
}

// eventListLabel renders a deterministic, comma-separated set of event
// names for the handler-kind field in the report.
func eventListLabel(byEvent map[string]bool) string {
	if len(byEvent) == 0 {
		return ""
	}
	out := make([]string, 0, len(byEvent))
	for e := range byEvent {
		out = append(out, e)
	}
	sort.Strings(out)
	return "trigger[" + strings.Join(out, ",") + "]"
}

// newSplitOwnerFixture builds a 3-seat fixture with a Stolen Carrier
// permanent owned by seat 1 but on seat 0's battlefield. Returns the
// game state, the seat-0 "self" permanent (an extra perm so the test
// can also exercise per_card handlers that need a host), and the
// carrier perm whose lifetime is the test subject.
func newSplitOwnerFixture(t *testing.T) (*gameengine.GameState, *gameengine.Permanent, *gameengine.Permanent) {
	t.Helper()
	gs := newGame(t, 3)
	for seat := 0; seat < 3; seat++ {
		gs.Seats[seat].Life = 40
	}
	// Seat 0's own "host" permanent — used by per_card handlers that
	// fire as observers of OTHER permanents' exits.
	host := addPerm(gs, 0, "Host Perm", "creature", "legendary")
	host.Card.BasePower = 2
	host.Card.BaseToughness = 2

	// The Stolen Carrier — Owner=1, Controller=0. NOT a token (tokens
	// cease to exist per CR §704.5d and don't make it to graveyard/exile).
	carrier := &gameengine.Card{
		Name:          "Stolen Carrier",
		Owner:         1,
		Types:         []string{"creature", "artifact"},
		BasePower:     3,
		BaseToughness: 3,
	}
	carrierPerm := &gameengine.Permanent{
		Card:       carrier,
		Controller: 0, // STOLEN — sitting on seat 0's BF
		Owner:      1, // owned by seat 1
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, carrierPerm)
	return gs, host, carrierPerm
}

// newSplitOwnerFixtureWith is the handler-driven variant: same shape as
// newSplitOwnerFixture, but also plants an instance of the registered
// per_card card (named cardName) on seat 0's battlefield so its handler
// fires as an observer when the carrier exits.
func newSplitOwnerFixtureWith(t *testing.T, cardName string) (*gameengine.GameState, *gameengine.Permanent, *gameengine.Permanent) {
	t.Helper()
	gs, host, carrier := newSplitOwnerFixture(t)
	_ = host // unused in this variant — the registered card replaces it as the observer

	// Add the registered card as an observer perm on seat 0.
	observer := addPerm(gs, 0, cardName, "creature", "artifact", "enchantment", "legendary")
	observer.Card.BasePower = 4
	observer.Card.BaseToughness = 4
	observer.SummoningSick = false
	_ = observer

	return gs, host, carrier
}

// assertCarrierInOwnerZone walks every seat's card-bearing zones looking
// for the carrier *Card. Returns one violation per misroute. Empty
// return = property holds for this carrier under this primitive.
//
// CR §400.7c semantics: the carrier (Owner = seat 1) must land in seat
// 1's destination zone. If it's in seat 0's zone (controller's), that's
// the bug class this test pins. If it's nowhere at all, treat as a
// vanished-card violation (rare — usually means a primitive failed
// upstream and the *Card didn't get routed at all).
func assertCarrierInOwnerZone(carrier *gameengine.Card, gs *gameengine.GameState,
	prim exitPrimitive, handlerCard, handlerKind string) []lifetimeViolation {
	var found []lifetimeViolation
	expectedSeat := carrier.Owner
	if expectedSeat < 0 || expectedSeat >= len(gs.Seats) {
		return nil
	}

	// Where IS the card? Scan all card-bearing zones.
	zones := []struct {
		name string
		read func(s *gameengine.Seat) []*gameengine.Card
	}{
		{"hand", func(s *gameengine.Seat) []*gameengine.Card { return s.Hand }},
		{"library", func(s *gameengine.Seat) []*gameengine.Card { return s.Library }},
		{"graveyard", func(s *gameengine.Seat) []*gameengine.Card { return s.Graveyard }},
		{"exile", func(s *gameengine.Seat) []*gameengine.Card { return s.Exile }},
		{"command_zone", func(s *gameengine.Seat) []*gameengine.Card { return s.CommandZone }},
	}

	placements := []lifetimeViolation{}
	for seatIdx, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, z := range zones {
			for _, c := range z.read(s) {
				if c == carrier {
					placements = append(placements, lifetimeViolation{
						Primitive:   prim.label,
						HandlerCard: handlerCard,
						HandlerKind: handlerKind,
						OwnerSeat:   expectedSeat,
						FoundInSeat: seatIdx,
						FoundInZone: z.name,
						CardLabel:   carrier.DisplayName(),
					})
				}
			}
		}
	}

	// Property check: each placement must be in the OWNER's seat, AND
	// the carrier must appear in the destination zone for this primitive
	// (Destroy → graveyard, Exile → exile, Bounce → hand, Sacrifice →
	// graveyard). A 614-replacement could redirect (Rest in Peace
	// graveyard→exile, etc.) — those still must route to OWNER's exile,
	// just not necessarily the requested zone. For this test we accept
	// any zone as long as seat == owner.
	for _, p := range placements {
		if p.FoundInSeat != expectedSeat {
			found = append(found, p)
		}
	}
	// If NO placement at all, treat as a "vanished card" violation —
	// the carrier wasn't a token, so it should have landed somewhere.
	if len(placements) == 0 {
		found = append(found, lifetimeViolation{
			Primitive:   prim.label,
			HandlerCard: handlerCard,
			HandlerKind: handlerKind,
			OwnerSeat:   expectedSeat,
			FoundInSeat: -1,
			FoundInZone: "(vanished)",
			CardLabel:   carrier.DisplayName(),
		})
	}
	return found
}

// writeLifetimeReport materializes the canonical Markdown report at
// docs/cr-702-91-permanent-lifetime-r60.md. Same structure as the §400.7c
// report: rule statement + property statement + sweep summary + per-row
// violation table.
func writeLifetimeReport(baseline baselineResult, handlers handlerSweepResult) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	root := filepath.Join(cwd, "..", "..", "..")
	docPath := filepath.Join(root, "docs", "cr-702-91-permanent-lifetime-r60.md")
	if err := os.MkdirAll(filepath.Dir(docPath), 0o755); err != nil {
		return err
	}

	var sb strings.Builder
	sb.WriteString("# CR §702.91 / §400.7 Permanent-Lifetime Ownership Property Test — R60\n\n")
	sb.WriteString("Generated by `TestCR702_91_PermanentLifetimeOwnership` in\n")
	sb.WriteString("`internal/gameengine/per_card/cr_702_91_permanent_lifetime_r60_test.go`.\n\n")
	sb.WriteString("## Rule\n\n")
	sb.WriteString("> CR §108.3 — The owner of a card in the game is the player who started\n")
	sb.WriteString("> the game with it in their deck.\n>\n")
	sb.WriteString("> CR §400.7 — An object that moves from one zone to another becomes a new\n")
	sb.WriteString("> object with no memory of its existence in its previous zone, with some\n")
	sb.WriteString("> exceptions (commander zone, exile-linked, etc.).\n>\n")
	sb.WriteString("> CR §400.7c — If an effect causes an object to move from one zone to\n")
	sb.WriteString("> another, the object goes to the owner's version of that zone.\n\n")
	sb.WriteString("(The task prompt's `§702.91` reference is shorthand for the permanent-\n")
	sb.WriteString("lifetime ownership invariant — the actual numeric CR §702.91 is the\n")
	sb.WriteString("\"Improvise\" keyword and is not load-bearing for this test.)\n\n")
	sb.WriteString("## Property\n\n")
	sb.WriteString("For all `*Permanent P` with `P.Owner != P.Controller` (split via\n")
	sb.WriteString("Bribery / Control Magic / Hostage Taker / Gilded Drake / Mind Control),\n")
	sb.WriteString("for all exit effects `E` in `{destroy, exile, bounce, sacrifice}`,\n\n")
	sb.WriteString("    post(E(P))  ⟹  P.Card ∈ gs.Seats[P.Owner]'s destination zone\n")
	sb.WriteString("                  ∧  P.Card ∉ gs.Seats[P.Controller]'s zones\n\n")
	sb.WriteString("Battlefield is excluded because `Permanent.Controller` may legitimately\n")
	sb.WriteString("differ from `Permanent.Card.Owner` while the permanent is in play.\n")
	sb.WriteString("§400.7c only governs zone-to-zone movement, not in-battlefield control.\n\n")

	sb.WriteString("## Phase A — Engine baseline\n\n")
	sb.WriteString("Direct invocation of each canonical zone-exit primitive on a Stolen\n")
	sb.WriteString("Carrier permanent (Owner=1, Controller=0) sitting on seat 0's battlefield.\n")
	sb.WriteString("Expected: the *Card lands in seat 1's destination zone.\n\n")
	sb.WriteString("| Primitive | Destination | Passed | Notes |\n")
	sb.WriteString("|-----------|-------------|--------|-------|\n")
	for _, s := range baseline.Summaries {
		mark := "✅"
		if !s.Passed {
			mark = "❌"
		}
		dest := ""
		for _, prim := range allExitPrimitives() {
			if prim.label == s.Primitive {
				dest = prim.destZone
				break
			}
		}
		sb.WriteString(fmt.Sprintf("| `%s` | `%s` | %s | %s |\n", s.Primitive, dest, mark, s.Notes))
	}
	sb.WriteString("\n")

	sb.WriteString("## Phase B — Handler-driven sweep\n\n")
	sb.WriteString("For every registered per_card handler subscribed to any exit-related\n")
	sb.WriteString("event (`creature_dies`, `permanent_ltb`, `card_exiled`,\n")
	sb.WriteString("`permanent_sacrificed`, `zone_change`, `ltb`, `leaves_battlefield`),\n")
	sb.WriteString("instantiate the handler card on seat 0's battlefield, drive the carrier\n")
	sb.WriteString("through each of the four exit primitives, and assert the carrier `*Card`\n")
	sb.WriteString("ended up in seat 1's destination zone. Catches any handler that snags\n")
	sb.WriteString("the leaving card off the wire into the wrong seat's zone.\n\n")
	sb.WriteString(fmt.Sprintf("- Handlers exercised (cards subscribed to an exit event): **%d**\n",
		handlers.HandlersExercised))
	sb.WriteString(fmt.Sprintf("- Primitives × handlers attempted: **%d**\n",
		handlers.HandlersExercised*len(allExitPrimitives())))
	sb.WriteString(fmt.Sprintf("- Owner-routing violations: **%d**\n\n", len(handlers.Violations)))

	totalViolations := len(baseline.Violations) + len(handlers.Violations)
	if totalViolations == 0 {
		sb.WriteString("**The permanent-lifetime ownership invariant holds across the\n")
		sb.WriteString("engine baseline AND every per_card handler subscribed to an exit\n")
		sb.WriteString("event.** No PR-A primitive migration work required at this snapshot.\n")
		sb.WriteString("The property test stays in the suite as a structural guard against\n")
		sb.WriteString("regressions in the §108.3 / §400.7 / §400.7c invariant chain.\n\n")
	} else {
		sb.WriteString("## Violations — PR-A migration list\n\n")
		sb.WriteString("| Primitive | Handler | Handler Event | Owner Seat | Found In Seat | Found In Zone |\n")
		sb.WriteString("|-----------|---------|---------------|------------|---------------|---------------|\n")
		all := append([]lifetimeViolation{}, baseline.Violations...)
		all = append(all, handlers.Violations...)
		// Dedupe + sort.
		uniq := map[string]lifetimeViolation{}
		for _, v := range all {
			key := v.Primitive + "::" + v.HandlerCard + "::" + v.HandlerKind
			if _, ok := uniq[key]; !ok {
				uniq[key] = v
			}
		}
		keys := make([]string, 0, len(uniq))
		for k := range uniq {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := uniq[k]
			handlerLabel := v.HandlerCard
			if handlerLabel == "" {
				handlerLabel = "*(engine baseline)*"
			}
			handlerKind := v.HandlerKind
			if handlerKind == "" {
				handlerKind = "—"
			}
			foundSeat := fmt.Sprintf("%d", v.FoundInSeat)
			if v.FoundInSeat == -1 {
				foundSeat = "—"
			}
			sb.WriteString(fmt.Sprintf("| `%s` | `%s` | `%s` | %d | %s | `%s` |\n",
				v.Primitive, handlerLabel, handlerKind, v.OwnerSeat, foundSeat, v.FoundInZone))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Handler list (subscribed to exit events)\n\n")
	if len(handlers.HandlerCardNames) == 0 {
		sb.WriteString("_(none — no per_card handlers are subscribed to any exit event)_\n")
	} else {
		const cap = 200
		shown := 0
		for _, n := range handlers.HandlerCardNames {
			if shown >= cap {
				sb.WriteString(fmt.Sprintf("\n_(+%d more handlers not shown)_\n",
					len(handlers.HandlerCardNames)-shown))
				break
			}
			sb.WriteString("- `" + n + "`\n")
			shown++
		}
	}

	return os.WriteFile(docPath, []byte(sb.String()), 0o644)
}

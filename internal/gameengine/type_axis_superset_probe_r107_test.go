package gameengine

import "testing"

// r107 — (E) TYPE axis probe.
//
// The open item asks whether gs.IsCreatureOf is a strict superset of the
// layer-blind p.IsCreature(), the way the keyword axis was claimed to be in
// r65 — a claim that turned out FALSE in four ways. r65's lesson is that
// "these two agree" from READING is not verification, so this probe asserts
// nothing on first run: it prints both answers for each case and lets the
// measurement name the direction of any divergence.
//
// Positive control included: an ordinary creature must read true on both,
// otherwise a run of all-false proves nothing (r103, lesson 53).

func r107Probe(t *testing.T, gs *GameState, label string, p *Permanent) (blind, layered bool) {
	t.Helper()
	gs.InvalidateCharacteristicsCache()
	blind = p.IsCreature()
	layered = gs.IsCreatureOf(p)
	verdict := "agree"
	if blind != layered {
		verdict = "DIVERGE"
	}
	t.Logf("%-46s  p.IsCreature()=%-5v  gs.IsCreatureOf()=%-5v  %s",
		label, blind, layered, verdict)
	return
}

func TestTypeAxisSupersetProbe_r107(t *testing.T) {
	gs := newFixtureGame(t)

	type row struct {
		label string
		blind bool
		layer bool
	}
	var rows []row
	rec := func(label string, b, l bool) { rows = append(rows, row{label, b, l}) }

	// (0) POSITIVE CONTROL — a plain creature.
	plain := addCreature(gs, 0, "r107 Plain Bear", 2, 2)
	b, l := r107Probe(t, gs, "(0) plain creature [control]", plain)
	rec("(0) plain creature [control]", b, l)
	if !b || !l {
		t.Fatalf("positive control failed: blind=%v layered=%v — the probe is dead, "+
			"every other row below would be meaningless", b, l)
	}

	// (1) IMPENDING — CR §702.176: while time counters remain it is NOT a creature.
	imp := addCreature(gs, 0, "r107 Impending Thing", 3, 3)
	imp.Flags["not_creature_while_impending"] = 1
	b, l = r107Probe(t, gs, "(1) impending  CR 702.176", imp)
	rec("(1) impending  CR 702.176", b, l)

	// (2) BESTOW — CR §702.103a: attached as an Aura, it is NOT a creature.
	bst := addCreature(gs, 0, "r107 Bestowed Spirit", 2, 2)
	bst.Flags["bestowed"] = 1
	b, l = r107Probe(t, gs, "(2) bestowed   CR 702.103a", bst)
	rec("(2) bestowed   CR 702.103a", b, l)

	// (3) RECONFIGURE — CR §702.151b: attached, it is just an Equipment.
	rec3 := addCreature(gs, 0, "r107 Reconfigured Blade", 1, 1)
	rec3.Flags["reconfigured"] = 1
	b, l = r107Probe(t, gs, "(3) reconfigured CR 702.151b", rec3)
	rec("(3) reconfigured CR 702.151b", b, l)

	// (4) TYPE-ADD — an artifact animated by a layer-4 effect (Ensoul Artifact
	//     / Opalescence). The layer-aware reader should see the creature type
	//     the printed card does not have.
	ens := addPerm(gs, 0, "r107 Ensouled Bauble", []string{"artifact"})
	gs.ContinuousEffects = append(gs.ContinuousEffects, &ContinuousEffect{
		Layer:          LayerType,
		Timestamp:      gs.NextTimestamp(),
		SourcePerm:     ens,
		SourceCardName: "r107 Ensoul Artifact",
		Predicate:      func(gs *GameState, target *Permanent) bool { return target == ens },
		ApplyFn: func(gs *GameState, target *Permanent, chars *Characteristics) {
			chars.Types = append(chars.Types, "creature")
		},
		Duration: DurationPermanent,
	})
	b, l = r107Probe(t, gs, "(4) layer-4 type-add (Ensoul)", ens)
	rec("(4) layer-4 type-add (Ensoul)", b, l)

	// ---- summary ----
	var blindOnly, layerOnly int
	for _, r := range rows {
		if r.blind && !r.layer {
			blindOnly++
		}
		if r.layer && !r.blind {
			layerOnly++
		}
	}
	t.Logf("")
	t.Logf("SUMMARY: blind-true-only=%d   layered-true-only=%d", blindOnly, layerOnly)
	switch {
	case blindOnly == 0 && layerOnly > 0:
		// NB: "superset" is NOT by itself a safety property. Before r107 this
		// same line printed with layerOnly=4, three of which were rules-WRONG
		// (§702.103a / §702.151b / §702.176). A superset only licenses
		// migrating call sites if every extra member is CORRECT — read the
		// rows above, not this line.
		t.Logf("=> gs.IsCreatureOf is a superset of p.IsCreature() on these cases; " +
			"check each extra true against its CR rule before trusting it")
	case blindOnly > 0 && layerOnly > 0:
		t.Logf("=> NEITHER is a superset — they diverge in BOTH directions")
	case blindOnly > 0 && layerOnly == 0:
		t.Logf("=> p.IsCreature() is the wider one; the layer reader MISSES cases")
	default:
		t.Logf("=> the two agree on every probed case")
	}
}

// r107 part 2 — does the divergence reach a PRODUCTION entry point?
//
// p.IsCreature()'s own comment for the reconfigure case says the permanent
// "isn't seen by creature SBAs" (CR §702.151b), and the bestow comment makes
// the same claim for §702.103a. But sba704_5f / _5g / _5h all gate on
// gs.IsCreatureOf, which the probe above measured as answering TRUE for both.
//
// This drives StateBasedActions — the real entry point — rather than the
// predicate, per the standing rule.

func r107OnBattlefield(gs *GameState, p *Permanent) bool {
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, q := range s.Battlefield {
			if q == p {
				return true
			}
		}
	}
	return false
}

func TestTypeAxisDivergenceReachesSBA_r107(t *testing.T) {
	cases := []struct {
		flag string
		rule string
		why  string
	}{
		{"bestowed", "702.103a", "attached as an Aura it is an enchantment, not a creature"},
		{"reconfigured", "702.151b", "attached it is just an Equipment, not a creature"},
		{"not_creature_while_impending", "702.176", "while time counters remain it is not a creature"},
	}

	for _, c := range cases {
		t.Run(c.flag, func(t *testing.T) {
			gs := newFixtureGame(t)

			// Control: an ordinary creature at lethal damage MUST die, else
			// the fixture proves nothing about the subject below.
			ctrl := addCreature(gs, 0, "r107 Control Bear", 2, 2)
			ctrl.MarkedDamage = 5

			subj := addCreature(gs, 0, "r107 Subject", 2, 2)
			subj.Flags[c.flag] = 1
			subj.MarkedDamage = 5
			// bestow and reconfigure are only type-suppressing WHILE
			// ATTACHED. An unattached Aura is binned by the §704.5m
			// attachment SBA, which would destroy the subject for a reason
			// that has nothing to do with the type axis — so give it a real
			// host and measure the thing under test.
			if c.flag == "bestowed" || c.flag == "reconfigured" {
				host := addCreature(gs, 0, "r107 Host", 4, 4)
				subj.AttachedTo = host
				// A real bestow/reconfigure permanent carries the attacher
				// type while attached. Without it sba.go:1232 detaches it as
				// "attached but not an aura/equipment", bestow falloff then
				// restores the creature type, and the subject dies for a
				// reason unrelated to the type axis — a fixture that omits a
				// field the real path always sets is not a fixture of that
				// thing (r100).
				if c.flag == "bestowed" {
					subj.Card.Types = append(subj.Card.Types, "enchantment", "aura")
				} else {
					subj.Card.Types = append(subj.Card.Types, "artifact", "equipment")
				}
			}

			// Read the predicates BEFORE the SBA: destroyPermSBA takes the
			// permanent off the battlefield and BestowFalloff clears the
			// flag, so a post-hoc read reports the wrong thing (lesson 29 —
			// an upstream call doing the test's work for it).
			gs.InvalidateCharacteristicsCache()
			blind := subj.IsCreature()
			layered := gs.IsCreatureOf(subj)

			StateBasedActions(gs)

			if r107OnBattlefield(gs, ctrl) {
				t.Fatalf("control creature at lethal damage survived — fixture is dead, "+
					"the subject result below would be meaningless (flag=%s)", c.flag)
			}

			survived := r107OnBattlefield(gs, subj)

			t.Logf("flag=%-30s p.IsCreature()=%-5v gs.IsCreatureOf()=%-5v survived_SBA=%v",
				c.flag, blind, layered, survived)

			if !survived {
				t.Errorf("CR §%s — %s, so §704.5g lethal damage must NOT destroy it, "+
					"but StateBasedActions did. p.IsCreature()=%v says not-a-creature; "+
					"gs.IsCreatureOf()=%v is what sba704_5g actually gates on.",
					c.rule, c.why, blind, layered)
			}
		})
	}
}

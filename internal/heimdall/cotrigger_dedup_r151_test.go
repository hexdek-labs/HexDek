package heimdall

import "testing"

// r151Storm builds the ETB map for one turn in which n copies of the same
// token name entered, which is what a token-storm turn actually looks like.
func r151Storm(turn, n int, name string) map[int][]string {
	cards := make([]string, 0, n)
	for i := 0; i < n; i++ {
		cards = append(cards, name)
	}
	return map[int][]string{turn: cards}
}

// Direction 1 — the explosion is gone. 500 same-name tokens used to emit
// C(500,2) = 124,750 pairs, every one of them ("Saproling","Saproling").
func TestExtractCoTriggersR151_TokenStormEmitsNoSelfPairs(t *testing.T) {
	pairs := ExtractCoTriggers(r151Storm(7, 500, "Saproling"))
	if len(pairs) != 0 {
		t.Fatalf("storm of one repeated name: want 0 pairs, got %d", len(pairs))
	}
	// Guard the property rather than only the count, so a future change that
	// reintroduces self-pairs under a cap still fails here.
	for _, p := range pairs {
		if p.CardA == p.CardB {
			t.Fatalf("self-pair emitted: %q/%q", p.CardA, p.CardB)
		}
	}
}

// The pre-r151 pairing, kept verbatim as the CONTROL so the test states the
// number it is preventing instead of asserting against a bare 0.
func r151RawSameTurnPairCount(cards []string) int {
	n := 0
	for i := 0; i < len(cards); i++ {
		for j := i + 1; j < len(cards); j++ {
			n++
		}
	}
	return n
}

func TestExtractCoTriggersR151_ControlRawPairingDoesExplode(t *testing.T) {
	storm := r151Storm(7, 500, "Saproling")
	raw := r151RawSameTurnPairCount(storm[7])
	if raw != 124750 {
		t.Fatalf("control arithmetic drifted: want 124750, got %d", raw)
	}
	got := len(ExtractCoTriggers(storm))
	if got >= raw {
		t.Fatalf("dedup bought nothing: raw=%d deduped=%d", raw, got)
	}
}

// Direction 2 — real signal is untouched. Distinct names still pair, in both
// the same-turn and the adjacent-turn arm, with the documented scores.
func TestExtractCoTriggersR151_DistinctNamesStillPair(t *testing.T) {
	pairs := ExtractCoTriggers(map[int][]string{
		3: {"Lotus Cobra", "Scute Swarm", "Lotus Cobra"}, // a dup among reals
		4: {"Tireless Provisioner"},
	})

	same, adj := 0, 0
	for _, p := range pairs {
		if p.CardA == p.CardB {
			t.Fatalf("self-pair emitted: %q", p.CardA)
		}
		switch p.TurnWindow {
		case 0:
			same++
			if p.ImpactScore != 1.0 {
				t.Fatalf("same-turn impact: want 1.0, got %v", p.ImpactScore)
			}
		case 1:
			adj++
			if p.ImpactScore != 0.5 {
				t.Fatalf("adjacent-turn impact: want 0.5, got %v", p.ImpactScore)
			}
		}
	}
	// Distinct names on turn 3 are {Lotus Cobra, Scute Swarm} -> C(2,2)=1 pair.
	if same != 1 {
		t.Fatalf("same-turn pairs: want 1, got %d", same)
	}
	// 2 distinct on turn 3 x 1 on turn 4 = 2 adjacent pairs.
	if adj != 2 {
		t.Fatalf("adjacent-turn pairs: want 2, got %d", adj)
	}
}

// distinctNames must preserve first-seen order, or the pair stream stops
// being deterministic across runs and the aggregation drifts.
func TestExtractCoTriggersR151_DedupPreservesFirstSeenOrder(t *testing.T) {
	got := distinctNames([]string{"c", "a", "c", "b", "a"})
	want := []string{"c", "a", "b"}
	if len(got) != len(want) {
		t.Fatalf("want %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("want %v, got %v", want, got)
		}
	}
}

package judge

import (
	"encoding/json"
	"testing"
)

// TestEventWireFormatUnchanged pins the JSON encoding of the promoted
// Event against the exact wire format internal/paritycheck produced
// before the promotion — replay archives and the Python parity bridge
// both speak this format, so it must survive the move byte-for-byte.
func TestEventWireFormatUnchanged(t *testing.T) {
	ev := Event{
		Seq:    7,
		Turn:   3,
		Phase:  "combat",
		Step:   "declare_attackers",
		Seat:   2,
		Kind:   "combat_damage",
		Source: "Edgar Markov",
		Target: 1,
		Amount: 5,
		Rule:   "510.2",
	}
	got, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"seq":7,"turn":3,"phase":"combat","step":"declare_attackers","seat":2,"kind":"combat_damage","source":"Edgar Markov","target":1,"amount":5,"rule":"510.2"}`
	if string(got) != want {
		t.Errorf("Event wire format changed:\n got %s\nwant %s", got, want)
	}

	// omitempty fields must vanish when zero — the parity differ
	// relies on absent-vs-zero equivalence for Source/Target/Amount/Rule.
	minimal, err := json.Marshal(Event{Seq: 1, Turn: 1, Phase: "main", Step: "precombat_main", Seat: 0, Kind: "turn_start"})
	if err != nil {
		t.Fatalf("marshal minimal: %v", err)
	}
	wantMin := `{"seq":1,"turn":1,"phase":"main","step":"precombat_main","seat":0,"kind":"turn_start"}`
	if string(minimal) != wantMin {
		t.Errorf("minimal Event wire format changed:\n got %s\nwant %s", minimal, wantMin)
	}
}

func TestValidationViolationString(t *testing.T) {
	v := ValidationViolation{
		Surface:  SurfaceInvariants,
		Name:     "ZoneConservation",
		Severity: SeverityCritical,
		Message:  "2 real cards disappeared",
	}
	want := "[critical] invariants/ZoneConservation: 2 real cards disappeared"
	if got := v.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestLossReasonString(t *testing.T) {
	cases := []struct {
		in   LossReason
		want string
	}{
		{LossReason{Category: LossCategoryLife, Rule: "704.5a"}, "life_zero_or_less (704.5a)"},
		{LossReason{Category: LossCategoryCommanderDamage, Rule: "704.6c", SourceCard: "Edgar Markov"}, "twenty_one_commander_damage (704.6c) by Edgar Markov"},
		{LossReason{Category: LossCategoryConcession}, "concession"},
	}
	for _, c := range cases {
		if got := c.in.String(); got != c.want {
			t.Errorf("String() = %q, want %q", got, c.want)
		}
	}
}

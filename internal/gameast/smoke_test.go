package gameast

import (
	"strings"
	"testing"
)

// TestConstructLightningBolt builds a Lightning Bolt AST in Go and verifies
// the types compose as expected. This is the simplest real card:
//
//	Lightning Bolt — {R} — Instant
//	Lightning Bolt deals 3 damage to any target.
func TestConstructLightningBolt(t *testing.T) {
	bolt := &CardAST{
		Name: "Lightning Bolt",
		Abilities: []Ability{
			&Activated{
				Cost: Cost{
					Mana: &ManaCost{
						Symbols: []ManaSymbol{{Raw: "{R}", Color: []string{"R"}}},
					},
				},
				Effect: &Damage{
					Amount: NumberOrRef{IsInt: true, Int: 3},
					Target: TargetAny(),
				},
				Raw: "Lightning Bolt deals 3 damage to any target.",
			},
		},
		FullyParsed: true,
	}
	if bolt.Signature() == "" {
		t.Fatal("signature should not be empty")
	}
	if len(bolt.Abilities) != 1 {
		t.Fatalf("expected 1 ability, got %d", len(bolt.Abilities))
	}
	// Verify type-switch navigation works.
	act, ok := bolt.Abilities[0].(*Activated)
	if !ok {
		t.Fatalf("expected *Activated, got %T", bolt.Abilities[0])
	}
	if act.Cost.Mana.CMC() != 1 {
		t.Errorf("expected CMC 1, got %d", act.Cost.Mana.CMC())
	}
	dmg, ok := act.Effect.(*Damage)
	if !ok {
		t.Fatalf("expected *Damage effect, got %T", act.Effect)
	}
	if v, ok := dmg.Amount.IntVal(); !ok || v != 3 {
		t.Errorf("expected Damage amount 3, got %v (ok=%v)", v, ok)
	}
	if dmg.Target.Base != "any_target" {
		t.Errorf("expected any_target, got %q", dmg.Target.Base)
	}
}

// TestConstructGrayMerchant exercises the ScalingAmount + Keyword + Triggered
// path. Gray Merchant of Asphodel:
//
//	When Gray Merchant enters, each opponent loses life equal to your
//	devotion to black. You gain life equal to the life lost this way.
func TestConstructGrayMerchant(t *testing.T) {
	devotion := &ScalingAmount{ScalingKind: "devotion", Args: []interface{}{"B"}}
	lifeLost := &ScalingAmount{ScalingKind: "life_lost_this_way"}

	merchant := &CardAST{
		Name: "Gray Merchant of Asphodel",
		Abilities: []Ability{
			&Triggered{
				Trigger: Trigger{Event: "etb"},
				Effect: &Sequence{
					Items: []Effect{
						&LoseLife{
							Amount: NumberOrRef{IsScaling: true, Scaling: devotion},
							Target: EachOpponent(),
						},
						&GainLife{
							Amount: NumberOrRef{IsScaling: true, Scaling: lifeLost},
							Target: Self(),
						},
					},
				},
			},
		},
		FullyParsed: true,
	}
	sig := merchant.Signature()
	if !strings.Contains(sig, "trig") || !strings.Contains(sig, "etb") {
		t.Errorf("expected 'trig' and 'etb' in signature, got %q", sig)
	}

	// Walk into the ScalingAmount.
	trig := merchant.Abilities[0].(*Triggered)
	seq := trig.Effect.(*Sequence)
	loseLife := seq.Items[0].(*LoseLife)
	scaling, ok := loseLife.Amount.ScalingVal()
	if !ok {
		t.Fatalf("expected ScalingAmount on LoseLife, got %+v", loseLife.Amount)
	}
	if scaling.ScalingKind != "devotion" {
		t.Errorf("expected scaling kind 'devotion', got %q", scaling.ScalingKind)
	}
}

// TestConstructDoublingSeason exercises Static + Modification with a §614
// replacement payload. Doubling Season:
//
//	If an effect would create one or more tokens under your control,
//	it creates twice that many tokens instead.
//	If an effect would put one or more counters on a permanent you control,
//	it puts twice that many of those counters on that permanent instead.
func TestConstructDoublingSeason(t *testing.T) {
	ds := &CardAST{
		Name: "Doubling Season",
		Abilities: []Ability{
			&Static{
				Modification: &Modification{
					ModKind: "replacement",
					Args:    []interface{}{"would_create_token", 2},
				},
			},
			&Static{
				Modification: &Modification{
					ModKind: "replacement",
					Args:    []interface{}{"would_put_counter", 2},
				},
			},
		},
		FullyParsed: true,
	}
	if len(ds.Abilities) != 2 {
		t.Fatalf("expected 2 static abilities, got %d", len(ds.Abilities))
	}
	for i, ab := range ds.Abilities {
		s, ok := ab.(*Static)
		if !ok {
			t.Fatalf("ability %d: expected *Static, got %T", i, ab)
		}
		if s.Modification == nil || s.Modification.ModKind != "replacement" {
			t.Errorf("ability %d: expected replacement modification", i)
		}
	}
}

// TestNumberOrRefAccessors exercises the three branches of NumberOrRef.
func TestNumberOrRefAccessors(t *testing.T) {
	n1 := NumInt(3)
	if v, ok := n1.IntVal(); !ok || v != 3 {
		t.Errorf("NumInt(3): got (%d, %v)", v, ok)
	}
	if _, ok := n1.StrVal(); ok {
		t.Errorf("NumInt(3) should not have a StrVal")
	}

	n2 := NumStr("x")
	if v, ok := n2.StrVal(); !ok || v != "x" {
		t.Errorf("NumStr(\"x\"): got (%q, %v)", v, ok)
	}

	n3 := NumScaling(&ScalingAmount{ScalingKind: "creatures_you_control"})
	sa, ok := n3.ScalingVal()
	if !ok || sa == nil || sa.ScalingKind != "creatures_you_control" {
		t.Errorf("NumScaling: got (%+v, %v)", sa, ok)
	}

	// nil pointer safe.
	var n4 *NumberOrRef
	if _, ok := n4.IntVal(); ok {
		t.Errorf("nil IntVal should return ok=false")
	}
	if _, ok := n4.StrVal(); ok {
		t.Errorf("nil StrVal should return ok=false")
	}
	if _, ok := n4.ScalingVal(); ok {
		t.Errorf("nil ScalingVal should return ok=false")
	}
}

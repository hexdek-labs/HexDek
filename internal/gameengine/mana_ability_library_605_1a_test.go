package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// mkManaAbilityPerm builds a permanent with a single activated ability.
func mkManaAbilityPerm(ab *gameast.Activated) *Permanent {
	return &Permanent{
		Card: &Card{
			Name:  "TestArtifact",
			Types: []string{"artifact"},
			AST:   &gameast.CardAST{Abilities: []gameast.Ability{ab}},
		},
	}
}

// TestIsManaAbility_LibraryMovement605_1a pins the Aug 7 2026 CR change:
// an activated ability whose COST or EFFECT moves a card to or from a
// library is no longer a mana ability (it uses the stack). Millikin is the
// anchor. Pure mana abilities — and scry, which stays within the library —
// must remain mana abilities so they still resolve inline (CR §605.3a).
func TestIsManaAbility_LibraryMovement605_1a(t *testing.T) {
	addMana := func() gameast.Effect {
		return &gameast.AddMana{Pool: []gameast.ManaSymbol{{Color: []string{"C"}}}}
	}

	cases := []struct {
		name string
		ab   *gameast.Activated
		want bool
	}{
		{
			name: "Millikin: {T}, Mill a card: Add {C} — NOT a mana ability (605.1a)",
			ab:   &gameast.Activated{Cost: gameast.Cost{Tap: true, Extra: []string{"mill a card"}}, Effect: addMana()},
			want: false,
		},
		{
			name: "plain mana rock: {T}: Add {C} — still a mana ability",
			ab:   &gameast.Activated{Cost: gameast.Cost{Tap: true}, Effect: addMana()},
			want: true,
		},
		{
			name: "mana + draw compound — NOT a mana ability (effect moves a library card)",
			ab: &gameast.Activated{Cost: gameast.Cost{Tap: true}, Effect: &gameast.Sequence{
				Items: []gameast.Effect{addMana(), &gameast.Draw{}},
			}},
			want: false,
		},
		{
			name: "scry then add mana — STILL a mana ability (scry stays within the library)",
			ab: &gameast.Activated{Cost: gameast.Cost{Tap: true}, Effect: &gameast.Sequence{
				Items: []gameast.Effect{&gameast.Scry{}, addMana()},
			}},
			want: true,
		},
		{
			name: "modal mana with a mill mode — NOT a mana ability",
			ab: &gameast.Activated{Cost: gameast.Cost{Tap: true}, Effect: &gameast.Choice{
				Options: []gameast.Effect{addMana(), &gameast.Mill{}},
			}},
			want: false,
		},
	}

	for _, tc := range cases {
		perm := mkManaAbilityPerm(tc.ab)
		if got := IsManaAbility(perm, 0); got != tc.want {
			t.Errorf("%s: IsManaAbility = %v, want %v", tc.name, got, tc.want)
		}
	}
}

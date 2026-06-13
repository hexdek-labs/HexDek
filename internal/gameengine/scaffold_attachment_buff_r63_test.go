package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r63 scaffold-kind coverage regressions: aura_buff / aura_buff_grant /
// equip_buff_grant. Each pins that a representative card now actually
// produces its P/T (and, for the _grant variants, keyword) effect on the
// creature it is attached to — previously all three were inert (Layer=""
// skipped them in registerASTStaticEffects).

// attachBuffPerm builds an Aura/Equipment permanent carrying a single
// attachment-buff static ability, attaches it to target, and registers
// its continuous effects (mirroring the ETB path).
func attachBuffPerm(gs *GameState, seat int, name, kind string, target *Permanent, args ...interface{}) *Permanent {
	p := addBattlefield(gs, seat, name, 0, 0, "enchantment")
	p.Card.AST = &gameast.CardAST{
		Name: name,
		Abilities: []gameast.Ability{
			&gameast.Static{
				Modification: &gameast.Modification{ModKind: kind, Args: args},
			},
		},
	}
	p.AttachedTo = target
	RegisterContinuousEffectsForPermanent(gs, p)
	return p
}

// enchantedFilter mimics the parser's trailing Filter arg (base
// enchanted_creature / equipped_creature). The handler ignores it.
func enchantedFilter(base string) map[string]interface{} {
	return map[string]interface{}{"__ast_type__": "Filter", "base": base}
}

func TestScaffold_AuraBuff_AppliesPT(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	// Demonic Vigor: enchanted creature gets +1/+1.
	attachBuffPerm(gs, 0, "Demonic Vigor", "aura_buff", bear, 1, 1, enchantedFilter("enchanted_creature"))

	chars := GetEffectiveCharacteristics(gs, bear)
	if chars.Power != 3 || chars.Toughness != 3 {
		t.Fatalf("aura_buff: bear should be 3/3, got %d/%d", chars.Power, chars.Toughness)
	}
}

func TestScaffold_AuraBuff_NegativeDelta(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	// Sensory Deprivation: enchanted creature gets -3/-0.
	attachBuffPerm(gs, 0, "Sensory Deprivation", "aura_buff", bear, -3, 0, enchantedFilter("enchanted_creature"))

	chars := GetEffectiveCharacteristics(gs, bear)
	if chars.Power != -1 || chars.Toughness != 2 {
		t.Fatalf("aura_buff: bear should be -1/2, got %d/%d", chars.Power, chars.Toughness)
	}
}

func TestScaffold_AuraBuffGrant_PTAndKeyword(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	// Dragon Fangs: enchanted creature gets +1/+1 and has trample.
	attachBuffPerm(gs, 0, "Dragon Fangs", "aura_buff_grant", bear, 1, 1, "trample", enchantedFilter("enchanted_creature"))

	chars := GetEffectiveCharacteristics(gs, bear)
	if chars.Power != 3 || chars.Toughness != 3 {
		t.Fatalf("aura_buff_grant: bear should be 3/3, got %d/%d", chars.Power, chars.Toughness)
	}
	if !gs.HasKeywordOf(bear, "trample") {
		t.Fatalf("aura_buff_grant: bear should have trample, keywords=%v", chars.Keywords)
	}
}

func TestScaffold_EquipBuffGrant_PTAndMultiKeyword(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	// Behemoth Sledge: equipped creature gets +2/+2 and has trample and lifelink.
	attachBuffPerm(gs, 0, "Behemoth Sledge", "equip_buff_grant", bear, 2, 2, "trample and lifelink", enchantedFilter("equipped_creature"))

	chars := GetEffectiveCharacteristics(gs, bear)
	if chars.Power != 4 || chars.Toughness != 4 {
		t.Fatalf("equip_buff_grant: bear should be 4/4, got %d/%d", chars.Power, chars.Toughness)
	}
	if !gs.HasKeywordOf(bear, "trample") || !gs.HasKeywordOf(bear, "lifelink") {
		t.Fatalf("equip_buff_grant: bear should have trample+lifelink, keywords=%v", chars.Keywords)
	}
}

func TestScaffold_AttachmentBuff_InlineAbilitySkippedPTStillApplies(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	// Commanding Presence: +2/+2 and 'first strike and "<inline ability>"'.
	// We grant the leading recognized keyword (first strike) and skip the
	// inline quoted ability; P/T must still apply.
	attachBuffPerm(gs, 0, "Commanding Presence", "aura_buff_grant", bear,
		2, 2, `first strike and "whenever this creature deals combat damage to a player, create a 1/1 token."`,
		enchantedFilter("enchanted_creature"))

	chars := GetEffectiveCharacteristics(gs, bear)
	if chars.Power != 4 || chars.Toughness != 4 {
		t.Fatalf("inline-ability aura: bear should be 4/4, got %d/%d", chars.Power, chars.Toughness)
	}
	if !gs.HasKeywordOf(bear, "first strike") {
		t.Fatalf("inline-ability aura: bear should have first strike, keywords=%v", chars.Keywords)
	}
}

func TestScaffold_ParseAttachmentKeywords(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"trample", []string{"trample"}},
		{"trample and lifelink", []string{"trample", "lifelink"}},
		{"protection from white", nil}, // not whitelisted
		{`first strike and "whenever ..."`, []string{"first strike"}},
		{"flying, vigilance and haste", []string{"flying", "vigilance", "haste"}},
	}
	for _, c := range cases {
		got := parseAttachmentKeywords(c.in)
		if len(got) != len(c.want) {
			t.Fatalf("parseAttachmentKeywords(%q) = %v, want %v", c.in, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("parseAttachmentKeywords(%q) = %v, want %v", c.in, got, c.want)
			}
		}
	}
}

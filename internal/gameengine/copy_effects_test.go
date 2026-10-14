package gameengine

// Odin Golden-Game Oracle — copy/clone effects test suite.
//
// Deterministic, state-based golden assertions for CR §707.2 / §613.1a
// copy effects across three interaction modes:
//
//   1. Phantasmal Image copying a tribal lord (Elvish Archdruid) —
//      verifies that the copy inherits the lord's anthem ability and
//      the +1/+1 buff applies to other Elves on the battlefield.
//   2. Sakashima of a Thousand Faces copying a commander — verifies
//      that the copy keeps its own legendary name (Sakashima's special
//      exception, modeled as a layer-1 name override stacked on top of
//      the copy effect) and that commander-tax tracking does NOT bleed
//      across the name boundary.
//   3. Clone targeting a token that is itself a copy of a creature —
//      verifies that the Clone inherits the *underlying* creature's
//      printed values (CR §707.2: a copy uses the source's copiable
//      values, which for a copy-token are the values it was created as
//      a copy of).
//
// No RNG, no shuffling, no turn-loop — these tests exercise the
// CR §613.1a layer-1 path directly via CopyPermanentLayered and the
// effective-characteristics query.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// -----------------------------------------------------------------------------
// Test 1 — Phantasmal Image copies Elvish Archdruid (a tribal lord).
// -----------------------------------------------------------------------------

func TestOdin_PhantasmalImage_CopiesElvishArchdruid_AnthemPropagates(t *testing.T) {
	gs := newFixtureGame(t)

	// Elvish Archdruid: 2/2 Elf Druid with "Other Elves you control get
	// +1/+1." Modeled with the AST-driven other_tribe_anthem ModKind so
	// the tribal predicate (permanentHasSubtype "elf") resolves on every
	// Elf permanent the controller owns.
	archdruidAST := &gameast.CardAST{
		Name: "Elvish Archdruid",
		Abilities: []gameast.Ability{
			&gameast.Static{Modification: &gameast.Modification{
				ModKind: "other_tribe_anthem",
				Args:    []interface{}{1, 1, "elf"},
				Layer:   "7c",
			}},
		},
	}
	archdruid := addBattlefieldWithAST(gs, 0, "Elvish Archdruid", 2, 2,
		archdruidAST, "creature")
	archdruid.Card.TypeLine = "Creature — Elf Druid"
	RegisterContinuousEffectsForPermanent(gs, archdruid)

	// Phantasmal Image enters as a 1/1 Illusion with no printed abilities.
	// CopyPermanentLayered will overwrite its copiable values with the
	// Archdruid's per CR §707.2.
	imageAST := &gameast.CardAST{Name: "Phantasmal Image"}
	image := addBattlefieldWithAST(gs, 0, "Phantasmal Image", 1, 1,
		imageAST, "creature")
	image.Card.TypeLine = "Creature — Illusion"

	// Apply the §707.2 / §613.1a copy. Permanent duration so the AST +
	// Card pointer get re-stamped — that's how the engine surfaces the
	// copied static to registerASTStaticEffects below.
	CopyPermanentLayered(gs, image, archdruid, DurationPermanent)
	RegisterContinuousEffectsForPermanent(gs, image)

	// Sanity: the image's effective name is now "Elvish Archdruid".
	if got := GetEffectiveCharacteristics(gs, image).Name; got != "Elvish Archdruid" {
		t.Fatalf("Phantasmal Image effective name should be Archdruid's after copy; got %q",
			got)
	}
	// And TypeLine carries the "elf" subtype so it qualifies as a buff
	// target for the original Archdruid's anthem.
	if image.Card.TypeLine != "Creature — Elf Druid" {
		t.Fatalf("Phantasmal Image TypeLine should be copied (Elf Druid); got %q",
			image.Card.TypeLine)
	}

	// Drop a vanilla Llanowar Elves on the battlefield. Both the
	// original and the copied lord buff "other Elves you control" —
	// Llanowar gets +2/+2 (one from each lord).
	llanowar := addBattlefield(gs, 0, "Llanowar Elves", 1, 1, "creature")
	llanowar.Card.TypeLine = "Creature — Elf Druid"

	llChars := GetEffectiveCharacteristics(gs, llanowar)
	if llChars.Power != 3 || llChars.Toughness != 3 {
		t.Errorf("Llanowar Elves should be 3/3 (1/1 + two +1/+1 lord buffs); got %d/%d",
			llChars.Power, llChars.Toughness)
	}

	// Original Archdruid is itself an Elf — the copy's anthem buffs
	// "other elves the copy controls", which includes the original.
	// 2/2 + 1/+1 = 3/3.
	adChars := GetEffectiveCharacteristics(gs, archdruid)
	if adChars.Power != 3 || adChars.Toughness != 3 {
		t.Errorf("original Archdruid should be 3/3 (2/2 + copy's anthem); got %d/%d",
			adChars.Power, adChars.Toughness)
	}

	// And by symmetry, the Image (now an Elf Druid 2/2 base) is buffed
	// by the original Archdruid's anthem: 2/2 + 1/+1 = 3/3.
	imgChars := GetEffectiveCharacteristics(gs, image)
	if imgChars.Power != 3 || imgChars.Toughness != 3 {
		t.Errorf("Phantasmal Image (copy of Archdruid) should be 3/3; got %d/%d",
			imgChars.Power, imgChars.Toughness)
	}

	// Opponent's vanilla Elf is not friendly to either lord — must
	// remain at base 1/1. Guards against the predicate accidentally
	// crossing controllers.
	oppElf := addBattlefield(gs, 1, "Opponent Elf", 1, 1, "creature")
	oppElf.Card.TypeLine = "Creature — Elf Warrior"
	oppChars := GetEffectiveCharacteristics(gs, oppElf)
	if oppChars.Power != 1 || oppChars.Toughness != 1 {
		t.Errorf("opponent's Elf must NOT be buffed by your lords; got %d/%d",
			oppChars.Power, oppChars.Toughness)
	}
}

// -----------------------------------------------------------------------------
// Test 2 — Sakashima of a Thousand Faces copies a commander.
// -----------------------------------------------------------------------------
//
// Sakashima's printed exception (CR §707.2 + card text): "...except its
// name is still Sakashima of a Thousand Faces..." — i.e. a layer-1 name
// override applied AFTER the copy effect. The §903.8 commander-tax
// tracker keys on name, so a Sakashima-copy of an opponent's commander
// must not affect that commander's tax counter, and Sakashima itself
// is not a commander.

func TestOdin_Sakashima_CopiesCommander_KeepsNameNoTax(t *testing.T) {
	gs := newCommanderGame(t, 4, "Krenko, Mob Boss", "Atraxa", "Edgar", "Ur-Dragon")

	// Pull Krenko out of the command zone onto seat 0's battlefield
	// (simulates the commander already being cast and resolved). This
	// is the Permanent that Sakashima will be copying from.
	krenkoCard := gs.Seats[0].CommandZone[0]
	gs.Seats[0].CommandZone = gs.Seats[0].CommandZone[:0]
	krenkoCard.TypeLine = "Legendary Creature — Goblin Warrior"
	krenko := &Permanent{
		Card:       krenkoCard,
		Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{},
		Flags:     map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, krenko)

	// Tax baseline: Krenko has been cast once before, so its tax sits
	// at 1 (2 generic cheaper than the post-cast bookkeeping but our
	// concern here is that Sakashima's existence does NOT mutate it).
	gs.Seats[0].CommanderCastCounts["Krenko, Mob Boss"] = 1
	const krenkoTaxBefore = 1
	if gs.Seats[0].CommanderCastCounts["Krenko, Mob Boss"] != krenkoTaxBefore {
		t.Fatalf("test setup: Krenko tax should be %d, got %d",
			krenkoTaxBefore, gs.Seats[0].CommanderCastCounts["Krenko, Mob Boss"])
	}

	// Sakashima the printed: a 3/3 Legendary Human Rogue. We give it an
	// empty AST — no printed lord/anthem — and let CopyPermanentLayered
	// pull in Krenko's copiable values.
	sakashimaAST := &gameast.CardAST{Name: "Sakashima of a Thousand Faces"}
	sakashima := addBattlefieldWithAST(gs, 0, "Sakashima of a Thousand Faces",
		3, 3, sakashimaAST, "creature", "legendary")
	sakashima.Card.TypeLine = "Legendary Creature — Human Rogue"

	// §707.2 copy: Sakashima becomes a copy of Krenko.
	CopyPermanentLayered(gs, sakashima, krenko, DurationPermanent)

	// Sakashima's exception: re-stamp the name back to "Sakashima of a
	// Thousand Faces" via a second layer-1 effect with a later timestamp
	// so it overrides the copy's name field. This is the modeled
	// equivalent of "...except its name is still Sakashima of a Thousand
	// Faces, it's legendary in addition to its other types..."
	const sakashimaName = "Sakashima of a Thousand Faces"
	gs.RegisterContinuousEffect(&ContinuousEffect{
		Layer:          LayerCopy,
		Sublayer:       "a", // applied after the bare layer-1 copy
		Timestamp:      gs.NextTimestamp(),
		SourcePerm:     sakashima,
		SourceCardName: "Sakashima exception",
		ControllerSeat: sakashima.Controller,
		HandlerID:      "sakashima_name_override:" + itoaForCopyTest(sakashima.Timestamp),
		Predicate: func(_ *GameState, p *Permanent) bool {
			return p == sakashima
		},
		ApplyFn: func(_ *GameState, _ *Permanent, chars *Characteristics) {
			chars.Name = sakashimaName
			// "...legendary in addition to its other types..."
			haveLegendary := false
			for _, st := range chars.Supertypes {
				if st == "legendary" {
					haveLegendary = true
					break
				}
			}
			if !haveLegendary {
				chars.Supertypes = append(chars.Supertypes, "legendary")
			}
		},
		Duration: DurationPermanent,
	})
	// CR §707.2 / Sakashima's printed text: the exception applies at
	// layer 1, so the runtime Card.Name (used by IsCommanderCard for
	// ID-by-name) must also reflect the exception. Re-stamp it.
	sakashima.Card.Name = sakashimaName

	// 1) Effective name reads as Sakashima, not Krenko.
	chars := GetEffectiveCharacteristics(gs, sakashima)
	if chars.Name != sakashimaName {
		t.Errorf("Sakashima copy must keep its own name; got %q", chars.Name)
	}
	// 2) The runtime Card.Name (driving DisplayName / IsCommanderCard)
	//    is also Sakashima.
	if sakashima.Card.DisplayName() != sakashimaName {
		t.Errorf("Sakashima Card.DisplayName must be %q; got %q",
			sakashimaName, sakashima.Card.DisplayName())
	}
	// 3) Sakashima is NOT seat 0's commander — IsCommanderCard rejects
	//    it because the name doesn't match seat 0's CommanderNames
	//    (which is ["Krenko, Mob Boss"]).
	if IsCommanderCard(gs, 0, sakashima.Card) {
		t.Errorf("Sakashima copy must not be recognized as a commander")
	}
	// 4) Sakashima still copied Krenko's printed P/T (3/3 Goblin chief
	//    base) — verify a copiable-value contract held while only the
	//    name was excepted. Krenko's printed P/T came from
	//    newCommanderGame's 4/4 Card. That's the value Sakashima
	//    inherits per the exception ("...except its name...").
	if chars.Power != 4 || chars.Toughness != 4 {
		t.Errorf("Sakashima should inherit Krenko's printed 4/4 P/T; got %d/%d",
			chars.Power, chars.Toughness)
	}
	// 5) Krenko's commander tax counter is untouched by the existence
	//    of a Sakashima copy. §903.8 keys on name — different name,
	//    different bucket, no contamination.
	if got := gs.Seats[0].CommanderCastCounts["Krenko, Mob Boss"]; got != krenkoTaxBefore {
		t.Errorf("Krenko commander tax must not change due to Sakashima copy; got %d, want %d",
			got, krenkoTaxBefore)
	}
	// 6) And there is no Sakashima entry in the tax map — the copy is
	//    not a commander, so it has no tax bucket at all.
	if _, ok := gs.Seats[0].CommanderCastCounts[sakashimaName]; ok {
		t.Errorf("Sakashima must NOT have a commander-tax bucket; map=%v",
			gs.Seats[0].CommanderCastCounts)
	}
}

// -----------------------------------------------------------------------------
// Test 3 — Clone targets a token that is itself a copy of a creature.
// -----------------------------------------------------------------------------
//
// CR §707.2: "the copy uses the copiable values of the source." A token
// that was created as a copy of Bear has Bear's copiable values; cloning
// the token must therefore yield a Bear-shape, not a generic-token shape.

func TestOdin_Clone_OfCopyToken_InheritsUnderlyingCreature(t *testing.T) {
	gs := newFixtureGame(t)

	// Underlying creature: Bear, 2/2 vanilla.
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	bear.Card.TypeLine = "Creature — Bear"

	// Token "create a token copy of Grizzly Bears". Phase 1 — the
	// token's printed identity (1/1 generic) is overwritten by the
	// copy effect to inherit Bear's copiable values per §707.2.
	tokenAST := (*gameast.CardAST)(nil) // tokens have no AST.
	_ = tokenAST
	token := addBattlefield(gs, 0, "Bear Token", 1, 1, "creature", "token")
	token.Card.TypeLine = "Token Creature"
	CopyPermanentLayered(gs, token, bear, DurationPermanent)

	// Sanity: the token's effective characteristics reflect Bear.
	tokChars := GetEffectiveCharacteristics(gs, token)
	if tokChars.Name != "Grizzly Bears" {
		t.Fatalf("token-copy of Bear should be named Grizzly Bears; got %q",
			tokChars.Name)
	}
	if tokChars.Power != 2 || tokChars.Toughness != 2 {
		t.Fatalf("token-copy of Bear should be 2/2; got %d/%d",
			tokChars.Power, tokChars.Toughness)
	}

	// Now the Clone: prints as a 0/0 Shapeshifter that ETBs as a copy
	// of any creature. We point CopyPermanentLayered at the token and
	// the engine reads the token's CURRENT BaseCharacteristics — which,
	// because the token has already been re-stamped to Bear's values,
	// returns Bear's printed values. CR §707.2: the Clone gets Bear.
	cloneAST := &gameast.CardAST{Name: "Clone"}
	clone := addBattlefieldWithAST(gs, 0, "Clone", 0, 0, cloneAST, "creature")
	clone.Card.TypeLine = "Creature — Shapeshifter"
	CopyPermanentLayered(gs, clone, token, DurationPermanent)

	cloneChars := GetEffectiveCharacteristics(gs, clone)
	// Name resolves to Bear, not "Bear Token" and not "Clone".
	if cloneChars.Name != "Grizzly Bears" {
		t.Errorf("Clone-of-token should resolve to underlying Bear name; got %q",
			cloneChars.Name)
	}
	// P/T resolves to Bear's printed 2/2.
	if cloneChars.Power != 2 || cloneChars.Toughness != 2 {
		t.Errorf("Clone-of-token should be 2/2 (Bear's printed); got %d/%d",
			cloneChars.Power, cloneChars.Toughness)
	}
	// Type line: a creature, NOT a token. CR §111.10b — token-ness is
	// not a copiable value, so a non-token Clone copying a token does
	// not become a token itself.
	if clone.IsToken() {
		t.Errorf("Clone is a real card; copying a token must not make it a token")
	}
	// And the Clone retains its own permanent identity (different
	// pointer + different timestamp than the token), even though its
	// characteristics now match.
	if clone == token {
		t.Errorf("Clone and token must be distinct Permanents")
	}
	if clone.Timestamp == token.Timestamp {
		t.Errorf("Clone must have a distinct timestamp from the token it copied")
	}
}

// itoaForCopyTest avoids importing strconv here and reuses the engine's
// existing tiny helper. Local re-export keeps the test file self-contained.
func itoaForCopyTest(n int) string { return itoaLayers(n) }

package main

// r63 — nightmare-board census vacuity pin. Pre-r63, runNightmareBoard
// built every Card raw (no MintOGInstanceID), so gs.MintedInstanceIDs
// stayed empty and checkZoneConservation silently fell back to the
// LEGACY count check — the strict InstanceID census (the headline of
// --instanceid-strict-census) never executed on a single nightmare
// board. 40k "clean" boards had measured nothing. The builder now mints
// OG IDs for battlefield cards and library filler; this test pins that
// minting so the census path stays live.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestNightmareBoard_MintsInstanceIDs_CensusLive(t *testing.T) {
	cc := gameengine.NewChaosCorpus([]*gameengine.ChaosCard{
		{Name: "Grizzly Bears", TypeLine: "Creature — Bear", Types: []string{"creature", "bear"}, Power: 2, Toughness: 2, CMC: 2},
		{Name: "Hill Giant", TypeLine: "Creature — Giant", Types: []string{"creature", "giant"}, Power: 3, Toughness: 3, CMC: 4},
		{Name: "Sol Ring", TypeLine: "Artifact", Types: []string{"artifact"}, CMC: 1},
		{Name: "Pacifism", TypeLine: "Enchantment — Aura", Types: []string{"enchantment", "aura"}, CMC: 2},
		{Name: "Crystal Ball", TypeLine: "Artifact", Types: []string{"artifact"}, CMC: 3},
		{Name: "Wind Drake", TypeLine: "Creature — Drake", Types: []string{"creature", "drake"}, Power: 2, Toughness: 2, CMC: 3},
		{Name: "Mind Stone", TypeLine: "Artifact", Types: []string{"artifact"}, CMC: 2},
		{Name: "Glory Seeker", TypeLine: "Creature — Human Soldier", Types: []string{"creature", "human", "soldier"}, Power: 2, Toughness: 2, CMC: 2},
		{Name: "Fountain of Renewal", TypeLine: "Artifact", Types: []string{"artifact"}, CMC: 1},
		{Name: "Serra Angel", TypeLine: "Creature — Angel", Types: []string{"creature", "angel"}, Power: 4, Toughness: 4, CMC: 5},
		{Name: "Runeclaw Bear", TypeLine: "Creature — Bear", Types: []string{"creature", "bear"}, Power: 2, Toughness: 2, CMC: 2},
		{Name: "Canyon Minotaur", TypeLine: "Creature — Minotaur", Types: []string{"creature", "minotaur"}, Power: 3, Toughness: 3, CMC: 4},
		{Name: "Coral Merfolk", TypeLine: "Creature — Merfolk", Types: []string{"creature", "merfolk"}, Power: 2, Toughness: 1, CMC: 2},
		{Name: "Sage of Lat-Nam", TypeLine: "Creature — Human Artificer", Types: []string{"creature", "human", "artificer"}, Power: 1, Toughness: 2, CMC: 2},
		{Name: "Tormod's Crypt", TypeLine: "Artifact", Types: []string{"artifact"}, CMC: 0},
		{Name: "Ornithopter", TypeLine: "Artifact Creature — Thopter", Types: []string{"artifact", "creature", "thopter"}, Power: 0, Toughness: 2, CMC: 0},
		{Name: "Leaden Myr", TypeLine: "Artifact Creature — Myr", Types: []string{"artifact", "creature", "myr"}, Power: 1, Toughness: 1, CMC: 1},
		{Name: "Wall of Wood", TypeLine: "Creature — Wall", Types: []string{"creature", "wall"}, Power: 0, Toughness: 3, CMC: 1},
		{Name: "Alpine Grizzly", TypeLine: "Creature — Bear", Types: []string{"creature", "bear"}, Power: 4, Toughness: 2, CMC: 3},
		{Name: "Armored Warhorse", TypeLine: "Creature — Horse", Types: []string{"creature", "horse"}, Power: 2, Toughness: 3, CMC: 2},
	})

	result := runNightmareBoard(0, cc, nil, minimalMeta(t), 4, 7301)

	if result.Crashed {
		t.Fatalf("fixture nightmare board crashed: %s", result.CrashErr)
	}
	// 4 seats × 5 battlefield perms + 4 × 10 library Plains = 60 OG IDs.
	if result.MintedCount == 0 {
		t.Fatal("nightmare board minted ZERO InstanceIDs — the strict census is vacuous again (checkZoneConservation falls back to the legacy count check)")
	}
	if result.MintedCount < 20 {
		t.Errorf("suspiciously few minted IDs: %d (want at least one per battlefield permanent)", result.MintedCount)
	}
}

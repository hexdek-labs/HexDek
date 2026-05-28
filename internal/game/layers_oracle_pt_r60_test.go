package game

import (
	"context"
	"database/sql"
	"testing"

	hexdekdb "github.com/hexdek/hexdek/internal/db"
)

// R60 — oracle-sourced P/T hydration + DFC face selection + §613 anthem
// layer. The previous r60 sweep (commit 2c594ade) sourced P/T from
// Moxfield deck JSON; this re-architecture pulls from card_oracle
// (canonical CR-compliant base) with the front-face pick from card_faces
// for DFC/MDFC and the controller-scoped anthem pump applied in combat.
//
// Five representative cards plus an anthem-pumped scenario.

// seedOracleRow inserts one card_oracle row with the given P/T (Scryfall
// string format) and optional card_faces JSON. Use either the top-level
// power/toughness (single-faced creatures) OR the cardFacesJSON
// (DFC/MDFC) — the hydration path prefers top-level when both are set.
func seedOracleRow(t *testing.T, ctx context.Context, d *sql.DB, name, power, toughness, cardFacesJSON, oracleText string) {
	t.Helper()
	if _, err := d.ExecContext(ctx, `INSERT OR REPLACE INTO card_oracle
		(name, display_name, scryfall_id, mana_cost, cmc, type_line, oracle_text,
		 image_uri_normal, image_uri_art, set_code, cached_at, legalities, prices,
		 power, toughness, card_faces)
		VALUES (?, ?, '', '', 0, '', ?, '', '', '', 0, '', '', ?, ?, ?)`,
		name, name, oracleText, power, toughness, cardFacesJSON); err != nil {
		t.Fatalf("seed oracle %q: %v", name, err)
	}
}

func TestCreateGameCard_HydratesPTFromOracle(t *testing.T) {
	// Serra Angel (4/4 vanilla single-faced) — top-level power/toughness
	// flows through hydratePTFromOracle directly. No card_faces.
	d, err := hexdekdb.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()
	ctx := context.Background()
	_ = seedPartyForGameTest(t, ctx, d, "party-r60-oracle-serra", "dev-r60-os")

	seedOracleRow(t, ctx, d, "serra angel", "4", "4", "", "Flying, vigilance")

	g, err := CreateGame(ctx, d, "party-r60-oracle-serra", "0000000000000001")
	if err != nil {
		t.Fatalf("create game: %v", err)
	}
	c := &Card{
		GameID: g.ID, InstanceID: "i-serra", Name: "Serra Angel",
		Types: []string{"Creature"}, OwnerSeat: 0, Zone: ZoneBattlefield,
	}
	if err := CreateGameCard(ctx, d, c); err != nil {
		t.Fatalf("create card: %v", err)
	}
	got, _ := GetGameCard(ctx, d, g.ID, "i-serra")
	if got.Power != 4 || got.Toughness != 4 {
		t.Fatalf("Serra Angel hydrated from oracle: want 4/4, got %d/%d", got.Power, got.Toughness)
	}
}

func TestCreateGameCard_HydratesAsymmetricWall(t *testing.T) {
	// Wall of Air — 0/5 asymmetric. The 0-power must round-trip through
	// the "both zero ↔ missing" branch correctly: toughness != 0 means
	// the hydration ran and the read-back doesn't false-positive the
	// 1/1 fallback.
	d, _ := hexdekdb.Open(":memory:")
	defer d.Close()
	ctx := context.Background()
	_ = seedPartyForGameTest(t, ctx, d, "party-r60-wall", "dev-r60-wall")
	seedOracleRow(t, ctx, d, "wall of air", "0", "5", "", "Flying. Defender")

	g, _ := CreateGame(ctx, d, "party-r60-wall", "0000000000000002")
	c := &Card{
		GameID: g.ID, InstanceID: "i-wall", Name: "Wall of Air",
		Types: []string{"Creature"}, OwnerSeat: 0, Zone: ZoneBattlefield,
	}
	if err := CreateGameCard(ctx, d, c); err != nil {
		t.Fatalf("create card: %v", err)
	}
	got, _ := GetGameCard(ctx, d, g.ID, "i-wall")
	// Hydration sets Toughness=5; Power=0 stays 0. creaturePower (the
	// base read) treats only "BOTH zero" as missing, so 0/5 doesn't
	// accidentally fall back to 1/5.
	if got.Power != 0 || got.Toughness != 5 {
		t.Fatalf("Wall of Air hydrated from oracle: want 0/5, got %d/%d", got.Power, got.Toughness)
	}
	if creaturePower(got) != 0 || creatureToughness(got) != 5 {
		t.Fatalf("creaturePower/Toughness on 0/5 wall: want 0/5, got %d/%d",
			creaturePower(got), creatureToughness(got))
	}
}

func TestCreateGameCard_HydratesExtremeFatty(t *testing.T) {
	// Phyrexian Dreadnought — 12/12. Picks up the high end of the
	// printed P/T range; no overflow / no clamp.
	d, _ := hexdekdb.Open(":memory:")
	defer d.Close()
	ctx := context.Background()
	_ = seedPartyForGameTest(t, ctx, d, "party-r60-dread", "dev-r60-dread")
	seedOracleRow(t, ctx, d, "phyrexian dreadnought", "12", "12", "", "Trample")

	g, _ := CreateGame(ctx, d, "party-r60-dread", "0000000000000003")
	c := &Card{
		GameID: g.ID, InstanceID: "i-dread", Name: "Phyrexian Dreadnought",
		Types: []string{"Artifact", "Creature"}, OwnerSeat: 0, Zone: ZoneBattlefield,
	}
	if err := CreateGameCard(ctx, d, c); err != nil {
		t.Fatalf("create card: %v", err)
	}
	got, _ := GetGameCard(ctx, d, g.ID, "i-dread")
	if got.Power != 12 || got.Toughness != 12 {
		t.Fatalf("Phyrexian Dreadnought from oracle: want 12/12, got %d/%d",
			got.Power, got.Toughness)
	}
}

func TestCreateGameCard_NonCreatureSkipsHydration(t *testing.T) {
	// Lightning Bolt — instant. card_oracle row has empty power/toughness.
	// Hydration finds the row but both string-P/T fields are empty so
	// neither Card.Power nor Card.Toughness gets touched. The 1/1
	// fallback in creaturePower then DOESN'T apply because the caller
	// never set a Creature type — but defensively, even if it did,
	// reading P/T on a non-creature returns the fallback, which is
	// harmless since non-creatures never enter the combat path.
	d, _ := hexdekdb.Open(":memory:")
	defer d.Close()
	ctx := context.Background()
	_ = seedPartyForGameTest(t, ctx, d, "party-r60-bolt", "dev-r60-bolt")
	seedOracleRow(t, ctx, d, "lightning bolt", "", "", "", "Lightning Bolt deals 3 damage to any target.")

	g, _ := CreateGame(ctx, d, "party-r60-bolt", "0000000000000004")
	c := &Card{
		GameID: g.ID, InstanceID: "i-bolt", Name: "Lightning Bolt",
		Types: []string{"Instant"}, OwnerSeat: 0, Zone: ZoneHand,
	}
	if err := CreateGameCard(ctx, d, c); err != nil {
		t.Fatalf("create card: %v", err)
	}
	got, _ := GetGameCard(ctx, d, g.ID, "i-bolt")
	if got.Power != 0 || got.Toughness != 0 {
		t.Fatalf("Lightning Bolt should stay 0/0 (non-creature): got %d/%d",
			got.Power, got.Toughness)
	}
}

func TestCreateGameCard_HydratesDFCFrontFace(t *testing.T) {
	// Delver of Secrets // Insectile Aberration — DFC. Scryfall stores
	// the front face (Delver, 1/1) at card_faces[0] and the back face
	// (Insectile Aberration, 3/2) at card_faces[1]. Top-level
	// power/toughness are empty for DFCs. Front-face selection picks
	// the 1/1 — what the card enters as.
	d, _ := hexdekdb.Open(":memory:")
	defer d.Close()
	ctx := context.Background()
	_ = seedPartyForGameTest(t, ctx, d, "party-r60-dfc", "dev-r60-dfc")
	dfcJSON := `[
		{"name":"Delver of Secrets","power":"1","toughness":"1","mana_cost":"{U}","type_line":"Creature — Human Wizard","oracle_text":""},
		{"name":"Insectile Aberration","power":"3","toughness":"2","mana_cost":"","type_line":"Creature — Human Insect","oracle_text":"Flying"}
	]`
	seedOracleRow(t, ctx, d, "delver of secrets // insectile aberration", "", "", dfcJSON, "")

	g, _ := CreateGame(ctx, d, "party-r60-dfc", "0000000000000005")
	c := &Card{
		GameID: g.ID, InstanceID: "i-delver",
		Name: "Delver of Secrets // Insectile Aberration",
		Types: []string{"Creature"}, OwnerSeat: 0, Zone: ZoneBattlefield,
	}
	if err := CreateGameCard(ctx, d, c); err != nil {
		t.Fatalf("create card: %v", err)
	}
	got, _ := GetGameCard(ctx, d, g.ID, "i-delver")
	if got.Power != 1 || got.Toughness != 1 {
		t.Fatalf("Delver DFC front face: want 1/1 (NOT the 3/2 back), got %d/%d",
			got.Power, got.Toughness)
	}
}

// -----------------------------------------------------------------------
// §613 layer: anthem-pumped combat
// -----------------------------------------------------------------------

func TestCombatPower_AnthemBuffsControllerCreatures(t *testing.T) {
	// Glorious Anthem ("Creatures you control get +1/+1") + Serra Angel
	// on the same controller's battlefield → combatPower 5, combatToughness 5.
	// Base (creaturePower) stays 4 — the anthem lives in the layer ctx,
	// not on the Card.
	d, _ := hexdekdb.Open(":memory:")
	defer d.Close()
	ctx := context.Background()
	deckID := seedPartyForGameTest(t, ctx, d, "party-r60-anthem", "dev-r60-anthem")
	seedOracleRow(t, ctx, d, "serra angel", "4", "4", "", "Flying, vigilance")
	seedOracleRow(t, ctx, d, "glorious anthem", "", "", "",
		"Creatures you control get +1/+1.")

	g, _ := CreateGame(ctx, d, "party-r60-anthem", "0000000000000006")
	for i := 0; i < 2; i++ {
		if err := CreateGamePlayer(ctx, d, &Player{
			GameID: g.ID, SeatPosition: i, DeviceID: "dev-r60-anthem",
			DeckID: deckID, Life: 40,
		}); err != nil {
			t.Fatalf("create player %d: %v", i, err)
		}
	}
	// Anthem + Serra both controlled by seat 0; defender on seat 1.
	if err := CreateGameCard(ctx, d, &Card{
		GameID: g.ID, InstanceID: "i-anthem", Name: "Glorious Anthem",
		Types: []string{"Enchantment"}, OwnerSeat: 0, Zone: ZoneBattlefield,
	}); err != nil {
		t.Fatalf("create anthem: %v", err)
	}
	serra := &Card{
		GameID: g.ID, InstanceID: "i-serra-pump", Name: "Serra Angel",
		Types: []string{"Creature"}, OwnerSeat: 0, Zone: ZoneBattlefield,
	}
	if err := CreateGameCard(ctx, d, serra); err != nil {
		t.Fatalf("create serra: %v", err)
	}

	// Direct base read — no anthem.
	got, _ := GetGameCard(ctx, d, g.ID, "i-serra-pump")
	if creaturePower(got) != 4 || creatureToughness(got) != 4 {
		t.Fatalf("base P/T (no anthem applied at this layer): want 4/4, got %d/%d",
			creaturePower(got), creatureToughness(got))
	}

	// Layered read — anthem applies.
	lc, err := BuildLayerContext(ctx, d, g.ID)
	if err != nil {
		t.Fatalf("build layer ctx: %v", err)
	}
	if got := combatPower(got, lc); got != 5 {
		t.Fatalf("Glorious Anthem + Serra: combatPower want 5, got %d", got)
	}
	if got := combatToughness(serra, lc); got != 5 {
		t.Fatalf("Glorious Anthem + Serra: combatToughness want 5, got %d", got)
	}
}

func TestCombatPower_AnthemDoesNotBuffOpponentCreatures(t *testing.T) {
	// Seat 0 has Glorious Anthem. Seat 1 has Serra Angel. The anthem is
	// controller-scoped — seat 1's Serra is NOT pumped.
	d, _ := hexdekdb.Open(":memory:")
	defer d.Close()
	ctx := context.Background()
	deckID := seedPartyForGameTest(t, ctx, d, "party-r60-anthem-opp", "dev-r60-anthem-opp")
	seedOracleRow(t, ctx, d, "serra angel", "4", "4", "", "Flying, vigilance")
	seedOracleRow(t, ctx, d, "glorious anthem", "", "", "",
		"Creatures you control get +1/+1.")

	g, _ := CreateGame(ctx, d, "party-r60-anthem-opp", "0000000000000007")
	for i := 0; i < 2; i++ {
		_ = CreateGamePlayer(ctx, d, &Player{
			GameID: g.ID, SeatPosition: i, DeviceID: "dev-r60-anthem-opp",
			DeckID: deckID, Life: 40,
		})
	}
	_ = CreateGameCard(ctx, d, &Card{
		GameID: g.ID, InstanceID: "i-anthem", Name: "Glorious Anthem",
		Types: []string{"Enchantment"}, OwnerSeat: 0, Zone: ZoneBattlefield,
	})
	oppSerra := &Card{
		GameID: g.ID, InstanceID: "i-opp-serra", Name: "Serra Angel",
		Types: []string{"Creature"}, OwnerSeat: 1, Zone: ZoneBattlefield,
	}
	_ = CreateGameCard(ctx, d, oppSerra)

	lc, _ := BuildLayerContext(ctx, d, g.ID)
	if got := combatPower(oppSerra, lc); got != 4 {
		t.Fatalf("opponent's Serra under MY anthem: want unchanged 4, got %d", got)
	}
}

func TestAnthemRegex_ParsesCanonicalShapes(t *testing.T) {
	cases := []struct {
		name      string
		text      string
		wantP     int
		wantT     int
	}{
		{"glorious anthem", "Creatures you control get +1/+1.", 1, 1},
		{"crusade-style", "creatures you control get +2/+1", 2, 1},
		{"other creatures (Honor of the Pure pattern)", "Other creatures you control get +1/+1.", 1, 1},
		{"no anthem", "Flying", 0, 0},
		{"global pump (not matched)", "Creatures get +1/+1", 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dp, dt := parseAnthemFromText(tc.text)
			if dp != tc.wantP || dt != tc.wantT {
				t.Fatalf("parseAnthemFromText(%q): want +%d/+%d, got +%d/+%d",
					tc.text, tc.wantP, tc.wantT, dp, dt)
			}
		})
	}
}

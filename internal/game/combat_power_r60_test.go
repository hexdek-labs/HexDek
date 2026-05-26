package game

import (
	"context"
	"database/sql"
	"testing"

	hexdekdb "github.com/hexdek/hexdek/internal/db"
)

// seedPartyForGameTest inserts the device + party + deck rows the game
// FK chain expects. Returns the deck id the caller uses on each Player
// row (game_player has NOT NULL FKs on device + deck).
func seedPartyForGameTest(t *testing.T, ctx context.Context, d *sql.DB, partyID, deviceID string) string {
	t.Helper()
	if _, err := d.ExecContext(ctx,
		`INSERT INTO device (id, display_name, created_at, last_seen_at) VALUES (?, ?, 0, 0)`,
		deviceID, deviceID); err != nil {
		t.Fatalf("seed device: %v", err)
	}
	if _, err := d.ExecContext(ctx,
		`INSERT INTO party (id, host_device_id, state, max_players, created_at) VALUES (?, ?, 'lobby', 4, 0)`,
		partyID, deviceID); err != nil {
		t.Fatalf("seed party: %v", err)
	}
	deckID := "deck-" + partyID
	if _, err := d.ExecContext(ctx,
		`INSERT INTO deck (id, owner_device_id, name, commander_name, format, imported_at, raw_json)
		 VALUES (?, ?, 'r60 pt test deck', 'Test Commander', 'commander', 0, '{}')`,
		deckID, deviceID); err != nil {
		t.Fatalf("seed deck: %v", err)
	}
	return deckID
}

// R60 — wires Card.Power / Card.Toughness through the live-game MVP so
// combat damage no longer treats every creature as a 1/1. Tests pin the
// three corners of the new contract:
//   - explicit P/T → read directly
//   - missing P/T (legacy deck JSON without the fields) → 1/1 fallback
//   - round-trip through marshal/hydrate preserves P/T
// plus a black-box ResolveCombat assertion that PlayerDamage tallies the
// real attacker.Power, not the pre-r60 hardcoded 1.
//
// See docs/half-finished-features-r48.md #5 for the original deferred
// migration.

// -----------------------------------------------------------------------
// creaturePower / creatureToughness — direct unit tests
// -----------------------------------------------------------------------

func TestCreaturePower_ExplicitFields(t *testing.T) {
	c := &Card{Name: "Craterhoof Behemoth", Power: 5, Toughness: 5}
	if got := creaturePower(c); got != 5 {
		t.Fatalf("Power: want 5, got %d", got)
	}
	if got := creatureToughness(c); got != 5 {
		t.Fatalf("Toughness: want 5, got %d", got)
	}
}

func TestCreaturePower_AsymmetricFields(t *testing.T) {
	// Tarmogoyf-ish: power != toughness.
	c := &Card{Name: "Big Boi", Power: 4, Toughness: 5}
	if creaturePower(c) != 4 || creatureToughness(c) != 5 {
		t.Fatalf("asymmetric P/T: got %d/%d, want 4/5",
			creaturePower(c), creatureToughness(c))
	}
}

func TestCreaturePower_MissingFallsBackTo1Slash1(t *testing.T) {
	// Legacy deck JSON without power/toughness keys → both unmarshal to 0
	// → fallback kicks in.
	c := &Card{Name: "Vanilla Goblin"}
	if got := creaturePower(c); got != 1 {
		t.Fatalf("missing-P fallback: want 1, got %d", got)
	}
	if got := creatureToughness(c); got != 1 {
		t.Fatalf("missing-T fallback: want 1, got %d", got)
	}
}

func TestCreaturePower_NilCardReturnsZero(t *testing.T) {
	// Defensive guard — nil should not panic.
	if creaturePower(nil) != 0 {
		t.Fatalf("nil card power: want 0")
	}
	if creatureToughness(nil) != 0 {
		t.Fatalf("nil card toughness: want 0")
	}
}

func TestCreaturePower_OnlyPowerSet(t *testing.T) {
	// One side zero, one side non-zero → NOT the fallback case. Real
	// 4/0 creatures (Mortivore? Phantasmal Bear when toughness=0) and
	// 0/X (Wall of Omens, Spellskite-as-printed before P boost) need
	// distinct reads. The fallback only triggers when BOTH are zero.
	c := &Card{Name: "Phyrexian Dreadnought", Power: 12, Toughness: 12}
	if creaturePower(c) != 12 || creatureToughness(c) != 12 {
		t.Fatalf("got %d/%d", creaturePower(c), creatureToughness(c))
	}
	// 0/X — real but unusual.
	wall := &Card{Name: "Wall of Air", Power: 0, Toughness: 5}
	if creaturePower(wall) != 0 || creatureToughness(wall) != 5 {
		t.Fatalf("0/5 wall: got %d/%d", creaturePower(wall), creatureToughness(wall))
	}
}

// -----------------------------------------------------------------------
// marshal/hydrate round-trip preserves P/T
// -----------------------------------------------------------------------

func TestMarshalHydrateCardData_PreservesPowerToughness(t *testing.T) {
	orig := &Card{
		Name:      "Lightning Angel",
		ManaCost:  "{1}{U}{R}{W}",
		CMC:       4,
		Power:     3,
		Toughness: 4,
		Types:     []string{"Creature"},
		Subtypes:  []string{"Angel"},
	}
	raw := marshalCardData(orig)
	if raw == "" {
		t.Fatal("marshalCardData returned empty")
	}

	rebuilt := &Card{Name: orig.Name}
	hydrateCardData(rebuilt, raw)

	if rebuilt.Power != 3 {
		t.Errorf("Power round-trip: want 3, got %d (raw=%q)", rebuilt.Power, raw)
	}
	if rebuilt.Toughness != 4 {
		t.Errorf("Toughness round-trip: want 4, got %d (raw=%q)", rebuilt.Toughness, raw)
	}
	if rebuilt.CMC != 4 || rebuilt.ManaCost != "{1}{U}{R}{W}" {
		t.Errorf("non-P/T fields drifted: cmc=%d cost=%q", rebuilt.CMC, rebuilt.ManaCost)
	}
}

func TestMarshalHydrateCardData_MissingPowerStaysZero(t *testing.T) {
	// A deck-JSON entry that legitimately omits power/toughness should
	// round-trip with zero values — the 1/1 fallback only applies in
	// creaturePower / creatureToughness, not at the storage layer.
	orig := &Card{Name: "Generic Card", ManaCost: "{2}", CMC: 2, Types: []string{"Artifact"}}
	raw := marshalCardData(orig)

	rebuilt := &Card{Name: orig.Name}
	hydrateCardData(rebuilt, raw)

	if rebuilt.Power != 0 || rebuilt.Toughness != 0 {
		t.Errorf("missing P/T should hydrate as 0/0, got %d/%d",
			rebuilt.Power, rebuilt.Toughness)
	}
}

// -----------------------------------------------------------------------
// End-to-end: CreateGameCard → GetGameCard preserves P/T
// -----------------------------------------------------------------------

func TestCreateGetGameCard_PreservesPowerToughness(t *testing.T) {
	d, err := hexdekdb.Open(":memory:")
	if err != nil {
		t.Fatalf("open mem db: %v", err)
	}
	defer d.Close()

	ctx := context.Background()
	_ = seedPartyForGameTest(t, ctx, d, "party-r60-pt-test", "dev-r60-pt")

	g, err := CreateGame(ctx, d, "party-r60-pt-test", "deadbeefcafebabe")
	if err != nil {
		t.Fatalf("create game: %v", err)
	}

	c := &Card{
		GameID:     g.ID,
		InstanceID: "instance-r60-pt-1",
		Name:       "Serra Angel",
		Power:      4,
		Toughness:  4,
		Types:      []string{"Creature"},
		Subtypes:   []string{"Angel"},
		OwnerSeat:  0,
		Zone:       ZoneBattlefield,
	}
	if err := CreateGameCard(ctx, d, c); err != nil {
		t.Fatalf("create card: %v", err)
	}

	got, err := GetGameCard(ctx, d, g.ID, c.InstanceID)
	if err != nil {
		t.Fatalf("get card: %v", err)
	}
	if got.Power != 4 || got.Toughness != 4 {
		t.Errorf("Serra Angel persisted P/T: want 4/4, got %d/%d",
			got.Power, got.Toughness)
	}
}

// -----------------------------------------------------------------------
// ResolveCombat — exercises the new reads through the actual combat path.
// -----------------------------------------------------------------------

func TestResolveCombat_AttackerPowerTaxesDefender(t *testing.T) {
	d, err := hexdekdb.Open(":memory:")
	if err != nil {
		t.Fatalf("open mem db: %v", err)
	}
	defer d.Close()
	ctx := context.Background()
	deckID := seedPartyForGameTest(t, ctx, d, "party-r60-combat", "dev-r60-combat")

	g, err := CreateGame(ctx, d, "party-r60-combat", "cafebabe00000001")
	if err != nil {
		t.Fatalf("create game: %v", err)
	}

	// Two seats: 0 attacks, 1 defends. Both start at life 40. Both reuse
	// the seeded device/deck — game_player has NOT NULL FK on both.
	for i := 0; i < 2; i++ {
		if err := CreateGamePlayer(ctx, d, &Player{
			GameID:       g.ID,
			SeatPosition: i,
			DeviceID:     "dev-r60-combat",
			DeckID:       deckID,
			Life:         40,
		}); err != nil {
			t.Fatalf("create player %d: %v", i, err)
		}
	}

	// 5/5 attacker on seat 0.
	attacker := &Card{
		GameID:     g.ID,
		InstanceID: "atk-5-5",
		Name:       "Craterhoof Behemoth",
		Power:      5,
		Toughness:  5,
		Types:      []string{"Creature"},
		OwnerSeat:  0,
		Zone:       ZoneBattlefield,
	}
	if err := CreateGameCard(ctx, d, attacker); err != nil {
		t.Fatalf("create attacker: %v", err)
	}

	// Need a turn state with active seat 0 + phase set to combat for
	// DeclareAttackers to accept the declaration.
	turn := &TurnState{
		GameID:     g.ID,
		ActiveSeat: 0,
		Phase:      "combat",
		TurnNumber: 1,
	}
	if err := CreateTurnState(ctx, d, turn); err != nil {
		t.Fatalf("init turn state: %v", err)
	}

	if err := DeclareAttackers(ctx, d, g.ID, 0, []AttackerSpec{
		{InstanceID: attacker.InstanceID, TargetSeat: 1},
	}); err != nil {
		t.Fatalf("declare attackers: %v", err)
	}

	report, err := ResolveCombat(ctx, d, g.ID)
	if err != nil {
		t.Fatalf("resolve combat: %v", err)
	}

	if report.PlayerDamage[1] != 5 {
		t.Fatalf("5/5 attacker should deal 5 damage to defender (pre-r60: would have been 1); got PlayerDamage[1]=%d",
			report.PlayerDamage[1])
	}

	// Confirm the defender's life dropped by exactly 5.
	defender, err := GetGamePlayer(ctx, d, g.ID, 1)
	if err != nil {
		t.Fatalf("get defender: %v", err)
	}
	if defender.Life != 35 {
		t.Errorf("defender life: want 40 - 5 = 35, got %d", defender.Life)
	}
}

func TestResolveCombat_MissingPowerFallsBackTo1(t *testing.T) {
	// A creature loaded without P/T metadata (legacy deck JSON) still
	// deals 1 damage — preserves the pre-r60 MVP behavior for the
	// portion of decks that haven't been re-imported with full P/T.
	d, err := hexdekdb.Open(":memory:")
	if err != nil {
		t.Fatalf("open mem db: %v", err)
	}
	defer d.Close()
	ctx := context.Background()
	deckID := seedPartyForGameTest(t, ctx, d, "party-r60-combat-fallback", "dev-r60-fallback")

	g, err := CreateGame(ctx, d, "party-r60-combat-fallback", "cafebabe00000002")
	if err != nil {
		t.Fatalf("create game: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := CreateGamePlayer(ctx, d, &Player{
			GameID:       g.ID,
			SeatPosition: i,
			DeviceID:     "dev-r60-fallback",
			DeckID:       deckID,
			Life:         40,
		}); err != nil {
			t.Fatalf("create player %d: %v", i, err)
		}
	}
	// No Power/Toughness fields set — both zero.
	attacker := &Card{
		GameID:     g.ID,
		InstanceID: "atk-vanilla",
		Name:       "Vanilla Bear",
		Types:      []string{"Creature"},
		OwnerSeat:  0,
		Zone:       ZoneBattlefield,
	}
	if err := CreateGameCard(ctx, d, attacker); err != nil {
		t.Fatalf("create attacker: %v", err)
	}
	turn := &TurnState{GameID: g.ID, ActiveSeat: 0, Phase: "combat", TurnNumber: 1}
	if err := CreateTurnState(ctx, d, turn); err != nil {
		t.Fatalf("init turn state: %v", err)
	}
	if err := DeclareAttackers(ctx, d, g.ID, 0, []AttackerSpec{
		{InstanceID: attacker.InstanceID, TargetSeat: 1},
	}); err != nil {
		t.Fatalf("declare attackers: %v", err)
	}

	report, err := ResolveCombat(ctx, d, g.ID)
	if err != nil {
		t.Fatalf("resolve combat: %v", err)
	}
	if report.PlayerDamage[1] != 1 {
		t.Fatalf("vanilla bear with no P/T metadata: pre-r60 fallback should still deal 1; got %d",
			report.PlayerDamage[1])
	}
}

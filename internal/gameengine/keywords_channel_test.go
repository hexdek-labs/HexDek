package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Channel tests — CR §702.74
// ---------------------------------------------------------------------------

func newChannelGame(t *testing.T) *GameState {
	t.Helper()
	return NewGameState(2, rand.New(rand.NewSource(54)), nil)
}

// channelLandCard mimics a NEO Channel land like Boseiju — a non-basic
// land that carries the Channel keyword on its AST.
func channelLandCard(name string) *Card {
	return &Card{
		Name:     name,
		Owner:    0,
		Types:    []string{"land", "legendary"},
		TypeLine: "Legendary Land",
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "channel"},
			},
		},
	}
}

func plainLandCard(name string) *Card {
	return &Card{
		Name:  name,
		Owner: 0,
		Types: []string{"land", "basic"},
		AST: &gameast.CardAST{
			Name:      name,
			Abilities: []gameast.Ability{},
		},
	}
}

// ---------------------------------------------------------------------------
// (e) HasChannel detect from oracle 'Channel —'
// ---------------------------------------------------------------------------

func TestHasChannel_Detects(t *testing.T) {
	if !HasChannel(channelLandCard("Boseiju, Who Endures")) {
		t.Fatal("HasChannel should detect channel keyword")
	}
}

func TestHasChannel_Negative(t *testing.T) {
	if HasChannel(plainLandCard("Forest")) {
		t.Fatal("HasChannel should be false for a vanilla land")
	}
	if HasChannel(nil) {
		t.Fatal("HasChannel(nil) should be false")
	}
}

// ---------------------------------------------------------------------------
// (a) Channel activate discards card + pays mana
// ---------------------------------------------------------------------------

func TestActivateChannel_DiscardsCardAndPaysMana(t *testing.T) {
	gs := newChannelGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 5
	boseiju := channelLandCard("Boseiju, Who Endures")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, boseiju)

	if err := ActivateChannel(gs, 0, boseiju, 2); err != nil {
		t.Fatalf("ActivateChannel returned error: %v", err)
	}

	// Mana paid (5 - 2 = 3).
	if gs.Seats[0].ManaPool != 3 {
		t.Fatalf("mana pool = %d, want 3", gs.Seats[0].ManaPool)
	}
	// Card discarded from hand.
	for _, c := range gs.Seats[0].Hand {
		if c == boseiju {
			t.Fatal("channeled card should be removed from hand")
		}
	}
	// Card lands in graveyard (DiscardCard path).
	found := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == boseiju {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("channeled card should be in graveyard after activation")
	}
}

func TestActivateChannel_PushesStackItem(t *testing.T) {
	gs := newChannelGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 5
	boseiju := channelLandCard("Boseiju, Who Endures")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, boseiju)

	if err := ActivateChannel(gs, 0, boseiju, 2); err != nil {
		t.Fatalf("ActivateChannel returned error: %v", err)
	}
	if len(gs.Stack) != 1 {
		t.Fatalf("expected 1 stack item, got %d", len(gs.Stack))
	}
	item := gs.Stack[0]
	if item.Card != boseiju {
		t.Fatal("stack item Card should reference the channeled card")
	}
	if item.Controller != 0 {
		t.Fatalf("stack item Controller = %d, want 0", item.Controller)
	}
	if item.Kind != "activated" {
		t.Fatalf("stack item Kind = %q, want \"activated\"", item.Kind)
	}
	if item.CastZone != ZoneHand {
		t.Fatalf("stack item CastZone = %q, want %q (channel activates from hand)", item.CastZone, ZoneHand)
	}
}

// ---------------------------------------------------------------------------
// (b) Channel can't activate from graveyard / battlefield (hand-only)
// ---------------------------------------------------------------------------

func TestActivateChannel_RejectsCardInGraveyard(t *testing.T) {
	gs := newChannelGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 5
	boseiju := channelLandCard("Boseiju, Who Endures")
	// In graveyard, not hand.
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, boseiju)

	err := ActivateChannel(gs, 0, boseiju, 2)
	if err == nil {
		t.Fatal("§702.74a: channel from graveyard must be rejected")
	}
	if gs.Seats[0].ManaPool != 5 {
		t.Fatal("rejected activation must not pay mana")
	}
	if len(gs.Stack) != 0 {
		t.Fatal("rejected activation must not push a stack item")
	}
}

func TestActivateChannel_RejectsCardOnBattlefield(t *testing.T) {
	gs := newChannelGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 5
	boseiju := channelLandCard("Boseiju, Who Endures")
	// On battlefield as a permanent — the engine would normally route the
	// land's regular activated abilities through OnActivated. But the
	// channel ability requires the CARD to be in hand, not the permanent
	// on the battlefield.
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, &Permanent{
		Card:       boseiju,
		Controller: 0,
		Owner:      0,
		Flags:      map[string]int{},
		Timestamp:  gs.NextTimestamp(),
	})

	err := ActivateChannel(gs, 0, boseiju, 2)
	if err == nil {
		t.Fatal("§702.74a: channel from battlefield must be rejected (card-on-battlefield, not in hand)")
	}
}

func TestActivateChannel_RejectsCardInLibraryOrExile(t *testing.T) {
	gs := newChannelGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 5
	boseiju := channelLandCard("Boseiju, Who Endures")
	gs.Seats[0].Library = append(gs.Seats[0].Library, boseiju)
	if err := ActivateChannel(gs, 0, boseiju, 2); err == nil {
		t.Fatal("channel from library must be rejected")
	}

	otawara := channelLandCard("Otawara, Soaring City")
	gs.Seats[0].Exile = append(gs.Seats[0].Exile, otawara)
	if err := ActivateChannel(gs, 0, otawara, 2); err == nil {
		t.Fatal("channel from exile must be rejected")
	}
}

// ---------------------------------------------------------------------------
// (c) Insufficient mana = rejected
// ---------------------------------------------------------------------------

func TestActivateChannel_InsufficientManaRejected(t *testing.T) {
	gs := newChannelGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 1 // need 2
	boseiju := channelLandCard("Boseiju, Who Endures")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, boseiju)

	err := ActivateChannel(gs, 0, boseiju, 2)
	if err == nil {
		t.Fatal("insufficient mana must reject the activation")
	}
	if gs.Seats[0].ManaPool != 1 {
		t.Fatal("rejected activation must not pay any mana")
	}
	// Card stays in hand (atomic — discard didn't happen either).
	stillInHand := false
	for _, c := range gs.Seats[0].Hand {
		if c == boseiju {
			stillInHand = true
			break
		}
	}
	if !stillInHand {
		t.Fatal("rejected activation must not discard the card")
	}
}

func TestActivateChannel_NoKeywordRejected(t *testing.T) {
	gs := newChannelGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 5
	forest := plainLandCard("Forest") // no channel keyword
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, forest)
	if err := ActivateChannel(gs, 0, forest, 2); err == nil {
		t.Fatal("cards without the channel keyword must not activate channel")
	}
	if gs.Seats[0].ManaPool != 5 {
		t.Fatal("rejected non-channel must not pay mana")
	}
}

// ---------------------------------------------------------------------------
// (d) CostMeta stamped
// ---------------------------------------------------------------------------

func TestActivateChannel_StampsCostMeta(t *testing.T) {
	gs := newChannelGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 5
	boseiju := channelLandCard("Boseiju, Who Endures")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, boseiju)

	if err := ActivateChannel(gs, 0, boseiju, 2); err != nil {
		t.Fatalf("ActivateChannel failed: %v", err)
	}
	if len(gs.Stack) != 1 {
		t.Fatalf("expected 1 stack item, got %d", len(gs.Stack))
	}
	item := gs.Stack[0]
	if item.CostMeta == nil {
		t.Fatal("CostMeta should be populated on a channel activation")
	}
	if v, ok := item.CostMeta["channel_activate"]; !ok || v != true {
		t.Fatalf("CostMeta[channel_activate] = %v, want true", item.CostMeta["channel_activate"])
	}
	if v, ok := item.CostMeta["channel_cost"]; !ok || v != 2 {
		t.Fatalf("CostMeta[channel_cost] = %v, want 2", item.CostMeta["channel_cost"])
	}
}

func TestActivateChannel_EmitsEventsAndFiresTrigger(t *testing.T) {
	gs := newChannelGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 5
	boseiju := channelLandCard("Boseiju, Who Endures")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, boseiju)

	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	var seenTrigger bool
	var triggerCost int
	TriggerHook = func(gs *GameState, ev string, ctx map[string]interface{}) {
		if ev != "channel_activated" {
			return
		}
		seenTrigger = true
		if v, ok := ctx["channel_cost"].(int); ok {
			triggerCost = v
		}
	}

	before := len(gs.EventLog)
	if err := ActivateChannel(gs, 0, boseiju, 2); err != nil {
		t.Fatalf("ActivateChannel failed: %v", err)
	}

	if !seenTrigger {
		t.Fatal("TriggerHook should observe channel_activated")
	}
	if triggerCost != 2 {
		t.Fatalf("trigger ctx[channel_cost] = %d, want 2", triggerCost)
	}

	// Log should include both pay_mana and channel_activate events.
	sawPay := false
	sawActivate := false
	for i := before; i < len(gs.EventLog); i++ {
		if gs.EventLog[i].Kind == "pay_mana" && gs.EventLog[i].Details["reason"] == "channel_cost" {
			sawPay = true
		}
		if gs.EventLog[i].Kind == "channel_activate" {
			sawActivate = true
		}
	}
	if !sawPay {
		t.Fatal("expected pay_mana event with reason=channel_cost")
	}
	if !sawActivate {
		t.Fatal("expected channel_activate event")
	}
}

// ---------------------------------------------------------------------------
// Free-channel (cost=0) is legal
// ---------------------------------------------------------------------------

func TestActivateChannel_ZeroCostAllowed(t *testing.T) {
	gs := newChannelGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 0
	free := channelLandCard("Mishra's Bauble-Channel")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, free)

	if err := ActivateChannel(gs, 0, free, 0); err != nil {
		t.Fatalf("free-channel (cost 0) should succeed with no mana: %v", err)
	}
	if len(gs.Stack) != 1 {
		t.Fatalf("expected 1 stack item, got %d", len(gs.Stack))
	}
	if gs.Stack[0].CostMeta["channel_cost"] != 0 {
		t.Fatalf("cost=0 should round-trip in CostMeta")
	}
}

// ---------------------------------------------------------------------------
// Discard-driven mechanics see the channel discard
// ---------------------------------------------------------------------------

func TestActivateChannel_FiresCardDiscardedTrigger(t *testing.T) {
	gs := newChannelGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 5
	boseiju := channelLandCard("Boseiju, Who Endures")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, boseiju)

	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	sawDiscard := false
	var discardedCard *Card
	TriggerHook = func(gs *GameState, ev string, ctx map[string]interface{}) {
		if ev != "card_discarded" {
			return
		}
		sawDiscard = true
		if c, ok := ctx["card"].(*Card); ok {
			discardedCard = c
		}
	}

	if err := ActivateChannel(gs, 0, boseiju, 2); err != nil {
		t.Fatalf("ActivateChannel failed: %v", err)
	}
	if !sawDiscard {
		t.Fatal("channel discard must fire the canonical card_discarded trigger so Madness/Mayhem/Tergrid observers see it (channel is an ability word, CR §207.2c)")
	}
	if discardedCard != boseiju {
		t.Fatal("discard ctx[card] should point at the channeled card")
	}
}

// ---------------------------------------------------------------------------
// Nil / invalid-input safety
// ---------------------------------------------------------------------------

func TestActivateChannel_NilSafe(t *testing.T) {
	gs := newChannelGame(t)
	if err := ActivateChannel(nil, 0, channelLandCard("X"), 0); err == nil {
		t.Fatal("nil game should return error")
	}
	if err := ActivateChannel(gs, -1, channelLandCard("X"), 0); err == nil {
		t.Fatal("invalid seat should return error")
	}
	if err := ActivateChannel(gs, 99, channelLandCard("X"), 0); err == nil {
		t.Fatal("out-of-range seat should return error")
	}
	if err := ActivateChannel(gs, 0, nil, 0); err == nil {
		t.Fatal("nil card should return error")
	}
	c := channelLandCard("X")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, c)
	if err := ActivateChannel(gs, 0, c, -1); err == nil {
		t.Fatal("negative cost should return error")
	}
}

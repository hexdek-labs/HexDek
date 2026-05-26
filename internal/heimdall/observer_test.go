package heimdall

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestObserver_RecordSeed_BuffersAndFlushes(t *testing.T) {
	dir := t.TempDir()
	obs := New(dir, nil, nil, nil)

	for i := 0; i < 10; i++ {
		obs.RecordSeed(GameSeed{
			RNGSeed:    int64(i),
			DeckKeys:   [4]string{"a", "b", "c", "d"},
			Winner:     i % 4,
			Turns:      10 + i,
			KillMethod: "combat",
		})
	}
	if len(obs.seedBuf) != 10 {
		t.Fatalf("expected 10 buffered seeds, got %d", len(obs.seedBuf))
	}

	obs.Flush()
	if len(obs.seedBuf) != 0 {
		t.Fatalf("expected 0 buffered seeds after flush, got %d", len(obs.seedBuf))
	}

	data, err := os.ReadFile(filepath.Join(dir, "heimdall", "seeds.jsonl"))
	if err != nil {
		t.Fatalf("reading seeds file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 10 {
		t.Fatalf("expected 10 lines in seeds file, got %d", len(lines))
	}

	var seed GameSeed
	if err := json.Unmarshal([]byte(lines[0]), &seed); err != nil {
		t.Fatalf("unmarshal first seed: %v", err)
	}
	if seed.RNGSeed != 0 || seed.Winner != 0 || seed.Turns != 10 {
		t.Errorf("unexpected first seed: %+v", seed)
	}
}

func TestObserver_AutoFlushAtCapacity(t *testing.T) {
	dir := t.TempDir()
	obs := New(dir, nil, nil, nil)

	for i := 0; i < seedBufSize+5; i++ {
		obs.RecordSeed(GameSeed{RNGSeed: int64(i), KillMethod: "combat"})
	}

	if len(obs.seedBuf) != 5 {
		t.Fatalf("expected 5 remaining in buffer after auto-flush, got %d", len(obs.seedBuf))
	}

	data, _ := os.ReadFile(filepath.Join(dir, "heimdall", "seeds.jsonl"))
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != seedBufSize {
		t.Fatalf("expected %d flushed seeds, got %d", seedBufSize, len(lines))
	}
}

type mockMuninn struct {
	mu         sync.Mutex
	gaps       []string
	triggers   []DeadTrigger
	crashes    int
	crashDecks []string
}

func (m *mockMuninn) RecordParserGaps(gaps []string, gameID string) {
	m.mu.Lock()
	m.gaps = append(m.gaps, gaps...)
	m.mu.Unlock()
}
func (m *mockMuninn) RecordDeadTriggers(triggers []DeadTrigger, gameID string) {
	m.mu.Lock()
	m.triggers = append(m.triggers, triggers...)
	m.mu.Unlock()
}
func (m *mockMuninn) RecordCrash(panicMsg string, stackTrace string, deckKeys []string) {
	m.mu.Lock()
	m.crashes++
	m.crashDecks = append(m.crashDecks, deckKeys...)
	m.mu.Unlock()
}

type mockHuginn struct {
	mu          sync.Mutex
	pairs       []CoTriggerPair
	zoneCasts   []ZoneCastEvent
	exileLinks  []ExileLinkEvent
}

func (m *mockHuginn) IngestCoTriggers(pairs []CoTriggerPair, deckNames []string) {
	m.mu.Lock()
	m.pairs = append(m.pairs, pairs...)
	m.mu.Unlock()
}

func (m *mockHuginn) IngestZoneCastEvents(events []ZoneCastEvent, deckNames []string, gameID string) {
	m.mu.Lock()
	m.zoneCasts = append(m.zoneCasts, events...)
	m.mu.Unlock()
}

func (m *mockHuginn) IngestExileLinkEvents(events []ExileLinkEvent, deckNames []string, gameID string) {
	m.mu.Lock()
	m.exileLinks = append(m.exileLinks, events...)
	m.mu.Unlock()
}

func TestObserver_RecordObservation_RoutesSinks(t *testing.T) {
	dir := t.TempDir()
	mun := &mockMuninn{}
	hug := &mockHuginn{}
	obs := New(dir, hug, mun, nil)

	obs.RecordObservation(Observation{
		Seed:       GameSeed{RNGSeed: 42},
		ParserGaps: []string{"Card A", "Card B"},
		DeadTriggers: []DeadTrigger{
			{CardName: "Dead Card", TriggerType: "etb"},
		},
		CoTriggers: []CoTriggerPair{
			{CardA: "Sol Ring", CardB: "Mana Crypt", ImpactScore: 0.8},
		},
		ZoneCastEvents: []ZoneCastEvent{
			{Kind: "zone_cast_grant_registered", Card: "Lightning Bolt", Zone: "graveyard", Keyword: "flashback", Seat: 0},
			{Kind: "adventure_exiled", Card: "Brazen Borrower", Seat: 1},
		},
		ExileLinkEvents: []ExileLinkEvent{
			{Kind: "exile_linked_created", Source: "Oblivion Ring", Card: "Krenko", FromZone: "battlefield", Seat: 2},
		},
	})

	if len(mun.gaps) != 2 {
		t.Errorf("expected 2 parser gaps, got %d", len(mun.gaps))
	}
	if len(mun.triggers) != 1 {
		t.Errorf("expected 1 dead trigger, got %d", len(mun.triggers))
	}
	if len(hug.pairs) != 1 {
		t.Errorf("expected 1 co-trigger pair, got %d", len(hug.pairs))
	}
	if hug.pairs[0].CardA != "Sol Ring" {
		t.Errorf("expected Sol Ring, got %s", hug.pairs[0].CardA)
	}
	if len(hug.zoneCasts) != 2 {
		t.Errorf("expected 2 zone-cast events, got %d", len(hug.zoneCasts))
	}
	if len(hug.exileLinks) != 1 {
		t.Errorf("expected 1 exile-link event, got %d", len(hug.exileLinks))
	}
}

func TestObserver_RecordObservation_OptionalSinkSkipped(t *testing.T) {
	// Verify a HuginnSink that does NOT implement the optional zone-cast/
	// exile-link sub-interfaces is left alone — no panic, just silent skip.
	dir := t.TempDir()
	hug := &basicHuginn{}
	obs := New(dir, hug, nil, nil)

	obs.RecordObservation(Observation{
		Seed: GameSeed{RNGSeed: 1},
		CoTriggers: []CoTriggerPair{
			{CardA: "A", CardB: "B"},
		},
		ZoneCastEvents:  []ZoneCastEvent{{Kind: "adventure_exiled", Card: "X"}},
		ExileLinkEvents: []ExileLinkEvent{{Kind: "exile_linked_created", Source: "O-Ring"}},
	})

	if len(hug.pairs) != 1 {
		t.Errorf("expected co-trigger to still route; got %d pairs", len(hug.pairs))
	}
}

type basicHuginn struct {
	pairs []CoTriggerPair
}

func (m *basicHuginn) IngestCoTriggers(pairs []CoTriggerPair, deckNames []string) {
	m.pairs = append(m.pairs, pairs...)
}

func TestObserver_RecordCrash(t *testing.T) {
	dir := t.TempDir()
	mun := &mockMuninn{}
	obs := New(dir, nil, mun, nil)

	obs.RecordCrash("runtime: goroutine stack overflow", []byte("goroutine 1..."), []string{"deck_a", "deck_b"})

	if mun.crashes != 1 {
		t.Errorf("expected 1 crash, got %d", mun.crashes)
	}
	if len(mun.crashDecks) != 2 {
		t.Errorf("expected 2 crash deck keys, got %d", len(mun.crashDecks))
	}
}

func TestObserver_NilSinks(t *testing.T) {
	// Explicit doesn't-panic assertion via recover() — Phase 2B audit.
	// All three observer methods must accept nil sinks without crashing
	// (verified callers pass nil during bootstrap before sinks register).
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Observer methods must accept nil sinks; got panic %v", r)
		}
	}()
	dir := t.TempDir()
	obs := New(dir, nil, nil, nil)
	obs.RecordObservation(Observation{
		ParserGaps: []string{"gap"},
		CoTriggers: []CoTriggerPair{{CardA: "a", CardB: "b"}},
	})
	obs.RecordCrash("panic", nil, nil)
	obs.Pulse(HealthPulse{GamesPlayed: 100})
}

func TestClassifyKill_LossReason(t *testing.T) {
	tests := []struct {
		name     string
		reason   string
		expected string
	}{
		{"poison via reason", "poison counters", "poison"},
		{"commander damage", "commander_damage from Krenko", "commander"},
		{"mill via 704.5b", "state-based 704.5b empty library", "mill"},
		{"combo reason", "infinite combo loop", "combo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gs := &gameengine.GameState{
				Seats: []*gameengine.Seat{
					{Lost: false},
					{Lost: true, LossReason: tt.reason},
				},
			}
			got := ClassifyKill(gs, 0)
			if got != tt.expected {
				t.Errorf("ClassifyKill = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestClassifyKill_HeuristicFallback(t *testing.T) {
	gs := &gameengine.GameState{
		Seats: []*gameengine.Seat{
			{Lost: false},
			{Lost: true, PoisonCounters: 10},
		},
	}
	if got := ClassifyKill(gs, 0); got != "poison" {
		t.Errorf("expected poison, got %s", got)
	}

	gs.Seats[1].PoisonCounters = 0
	gs.Seats[1].CommanderDamage = map[int]map[string]int{
		0: {"Krenko, Mob Boss": 21},
	}
	if got := ClassifyKill(gs, 0); got != "commander" {
		t.Errorf("expected commander, got %s", got)
	}

	gs.Seats[1].CommanderDamage = nil
	gs.Seats[1].Life = 20
	if got := ClassifyKill(gs, 0); got != "mill" {
		t.Errorf("expected mill, got %s", got)
	}
}

func TestClassifyKill_NilGameState(t *testing.T) {
	if got := ClassifyKill(nil, 0); got != "combat" {
		t.Errorf("expected combat for nil gs, got %s", got)
	}
}

func TestClassifyKillWithMaxTurns_Timeout(t *testing.T) {
	gs := &gameengine.GameState{Turn: 100}
	if got := ClassifyKillWithMaxTurns(gs, 0, 100); got != "timeout" {
		t.Errorf("expected timeout, got %s", got)
	}
	if got := ClassifyKillWithMaxTurns(gs, 0, 200); got != "combat" {
		t.Errorf("expected combat (under limit), got %s", got)
	}
}

func TestObserver_ConcurrentRecordSeed(t *testing.T) {
	dir := t.TempDir()
	obs := New(dir, nil, nil, nil)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			obs.RecordSeed(GameSeed{RNGSeed: int64(n), KillMethod: "combat"})
		}(i)
	}
	wg.Wait()
	obs.Flush()

	data, _ := os.ReadFile(filepath.Join(dir, "heimdall", "seeds.jsonl"))
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 100 {
		t.Fatalf("expected 100 seeds from concurrent writes, got %d", len(lines))
	}
}

func TestSnapshotTurnCounters(t *testing.T) {
	gs := &gameengine.GameState{
		Seats: []*gameengine.Seat{
			{Turn: gameengine.TurnCounters{LifeGained: 5, SpellsCast: 3, Attacked: true}},
			{Turn: gameengine.TurnCounters{Milled: 7, CastFromExile: 2}},
			nil, // exercises nil-seat skip
			{Turn: gameengine.TurnCounters{LandsPlayed: 1, Descended: true}},
		},
	}
	snap := SnapshotTurnCounters(gs)
	if snap[0].LifeGained != 5 || snap[0].SpellsCast != 3 || !snap[0].Attacked {
		t.Errorf("seat 0 snapshot mismatch: %+v", snap[0])
	}
	if snap[1].Milled != 7 || snap[1].CastFromExile != 2 {
		t.Errorf("seat 1 snapshot mismatch: %+v", snap[1])
	}
	if snap[2] != (TurnCounterSnapshot{}) {
		t.Errorf("nil seat 2 should produce zero snapshot, got %+v", snap[2])
	}
	if snap[3].LandsPlayed != 1 || !snap[3].Descended {
		t.Errorf("seat 3 snapshot mismatch: %+v", snap[3])
	}
}

func TestSnapshotTurnCounters_NilGameState(t *testing.T) {
	snap := SnapshotTurnCounters(nil)
	for i, s := range snap {
		if s != (TurnCounterSnapshot{}) {
			t.Errorf("snap[%d] should be zero for nil gs, got %+v", i, s)
		}
	}
}

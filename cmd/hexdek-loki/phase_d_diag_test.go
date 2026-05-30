package main

import (
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
	"github.com/hexdek/hexdek/internal/tournament"
)

func getenvInt(k string, def int) int {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// TestPhaseD_DisappearanceDiagnostic runs a single chaos game and dumps the
// first disappearing InstanceIDs along with their last-seen event-log
// breadcrumbs so we can identify which engine code path is dropping refs
// without ceasing the ID. Phase D investigation harness — not a regression.
func TestPhaseD_DisappearanceDiagnostic(t *testing.T) {
	t.Skip("diagnostic — run manually with `DIAG_GAME=N go test ./cmd/hexdek-loki/ -run TestPhaseD_DisappearanceDiagnostic -v` (some games need Loki's parallel runner / different seat/RNG sequencing to reproduce; this single-game harness is for spot-checks)")

	gameengine.SetStrictCensusDefault(true)

	corpus, err := astload.Load("../../data/rules/ast_dataset.jsonl")
	if err != nil {
		t.Fatalf("astload: %v", err)
	}
	meta, err := deckparser.LoadMetaFromJSONL("../../data/rules/ast_dataset.jsonl")
	if err != nil {
		t.Fatalf("meta: %v", err)
	}
	chaosCorpus, err := loadOracleCorpus("../../data/rules/oracle-cards.json")
	if err != nil {
		t.Fatalf("chaos corpus: %v", err)
	}

	// Same seed math as runChaosGame. Use a non-clean game by scanning.
	gameIdx := int64(0)
	if v := getenvInt("DIAG_GAME", 0); v > 0 {
		gameIdx = int64(v)
	}
	deckSeed := int64(42) + gameIdx*10000 + 1
	shuffleSeed := deckSeed + 0*100 + 7
	t.Logf("== DIAG game=%d ==", gameIdx)

	deckRng := rand.New(rand.NewSource(deckSeed))
	gameRng := rand.New(rand.NewSource(shuffleSeed))

	nSeats := 4
	chaosDecks := make([]*gameengine.ChaosDeck, nSeats)
	for i := 0; i < nSeats; i++ {
		chaosDecks[i] = gameengine.GenerateChaosDeck(chaosCorpus, deckRng)
	}

	gs := gameengine.NewGameState(nSeats, gameRng, corpus)
	commanderDecks := make([]*gameengine.CommanderDeck, nSeats)
	for i, cd := range chaosDecks {
		cmdrCard := buildCardFromName(cd.Commander.Name, corpus, meta)
		if cmdrCard == nil {
			cmdrCard = &gameengine.Card{
				Name:          cd.Commander.Name,
				Owner:         i,
				Types:         []string{"legendary", "creature"},
				BasePower:     cd.Commander.Power,
				BaseToughness: cd.Commander.Toughness,
				CMC:           cd.Commander.CMC,
				Colors:        cd.Commander.Colors,
			}
			if cmdrCard.BaseToughness == 0 {
				cmdrCard.BaseToughness = 1
			}
		} else {
			cmdrCard.Owner = i
		}
		lib := make([]*gameengine.Card, 0, len(cd.Cards))
		for _, name := range cd.Cards {
			c := buildCardFromName(name, corpus, meta)
			if c == nil {
				c = &gameengine.Card{Name: name, Owner: i}
				for _, cc := range chaosCorpus.All {
					if cc.Name == name {
						c.Types = cc.Types
						c.BasePower = cc.Power
						c.BaseToughness = cc.Toughness
						c.CMC = cc.CMC
						c.Colors = cc.Colors
						break
					}
				}
			} else {
				c.Owner = i
			}
			lib = append(lib, c)
		}
		gameRng.Shuffle(len(lib), func(a, b int) { lib[a], lib[b] = lib[b], lib[a] })
		commanderDecks[i] = &gameengine.CommanderDeck{
			CommanderCards: []*gameengine.Card{cmdrCard},
			Library:        lib,
		}
	}
	gameengine.SetupCommanderGame(gs, commanderDecks)

	// Build the ID→name map from every minted card.
	idToName := map[string]string{}
	captureIDs := func() {
		walk := func(c *gameengine.Card) {
			if c != nil && c.InstanceID != "" {
				if _, ok := idToName[c.InstanceID]; !ok {
					idToName[c.InstanceID] = c.Name
				}
			}
		}
		for _, s := range gs.Seats {
			if s == nil {
				continue
			}
			for _, c := range s.Library {
				walk(c)
			}
			for _, c := range s.Hand {
				walk(c)
			}
			for _, c := range s.Graveyard {
				walk(c)
			}
			for _, c := range s.Exile {
				walk(c)
			}
			for _, c := range s.CommandZone {
				walk(c)
			}
			for _, p := range s.Battlefield {
				if p != nil {
					walk(p.Card)
					for _, mc := range p.MergedCardPtrs {
						walk(mc)
					}
				}
			}
		}
		for _, item := range gs.Stack {
			if item != nil {
				walk(item.Card)
			}
		}
	}
	captureIDs()

	for i := 0; i < nSeats; i++ {
		gs.Seats[i].Hat = &hat.GreedyHat{}
		for j := 0; j < 7; j++ {
			if len(gs.Seats[i].Library) == 0 {
				break
			}
			c := gs.Seats[i].Library[0]
			gs.Seats[i].Library = gs.Seats[i].Library[1:]
			gs.Seats[i].Hand = append(gs.Seats[i].Hand, c)
		}
	}
	gs.Active = gameRng.Intn(nSeats)
	gs.Turn = 1
	captureIDs()

	// Track each minted ID's *Card pointer so we can chase missing IDs
	// across every zone after the fact.
	idToCard := map[string]*gameengine.Card{}
	captureCardPtrs := func() {
		walk := func(c *gameengine.Card) {
			if c != nil && c.InstanceID != "" {
				if _, ok := idToCard[c.InstanceID]; !ok {
					idToCard[c.InstanceID] = c
				}
			}
		}
		for _, s := range gs.Seats {
			if s == nil {
				continue
			}
			for _, c := range s.Library {
				walk(c)
			}
			for _, c := range s.Hand {
				walk(c)
			}
			for _, c := range s.Graveyard {
				walk(c)
			}
			for _, c := range s.Exile {
				walk(c)
			}
			for _, c := range s.CommandZone {
				walk(c)
			}
			for _, p := range s.Battlefield {
				if p != nil {
					walk(p.Card)
					for _, mc := range p.MergedCardPtrs {
						walk(mc)
					}
				}
			}
			for _, c := range s.ForetellExile {
				walk(c)
			}
		}
		for _, item := range gs.Stack {
			if item != nil {
				walk(item.Card)
			}
		}
		for c := range gs.MadnessExile {
			walk(c)
		}
		for c := range gs.PlotExile {
			walk(c)
		}
		for c := range gs.MayhemDiscards {
			walk(c)
		}
		for c := range gs.ZoneCastGrants {
			walk(c)
		}
	}
	captureCardPtrs()

	zoneOf := func(card *gameengine.Card) string {
		for _, s := range gs.Seats {
			if s == nil {
				continue
			}
			for _, c := range s.Library {
				if c == card {
					return fmt.Sprintf("seat%d.Library", s.Idx)
				}
			}
			for _, c := range s.Hand {
				if c == card {
					return fmt.Sprintf("seat%d.Hand", s.Idx)
				}
			}
			for _, c := range s.Graveyard {
				if c == card {
					return fmt.Sprintf("seat%d.Graveyard", s.Idx)
				}
			}
			for _, c := range s.Exile {
				if c == card {
					return fmt.Sprintf("seat%d.Exile", s.Idx)
				}
			}
			for _, c := range s.CommandZone {
				if c == card {
					return fmt.Sprintf("seat%d.CommandZone", s.Idx)
				}
			}
			for _, p := range s.Battlefield {
				if p != nil && p.Card == card {
					return fmt.Sprintf("seat%d.Battlefield", s.Idx)
				}
			}
			for _, c := range s.ForetellExile {
				if c == card {
					return fmt.Sprintf("seat%d.ForetellExile", s.Idx)
				}
			}
			if s.Companion == card {
				return fmt.Sprintf("seat%d.Companion", s.Idx)
			}
		}
		if _, ok := gs.MadnessExile[card]; ok {
			return "gs.MadnessExile"
		}
		if _, ok := gs.PlotExile[card]; ok {
			return "gs.PlotExile"
		}
		if _, ok := gs.MayhemDiscards[card]; ok {
			return "gs.MayhemDiscards"
		}
		if _, ok := gs.ZoneCastGrants[card]; ok {
			return "gs.ZoneCastGrants"
		}
		for _, item := range gs.Stack {
			if item != nil && item.Card == card {
				return fmt.Sprintf("gs.Stack[%s]", item.Kind)
			}
		}
		return "NOWHERE"
	}

	dumpMissing := func(label string) bool {
		present := map[string]struct{}{}
		add := func(c *gameengine.Card) {
			if c != nil && c.InstanceID != "" {
				present[c.InstanceID] = struct{}{}
			}
		}
		for _, s := range gs.Seats {
			if s == nil || s.LeftGame {
				continue
			}
			for _, c := range s.Library {
				add(c)
			}
			for _, c := range s.Hand {
				add(c)
			}
			for _, c := range s.Graveyard {
				add(c)
			}
			for _, c := range s.Exile {
				add(c)
			}
			for _, c := range s.CommandZone {
				add(c)
			}
			for _, p := range s.Battlefield {
				if p == nil {
					continue
				}
				add(p.Card)
				for _, mc := range p.MergedCardPtrs {
					add(mc)
				}
			}
			for _, c := range s.ForetellExile {
				add(c)
			}
			if s.Companion != nil {
				add(s.Companion)
			}
		}
		for c := range gs.ZoneCastGrants {
			add(c)
		}
		for c := range gs.MadnessExile {
			add(c)
		}
		for c := range gs.PlotExile {
			add(c)
		}
		for c := range gs.MayhemDiscards {
			add(c)
		}
		for _, cards := range gs.ParadigmExile {
			for _, c := range cards {
				add(c)
			}
		}
		for _, item := range gs.Stack {
			if item == nil {
				continue
			}
			if item.Source != nil || item.Kind == "triggered" || item.Kind == "activated" {
				continue
			}
			add(item.Card)
		}

		var missing []string
		for id := range gs.MintedInstanceIDs {
			if _, ceased := gs.CeasedInstanceIDs[id]; ceased {
				continue
			}
			if len(id) < 4 || id[2:4] == "AB" {
				continue
			}
			if _, ok := present[id]; !ok {
				missing = append(missing, id)
			}
		}
		if len(missing) == 0 {
			return false
		}
		sort.Strings(missing)
		t.Logf("=== %s: %d missing IDs ===", label, len(missing))
		max := 25
		if len(missing) < max {
			max = len(missing)
		}
		for _, id := range missing[:max] {
			name := idToName[id]
			if name == "" {
				name = "<unknown>"
			}
			card := idToCard[id]
			zone := "<no-ptr-captured>"
			cardTypes := ""
			if card != nil {
				zone = zoneOf(card)
				cardTypes = strings.Join(card.Types, ",")
			}
			// Search event log for any mentions of this card name or ID.
			var lastEvents []string
			for i := len(gs.EventLog) - 1; i >= 0 && len(lastEvents) < 4; i-- {
				ev := &gs.EventLog[i]
				match := false
				if ev.Source == name {
					match = true
				}
				if !match && ev.Details != nil {
					for _, v := range ev.Details {
						if s, ok := v.(string); ok && (s == id || s == name) {
							match = true
							break
						}
					}
				}
				if match {
					lastEvents = append(lastEvents, fmt.Sprintf("[%d] %s seat=%d src=%q", i, ev.Kind, ev.Seat, ev.Source))
				}
			}
			t.Logf("  %s = %q types=[%s] zone=%s | events: %s", id, name, cardTypes, zone, strings.Join(lastEvents, " ; "))
		}
		return true
	}

	maxTurn := 60
	for turn := 1; turn <= maxTurn; turn++ {
		gs.Turn = turn
		func() {
			defer func() { _ = recover() }()
			tournament.TakeTurn(gs)
		}()
		captureIDs()
		captureCardPtrs()
		if dumpMissing(fmt.Sprintf("turn %d (after TakeTurn)", turn)) {
			t.Logf("first disappearance at turn %d — stopping diagnostic", turn)
			return
		}
		gameengine.StateBasedActions(gs)
		captureIDs()
		captureCardPtrs()
		if dumpMissing(fmt.Sprintf("turn %d (after SBA)", turn)) {
			t.Logf("first disappearance at turn %d (post-SBA) — stopping diagnostic", turn)
			return
		}
		if gs.CheckEnd() {
			t.Logf("game ended at turn %d (CheckEnd=true)", turn)
			break
		}
		gs.Active = gameengine.NextLivingSeat(gs)
	}
	t.Logf("game completed cleanly — no disappearances in %d turns", gs.Turn)
}

package main

import (
	"strings"
	"testing"
)

// TestDrawTokenTrigger_NotInfinite is the R59 regression guard. A card whose
// only "card draw" mention sits inside a trigger condition ("Whenever you
// draw a card, create a 1/1 ... token") is NOT a card-producer — the token
// it makes is a permanent, not a card in hand. Pairing it with a one-shot
// rummage sorcery used to close a spurious card↔card cycle and Freya tagged
// the pair as TRUE INFINITE.
func TestDrawTokenTrigger_NotInfinite(t *testing.T) {
	locustGod := ClassifyCard(
		"The Locust God",
		"Flying\nWhenever you draw a card, create a 1/1 blue and red Insect creature token with flying and haste.\n{2}{U}{R}: Draw a card, then discard a card.\nWhen The Locust God dies, return it to its owner's hand at the beginning of the next end step.",
		"Legendary Creature — God",
		"{3}{U}{R}",
		5,
		"4",
	)
	waterbendingLesson := ClassifyCard(
		"Waterbending Lesson",
		"Draw three cards. Then discard a card unless you waterbend {2}.",
		"Sorcery",
		"{3}{U}",
		4,
		"",
	)

	report := AnalyzeDeck(
		[]CardProfile{locustGod, waterbendingLesson},
		"locust+waterbending",
		"",
		"The Locust God",
	)

	for _, c := range report.TrueInfinites {
		if comboContains(c.Cards, "The Locust God") &&
			comboContains(c.Cards, "Waterbending Lesson") {
			t.Fatalf("draw→token trigger paired with rummage sorcery must not be true_infinite: %+v", c)
		}
	}
}

// TestDrawTokenTrigger_ProducesTokenNotCard verifies the resource model: a
// "Whenever you draw a card, create a 1/1 token" card produces ResToken (the
// token is a permanent) and must NOT produce ResCard purely from the trigger
// condition. Activated draw effects on the same card still register ResCard.
func TestDrawTokenTrigger_ProducesTokenNotCard(t *testing.T) {
	// Hypothetical card with ONLY a draw-trigger + token creation, no
	// other draw effects. The "draw a card" substring in the trigger must
	// not register as ResCard production.
	p := ClassifyCard(
		"Synthetic Drawer",
		"Whenever you draw a card, create a 1/1 colorless Construct artifact creature token.",
		"Legendary Creature — Construct",
		"{2}{U}",
		3,
		"2",
	)
	if !containsRes(p.Produces, ResToken) {
		t.Errorf("expected ResToken in Produces, got %v", p.Produces)
	}
	if containsRes(p.Produces, ResCard) {
		t.Errorf("trigger-only draw mention must not register as ResCard production; got %v", p.Produces)
	}
}

// TestEffectDiscardNotCost guards the second half of R59: a rummage / loot
// pattern ("draw a card, then discard a card") is a hand-filter EFFECT, not
// a discard cost — it must not mark the card as a card-consumer.
func TestEffectDiscardNotCost(t *testing.T) {
	rummage := ClassifyCard(
		"Plain Rummager",
		"Draw two cards. Then discard a card.",
		"Sorcery",
		"{2}{U}",
		3,
		"",
	)
	if containsRes(rummage.Consumes, ResCard) {
		t.Errorf("post-draw effect-discard must not register as ResCard consumption; got %v", rummage.Consumes)
	}
}

// TestAdditionalCostDiscardStillRegisters guards against over-correction: an
// "as an additional cost ... discard a card" pattern IS a real discard cost
// and must continue to register as ResCard consumption.
func TestAdditionalCostDiscardStillRegisters(t *testing.T) {
	thrill := ClassifyCard(
		"Thrill of Possibility",
		"As an additional cost to cast this spell, discard a card.\nDraw two cards.",
		"Instant",
		"{1}{R}",
		2,
		"",
	)
	if !containsRes(thrill.Consumes, ResCard) {
		t.Errorf("explicit additional-cost discard must register as ResCard consumption; got %v", thrill.Consumes)
	}
}

// TestActivatedDiscardCostStillRegisters guards the activated-ability
// discard-cost case ("Discard a card: ..." or "{cost}, Discard a card: ...").
func TestActivatedDiscardCostStillRegisters(t *testing.T) {
	imp := ClassifyCard(
		"Putrid Imp",
		"{B}: Putrid Imp becomes a Zombie until end of turn.\nDiscard a card: Putrid Imp gains flying until end of turn.",
		"Creature — Zombie Imp",
		"{B}",
		1,
		"1",
	)
	if !containsRes(imp.Consumes, ResCard) {
		t.Errorf("activated-ability discard cost must register as ResCard consumption; got %v", imp.Consumes)
	}
}

func comboContains(cards []string, name string) bool {
	for _, c := range cards {
		if strings.EqualFold(c, name) {
			return true
		}
	}
	return false
}

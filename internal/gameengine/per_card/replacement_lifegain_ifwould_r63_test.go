package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Regressions for replacement_lifegain_ifwould_r63.go (CR §614). Each card
// was INERT before this handler — the would_gain_life / would_draw event
// chain saw no replacement. We register on ETB then fire the event hook
// and assert the modified count / cancellation.

// A. "you gain that much life plus 1 instead" — additive +1.
func TestIfWould_LifeGainPlusOne(t *testing.T) {
	for _, name := range []string{
		"Angel of Vitality", "Heron of Hope", "Honor Troll",
		"Knight of Dawn's Light", "Cleric Class", "Leyline of Hope", "Pest Rescuer",
	} {
		t.Run(name, func(t *testing.T) {
			gs := newGame(t, 2)
			p := addPerm(gs, 0, name, "enchantment")
			gameengine.InvokeETBHook(gs, p)

			got, cancelled := gameengine.FireGainLifeEvent(gs, 0, 3, p)
			if cancelled {
				t.Fatalf("%s: gain should not be cancelled", name)
			}
			if got != 4 {
				t.Errorf("%s: gain 3 -> want 4 (plus 1), got %d", name, got)
			}
			// Opponent's gain is untouched.
			if g2, _ := gameengine.FireGainLifeEvent(gs, 1, 3, p); g2 != 3 {
				t.Errorf("%s: opponent gain must be unaffected, got %d", name, g2)
			}
		})
	}
}

// B. "you gain twice that much life instead" — ×2, unconditional.
func TestIfWould_LifeGainTwice_WindCrystal(t *testing.T) {
	gs := newGame(t, 2)
	p := addPerm(gs, 0, "The Wind Crystal", "artifact")
	gameengine.InvokeETBHook(gs, p)

	got, cancelled := gameengine.FireGainLifeEvent(gs, 0, 5, p)
	if cancelled || got != 10 {
		t.Errorf("The Wind Crystal: gain 5 -> want 10 (×2), got %d (cancelled=%v)", got, cancelled)
	}
}

// B'. Phial of Galadriel — ×2 only while controller has 5 or less life.
func TestIfWould_LifeGainTwice_Phial_Conditional(t *testing.T) {
	gs := newGame(t, 2)
	p := addPerm(gs, 0, "Phial of Galadriel", "artifact")
	gameengine.InvokeETBHook(gs, p)

	gs.Seats[0].Life = 5 // threshold met
	if got, _ := gameengine.FireGainLifeEvent(gs, 0, 4, p); got != 8 {
		t.Errorf("Phial @5 life: gain 4 -> want 8 (×2), got %d", got)
	}
	gs.Seats[0].Life = 6 // above threshold — replacement inactive
	if got, _ := gameengine.FireGainLifeEvent(gs, 0, 4, p); got != 4 {
		t.Errorf("Phial @6 life: gain 4 -> want 4 (unchanged), got %d", got)
	}
}

// C. Opponent would gain life → loses that much instead (Tainted Remedy,
// Plague Drone). The gain is cancelled and the opponent loses life.
func TestIfWould_OpponentGainBecomesLoss(t *testing.T) {
	for _, name := range []string{"Tainted Remedy", "Plague Drone"} {
		t.Run(name, func(t *testing.T) {
			gs := newGame(t, 2)
			p := addPerm(gs, 0, name, "creature")
			gameengine.InvokeETBHook(gs, p)
			gs.Seats[1].Life = 40

			got, cancelled := gameengine.FireGainLifeEvent(gs, 1, 6, p)
			if !cancelled {
				t.Errorf("%s: opponent gain must be cancelled", name)
			}
			if got != 6 {
				// count is reported but the gain is cancelled; the loss happened in ApplyFn.
				t.Logf("%s: reported count %d (gain cancelled)", name, got)
			}
			if gs.Seats[1].Life != 34 {
				t.Errorf("%s: opponent should LOSE 6 (40 -> 34), got life %d", name, gs.Seats[1].Life)
			}
			// Controller's own gain is unaffected.
			gs.Seats[0].Life = 40
			if g2, c2 := gameengine.FireGainLifeEvent(gs, 0, 6, p); c2 || g2 != 6 {
				t.Errorf("%s: controller gain must pass through (6), got %d cancelled=%v", name, g2, c2)
			}
		})
	}
}

// D. "if you would draw a card, draw two cards instead" — ×2 per draw.
func TestIfWould_DrawTwice_ThoughtReflection(t *testing.T) {
	gs := newGame(t, 2)
	p := addPerm(gs, 0, "Thought Reflection", "enchantment")
	gameengine.InvokeETBHook(gs, p)

	// FireDrawEvent fires per-card with count 1; "draw two instead" -> 2.
	got, cancelled := gameengine.FireDrawEvent(gs, 0, p)
	if cancelled || got != 2 {
		t.Errorf("Thought Reflection: draw 1 -> want 2 (×2), got %d (cancelled=%v)", got, cancelled)
	}
	// Opponent draw unaffected.
	if g2, _ := gameengine.FireDrawEvent(gs, 1, p); g2 != 1 {
		t.Errorf("Thought Reflection: opponent draw must be unaffected, got %d", g2)
	}
}

package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerRienneAngelOfRebirthCustom wires Rienne's
// "Other multicolored creatures you control get +1/+0" anthem.
// The gen_*.go handler covers Rienne's flying + the
// "another multicolored dies → return at next end step" recursion;
// its partial breadcrumb covers the +1/+0 anthem static.
//
// Multicolored = card has ≥2 distinct colors. We honor printed colors
// via Card.Colors and Card.Types "pip:X" markers (token shortcut).
const rienneMulticolorAnthemTag = "rienne_multicolor_anthem"

func registerRienneAngelOfRebirthCustom(r *Registry) {
	r.OnETB("Rienne, Angel of Rebirth", rienneRefreshAnthemOnETB)
	r.OnTrigger("Rienne, Angel of Rebirth", "permanent_etb", rienneRefreshAnthemOnEvent)
	r.OnTrigger("Rienne, Angel of Rebirth", "permanent_ltb", rienneRefreshAnthemOnEvent)
}

func rienneRefreshAnthemOnETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	rienneRefreshMulticolorAnthem(gs, perm)
}

func rienneRefreshAnthemOnEvent(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	rienneRefreshMulticolorAnthem(gs, perm)
}

func rienneRefreshMulticolorAnthem(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "rienne_multicolor_anthem_refresh"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	stamped := 0
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || p.Card == nil || !p.IsCreature() {
			stripTaggedModifications(p, rienneMulticolorAnthemTag)
			continue
		}
		stripTaggedModifications(p, rienneMulticolorAnthemTag)
		if !rienneIsMulticolor(p.Card) {
			continue
		}
		p.Modifications = append(p.Modifications, gameengine.Modification{
			Power:     1,
			Toughness: 0,
			Duration:  rienneMulticolorAnthemTag,
			Timestamp: gs.NextTimestamp(),
		})
		stamped++
	}
	if stamped > 0 {
		gs.InvalidateCharacteristicsCache()
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":    perm.Controller,
			"stamped": stamped,
		})
	}
}

func rienneIsMulticolor(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	seen := map[string]bool{}
	for _, col := range c.Colors {
		switch col {
		case "W", "U", "B", "R", "G", "w", "u", "b", "r", "g":
			seen[normColor(col)] = true
		}
	}
	for _, t := range c.Types {
		switch t {
		case "pip:W", "pip:U", "pip:B", "pip:R", "pip:G":
			seen[t[len(t)-1:]] = true
		}
	}
	return len(seen) >= 2
}

func normColor(c string) string {
	switch c {
	case "W", "w":
		return "W"
	case "U", "u":
		return "U"
	case "B", "b":
		return "B"
	case "R", "r":
		return "R"
	case "G", "g":
		return "G"
	}
	return c
}

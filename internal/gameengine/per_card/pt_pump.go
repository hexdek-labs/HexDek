package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Canonical power/toughness pumping for per-card handlers.
//
// Historical note — the bug this file exists to close:
//
// A large family of handlers pumped a creature by writing
// Permanent.Flags["temp_power"] / ["temp_toughness"] and nothing else.
// Several of them documented that flag as "the +1/+1-until-EOT
// convention used across per_card" which "the layers/combat pipeline
// consumes" and which "the cleanup-step sweep clears".
//
// None of that was true. Outside of per_card the only mention of
// temp_power in the whole engine is keywords_bloodrush.go, which writes
// it as a *diagnostic stamp* alongside the real pump. No layer reads it,
// Permanent.Power()/Toughness() do not read it, combat damage does not
// read it, and nothing clears it. Every handler that pumped only through
// the flag therefore did nothing at all: the creature's power never
// changed, combat damage never changed, and the flag accumulated for the
// rest of the game.
//
// The mechanism the engine actually reads is Permanent.Modifications
// (applied in CR §613.4c sublayer 7c by applyCountersAndMods, and
// stripped at cleanup by the §514.2 sweep in phases.go for the
// "until_end_of_turn" duration). The helpers below route through it, and
// keep writing the legacy flag so existing diagnostics and tests that
// assert on the stamp continue to read the same value.

// pumpUntilEOT applies a +dp/+dt pump to p that the engine actually
// reads, expiring at the next cleanup step per CR §514.2. Additive:
// calling it twice pumps twice, which is what a repeatable trigger
// ("whenever you cast an instant or sorcery, …get +1/+1") wants.
func pumpUntilEOT(gs *gameengine.GameState, p *gameengine.Permanent, dp, dt int) {
	if gs == nil || p == nil || (dp == 0 && dt == 0) {
		return
	}
	if p.Flags == nil {
		p.Flags = map[string]int{}
	}
	p.Flags["temp_power"] += dp
	p.Flags["temp_toughness"] += dt
	p.Modifications = append(p.Modifications, gameengine.Modification{
		Power:     dp,
		Toughness: dt,
		Duration:  "until_end_of_turn",
		Timestamp: gs.NextTimestamp(),
	})
	gs.InvalidateCharacteristicsCache()
}

// setStaticPT re-points a *recomputed* static buff (Jarad's
// "+1/+1 for each creature card in your graveyard", Zinnia's "+X/+0")
// to a new absolute total.
//
// These handlers do not add a pump, they restate one: each refresh
// replaces the previous value rather than stacking on it. The previous
// total is read back from the legacy flag stamp, so only the difference
// is applied — a refresh that lowers the count applies a negative
// modification and the buff shrinks correctly.
//
// The modification is "permanent" because the underlying ability is a
// static one; it is not swept at cleanup. Coverage is still only as good
// as the refresh cadence (these handlers refresh on ETB and on related
// triggers, not continuously), which the handlers flag via emitPartial —
// but between refreshes the creature now actually has the buff.
func setStaticPT(gs *gameengine.GameState, p *gameengine.Permanent, totalP, totalT int) {
	if gs == nil || p == nil {
		return
	}
	if p.Flags == nil {
		p.Flags = map[string]int{}
	}
	dp := totalP - p.Flags["temp_power"]
	dt := totalT - p.Flags["temp_toughness"]
	p.Flags["temp_power"] = totalP
	p.Flags["temp_toughness"] = totalT
	if dp == 0 && dt == 0 {
		return
	}
	p.Modifications = append(p.Modifications, gameengine.Modification{
		Power:     dp,
		Toughness: dt,
		Duration:  "permanent",
		Timestamp: gs.NextTimestamp(),
	})
	gs.InvalidateCharacteristicsCache()
}

// grantUntilEOT grants a keyword the engine actually reads.
//
// Same bug, different axis: handlers granting a keyword "until end of
// turn" wrote Permanent.Flags["temp_deathtouch"] / ["temp_vigilance"],
// which nothing reads either. HasKeyword (combat.go) looks at the AST,
// Permanent.GrantedAbilities, Flags["kw:<name>"] and keyword counters —
// never at a "temp_" flag. GrantedAbilities is the right home: it is the
// surface bloodrush's UEOT ability grants use, and the §514.2 cleanup
// sweep in phases.go clears it at end of turn.
//
// Idempotent: a creature that already has the keyword granted does not
// collect a duplicate entry.
func grantUntilEOT(p *gameengine.Permanent, keyword string) {
	if p == nil || keyword == "" {
		return
	}
	for _, g := range p.GrantedAbilities {
		if g == keyword {
			return
		}
	}
	p.GrantedAbilities = append(p.GrantedAbilities, keyword)
}

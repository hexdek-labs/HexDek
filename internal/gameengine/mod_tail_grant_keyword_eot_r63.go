package gameengine

// mod_tail_grant_keyword_eot_r63.go — generic runtime handler for the
// "<subject> gains <keyword(s)> until end of turn" parsed_tail cluster
// (~105 cards: 72 instant/sorcery standalone — Heroic Intervention, The
// Crowd Goes Wild, Dinosaur Stampede, Undercity Uprising, Rally at the
// Hornburg, … — plus the keyword-grant ARMS of modal Charms/Commands like
// Mardu Charm, Atarka's Command, Valorous Stance, which route here via the
// modal handler's per-mode fallback).
//
// Genuine runtime-handler gap (NOT a parser gap): the consumer
// infrastructure already exists — combat reads these keywords via
// p.HasKeyword (combat.go: trample/deathtouch/lifelink/first strike/double
// strike/vigilance/haste/menace/flying), destroy/SBA read
// p.IsIndestructible (which reads GrantedAbilities), targeting reads
// hexproof via p.HasKeyword — and GrantedAbilities is cleared at end of
// turn (phases.go:401). Only the text→grant step was missing, so these
// spells did NOTHING. We append the granted keyword(s) to the chosen
// creatures' GrantedAbilities (the proven Bloodrush pattern: until-EOT,
// auto-cleaned), honored by every existing p.HasKeyword consumer.
//
// Conservative:
//   - whitelisted single-word evergreen/known keywords only (skip
//     "protection from <X>" and other arg-bearing grants → stay inert);
//   - buffs go to the controller's OWN creatures (every grantable keyword
//     here is beneficial/protective, so own-creature targeting is always
//     the correct, safe line — never a downside);
//   - unrecognized subjects / no legal creature → return false (inert),
//     never mis-resolve.

import (
	"regexp"
	"strings"
)

// reTailGrantKw captures the keyword list in "... gains <kws> until end of
// turn". The subject is the text preceding "gain(s)".
var reTailGrantKw = regexp.MustCompile(`(?i)\bgains?\s+([a-z, ]+?)\s+until end of turn\b`)

// grantableKeywords are single-word (no-argument) keywords that the engine's
// p.HasKeyword / IsIndestructible consumers honor when present on
// GrantedAbilities. Arg-bearing grants ("protection from X", "ward {2}")
// are intentionally excluded — they need parsing the engine doesn't do here.
var grantableKeywords = map[string]bool{
	"flying": true, "first strike": true, "double strike": true,
	"deathtouch": true, "lifelink": true, "vigilance": true,
	"trample": true, "haste": true, "menace": true, "reach": true,
	"hexproof": true, "indestructible": true, "shroud": true,
	"defender": true, "intimidate": true, "fear": true, "wither": true,
	"infect": true, "skulk": true, "flanking": true, "horsemanship": true,
	"unblockable": true, "lure": true,
}

// resolveGrantKeywordEOTText recognizes "<subject> gains <kw(s)> until end of
// turn" and grants the whitelisted keyword(s) to the controller's creatures
// for the turn. Returns true iff it granted ≥1 keyword to ≥1 creature.
func resolveGrantKeywordEOTText(gs *GameState, src *Permanent, raw string) bool {
	if gs == nil || src == nil {
		return false
	}
	low := strings.ToLower(raw)
	mm := reTailGrantKw.FindStringSubmatchIndex(low)
	if mm == nil {
		return false
	}
	kwList := low[mm[2]:mm[3]]
	kws := parseGrantableKeywords(kwList)
	if len(kws) == 0 {
		return false
	}
	// Subject = the clause preceding "gains".
	subject := strings.TrimSpace(low[:mm[0]])
	// Conservative guard: if the clause carries trigger/conditional framing
	// ("whenever …", "when …", "at the beginning …", "if …", "as long as
	// …"), this is a triggered/conditional ability the parser failed to
	// structure — NOT an unconditional one-shot grant. Granting it now would
	// ignore the trigger/condition. Stay inert (parser gap, not ours).
	for _, frame := range []string{"whenever", "when ", "at the beginning", "as long as"} {
		if strings.Contains(subject, frame) {
			return false
		}
	}
	if strings.HasPrefix(subject, "if ") || strings.Contains(subject, " if ") {
		return false
	}
	targets := pickGrantSubjects(gs, src, subject)
	if len(targets) == 0 {
		return false
	}

	granted := 0
	for _, p := range targets {
		for _, kw := range kws {
			if !permHasGranted(p, kw) {
				p.GrantedAbilities = append(p.GrantedAbilities, kw)
				granted++
			}
		}
	}
	if granted == 0 {
		return false
	}
	gs.InvalidateCharacteristicsCache()
	gs.LogEvent(Event{
		Kind:   "grant_keyword_eot",
		Seat:   controllerSeat(src),
		Source: sourceName(src),
		Amount: len(targets),
		Details: map[string]interface{}{
			"keywords": kws,
			"targets":  len(targets),
		},
	})
	return true
}

// parseGrantableKeywords splits "menace, lifelink, and haste" into the
// whitelisted single-word keywords, dropping any non-whitelisted token.
func parseGrantableKeywords(s string) []string {
	s = strings.ReplaceAll(s, " and ", ", ")
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		k := strings.TrimSpace(p)
		if grantableKeywords[k] {
			out = append(out, k)
		}
	}
	return out
}

// pickGrantSubjects maps the subject clause to the controller's creature(s)
// that receive the grant. Conservative: own creatures only.
//   - "creatures you control" / bare "creatures" / "<N> or more creatures" /
//     "<color/type> creatures you control" → all (matching) own creatures;
//   - "target creature [you control]" / "~" / "this/that creature" / "it" →
//     the controller's highest-power creature (single buff).
func pickGrantSubjects(gs *GameState, src *Permanent, subject string) []*Permanent {
	seat := controllerSeat(src)
	if seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return nil
	}
	bf := gs.Seats[seat].Battlefield

	// Mass grant: "creatures you control [...]" / "creatures gain" (when the
	// subject is plural "creatures ..."). Require "you control" OR a plain
	// "creatures" subject to stay own-only and avoid opponent-mass matches.
	if strings.Contains(subject, "creatures you control") ||
		strings.HasPrefix(subject, "creatures") ||
		(strings.Contains(subject, "creatures") && strings.Contains(subject, "you control")) {
		// Optional color filter ("white creatures you control").
		color := leadingColorWord(subject)
		var out []*Permanent
		for _, p := range bf {
			if p == nil || p.PhasedOut || !gs.IsCreatureOf(p) {
				continue
			}
			if color != "" && !permHasColor(gs, p, color) {
				continue
			}
			out = append(out, p)
		}
		return out
	}

	// Single-target grant: pick own highest-power creature.
	if strings.Contains(subject, "creature") || subject == "~" ||
		strings.Contains(subject, "this creature") || strings.Contains(subject, "that creature") ||
		strings.HasSuffix(subject, " it") || subject == "it" {
		var best *Permanent
		bestPow := -1
		for _, p := range bf {
			if p == nil || p.PhasedOut || !gs.IsCreatureOf(p) {
				continue
			}
			if pw := gs.PowerOf(p); pw > bestPow {
				bestPow, best = pw, p
			}
		}
		if best != nil {
			return []*Permanent{best}
		}
	}
	return nil
}

func permHasGranted(p *Permanent, kw string) bool {
	if p == nil {
		return false
	}
	for _, a := range p.GrantedAbilities {
		if a == kw {
			return true
		}
	}
	return false
}

// leadingColorWord returns a WUBRG color word if the subject names one
// ("white creatures you control"), else "".
func leadingColorWord(subject string) string {
	for _, c := range []string{"white", "blue", "black", "red", "green"} {
		if strings.Contains(subject, c+" creature") {
			return c
		}
	}
	return ""
}

func permHasColor(gs *GameState, p *Permanent, colorWord string) bool {
	letter := map[string]string{"white": "W", "blue": "U", "black": "B", "red": "R", "green": "G"}[colorWord]
	if letter == "" || p == nil || p.Card == nil {
		return false
	}
	for _, c := range p.Card.Colors {
		if strings.EqualFold(c, letter) {
			return true
		}
	}
	return false
}

package gameengine

import (
	"regexp"
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// mod_tail_conditional_keyword.go — generic handler for the biggest
// cleanly-evaluable conditional static parsed_tail family:
//
//	"This creature has <evergreen keyword> as long as you control a/an/
//	 another <color>/<type>/<subtype> [permanent|creature]."
//
// ~37 cards (Wingrattle Scarecrow, Markov Crusader, Pterodon Knight,
// Aeronaut Tinkerer, Armory Guard, Toxic Iguanar, …). Thor emitted these
// as inert parsed_tail text, so the conditional keyword was never
// conferred. They are STATIC (continuous), so the resolveResidualByText
// resolution hook is the wrong place — instead this registers a layer-6
// keyword grant on the SOURCE, gated by a DYNAMIC predicate that
// re-evaluates "do I control ≥1 matching permanent" on every
// characteristic computation. Layer-6 keyword grants are honored by
// combat + targeting (hex-dev-keywords-10 fix).
//
// CONSERVATIVE: only the regular "you control a/an/another <X>" condition
// shape is parsed; the keyword clause is filtered through the honored
// evergreen whitelist (parseAttachmentKeywords); any condition we cannot
// confidently evaluate is skipped (the clause stays inert) — never
// granted on a guess, so the engine's correctness is never regressed.
//
// Registered from RegisterContinuousEffectsForPermanent (layers.go),
// alongside the other parsed-tail static registrars.

var reTailCondKw = regexp.MustCompile(
	`(?i)^(?:this creature|this permanent|~) has (.+?) as long as you control (a|an|another) (.+?)\.?$`)

var colorWord = map[string]string{
	"white": "W", "blue": "U", "black": "B", "red": "R", "green": "G",
}

// cardTypeWord is the set of head nouns that name a card type (vs a subtype).
var cardTypeWord = map[string]bool{
	"creature": true, "permanent": true, "artifact": true, "enchantment": true,
	"land": true, "planeswalker": true, "instant": true, "sorcery": true,
}

func registerConditionalKeywordTails(gs *GameState, p *Permanent) {
	if gs == nil || p == nil || p.Card == nil || p.Card.AST == nil {
		return
	}
	src := p
	for i, ab := range p.Card.AST.Abilities {
		st, ok := ab.(*gameast.Static)
		if !ok || st.Modification == nil || st.Modification.ModKind != "parsed_tail" {
			continue
		}
		raw := ""
		for _, a := range st.Modification.Args {
			if s, ok := a.(string); ok && len(s) > len(raw) {
				raw = s
			}
		}
		raw = stripAbilityWordPrefix(strings.ToLower(strings.TrimSpace(raw)))
		m := reTailCondKw.FindStringSubmatch(raw)
		if m == nil {
			continue
		}
		kws := parseAttachmentKeywords(m[1]) // honored-evergreen whitelist
		if len(kws) == 0 {
			continue
		}
		another := strings.EqualFold(m[2], "another")
		matcher, ok := parseControlMatcher(m[3])
		if !ok {
			continue // condition not cleanly evaluable — leave inert
		}
		cond := func(g *GameState) bool {
			if src.Controller < 0 || src.Controller >= len(g.Seats) || g.Seats[src.Controller] == nil {
				return false
			}
			for _, q := range g.Seats[src.Controller].Battlefield {
				if q == nil {
					continue
				}
				if another && q == src {
					continue
				}
				if matcher(g, q) {
					return true
				}
			}
			return false
		}
		disc := itoaLayers(i)
		for _, kw := range kws {
			pred := func(g *GameState, t *Permanent) bool {
				return t == src && cond(g)
			}
			registerKeywordGrant(gs, p, kw, "ast-cond-kw-"+disc+"-"+kw, pred)
		}
	}
}

// stripAbilityWordPrefix removes a leading "ability word — " prefix
// ("alliance — …") if present, mirroring the parser's tail framing.
func stripAbilityWordPrefix(r string) string {
	if i := strings.IndexAny(r, "-—"); i > 0 && i < 26 {
		// only treat as a prefix when it's "<short words> - <rest>"
		head := strings.TrimSpace(r[:i])
		if head != "" && len(head) < 24 && !strings.ContainsAny(head, ".,;:") &&
			head != "this creature" && head != "this permanent" {
			rest := strings.TrimSpace(strings.TrimLeft(r[i:], "-— "))
			if rest != "" {
				return rest
			}
		}
	}
	return r
}

// parseControlMatcher turns a "you control a/an/another <X>" object phrase
// into a permanent matcher. Handles "<color> creature/permanent",
// bare card types, single subtypes, "colorless creature", and "A or B"
// (the permanent-side of each alternative). Returns ok=false for shapes it
// can't confidently evaluate (e.g. graveyard riders) so the caller skips.
func parseControlMatcher(phrase string) (func(*GameState, *Permanent) bool, bool) {
	phrase = strings.TrimSpace(strings.ToLower(phrase))
	// "A or B" → OR the parseable alternatives.
	if strings.Contains(phrase, " or ") {
		var ms []func(*GameState, *Permanent) bool
		for _, part := range strings.Split(phrase, " or ") {
			part = strings.TrimSpace(part)
			// drop graveyard / non-permanent riders we can't model here
			if strings.Contains(part, "graveyard") || strings.Contains(part, "there is") {
				continue
			}
			if mm, ok := parseSingleControlMatcher(part); ok {
				ms = append(ms, mm)
			}
		}
		if len(ms) == 0 {
			return nil, false
		}
		return func(g *GameState, q *Permanent) bool {
			for _, mm := range ms {
				if mm(g, q) {
					return true
				}
			}
			return false
		}, true
	}
	return parseSingleControlMatcher(phrase)
}

func parseSingleControlMatcher(phrase string) (func(*GameState, *Permanent) bool, bool) {
	words := strings.Fields(phrase)
	if len(words) == 0 {
		return nil, false
	}
	head := words[len(words)-1] // e.g. "creature", "permanent", "artifact", or a subtype
	// strip a leading planeswalker-subtype qualifier: "liliana planeswalker"
	// → require a planeswalker (ignore the specific walker subtype).
	colorLetter := ""
	colorless := false
	for _, w := range words[:len(words)-1] {
		if l, ok := colorWord[w]; ok {
			colorLetter = l
		} else if w == "colorless" {
			colorless = true
		}
	}

	switch {
	case head == "permanent":
		if colorLetter != "" {
			return func(_ *GameState, q *Permanent) bool { return q.Card != nil && CardHasColor(q.Card, colorLetter) }, true
		}
		if colorless {
			return func(_ *GameState, q *Permanent) bool { return q.Card != nil && len(q.Card.Colors) == 0 }, true
		}
		return func(_ *GameState, q *Permanent) bool { return true }, true
	case head == "creature":
		if colorLetter != "" {
			return func(_ *GameState, q *Permanent) bool {
				return q.IsCreature() && q.Card != nil && CardHasColor(q.Card, colorLetter)
			}, true
		}
		if colorless {
			return func(_ *GameState, q *Permanent) bool { return q.IsCreature() && q.Card != nil && len(q.Card.Colors) == 0 }, true
		}
		return func(_ *GameState, q *Permanent) bool { return q.IsCreature() }, true
	case head == "artifact":
		return func(_ *GameState, q *Permanent) bool { return q.IsArtifact() }, true
	case head == "enchantment":
		return func(_ *GameState, q *Permanent) bool { return q.IsEnchantment() }, true
	case head == "land":
		return func(_ *GameState, q *Permanent) bool { return q.IsLand() }, true
	case head == "planeswalker":
		return func(_ *GameState, q *Permanent) bool { return q.IsPlaneswalker() }, true
	default:
		// Treat head as a subtype (dinosaur, gate, vampire, rogue, …).
		sub := head
		return func(_ *GameState, q *Permanent) bool { return permanentHasSubtype(q, sub) }, true
	}
}

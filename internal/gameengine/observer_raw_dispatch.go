package gameengine

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// observer_raw_dispatch.go — raw-aware observer-trigger classification
// (r63 PROGRESSION widening; the same recover-from-raw pattern as
// attack_trigger_dispatch.go and triggerControllerMatchesRaw).
//
// The parser drops trigger ACTOR phrases, so the corpus's observer-class
// triggers (cast_filtered/cast_any/opp_cast ~1,000; another_typed_enters/
// tribe_you_control_etb/etb_or_another ~290) arrive with Actor == nil.
// Both observer walks gated on `isSelfTrigger` (nil actor ⇒ self by
// convention) and on a non-nil Actor for filter matching — so the
// entire observer dispatch was DEAD for these families: Ajani's Welcome
// never gained life, Archmage of Runes never drew. The PROGRESSION
// checker's phase-3 widening exposed the class.
//
// The event NAME carries the observer semantics; the type/color/spell
// filters are recovered from the raw oracle text fail-closed —
// unrecognized wordings keep the historical (silent) behavior rather
// than guessing.

// observerOnlyEvents are trigger events that are NEVER self-triggers —
// their bearers observe someone else's action.
var observerOnlyEvents = map[string]bool{
	"another_typed_enters":   true,
	"tribe_you_control_etb":  true,
	"another_typed_etb":      true,
	"ally_etb":               true,
	"creature_etb_any":       true,
	"cast_filtered":          true,
	"cast_any":               true,
	"cast_spell":             true,
	"any_player_cast":        true,
	"opp_cast":               true,
	"cast_color_spell":       true,
	"another_typed_dies":     true,
	"tribe_you_control_dies": true,
}

// dualSelfObserverEvents fire BOTH when the bearer itself acts and when
// an ally does ("whenever ~ or another creature you control enters").
var dualSelfObserverEvents = map[string]bool{
	"etb_or_another": true,
}

// isObserverClassEvent reports whether the event name denotes an
// observer (or dual self+observer) trigger despite a nil Actor.
func isObserverClassEvent(event string) bool {
	e := strings.ToLower(strings.TrimSpace(event))
	return observerOnlyEvents[e] || dualSelfObserverEvents[e]
}

// allyETBRawSpec is the recovered "ally enters" filter.
type allyETBRawSpec struct {
	includesSelf bool
	excludesSelf bool   // "another ..."
	subtype      string // single-token subtype ("zombie") or ""
	colorLetter  string // "G" etc. or ""
}

var observerColorWords = map[string]string{
	"white": "W", "blue": "U", "black": "B", "red": "R", "green": "G",
}

// allyETBSpecFromRaw recovers the ally-ETB filter from the oracle
// wording. ok=false means unrecognized — caller must NOT fire.
func allyETBSpecFromRaw(raw string) (allyETBRawSpec, bool) {
	r := strings.ToLower(raw)
	none := allyETBRawSpec{}
	if strings.Contains(r, "or another creature you control enters") {
		return allyETBRawSpec{includesSelf: true}, true
	}
	if i := strings.Index(r, "whenever another "); i >= 0 {
		rest := r[i+len("whenever another "):]
		if strings.HasPrefix(rest, "creature you control enters") {
			return allyETBRawSpec{excludesSelf: true}, true
		}
		if j := strings.Index(rest, " creature you control enters"); j > 0 {
			word := rest[:j]
			if !strings.Contains(word, " ") {
				if letter, isColor := observerColorWords[word]; isColor {
					return allyETBRawSpec{excludesSelf: true, colorLetter: letter}, true
				}
				return allyETBRawSpec{excludesSelf: true, subtype: word}, true
			}
		}
		return none, false
	}
	for _, lead := range []string{"whenever a ", "whenever an "} {
		if i := strings.Index(r, lead); i >= 0 {
			rest := r[i+len(lead):]
			if j := strings.Index(rest, " you control enters"); j > 0 {
				word := rest[:j]
				if word == "creature" {
					return allyETBRawSpec{includesSelf: true}, true
				}
				if !strings.Contains(word, " ") {
					return allyETBRawSpec{subtype: word}, true
				}
			}
			return none, false
		}
	}
	return none, false
}

// observerETBMatchesByRaw matches a nil-Actor ally-ETB trigger against
// the entering permanent. Controller-gated by construction (every
// recognized wording is a "you control" form).
func observerETBMatchesByRaw(trig *gameast.Triggered, observer, entering *Permanent) bool {
	spec, ok := allyETBSpecFromRaw(trig.Raw)
	if !ok {
		return false
	}
	if entering.Controller != observer.Controller {
		return false
	}
	if spec.excludesSelf && entering == observer {
		return false
	}
	if !entering.IsCreature() && (entering.Card == nil || !cardHasType(entering.Card, "creature")) {
		// All recognized wordings are creature-scoped; subtype forms
		// ("a zombie you control") still require the type below.
		if spec.subtype == "" {
			return false
		}
	}
	if spec.subtype != "" {
		if entering.Card == nil ||
			(!cardHasSubtype(entering.Card, spec.subtype) && !entering.hasType(spec.subtype)) {
			return false
		}
	}
	if spec.colorLetter != "" {
		found := false
		if entering.Card != nil {
			for _, c := range entering.Card.Colors {
				if strings.EqualFold(c, spec.colorLetter) {
					found = true
					break
				}
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// castRawSpec is the recovered cast-trigger scope+filter.
type castRawSpec struct {
	who    string // "you" / "any" / "opp"
	filter string // "any" / "noncreature" / "iss" / "creature" / "artifact" / "enchantment"
}

// castSpecFromRaw recovers who/filter from the oracle wording.
// ok=false ⇒ unrecognized ⇒ caller must NOT fire (mana-value riders,
// from-exile clauses, copy clauses all fail closed).
func castSpecFromRaw(raw string) (castRawSpec, bool) {
	r := strings.ToLower(raw)
	none := castRawSpec{}
	var who, rest string
	switch {
	case strings.Contains(r, "whenever you cast "):
		who = "you"
		rest = r[strings.Index(r, "whenever you cast ")+len("whenever you cast "):]
	case strings.Contains(r, "whenever a player casts "):
		who = "any"
		rest = r[strings.Index(r, "whenever a player casts ")+len("whenever a player casts "):]
	case strings.Contains(r, "whenever an opponent casts "):
		who = "opp"
		rest = r[strings.Index(r, "whenever an opponent casts ")+len("whenever an opponent casts "):]
	default:
		return none, false
	}
	var filter string
	switch {
	case strings.HasPrefix(rest, "a spell,"):
		filter = "any"
	case strings.HasPrefix(rest, "a noncreature spell,"):
		filter = "noncreature"
	case strings.HasPrefix(rest, "an instant or sorcery spell,"):
		filter = "iss"
	case strings.HasPrefix(rest, "a creature spell,"):
		filter = "creature"
	case strings.HasPrefix(rest, "an artifact spell,"):
		filter = "artifact"
	case strings.HasPrefix(rest, "an enchantment spell,"):
		filter = "enchantment"
	default:
		return none, false
	}
	return castRawSpec{who: who, filter: filter}, true
}

// observerCastMatchesByRaw matches a nil-Actor cast trigger against the
// cast (caster seat + spell card).
func observerCastMatchesByRaw(trig *gameast.Triggered, observer *Permanent, casterSeat int, card *Card) bool {
	spec, ok := castSpecFromRaw(trig.Raw)
	if !ok {
		return false
	}
	switch spec.who {
	case "you":
		if casterSeat != observer.Controller {
			return false
		}
	case "opp":
		if casterSeat == observer.Controller {
			return false
		}
	}
	switch spec.filter {
	case "noncreature":
		return !cardHasType(card, "creature")
	case "iss":
		return cardHasType(card, "instant") || cardHasType(card, "sorcery")
	case "creature":
		return cardHasType(card, "creature")
	case "artifact":
		return cardHasType(card, "artifact")
	case "enchantment":
		return cardHasType(card, "enchantment")
	}
	return true // "any"
}

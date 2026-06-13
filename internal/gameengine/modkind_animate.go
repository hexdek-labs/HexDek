package gameengine

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// modkind_animate.go — manland / creature-land animation (CR §706).
//
// Two parser shapes reach here:
//
//   animate          args = [power, toughness, (type)]  — the "Restless"
//                    cycle and newer becomes-a-N/N lands. (was already
//                    handled inline in resolve_helpers.go; now routed
//                    through applyAnimateUntilEOT so the added creature
//                    type is reverted at cleanup.)
//
//   animate_subject  args = [["subj","this land"],["pt","2/1"],
//                    ["descr","blue faerie"],["type","creature"]] —
//                    24 iconic manlands (Mutavault, Mishra's Factory,
//                    Inkmoth/Blinkmoth Nexus, Treetop Village, Faerie
//                    Conclave, Ghitu Encampment, …). This shape had NO
//                    handler and fell through the switch default — every
//                    one of these lands was INERT (activating did
//                    nothing). This file makes them animate.
//
// Scope / known limitations (documented in the audit report):
//   - Keywords printed on the manland ("with flying/trample/first
//     strike/vigilance") are not in the structured args (raw-only), so
//     they are not granted — same pre-existing limitation as the plain
//     `animate` case.
//   - Subtypes from the description are not added (only the core
//     creature / artifact types) to avoid parsing noise from truncated
//     descr text.
//   - Summoning sickness: lands enter with SummoningSick unset, so a
//     manland played AND animated on the same turn is not prevented from
//     attacking. This pre-dates the fix (affects the plain `animate`
//     cards too) and needs a land-ETB sickness change to close.

var reAnimatePT = regexp.MustCompile(`(\d+)/(\d+)`)

// applyAnimateUntilEOT turns src into a creature with the given P/T and
// extra card types until end of turn (CR §706/§613 layer 7b for P/T,
// layer 4 for the type-add). The P/T is an until-EOT Modification (auto-
// reverted by the §514.2 cleanup); the added types are recorded in
// src.AnimatedAddedTypes and stripped by the same cleanup so the land
// does not remain a creature once the turn ends. SummoningSick is left
// untouched — animation does not change how long the permanent has been
// under its controller's control.
func applyAnimateUntilEOT(gs *GameState, src *Permanent, pow, tough int, addTypes []string) {
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	src.Modifications = append(src.Modifications, Modification{
		Power:     pow - src.Card.BasePower,
		Toughness: tough - src.Card.BaseToughness,
		Duration:  "until_end_of_turn",
		Timestamp: gs.NextTimestamp(),
	})
	addType := func(t string) {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" || src.hasType(t) {
			return
		}
		src.Card.Types = append(src.Card.Types, t)
		src.AnimatedAddedTypes = append(src.AnimatedAddedTypes, t)
	}
	for _, t := range addTypes {
		addType(t)
	}
	// A becomes-a-creature animation always grants the creature type.
	addType("creature")
	gs.InvalidateCharacteristicsCache()
}

// resolveAnimateSubject parses the `animate_subject` key/value arg pairs
// and animates the source land (CR §706). Targeted variants (Mech Hangar
// "target vehicle") are not self-animations and are left for a per_card
// handler — logged as a gap rather than mis-animating the land itself.
func resolveAnimateSubject(gs *GameState, src *Permanent, e *gameast.ModificationEffect) {
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	fields := map[string]string{}
	for _, a := range e.Args {
		pair, ok := a.([]interface{})
		if !ok || len(pair) != 2 {
			continue
		}
		k, _ := pair[0].(string)
		v, _ := pair[1].(string)
		fields[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
	}

	subj := strings.ToLower(fields["subj"])
	if subj != "" && !animateSelfSubject(subj) {
		gs.LogEvent(Event{
			Kind:   "animate_subject_skipped",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{
				"subj":   subj,
				"reason": "non_self_subject_needs_per_card",
			},
		})
		return
	}

	pow, tough, ok := parseAnimatePT(fields["pt"])
	if !ok {
		// Mutavault / Soulstone Sanctuary / Faceless Haven carry the P/T
		// in the description ("2/2", "3/3", "4/3") with an empty pt arg.
		pow, tough, ok = parseAnimatePT(fields["descr"])
	}
	if !ok {
		gs.LogEvent(Event{
			Kind:   "animate_subject_skipped",
			Seat:   controllerSeat(src),
			Source: sourceName(src),
			Details: map[string]interface{}{"reason": "no_power_toughness"},
		})
		return
	}

	types := []string{}
	blob := strings.ToLower(fields["type"] + " " + fields["descr"])
	if strings.Contains(blob, "artifact") {
		types = append(types, "artifact")
	}
	types = append(types, "creature")

	applyAnimateUntilEOT(gs, src, pow, tough, types)
	gs.LogEvent(Event{
		Kind:   "animate",
		Seat:   controllerSeat(src),
		Source: sourceName(src),
		Details: map[string]interface{}{
			"power":     pow,
			"toughness": tough,
			"types":     types,
			"shape":     "animate_subject",
		},
	})
}

// animateSelfSubject reports whether the animate subject phrase refers to
// the source permanent itself.
func animateSelfSubject(subj string) bool {
	switch subj {
	case "this land", "this permanent", "this creature", "~", "it", "this":
		return true
	}
	return strings.HasPrefix(subj, "this ")
}

// parseAnimatePT extracts an "N/M" power/toughness from a string.
func parseAnimatePT(s string) (int, int, bool) {
	m := reAnimatePT.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, 0, false
	}
	pow, err1 := strconv.Atoi(m[1])
	tough, err2 := strconv.Atoi(m[2])
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return pow, tough, true
}

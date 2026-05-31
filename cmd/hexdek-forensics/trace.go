package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Fabrication-arm violation messages have the canonical shape:
//
//	`ZoneConservation: InstanceID "hSPRCCMSSS" present in a zone but
//	 not in (Minted - Ceased) — fabrication or stale ceased entry`
//
// Disappearance-arm has a different shape and is handled separately
// (those have card_name in the message already; the disappearance
// signal isn't a mint-bypass — it's a missed cease).
var fabricationIDRegex = regexp.MustCompile(`InstanceID "([a-zA-Z0-9]+)" present in a zone but not in \(Minted - Ceased\)`)

// ExtractFabricatedInstanceID pulls the InstanceID out of a fabrication-
// arm ZoneConservation violation message. Returns "" if the message
// doesn't match the fabrication signature (caller should skip).
func ExtractFabricatedInstanceID(message string) string {
	m := fabricationIDRegex.FindStringSubmatch(message)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// DecodedInstanceID is the human-readable expansion of an InstanceID's
// canonical encoding per docs/instanceid-system-v2-r60.md §3.
//
//	Layout: h{seat}{prov}{visibility}{color}{cmc}{seq}
//	  seat:        1 char digit (0-3 typical, 4 chars max)
//	  prov:        2 chars (OG | TK | CP | AB)
//	  visibility:  1 char (V = visible, H = hidden)
//	  color:       1 char (W U B R G C M = color identity, M = multi)
//	  cmc:         1 char (0-9, then 0 sticky for >9)
//	  seq:         5 chars zero-padded decimal sequence
type DecodedInstanceID struct {
	Raw        string
	Seat       int    // -1 if undecodable
	Provenance string // OG / TK / CP / AB or "<bad>"
	Visibility string // V / H or "<bad>"
	Color      string // W U B R G C M or "<bad>"
	CMC        int    // -1 if undecodable
	Sequence   int    // -1 if undecodable
	Valid      bool   // false if the layout didn't match
}

// DecodeInstanceID applies the §3 layout to an InstanceID string.
// Defensive: out-of-shape inputs return Valid=false with the raw
// preserved so the trace report can still print "<undecodable> X".
func DecodeInstanceID(id string) DecodedInstanceID {
	d := DecodedInstanceID{Raw: id, Seat: -1, CMC: -1, Sequence: -1}
	if len(id) < 11 || id[0] != 'h' {
		return d
	}
	seat := int(id[1] - '0')
	if seat < 0 || seat > 9 {
		return d
	}
	prov := id[2:4]
	if prov != "OG" && prov != "TK" && prov != "CP" && prov != "AB" {
		return d
	}
	visibility := string(id[4])
	if visibility != "V" && visibility != "H" {
		return d
	}
	color := string(id[5])
	switch color {
	case "W", "U", "B", "R", "G", "C", "M":
	default:
		return d
	}
	cmc := int(id[6] - '0')
	if cmc < 0 || cmc > 9 {
		return d
	}
	seqStr := id[7:]
	var seq int
	if _, err := fmt.Sscanf(seqStr, "%d", &seq); err != nil {
		return d
	}
	return DecodedInstanceID{
		Raw:        id,
		Seat:       seat,
		Provenance: prov,
		Visibility: visibility,
		Color:      color,
		CMC:        cmc,
		Sequence:   seq,
		Valid:      true,
	}
}

// FirstAppearance is the result of walking the event log for the first
// reference to a fabricated InstanceID — typically the mint-bypass site
// or the closest causal event for further bisect.
type FirstAppearance struct {
	EventIdx  int                   // -1 if not found
	Event     gameengine.Event      // zero-value if not found
	MatchKind string                // "instance_id" / "card_name" / "<none>"
	NotFound  bool
}

// TraceFirstAppearance walks events looking for the first one that
// mentions the InstanceID directly (in Details["instance_id"]) OR the
// card's display name (in event.Source). For fabricated IDs (which by
// definition are NOT in any Mint event), the name-match path is the
// usual win — it surfaces the first canonical engine event (creature_
// dies, zone_change, enter_battlefield, ...) that observed the rogue
// *Card after some non-Mint helper constructed it.
//
// Returns NotFound=true when neither key matches across all events.
// In that case the *Card likely came from a struct-literal in test
// scaffold land — the production engine logs SOME event on every real
// card-bearing operation.
func TraceFirstAppearance(events []gameengine.Event, id, cardName string) FirstAppearance {
	for i, ev := range events {
		// Direct ID match — strongest signal.
		if ev.Details != nil {
			if got, ok := ev.Details["instance_id"]; ok {
				if s, ok := got.(string); ok && s == id {
					return FirstAppearance{EventIdx: i, Event: ev, MatchKind: "instance_id"}
				}
			}
		}
		// Card-name match — fallback when the event doesn't carry IID
		// (most engine events log the name in Source).
		if cardName != "" && ev.Source == cardName {
			return FirstAppearance{EventIdx: i, Event: ev, MatchKind: "card_name"}
		}
	}
	return FirstAppearance{EventIdx: -1, NotFound: true, MatchKind: "<none>"}
}

// TraceResult is the per-violation forensics output. Captures the
// violation context + decoded InstanceID + first-appearance event so
// the caller can render a unified trace report.
type TraceResult struct {
	Violation   ViolationRecord
	InstanceID  string
	Decoded     DecodedInstanceID
	CardName    string
	First       FirstAppearance
}

// AnalyzeReplay walks the replay's violations and produces a
// TraceResult per fabrication-arm hit. Non-fabrication violations are
// skipped — they have separate forensics workflows (disappearance
// needs cease-path audit; ExileLinkageIntegrity needs LTB-path audit).
func AnalyzeReplay(r *Replay) []TraceResult {
	if r == nil {
		return nil
	}
	var out []TraceResult
	for _, v := range r.Violations {
		id := ExtractFabricatedInstanceID(v.Message)
		if id == "" {
			continue
		}
		name := r.CardIndex[id]
		tr := TraceResult{
			Violation:  v,
			InstanceID: id,
			Decoded:    DecodeInstanceID(id),
			CardName:   name,
			First:      TraceFirstAppearance(r.Events, id, name),
		}
		out = append(out, tr)
	}
	return out
}

// RenderTrace formats one TraceResult as a multi-line human-readable
// block for the CLI to print. Idempotent / pure — used by main + tests.
func RenderTrace(t TraceResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "═══ violation @ turn %d ═══\n", t.Violation.Turn)
	fmt.Fprintf(&b, "  invariant : %s\n", t.Violation.Invariant)
	fmt.Fprintf(&b, "  message   : %s\n", t.Violation.Message)
	fmt.Fprintf(&b, "  instance  : %s", t.InstanceID)
	if t.Decoded.Valid {
		fmt.Fprintf(&b, "  (seat=%d prov=%s vis=%s color=%s cmc=%d seq=%d)",
			t.Decoded.Seat, t.Decoded.Provenance, t.Decoded.Visibility,
			t.Decoded.Color, t.Decoded.CMC, t.Decoded.Sequence)
	} else {
		fmt.Fprint(&b, "  (undecodable)")
	}
	fmt.Fprintln(&b)
	if t.CardName != "" {
		fmt.Fprintf(&b, "  card name : %s\n", t.CardName)
	} else {
		fmt.Fprintln(&b, "  card name : <not in CardIndex — replay missing snapshot>")
	}
	if t.First.NotFound {
		fmt.Fprintln(&b, "  first ref : <none in event log — likely struct-literal mint-bypass>")
	} else {
		fmt.Fprintf(&b, "  first ref : event #%d (matched via %s)\n", t.First.EventIdx, t.First.MatchKind)
		fmt.Fprintf(&b, "              kind=%q source=%q seat=%d\n",
			t.First.Event.Kind, t.First.Event.Source, t.First.Event.Seat)
		if len(t.First.Event.Details) > 0 {
			fmt.Fprintf(&b, "              details=%v\n", t.First.Event.Details)
		}
	}
	return b.String()
}

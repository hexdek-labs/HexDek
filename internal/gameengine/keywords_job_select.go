package gameengine

// keywords_job_select.go — Job Select (Aetherdrift) mechanic stub.
//
// Job Select is a static ability of a spell that functions on the stack.
// "Job select — [list]" means "As you cast this spell, choose one of the
// listed jobs. The chosen job is recorded on the permanent (or on the spell
// while it's on the stack) and modifies the resolving spell's effect, and/or
// grants the resulting permanent a job-specific characteristic (typically a
// creature-type or ability rider) until that permanent leaves the battlefield."
//
// Once a job has been selected, it can't be changed while the spell is on
// the stack or the permanent is on the battlefield except by an effect that
// explicitly re-selects.
//
// This file is a STUB: it provides the helpers callers need
// (HasJobSelect, JobSelectOptions, JobSelectChoice, SetJobSelectChoice,
// ChooseJobSelect) and the event hooks for the rules engine, but the
// job-specific characteristic dispatch — applying job riders, gating
// effects on chosen job — is left to a follow-up wired through the
// per_card layer once Aetherdrift card data lands. The cast-time
// validation and resolve-time tracking are implemented and tested.

import (
	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// HasJobSelect / JobSelectOptions
// ---------------------------------------------------------------------------

// HasJobSelect returns true if the card has the job_select keyword in
// its AST.
func HasJobSelect(card *Card) bool {
	if card == nil || card.AST == nil {
		return false
	}
	for _, ab := range card.AST.Abilities {
		if kw, ok := ab.(*gameast.Keyword); ok && keywordNameEquals(kw, "job select") {
			return true
		}
	}
	return false
}

// JobSelectOptions returns the ordered list of job names declared on
// the card's job_select keyword. Each option is the lowercased,
// whitespace-trimmed name pulled directly from the keyword args (the
// printed labels — e.g. "Pilot", "Captain", "Engineer", "Gunner").
//
// Returns nil if the card has no job_select keyword.
func JobSelectOptions(card *Card) []string {
	if card == nil || card.AST == nil {
		return nil
	}
	for _, ab := range card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		if !keywordNameEquals(kw, "job select") {
			continue
		}
		out := make([]string, 0, len(kw.Args))
		for _, arg := range kw.Args {
			switch v := arg.(type) {
			case string:
				if v == "" {
					continue
				}
				out = append(out, normalizeJob(v))
			}
		}
		return out
	}
	return nil
}

// normalizeJob lowercases and trims ASCII whitespace from a job label
// so equality checks are robust across deckfiles and JSON.
func normalizeJob(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

// ---------------------------------------------------------------------------
// JobSelectChoice / SetJobSelectChoice
// ---------------------------------------------------------------------------

// JobSelectChoice returns the currently-chosen job recorded on a
// permanent ("" if none has been chosen, or if the permanent does not
// have job select). Reads from p.Flags via the dedicated key
// "job_select_choice:<normalizedJobName>" set to 1 — only one such key
// is ever present at a time (job uniqueness constraint).
func JobSelectChoice(p *Permanent) string {
	if p == nil || p.Flags == nil {
		return ""
	}
	const prefix = "job_select_choice:"
	for k, v := range p.Flags {
		if v != 1 {
			continue
		}
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			return k[len(prefix):]
		}
	}
	return ""
}

// SetJobSelectChoice records `job` as the chosen job on the permanent.
// Existing job_select_choice flags are cleared first so only one job
// is ever active (job uniqueness constraint). The job name is normalized
// (lowercased, trimmed) before storage so it can be matched against
// JobSelectOptions output. Passing "" clears the choice entirely (for
// effects that explicitly re-select).
func SetJobSelectChoice(p *Permanent, job string) {
	if p == nil {
		return
	}
	if p.Flags == nil {
		p.Flags = map[string]int{}
	}
	const prefix = "job_select_choice:"
	for k := range p.Flags {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(p.Flags, k)
		}
	}
	if job == "" {
		return
	}
	p.Flags[prefix+normalizeJob(job)] = 1
}

// ---------------------------------------------------------------------------
// ChooseJobSelect — cast-time choice (CR §702.183a)
// ---------------------------------------------------------------------------

// ChooseJobSelect records the caster's job choice on a StackItem at
// cast time. CR §702.183a — the choice is made "as you cast," so
// callers should invoke this from within their CastSpellWithCosts /
// CastFromZone happy path, after the StackItem has been constructed
// but before PushStackItem.
//
// On success the choice is written to item.CostMeta["job_select"] and
// a "job_select_chosen" event is logged. The resolve path (or the
// per_card ETB hook for permanent spells) is responsible for
// transferring the choice onto the resulting permanent via
// SetJobSelectChoice once the spell resolves.
//
// Returns CastError if:
//   - item or item.Card is nil
//   - the card has no job_select keyword
//   - `job` is not one of the card's listed options (case-insensitive)
func ChooseJobSelect(gs *GameState, item *StackItem, job string) error {
	if item == nil || item.Card == nil {
		return &CastError{Reason: "nil_stack_item"}
	}
	if !HasJobSelect(item.Card) {
		return &CastError{Reason: "no_job_select"}
	}
	want := normalizeJob(job)
	if want == "" {
		return &CastError{Reason: "empty_job_choice"}
	}
	options := JobSelectOptions(item.Card)
	matched := false
	for _, opt := range options {
		if opt == want {
			matched = true
			break
		}
	}
	if !matched {
		return &CastError{Reason: "invalid_job_choice"}
	}
	if item.CostMeta == nil {
		item.CostMeta = map[string]interface{}{}
	}
	item.CostMeta["job_select"] = want

	if gs != nil {
		gs.LogEvent(Event{
			Kind:   "job_select_chosen",
			Seat:   item.Controller,
			Source: item.Card.DisplayName(),
			Details: map[string]interface{}{
				"job":  want,
				"rule": "702.183a",
			},
		})
	}
	return nil
}

// JobSelectFromStack reads the chosen job off a StackItem's CostMeta
// (the canonical location set by ChooseJobSelect). Returns "" if no
// choice has been recorded. Used by the resolve path to transfer the
// choice onto the resulting permanent.
func JobSelectFromStack(item *StackItem) string {
	if item == nil || item.CostMeta == nil {
		return ""
	}
	v, ok := item.CostMeta["job_select"]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

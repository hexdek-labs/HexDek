package astload

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// Known `__ast_type__` discriminator strings emitted by
// scripts/export_ast_dataset.py. Kept as a package-level var (not a typed
// const block) because it mirrors the exporter's typename list verbatim;
// adding a new AST type in Python should require exactly one line here.
var knownASTTypes = func() map[string]bool {
	names := []string{
		// Top-level / composite
		"CardAST", "Static", "Activated", "Triggered", "Keyword",
		"Modification", "Trigger", "Cost", "Condition", "Filter",
		"ManaSymbol", "ManaCost", "ScalingAmount",
		// Control-flow effects
		"Sequence", "Choice", "Optional_", "Conditional",
		// Leaf effects
		"Damage", "Draw", "Discard", "Mill", "Scry", "Surveil",
		"CounterSpell", "Destroy", "Exile", "Bounce", "Tutor",
		"Reanimate", "Recurse", "GainLife", "LoseLife", "SetLife",
		"Sacrifice", "CreateToken", "CounterMod", "Buff", "GrantAbility",
		"TapEffect", "UntapEffect", "AddMana", "GainControl", "CopySpell",
		"CopyPermanent", "Fight", "Reveal", "LookAt", "Shuffle",
		"ExtraTurn", "ExtraCombat", "WinGame", "LoseGame", "Replacement",
		"Prevent", "UnknownEffect",
	}
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}()

// decodeCardLine parses one JSONL row into a *gameast.CardAST.
//
// Returns:
//   - card:     the populated AST (nil only if the line's `ast` field is null)
//   - warnings: non-fatal issues encountered decoding THIS card (unknown
//     __ast_type__, malformed NumberOrRef, etc.). Caller tags these with
//     the card name before storing in Corpus.ParseWarnings.
//   - err:      fatal JSON error; skip the line.
func decodeCardLine(line []byte) (*gameast.CardAST, []string, error) {
	// Top-level row shape from export_ast_dataset.py.
	var row struct {
		Name        string          `json:"name"`
		OracleText  string          `json:"oracle_text"`
		TypeLine    string          `json:"type_line"`
		ManaCost    string          `json:"mana_cost"`
		CMC         json.RawMessage `json:"cmc"`
		Colors      []string        `json:"colors"`
		AST         json.RawMessage `json:"ast"`
	}
	if err := json.Unmarshal(line, &row); err != nil {
		return nil, nil, fmt.Errorf("json: %w", err)
	}
	if len(row.AST) == 0 || string(row.AST) == "null" {
		return nil, nil, nil
	}

	d := &decoder{}
	card := d.decodeCardAST(row.AST)
	if card == nil {
		return nil, d.warnings, fmt.Errorf("ast field did not decode to a CardAST")
	}
	// Guarantee the card's Name field is populated even if the nested AST's
	// name was missing — fall back to the top-level row.name.
	if card.Name == "" {
		card.Name = row.Name
	}
	postProcessFightOneSided(card)
	return card, d.warnings, nil
}

// postProcessFightMutual walks the CardAST and sets Fight.Mutual=false for
// "bite" effects — cards whose oracle text says "deals damage equal to its
// power to" rather than "fights". CR §701.12 defines fight as mutual; bite
// effects are one-directional and use the same AST node for simplicity.
func postProcessFightOneSided(card *gameast.CardAST) {
	if card == nil {
		return
	}
	for _, ab := range card.Abilities {
		var raw string
		var eff gameast.Effect
		switch a := ab.(type) {
		case *gameast.Static:
			raw = a.Raw
			if a.Modification != nil {
				for _, e := range a.Modification.Args {
					if ef, ok := e.(gameast.Effect); ok {
						setFightOneSidedFromRaw(ef, raw)
					}
				}
			}
			continue
		case *gameast.Triggered:
			raw = a.Raw
			eff = a.Effect
		case *gameast.Activated:
			raw = a.Raw
			eff = a.Effect
		default:
			continue
		}
		setFightOneSidedFromRaw(eff, raw)
	}
}

func setFightOneSidedFromRaw(eff gameast.Effect, raw string) {
	f, ok := eff.(*gameast.Fight)
	if !ok || f == nil {
		return
	}
	// "fights" = mutual (default, OneSided=false).
	// "deals damage equal to" without "fights" = bite (OneSided=true).
	hasFight := strings.Contains(raw, "fight")
	hasBite := strings.Contains(raw, "deals damage equal to")
	if hasBite && !hasFight {
		f.OneSided = true
	}
}

// decoder accumulates per-card warnings so callers don't need to thread a
// slice through every method signature.
type decoder struct {
	warnings []string
}

func (d *decoder) warnf(format string, args ...interface{}) {
	d.warnings = append(d.warnings, fmt.Sprintf(format, args...))
}

// rawObject unmarshals a json.RawMessage into a map[string]json.RawMessage
// keeping nested values as RawMessage so we can re-dispatch on
// __ast_type__. Returns nil if the raw is null or not an object.
func rawObject(raw json.RawMessage) map[string]json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	return m
}

// astType reads the __ast_type__ discriminator off a raw object.
// Returns "" if the object is null, not an object, or has no discriminator.
func astType(obj map[string]json.RawMessage) string {
	if obj == nil {
		return ""
	}
	raw, ok := obj["__ast_type__"]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}

// -----------------------------------------------------------------------------
// CardAST / Abilities
// -----------------------------------------------------------------------------

func (d *decoder) decodeCardAST(raw json.RawMessage) *gameast.CardAST {
	obj := rawObject(raw)
	if obj == nil {
		return nil
	}
	// Sanity: verify the discriminator (not fatal if missing — we may be
	// called on a wrapper that elided the tag).
	if t := astType(obj); t != "" && t != "CardAST" {
		d.warnf("expected CardAST at root, got %q", t)
	}

	card := &gameast.CardAST{}
	_ = jsonString(obj["name"], &card.Name)
	_ = jsonBool(obj["fully_parsed"], &card.FullyParsed)

	// parse_errors is a tuple[str, ...] on the Python side → JSON array of strings.
	if raw, ok := obj["parse_errors"]; ok && string(raw) != "null" {
		var errs []string
		if err := json.Unmarshal(raw, &errs); err == nil {
			card.ParseErrors = errs
		}
	}

	// abilities is a tuple[Ability, ...] → JSON array of ability objects.
	if raw, ok := obj["abilities"]; ok && string(raw) != "null" {
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err == nil {
			card.Abilities = make([]gameast.Ability, 0, len(items))
			for _, ab := range items {
				if decoded := d.decodeAbility(ab); decoded != nil {
					card.Abilities = append(card.Abilities, decoded)
				}
			}
		}
	}

	return card
}

func (d *decoder) decodeAbility(raw json.RawMessage) gameast.Ability {
	obj := rawObject(raw)
	if obj == nil {
		return nil
	}
	t := astType(obj)
	switch t {
	case "Static":
		return d.decodeStatic(obj)
	case "Activated":
		return d.decodeActivated(obj)
	case "Triggered":
		return d.decodeTriggered(obj)
	case "Keyword":
		return d.decodeKeyword(obj)
	}

	// Python-side corpus quirk: a few cards (Deep Analysis, Overmaster,
	// Borne Upon a Wind) emit a raw effect (e.g. "Draw") directly at the
	// ability position instead of wrapping it in Static/spell_effect.
	// Rather than drop the ability, wrap it in a synthetic Static so the
	// downstream resolver still sees it. This preserves fidelity without
	// modifying gameast/.
	if knownASTTypes[t] && t != "Modification" && t != "Condition" &&
		t != "Trigger" && t != "Cost" && t != "Filter" &&
		t != "ManaSymbol" && t != "ManaCost" && t != "ScalingAmount" &&
		t != "CardAST" {
		if eff := d.decodeEffect(raw); eff != nil {
			d.warnf("ability position held a %q effect; wrapping in synthetic Static/spell_effect", t)
			return &gameast.Static{
				Modification: &gameast.Modification{
					ModKind: "spell_effect",
					Args:    []interface{}{eff},
				},
				Raw: strings.ToLower("(corpus-wrap) " + t),
			}
		}
	}

	d.warnf("unknown ability __ast_type__ %q", t)
	return nil
}

func (d *decoder) decodeStatic(obj map[string]json.RawMessage) *gameast.Static {
	s := &gameast.Static{}
	_ = jsonString(obj["raw"], &s.Raw)
	s.Raw = strings.ToLower(s.Raw)
	if cond := d.decodeCondition(obj["condition"]); cond != nil {
		s.Condition = cond
	}
	if mod := d.decodeModification(obj["modification"]); mod != nil {
		s.Modification = mod
	}
	return s
}

func (d *decoder) decodeActivated(obj map[string]json.RawMessage) *gameast.Activated {
	a := &gameast.Activated{}
	_ = jsonString(obj["raw"], &a.Raw)
	a.Raw = strings.ToLower(a.Raw)
	_ = jsonString(obj["timing_restriction"], &a.TimingRestriction)
	if cost := d.decodeCost(obj["cost"]); cost != nil {
		a.Cost = *cost
	}
	a.Effect = d.decodeEffect(obj["effect"])
	return a
}

func (d *decoder) decodeTriggered(obj map[string]json.RawMessage) *gameast.Triggered {
	t := &gameast.Triggered{}
	_ = jsonString(obj["raw"], &t.Raw)
	t.Raw = strings.ToLower(t.Raw)
	if trig := d.decodeTrigger(obj["trigger"]); trig != nil {
		t.Trigger = *trig
	}
	t.Effect = d.decodeEffect(obj["effect"])
	if cond := d.decodeCondition(obj["intervening_if"]); cond != nil {
		t.InterveningIf = cond
	}
	return t
}

func (d *decoder) decodeKeyword(obj map[string]json.RawMessage) *gameast.Keyword {
	k := &gameast.Keyword{}
	_ = jsonString(obj["name"], &k.Name)
	_ = jsonString(obj["raw"], &k.Raw)
	k.Raw = strings.ToLower(k.Raw)
	// args is a Python tuple — arbitrary values, potentially nested AST.
	if raw, ok := obj["args"]; ok && string(raw) != "null" {
		k.Args = d.decodeArgsList(raw)
	}
	return k
}

// -----------------------------------------------------------------------------
// Modification / Condition / Trigger / Cost / Filter / ManaCost / ManaSymbol
// -----------------------------------------------------------------------------

func (d *decoder) decodeModification(raw json.RawMessage) *gameast.Modification {
	obj := rawObject(raw)
	if obj == nil {
		return nil
	}
	m := &gameast.Modification{}
	_ = jsonString(obj["kind"], &m.ModKind)
	_ = jsonString(obj["layer"], &m.Layer)
	if raw, ok := obj["args"]; ok && string(raw) != "null" {
		m.Args = d.decodeArgsList(raw)
	}
	return m
}

func (d *decoder) decodeCondition(raw json.RawMessage) *gameast.Condition {
	obj := rawObject(raw)
	if obj == nil {
		return nil
	}
	c := &gameast.Condition{}
	_ = jsonString(obj["kind"], &c.Kind)
	if raw, ok := obj["args"]; ok && string(raw) != "null" {
		c.Args = d.decodeArgsList(raw)
	}
	return c
}

func (d *decoder) decodeTrigger(raw json.RawMessage) *gameast.Trigger {
	obj := rawObject(raw)
	if obj == nil {
		return nil
	}
	t := &gameast.Trigger{}
	_ = jsonString(obj["event"], &t.Event)
	_ = jsonString(obj["phase"], &t.Phase)
	_ = jsonString(obj["controller"], &t.Controller)
	if actor := d.decodeFilterPtr(obj["actor"]); actor != nil {
		t.Actor = actor
	}
	if tf := d.decodeFilterPtr(obj["target_filter"]); tf != nil {
		t.TargetFilter = tf
	}
	if cond := d.decodeCondition(obj["condition"]); cond != nil {
		t.Condition = cond
	}
	return t
}

func (d *decoder) decodeCost(raw json.RawMessage) *gameast.Cost {
	obj := rawObject(raw)
	if obj == nil {
		return nil
	}
	c := &gameast.Cost{}
	_ = jsonBool(obj["tap"], &c.Tap)
	_ = jsonBool(obj["untap"], &c.Untap)
	_ = jsonBool(obj["exile_self"], &c.ExileSelf)
	_ = jsonBool(obj["return_self_to_hand"], &c.ReturnSelfToHand)
	// pay_life and discard are Optional[int] → *int.
	if n, ok := jsonIntPtr(obj["discard"]); ok {
		c.Discard = n
	}
	if n, ok := jsonIntPtr(obj["pay_life"]); ok {
		c.PayLife = n
	}
	if sac := d.decodeFilterPtr(obj["sacrifice"]); sac != nil {
		c.Sacrifice = sac
	}
	if mc := d.decodeManaCost(obj["mana"]); mc != nil {
		c.Mana = mc
	}
	// remove_counters is Optional[tuple[int, str]] in Python; JSON array
	// [count, kind] or null.
	if rc, ok := obj["remove_counters"]; ok && string(rc) != "null" {
		var parts []json.RawMessage
		if err := json.Unmarshal(rc, &parts); err == nil && len(parts) == 2 {
			var count int
			var kind string
			if err := json.Unmarshal(parts[0], &count); err == nil {
				c.RemoveCountersN = &count
			}
			if err := json.Unmarshal(parts[1], &kind); err == nil {
				c.RemoveCountersKnd = kind
			}
		}
	}
	// extra is tuple[str, ...].
	if raw, ok := obj["extra"]; ok && string(raw) != "null" {
		var extras []string
		if err := json.Unmarshal(raw, &extras); err == nil {
			c.Extra = extras
		}
	}
	return c
}

func (d *decoder) decodeFilterPtr(raw json.RawMessage) *gameast.Filter {
	obj := rawObject(raw)
	if obj == nil {
		return nil
	}
	f := d.decodeFilterObj(obj)
	return &f
}

func (d *decoder) decodeFilter(raw json.RawMessage) gameast.Filter {
	obj := rawObject(raw)
	if obj == nil {
		return gameast.Filter{}
	}
	return d.decodeFilterObj(obj)
}

func (d *decoder) decodeFilterObj(obj map[string]json.RawMessage) gameast.Filter {
	f := gameast.Filter{}
	_ = jsonString(obj["base"], &f.Base)
	_ = jsonString(obj["quantifier"], &f.Quantifier)
	_ = jsonBool(obj["targeted"], &f.Targeted)
	_ = jsonBool(obj["you_control"], &f.YouControl)
	_ = jsonBool(obj["opponent_controls"], &f.OpponentControls)
	_ = jsonBool(obj["nontoken"], &f.NonToken)
	_ = jsonString(obj["mana_value_op"], &f.ManaValueOp)
	if n, ok := jsonIntPtr(obj["mana_value"]); ok {
		f.ManaValue = n
	}
	// Filter.count is Union[int, str, None] — use NumberOrRef.
	if n := d.decodeNumberOrRef(obj["count"]); n != nil {
		f.Count = n
	}
	f.CreatureTypes = jsonStringSlice(obj["creature_types"])
	f.ColorFilter = jsonStringSlice(obj["color_filter"])
	f.ColorExclude = jsonStringSlice(obj["color_exclude"])
	f.Extra = jsonStringSlice(obj["extra"])
	return f
}

func (d *decoder) decodeManaCost(raw json.RawMessage) *gameast.ManaCost {
	obj := rawObject(raw)
	if obj == nil {
		return nil
	}
	mc := &gameast.ManaCost{}
	if raw, ok := obj["symbols"]; ok && string(raw) != "null" {
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err == nil {
			mc.Symbols = make([]gameast.ManaSymbol, 0, len(items))
			for _, item := range items {
				if sym := d.decodeManaSymbol(item); sym != nil {
					mc.Symbols = append(mc.Symbols, *sym)
				}
			}
		}
	}
	return mc
}

func (d *decoder) decodeManaSymbol(raw json.RawMessage) *gameast.ManaSymbol {
	obj := rawObject(raw)
	if obj == nil {
		return nil
	}
	s := &gameast.ManaSymbol{}
	_ = jsonString(obj["raw"], &s.Raw)
	_ = jsonInt(obj["generic"], &s.Generic)
	_ = jsonBool(obj["is_x"], &s.IsX)
	_ = jsonBool(obj["is_phyrexian"], &s.IsPhyrexian)
	_ = jsonBool(obj["is_snow"], &s.IsSnow)
	s.Color = jsonStringSlice(obj["color"])
	return s
}

// -----------------------------------------------------------------------------
// Effects — big type-switch on __ast_type__.
// -----------------------------------------------------------------------------

func (d *decoder) decodeEffect(raw json.RawMessage) gameast.Effect {
	obj := rawObject(raw)
	if obj == nil {
		return nil
	}
	t := astType(obj)

	switch t {
	// ------ Control flow ------
	case "Sequence":
		e := &gameast.Sequence{}
		if raw, ok := obj["items"]; ok && string(raw) != "null" {
			var items []json.RawMessage
			if err := json.Unmarshal(raw, &items); err == nil {
				e.Items = make([]gameast.Effect, 0, len(items))
				for _, it := range items {
					if dec := d.decodeEffect(it); dec != nil {
						e.Items = append(e.Items, dec)
					}
				}
			}
		}
		return e
	case "Choice":
		e := &gameast.Choice{}
		_ = jsonBool(obj["or_more"], &e.OrMore)
		if raw, ok := obj["options"]; ok && string(raw) != "null" {
			var items []json.RawMessage
			if err := json.Unmarshal(raw, &items); err == nil {
				e.Options = make([]gameast.Effect, 0, len(items))
				for _, it := range items {
					if dec := d.decodeEffect(it); dec != nil {
						e.Options = append(e.Options, dec)
					}
				}
			}
		}
		if pick := d.decodeNumberOrRef(obj["pick"]); pick != nil {
			e.Pick = *pick
		}
		return e
	case "Optional_":
		e := &gameast.Optional_{}
		e.Body = d.decodeEffect(obj["body"])
		return e
	case "Conditional":
		e := &gameast.Conditional{}
		if cond := d.decodeCondition(obj["condition"]); cond != nil {
			e.Condition = cond
		}
		e.Body = d.decodeEffect(obj["body"])
		e.ElseBody = d.decodeEffect(obj["else_body"])
		return e

	// ------ Leaf effects ------
	case "Damage":
		e := &gameast.Damage{}
		if n := d.decodeNumberOrRef(obj["amount"]); n != nil {
			e.Amount = *n
		}
		e.Target = d.decodeFilter(obj["target"])
		_ = jsonBool(obj["divided"], &e.Divided)
		return e
	case "Draw":
		e := &gameast.Draw{}
		if n := d.decodeNumberOrRef(obj["count"]); n != nil {
			e.Count = *n
		}
		e.Target = d.decodeFilter(obj["target"])
		return e
	case "Discard":
		e := &gameast.Discard{}
		if n := d.decodeNumberOrRef(obj["count"]); n != nil {
			e.Count = *n
		}
		e.Target = d.decodeFilter(obj["target"])
		_ = jsonString(obj["chosen_by"], &e.ChosenBy)
		return e
	case "Mill":
		e := &gameast.Mill{}
		if n := d.decodeNumberOrRef(obj["count"]); n != nil {
			e.Count = *n
		}
		e.Target = d.decodeFilter(obj["target"])
		return e
	case "Scry":
		e := &gameast.Scry{}
		if n := d.decodeNumberOrRef(obj["count"]); n != nil {
			e.Count = *n
		}
		return e
	case "Surveil":
		e := &gameast.Surveil{}
		if n := d.decodeNumberOrRef(obj["count"]); n != nil {
			e.Count = *n
		}
		return e
	case "CounterSpell":
		e := &gameast.CounterSpell{}
		e.Target = d.decodeFilter(obj["target"])
		if cost := d.decodeCost(obj["unless"]); cost != nil {
			e.Unless = cost
		}
		return e
	case "Destroy":
		e := &gameast.Destroy{}
		e.Target = d.decodeFilter(obj["target"])
		return e
	case "Exile":
		e := &gameast.Exile{}
		e.Target = d.decodeFilter(obj["target"])
		_ = jsonString(obj["until"], &e.Until)
		_ = jsonBool(obj["face_down"], &e.FaceDown)
		return e
	case "Bounce":
		e := &gameast.Bounce{}
		e.Target = d.decodeFilter(obj["target"])
		_ = jsonString(obj["to"], &e.To)
		return e
	case "Tutor":
		e := &gameast.Tutor{}
		e.Query = d.decodeFilter(obj["query"])
		_ = jsonString(obj["destination"], &e.Destination)
		if n := d.decodeNumberOrRef(obj["count"]); n != nil {
			e.Count = *n
		}
		_ = jsonBool(obj["optional"], &e.Optional)
		_ = jsonBool(obj["shuffle_after"], &e.ShuffleAfter)
		_ = jsonBool(obj["reveal"], &e.Reveal)
		_ = jsonString(obj["rest"], &e.Rest)
		return e
	case "Reanimate":
		e := &gameast.Reanimate{}
		e.Query = d.decodeFilter(obj["query"])
		_ = jsonString(obj["from_zone"], &e.FromZone)
		_ = jsonString(obj["destination"], &e.Destination)
		_ = jsonString(obj["controller"], &e.Controller)
		e.WithModifications = jsonStringSlice(obj["with_modifications"])
		return e
	case "Recurse":
		e := &gameast.Recurse{}
		e.Query = d.decodeFilter(obj["query"])
		_ = jsonString(obj["from_zone"], &e.FromZone)
		_ = jsonString(obj["destination"], &e.Destination)
		return e
	case "GainLife":
		e := &gameast.GainLife{}
		if n := d.decodeNumberOrRef(obj["amount"]); n != nil {
			e.Amount = *n
		}
		e.Target = d.decodeFilter(obj["target"])
		return e
	case "LoseLife":
		e := &gameast.LoseLife{}
		if n := d.decodeNumberOrRef(obj["amount"]); n != nil {
			e.Amount = *n
		}
		e.Target = d.decodeFilter(obj["target"])
		return e
	case "SetLife":
		e := &gameast.SetLife{}
		if n := d.decodeNumberOrRef(obj["amount"]); n != nil {
			e.Amount = *n
		}
		e.Target = d.decodeFilter(obj["target"])
		return e
	case "Sacrifice":
		e := &gameast.Sacrifice{}
		e.Query = d.decodeFilter(obj["query"])
		_ = jsonString(obj["actor"], &e.Actor)
		return e
	case "CreateToken":
		e := &gameast.CreateToken{}
		if n := d.decodeNumberOrRef(obj["count"]); n != nil {
			e.Count = *n
		}
		// pt is Optional[tuple[int, int]] → [2]int or null.
		if raw, ok := obj["pt"]; ok && string(raw) != "null" {
			var parts []int
			if err := json.Unmarshal(raw, &parts); err == nil && len(parts) == 2 {
				pt := [2]int{parts[0], parts[1]}
				e.PT = &pt
			}
		}
		e.Types = jsonStringSlice(obj["types"])
		e.Color = jsonStringSlice(obj["color"])
		e.Keywords = jsonStringSlice(obj["keywords"])
		if raw, ok := obj["is_copy_of"]; ok && string(raw) != "null" {
			if fp := d.decodeFilterPtr(raw); fp != nil {
				e.IsCopyOf = fp
			}
		}
		_ = jsonBool(obj["legendary"], &e.Legendary)
		_ = jsonBool(obj["tapped"], &e.Tapped)
		return e
	case "CounterMod":
		e := &gameast.CounterMod{}
		_ = jsonString(obj["op"], &e.Op)
		if n := d.decodeNumberOrRef(obj["count"]); n != nil {
			e.Count = *n
		}
		_ = jsonString(obj["counter_kind"], &e.CounterKind)
		e.Target = d.decodeFilter(obj["target"])
		return e
	case "Buff":
		e := &gameast.Buff{}
		_ = jsonInt(obj["power"], &e.Power)
		_ = jsonInt(obj["toughness"], &e.Toughness)
		e.Target = d.decodeFilter(obj["target"])
		_ = jsonString(obj["duration"], &e.Duration)
		return e
	case "GrantAbility":
		e := &gameast.GrantAbility{}
		_ = jsonString(obj["ability_name"], &e.AbilityName)
		e.Target = d.decodeFilter(obj["target"])
		_ = jsonString(obj["duration"], &e.Duration)
		return e
	case "TapEffect":
		e := &gameast.TapEffect{}
		e.Target = d.decodeFilter(obj["target"])
		return e
	case "UntapEffect":
		e := &gameast.UntapEffect{}
		e.Target = d.decodeFilter(obj["target"])
		return e
	case "AddMana":
		e := &gameast.AddMana{}
		_ = jsonInt(obj["any_color_count"], &e.AnyColorCount)
		if raw, ok := obj["pool"]; ok && string(raw) != "null" {
			var items []json.RawMessage
			if err := json.Unmarshal(raw, &items); err == nil {
				e.Pool = make([]gameast.ManaSymbol, 0, len(items))
				for _, item := range items {
					if sym := d.decodeManaSymbol(item); sym != nil {
						e.Pool = append(e.Pool, *sym)
					}
				}
			}
		}
		return e
	case "GainControl":
		e := &gameast.GainControl{}
		e.Target = d.decodeFilter(obj["target"])
		_ = jsonString(obj["duration"], &e.Duration)
		return e
	case "CopySpell":
		e := &gameast.CopySpell{}
		e.Target = d.decodeFilter(obj["target"])
		_ = jsonBool(obj["may_choose_new_targets"], &e.MayChooseNewTargets)
		return e
	case "CopyPermanent":
		e := &gameast.CopyPermanent{}
		e.Target = d.decodeFilter(obj["target"])
		_ = jsonBool(obj["as_token"], &e.AsToken)
		return e
	case "Fight":
		e := &gameast.Fight{}
		e.A = d.decodeFilter(obj["a"])
		e.B = d.decodeFilter(obj["b"])
		_ = jsonBool(obj["one_sided"], &e.OneSided)
		return e
	case "Reveal":
		e := &gameast.Reveal{}
		_ = jsonString(obj["source"], &e.Source)
		_ = jsonString(obj["actor"], &e.Actor)
		if n := d.decodeNumberOrRef(obj["count"]); n != nil {
			e.Count = *n
		}
		return e
	case "LookAt":
		e := &gameast.LookAt{}
		e.Target = d.decodeFilter(obj["target"])
		_ = jsonString(obj["zone"], &e.Zone)
		if n := d.decodeNumberOrRef(obj["count"]); n != nil {
			e.Count = *n
		}
		return e
	case "Shuffle":
		e := &gameast.Shuffle{}
		e.Target = d.decodeFilter(obj["target"])
		return e
	case "ExtraTurn":
		e := &gameast.ExtraTurn{}
		_ = jsonBool(obj["after_this"], &e.AfterThis)
		e.Target = d.decodeFilter(obj["target"])
		return e
	case "ExtraCombat":
		e := &gameast.ExtraCombat{}
		_ = jsonBool(obj["after_this"], &e.AfterThis)
		return e
	case "WinGame":
		e := &gameast.WinGame{}
		e.Target = d.decodeFilter(obj["target"])
		return e
	case "LoseGame":
		e := &gameast.LoseGame{}
		e.Target = d.decodeFilter(obj["target"])
		return e
	case "Replacement":
		e := &gameast.Replacement{}
		_ = jsonString(obj["trigger_event"], &e.TriggerEvent)
		e.ReplaceWith = d.decodeEffect(obj["replacement"])
		return e
	case "Prevent":
		e := &gameast.Prevent{}
		if n := d.decodeNumberOrRef(obj["amount"]); n != nil {
			e.Amount = *n
		}
		if fp := d.decodeFilterPtr(obj["damage_filter"]); fp != nil {
			e.DamageFilter = fp
		}
		_ = jsonString(obj["duration"], &e.Duration)
		return e
	case "UnknownEffect":
		e := &gameast.UnknownEffect{}
		_ = jsonString(obj["raw_text"], &e.RawText)
		return e
	case "Modification":
		// Wave 1a promotions emit Modification(kind="...", args=(...)) at
		// effect positions for labelled-but-not-yet-typed shapes (goad,
		// phase_out_self, stun, suspect, investigate, etc.). Decode into
		// ModificationEffect so the resolver can dispatch on ModKind.
		m := d.decodeModification(raw)
		if m != nil {
			return &gameast.ModificationEffect{
				ModKind: m.ModKind,
				Args:    m.Args,
			}
		}
		return &gameast.UnknownEffect{RawText: string(raw)}
	}

	// Python-side corpus quirk: some cards (e.g. The Dalek Emperor) emit a
	// Static at the effect position inside a Triggered. Unwrap it: lift the
	// Modification.args[0] (usually an effect node) up to the effect slot,
	// falling back to an UnknownEffect carrying the raw Static text if
	// that doesn't pan out.
	if t == "Static" {
		s := d.decodeStatic(obj)
		if s != nil && s.Modification != nil {
			// Look for an embedded effect node in args.
			for _, a := range s.Modification.Args {
				if e, ok := a.(gameast.Effect); ok {
					d.warnf("Static at effect position; unwrapped its embedded %s", e.Kind())
					return e
				}
			}
			// No embedded effect, but the Modification has a ModKind
			// (e.g. gain_energy). Promote to ModificationEffect so the
			// resolver can dispatch on it instead of falling to UnknownEffect.
			if s.Modification.ModKind != "" {
				d.warnf("Static at effect position; promoting Modification kind=%s to ModificationEffect", s.Modification.ModKind)
				return &gameast.ModificationEffect{
					ModKind: s.Modification.ModKind,
					Args:    s.Modification.Args,
				}
			}
		}
		d.warnf("Static at effect position with no embedded effect; preserving raw")
		if s != nil {
			return &gameast.UnknownEffect{RawText: s.Raw}
		}
		return &gameast.UnknownEffect{RawText: string(raw)}
	}

	// Unknown discriminator — fall through to UnknownEffect with the raw
	// JSON as RawText. Log a warning so the corpus load surfaces it.
	if t == "" {
		d.warnf("effect object with no __ast_type__: %s", previewRaw(raw, 80))
	} else if !knownASTTypes[t] {
		d.warnf("unknown effect __ast_type__ %q; dropping to UnknownEffect", t)
	} else {
		// Known to the parser but routed here means our switch missed a case.
		// Flag loudly.
		d.warnf("effect __ast_type__ %q known but unhandled; dropping to UnknownEffect", t)
	}
	return &gameast.UnknownEffect{RawText: string(raw)}
}

// -----------------------------------------------------------------------------
// NumberOrRef + ScalingAmount
// -----------------------------------------------------------------------------

// decodeNumberOrRef accepts an int, a string, or a ScalingAmount dict and
// returns the discriminated Go union. Returns nil if the raw is null or
// absent (caller decides the default).
func (d *decoder) decodeNumberOrRef(raw json.RawMessage) *gameast.NumberOrRef {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	trimmed := strings.TrimSpace(string(raw))
	if len(trimmed) == 0 {
		return nil
	}
	switch trimmed[0] {
	case '"':
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return gameast.NumStr(s)
		}
	case '{':
		// Expect ScalingAmount dict.
		obj := rawObject(raw)
		if obj == nil {
			return nil
		}
		if t := astType(obj); t != "ScalingAmount" && t != "" {
			d.warnf("expected ScalingAmount in NumberOrRef slot, got %q", t)
			return nil
		}
		sa := d.decodeScalingAmount(obj)
		if sa == nil {
			return nil
		}
		return gameast.NumScaling(sa)
	default:
		// Numeric. Python exports ints, but be liberal with floats too
		// (cmc in the file is float — doesn't apply here, but defensive).
		var n float64
		if err := json.Unmarshal(raw, &n); err == nil {
			return gameast.NumInt(int(n))
		}
	}
	return nil
}

func (d *decoder) decodeScalingAmount(obj map[string]json.RawMessage) *gameast.ScalingAmount {
	if obj == nil {
		return nil
	}
	sa := &gameast.ScalingAmount{}
	_ = jsonString(obj["kind"], &sa.ScalingKind)
	if raw, ok := obj["args"]; ok && string(raw) != "null" {
		sa.Args = d.decodeArgsList(raw)
	}
	return sa
}

// -----------------------------------------------------------------------------
// Heterogeneous args lists
// -----------------------------------------------------------------------------
//
// Python tuples on Modification.args, Keyword.args, Condition.args,
// ScalingAmount.args can contain any mix of strings, ints, nested AST
// nodes, filters, and lists. The Go type is []interface{} to mirror that.
// We walk the raw JSON once and convert any dict-with-__ast_type__ to the
// appropriate Go struct, leaving scalars as their native Go types.

func (d *decoder) decodeArgsList(raw json.RawMessage) []interface{} {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil
	}
	out := make([]interface{}, 0, len(items))
	for _, it := range items {
		out = append(out, d.decodeArg(it))
	}
	return out
}

// decodeArg converts a single tuple element. Strings/numbers/bools/null
// stay scalar; a dict with __ast_type__ gets routed through the AST decoder
// (effects, filters, modifications, scalings, etc.); arrays recurse.
func (d *decoder) decodeArg(raw json.RawMessage) interface{} {
	if len(raw) == 0 {
		return nil
	}
	s := strings.TrimSpace(string(raw))
	if s == "null" {
		return nil
	}
	switch s[0] {
	case '"':
		var v string
		_ = json.Unmarshal(raw, &v)
		return v
	case '[':
		// Recurse on sub-array.
		return d.decodeArgsList(raw)
	case '{':
		obj := rawObject(raw)
		if obj == nil {
			var v interface{}
			_ = json.Unmarshal(raw, &v)
			return v
		}
		t := astType(obj)
		switch t {
		case "":
			// Plain dict (rare in args but possible). Preserve as
			// map[string]interface{} — don't recurse into it.
			var v map[string]interface{}
			_ = json.Unmarshal(raw, &v)
			return v
		case "Filter":
			f := d.decodeFilterObj(obj)
			return &f
		case "ManaSymbol":
			return d.decodeManaSymbol(raw)
		case "ManaCost":
			return d.decodeManaCost(raw)
		case "Modification":
			return d.decodeModification(raw)
		case "Condition":
			return d.decodeCondition(raw)
		case "Trigger":
			return d.decodeTrigger(raw)
		case "Cost":
			return d.decodeCost(raw)
		case "ScalingAmount":
			return d.decodeScalingAmount(obj)
		case "Static", "Activated", "Triggered", "Keyword":
			return d.decodeAbility(raw)
		case "CardAST":
			return d.decodeCardAST(raw)
		default:
			// Assume it's an effect node. decodeEffect handles the
			// full leaf-effect switch + unknown-type fallback.
			return d.decodeEffect(raw)
		}
	case 't', 'f':
		var v bool
		_ = json.Unmarshal(raw, &v)
		return v
	default:
		// Numeric (int or float). Export emits ints, but floats are legal JSON.
		var n float64
		if err := json.Unmarshal(raw, &n); err == nil {
			if n == float64(int(n)) {
				return int(n)
			}
			return n
		}
	}
	return nil
}

// -----------------------------------------------------------------------------
// Small JSON helpers.
// -----------------------------------------------------------------------------

func jsonString(raw json.RawMessage, out *string) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func jsonBool(raw json.RawMessage, out *bool) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func jsonInt(raw json.RawMessage, out *int) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err != nil {
		return err
	}
	*out = int(f)
	return nil
}

// jsonIntPtr returns (*int, true) if the raw is a number, (nil, false)
// if it's null/absent/non-numeric.
func jsonIntPtr(raw json.RawMessage) (*int, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, false
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, false
	}
	n := int(f)
	return &n, true
}

// jsonStringSlice decodes a JSON array of strings; returns nil on null/empty.
// Skips silently on type mismatch — the parser may emit ints intermixed
// in some experimental paths; we don't want to hard-fail the whole card.
func jsonStringSlice(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// previewRaw returns at most `n` bytes of raw for error messages.
func previewRaw(raw json.RawMessage, n int) string {
	s := string(raw)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

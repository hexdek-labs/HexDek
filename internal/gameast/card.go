package gameast

import (
	"fmt"
	"sort"
	"strings"
)

// CardAST is the top-level AST for a single card. Mirrors
// scripts/mtg_ast.py :: CardAST.
type CardAST struct {
	Name         string    `json:"name"`
	Abilities    []Ability `json:"abilities"`
	ParseErrors  []string  `json:"parse_errors,omitempty"`
	FullyParsed  bool      `json:"fully_parsed"`
}

// Signature returns a stable structural fingerprint of the AST. Two cards
// with identical signatures share an effect handler shape.
//
// Mirrors scripts/mtg_ast.py :: signature().
func (c *CardAST) Signature() string {
	parts := make([]string, 0, len(c.Abilities))
	for _, ab := range c.Abilities {
		parts = append(parts, abilitySig(ab))
	}
	sort.Strings(parts)
	return "[" + strings.Join(parts, ",") + "]"
}

func abilitySig(ab Ability) string {
	switch a := ab.(type) {
	case *Keyword:
		return fmt.Sprintf("(kw %s)", a.Name)
	case *Static:
		modKind := "?"
		if a.Modification != nil {
			modKind = a.Modification.ModKind
		}
		condKind := "_"
		if a.Condition != nil {
			condKind = a.Condition.Kind
		}
		return fmt.Sprintf("(static %s %s)", modKind, condKind)
	case *Activated:
		return fmt.Sprintf("(act %s %s)", costSig(a.Cost), effectSig(a.Effect))
	case *Triggered:
		return fmt.Sprintf("(trig %s %s)", a.Trigger.Event, effectSig(a.Effect))
	}
	return fmt.Sprintf("(? %T)", ab)
}

func costSig(c Cost) string {
	flags := []string{}
	if c.Tap {
		flags = append(flags, "T")
	}
	if c.Untap {
		flags = append(flags, "Q")
	}
	if c.Sacrifice != nil {
		flags = append(flags, "sac:"+c.Sacrifice.Base)
	}
	if c.Discard != nil {
		flags = append(flags, fmt.Sprintf("discard:%d", *c.Discard))
	}
	if c.PayLife != nil {
		flags = append(flags, fmt.Sprintf("life:%d", *c.PayLife))
	}
	if c.ExileSelf {
		flags = append(flags, "exile_self")
	}
	if c.Mana != nil {
		flags = append(flags, fmt.Sprintf("mana:%d", c.Mana.CMC()))
	}
	if len(flags) == 0 {
		return "(free)"
	}
	return "(" + strings.Join(flags, " ") + ")"
}

func effectSig(e Effect) string {
	if e == nil {
		return "(none)"
	}
	switch ee := e.(type) {
	case *Sequence:
		items := make([]string, 0, len(ee.Items))
		for _, it := range ee.Items {
			items = append(items, effectSig(it))
		}
		return "(seq " + strings.Join(items, " ") + ")"
	case *Choice:
		opts := make([]string, 0, len(ee.Options))
		for _, o := range ee.Options {
			opts = append(opts, effectSig(o))
		}
		pickStr := numberSig(&ee.Pick)
		return "(choice " + pickStr + " " + strings.Join(opts, " ") + ")"
	case *Optional_:
		return "(opt " + effectSig(ee.Body) + ")"
	case *Conditional:
		condKind := "?"
		if ee.Condition != nil {
			condKind = ee.Condition.Kind
		}
		return "(if " + condKind + " " + effectSig(ee.Body) + ")"
	}
	// Leaf effect — include a small distinguishing param.
	return "(" + e.Kind() + leafExtras(e) + ")"
}

func leafExtras(e Effect) string {
	// Mirror the Python "extras" fallback: amount → count → pt.
	type amountish interface{ amt() *NumberOrRef }
	type countish interface{ cnt() *NumberOrRef }
	type ptish interface{ ptPair() *[2]int }
	// (Leaf effects don't all expose these uniformly; the signature is
	// coarse on purpose — we only use it for clustering.)
	if v, ok := e.(*Damage); ok {
		return " " + numberSig(&v.Amount)
	}
	if v, ok := e.(*Draw); ok {
		return " " + numberSig(&v.Count)
	}
	if v, ok := e.(*Discard); ok {
		return " " + numberSig(&v.Count)
	}
	if v, ok := e.(*Mill); ok {
		return " " + numberSig(&v.Count)
	}
	if v, ok := e.(*GainLife); ok {
		return " " + numberSig(&v.Amount)
	}
	if v, ok := e.(*LoseLife); ok {
		return " " + numberSig(&v.Amount)
	}
	if v, ok := e.(*CreateToken); ok {
		if v.PT != nil {
			return fmt.Sprintf(" pt(%d/%d)", v.PT[0], v.PT[1])
		}
		return " " + numberSig(&v.Count)
	}
	return ""
	// (amountish, countish, ptish reserved for future extension.)
}

func numberSig(n *NumberOrRef) string {
	if n == nil {
		return "_"
	}
	if n.IsInt {
		return fmt.Sprintf("%d", n.Int)
	}
	if n.IsStr {
		return n.Str
	}
	if n.IsScaling && n.Scaling != nil {
		return "scaling:" + n.Scaling.ScalingKind
	}
	return "_"
}

package gameengine

// mod_kind_static_buffs.go — generic handlers for static continuous
// self-buff scaffold KINDS that previously fell through inert.
//
// Worker hex-dev-5, category: STATIC + continuous effects.
//
// These Modification kinds sit on Static abilities whose `layer` field
// is null in the dataset, so registerASTStaticEffects (layers.go) skipped
// them at its `Layer == ""` gate before reaching the switch. The gate is
// relaxed via staticKindAllowedLayerless (below) for the specific kinds
// we implement, and the cases register §613 continuous effects through
// the existing registerAnthemPT primitive — so combat / SBA / any
// gs.PowerOf / gs.ToughnessOf query reflects the buff.

// staticKindAllowedLayerless lists Modification kinds that
// registerASTStaticEffects should process even though the parser left
// their `layer` tag empty. Keeping this allowlist explicit means the
// `Layer == ""` skip still protects every other (genuinely Layer-tagged)
// static path from accidental processing.
func staticKindAllowedLayerless(kind string) bool {
	switch kind {
	case "self_buff", "self_calculated_pt",
		// commander_anthem (Background "commander creatures you own …",
		// CR §702.124e) — the parser leaves its layer tag empty, so the
		// §613 gate skipped it entirely and the grant was fully inert.
		"commander_anthem":
		return true
	default:
		return false
	}
}

// registerSelfBuffStatic implements the `self_buff` kind: "this creature
// gets +X/+Y" as an always-on static (CR §613 layer 7c). 144/165 corpus
// uses carry integer (power, toughness) args (Sedge Sliver +1/+1,
// Isleback Spawn +4/+8); the remaining 21 are scaling "+1/+0 for each …"
// shapes whose args are a raw string — extractPT yields (0,0) for those,
// and we skip the no-op registration, leaving them for a future scaling
// handler.
func registerSelfBuffStatic(gs *GameState, p *Permanent, args []interface{}) {
	pow, tough := extractPT(args, 0, 0)
	if pow == 0 && tough == 0 {
		return // "for each" scaling shape (string arg) — not handled here
	}
	src := p
	registerAnthemPT(gs, p, pow, tough, "ast-self-buff", func(_ *GameState, t *Permanent) bool {
		return t == src
	})
}

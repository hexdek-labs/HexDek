package gameengine

// Generic "token_anthem" modification kind — power/toughness modification
// scoped to CREATURE TOKENS. "Creature tokens [you control] get +X/+Y."
//
// AST args = [pow, tough, ...]. The parser does not encode the controller
// scope explicitly, but it correlates reliably with sign:
//   - positive P/T  → "creature tokens YOU CONTROL get +X/+Y" (token payoffs:
//     Intangible Virtue, Leyline of the Meek, Phantom General, Inspiring
//     Leader, Caretaker's Talent, Dramatic Finale, Hildibrand, Invasion of
//     Tolvada).
//   - negative P/T  → "creature tokens get -X/-Y" globally (token hate:
//     Illness in the Ranks, Virulent Plague, Amulet of Safekeeping).
// You never shrink your own tokens, and global token-hate is always a debuff,
// so the sign split is sound.
//
// A trailing keyword arg ("vigilance", "lifelink") grants that keyword to the
// tokens; keyword grants are NOT honored by combat's p.HasKeyword (layer-
// awareness blocker, see scaffold-claims), so only the P/T half is applied
// here — the dominant board effect. The keyword half is logged as a remainder.
//
// Every token_anthem card in the corpus is a STATIC ability on a permanent,
// so this handler wires only the static path (registerASTStaticEffects in
// layers.go) — one minimal additive switch case. Mirrors the generic `anthem`
// handler (mod_kind_anthem.go).

// tokenAnthemPredicate builds the creature-token filter for a token anthem.
// Negative buffs apply to ALL creature tokens; positive buffs apply only to
// the source controller's creature tokens.
func tokenAnthemPredicate(srcController, pow, tough int) func(*GameState, *Permanent) bool {
	global := pow < 0 || tough < 0
	return func(_ *GameState, t *Permanent) bool {
		if t == nil || !t.IsToken() {
			return false
		}
		if global {
			return true
		}
		return t.Controller == srcController
	}
}

// registerTokenAnthemStatic registers a continuous §613 P/T anthem scoped to
// creature tokens for a permanent's static ability.
func registerTokenAnthemStatic(gs *GameState, p *Permanent, args []interface{}) {
	if gs == nil || p == nil || p.Card == nil {
		return
	}
	pow, tough := extractPT(args, 1, 1)
	pred := tokenAnthemPredicate(p.Controller, pow, tough)
	registerAnthemPT(gs, p, pow, tough, "ast-token-anthem", pred)
}

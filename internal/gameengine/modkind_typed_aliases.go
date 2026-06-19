package gameengine

import "github.com/hexdek/hexdek/internal/gameast"

// modkind_typed_aliases.go — generic coverage for parser "_typed"
// scaffold variants of already-implemented Modification kinds.
//
// The card-AST parser emits a `_typed` (or similar) suffixed scaffold
// node for a number of mechanics whose BASE kind resolveModificationEffect
// already implements — e.g. `become_monarch_typed` alongside the handled
// `become_monarch`, `discover_typed` alongside `discover`. The base kind
// resolves correctly; the `_typed` variant falls through to the switch
// default and does NOTHING (logged only as a parser_gap). That left ~180
// cards across these mechanics inert despite the engine already owning
// the primitive.
//
// Rather than duplicate each base case, this aliaser re-dispatches a
// `_typed` node to its base ModKind with identical args — reusing the
// existing, tested base logic verbatim. One generic seam unlocks every
// listed mechanic at once, and any future card the parser routes to one
// of these `_typed` kinds is covered automatically.
//
// Scope note: `regenerate_typed` is intentionally EXCLUDED — it is owned
// by the regeneration-category worker (it additionally requires the
// regeneration-shield CONSUMPTION fix, not just a re-dispatch, so a bare
// alias would be misleading). See /tmp/fable-review/scaffold-claims.txt.
//
// Arg compatibility was verified per kind against the dataset: the base
// cases either ignore args (monarch/explore/venture/ring/learn/cloak/
// suspect/choose_color) or read a leading int count (discover N,
// roll_die sides) that the `_typed` node carries in the same position
// (roll_die_typed's descriptive-string arg falls back to the correct
// six-sided default).

// typedAliasBase maps an inert `_typed` scaffold ModKind to the handled
// base ModKind it should resolve as. Every value here MUST be a kind that
// resolveModificationEffect handles with a real (non-default) case.
var typedAliasBase = map[string]string{
	"choose_color_typed":    "choose_color",
	"become_monarch_typed":  "become_monarch",
	"venture_dungeon_typed": "venture_dungeon",
	"explore_typed":         "explore",
	"ring_tempts_typed":     "ring_tempts",
	"discover_typed":        "discover",
	"learn_typed":           "learn",
	"cloak_typed":           "cloak",
	"roll_die_typed":        "roll_die",
	// suspect_typed is handled by its own resolveModificationEffect case
	// (it suspects a TARGET creature, not the source — re-dispatching to
	// "suspect" wrongly suspected the source).
}

// resolveTypedAliasModKind handles a parser `_typed` scaffold variant by
// re-dispatching to its already-implemented base ModKind with identical
// args. Returns true iff it recognized and handled the kind.
//
// The base ModKind never ends in a suffix in typedAliasBase, so the
// re-dispatched call cannot re-enter this aliaser — no recursion risk.
func resolveTypedAliasModKind(gs *GameState, src *Permanent, e *gameast.ModificationEffect) bool {
	if e == nil {
		return false
	}
	base, ok := typedAliasBase[e.ModKind]
	if !ok {
		return false
	}
	alias := *e
	alias.ModKind = base
	resolveModificationEffect(gs, src, &alias)
	return true
}

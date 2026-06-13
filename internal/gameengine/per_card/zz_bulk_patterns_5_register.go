package per_card

// dev/muninn-bulk-patterns-5 registration entry point.
//
// We can't safely edit registry.go's registerDefaults() while concurrent
// dev agents are also amending it on adjacent branches, so the two
// bulk-pattern families added in this branch hook themselves into the
// global registry via this init(). Runs after registry.go's init()
// because Go runs same-package init()s in file lexical order and
// "zz_..." sorts after "registry.go" — same pattern used by
// zz_bulk_patterns_4_register.go and friends.
//
// Families wired here:
//
//   - etb_basic_land_ramp_family.go — "When this creature enters, [you
//     may] search your library for a basic land card, put it onto the
//     battlefield tapped / into your hand, then shuffle." Members:
//     Farhaven Elf (battlefield_tapped), Civic Wayfinder, Borderland
//     Ranger, Sylvan Ranger, Pilgrim's Eye (all → hand). Covers the
//     un-gated single-basic ETB ramp shape that land_tax_family does
//     NOT cover (no opp-more-lands gate). Wood Elves (Forest filter,
//     untapped), Solemn Simulacrum (extra die-draw), Yavimaya Granger
//     (Echo), Gatecreeper Vine / District Guide (basic OR Gate),
//     Primeval Herald (enters-or-attacks), and Loam Larva (top-of-lib
//     destination) are intentional skips with rationale in the family
//     docstring.
//
//   - etb_drain_target_opponent_family.go — "When this creature enters,
//     target opponent loses N life and you gain N life." Members:
//     Skymarch Bloodletter (1), Vampire Sovereign (3), Highway Robber
//     (2), Dakmor Ghoul (2), Bloodborn Scoundrels (2). Picks the
//     lowest-life living opponent (same heuristic as Athreos / Belakor
//     / Ajani Nacatl Pariah) so lethal pressure stacks correctly into
//     win-line detection. Hand-rolled siblings with gates (Kalastria
//     Healer ally, Tithe Drinker extort, Blood Artist creature-died,
//     Falkenrath Noble damage-trigger, Vito conversion) keep their
//     bespoke handlers — the family only owns the un-gated shape.
//
// Both families coexist cleanly with anything registry.go registers for
// the same card name: dispatcher iterates every registered handler, so
// per-card handlers and family handlers stack rather than replace. None
// of the 10 cards listed above has a competing handler today (verified
// at write-time by grep across internal/gameengine/per_card/*.go).

func init() {
	r := Global()
	registerEtbBasicLandRampFamily(r)
	registerEtbDrainTargetOpponentFamily(r)
	// r63: the drain family declares OwnsETBTrigger (double-fire gate). A
	// bare init() registration is stripped by Reset() (the Sai-fix gap),
	// so re-register on Reset to keep ownership — and the handlers — alive.
	AddResetHook(registerEtbDrainTargetOpponentFamily)
}

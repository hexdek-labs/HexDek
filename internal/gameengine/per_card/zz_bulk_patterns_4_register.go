package per_card

// dev/muninn-bulk-patterns-4 registration entry point.
//
// We can't safely edit registry.go's registerDefaults() while concurrent
// dev agents are also amending it on adjacent branches, so the two
// bulk-pattern families added in this branch hook themselves into the
// global registry via this init(). It runs after registry.go's init()
// because Go runs same-package init()s in file lexical order and
// "zz_..." sorts after "registry.go" — same pattern used by
// zz_handler_coverage_2_register.go and friends.
//
// Families wired here:
//
//   - shuffle_self_from_grave_family.go — "When ~ is put into a
//     graveyard from anywhere, shuffle it into its owner's library."
//     Members: Dread, Purity, Guile, Vigor.
//
//   - etb_library_tutor_family.go — "When this creature enters, [you
//     may] search your library for a <filter> card, put it into your
//     hand, then shuffle." Members: Trophy Mage, Treasure Mage,
//     Trinket Mage, Stoneforge Mystic, Heliod's Pilgrim, Spellseeker,
//     Imperial Recruiter, Fierce Empath, Thalia's Lancers.
//
// Both families coexist cleanly with anything registry.go registers
// for the same card name: the dispatcher iterates every registered
// handler, so per-card handlers (e.g. Stoneforge Mystic) and family
// handlers stack rather than replace. None of the cards listed above
// has a competing handler today (we verified at write-time), so the
// family is the sole owner of their ETB / creature_dies behavior.

func init() {
	r := Global()
	registerShuffleSelfFromGraveFamily(r)
	registerEtbLibraryTutorFamily(r)
	// r63: the tutor family declares OwnsETBTrigger (double-fire gate). A
	// bare init() registration is stripped by Reset() (the Sai-fix gap),
	// so re-register on Reset to keep ownership — and the handlers — alive.
	AddResetHook(registerEtbLibraryTutorFamily)
}

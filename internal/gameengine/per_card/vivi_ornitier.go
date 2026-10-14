package per_card

// registerViviOrnitier — deliberately registers NOTHING (r98).
//
// # WHAT THE CARD ACTUALLY SAYS
//
// Oracle, fetched r98 (Scryfall ecc1027a-8c07-44a0-bdde-fa2844cff694,
// set fin, Legendary Creature — Wizard, {1}{U}{R}):
//
//	{0}: Add X mana in any combination of {U} and/or {R}, where X is
//	Vivi Ornitier's power. Activate only during your turn and only once
//	each turn.
//	Whenever you cast a noncreature spell, put a +1/+1 counter on Vivi
//	Ornitier and it deals 1 damage to each opponent.
//
// # WHAT THE HANDLER USED TO DO
//
// It registered OnTrigger("Vivi Ornitier", "combat_damage_player") and,
// on combat damage, granted a free ZoneCastPermission (ManaCost 0,
// ExileOnResolve true) on every instant and sorcery in the controller's
// graveyard. Its doc comment quoted this as the card's oracle text:
//
//	"Whenever Vivi Ornitier deals combat damage to a player, you may
//	cast an instant or sorcery spell from your graveyard without paying
//	its mana cost. If a spell cast this way would be put into your
//	graveyard, exile it instead."
//
// That text is not on the card and never was. Vivi has no combat-damage
// trigger, no graveyard-cast ability and no exile clause — the quoted
// sentence is the Kess / Maestros Ascendancy wording, both of which do
// carry it and were verified in the same r98 pass.
//
// So this was not a wrong flag on a real ability; it was a whole
// fabricated ability handing its controller free spells out of the
// graveyard every time a 3-mana creature connected. Registering nothing
// is strictly more correct than registering that.
//
// # WHAT IS NOW MISSING, filed in CLAUDE.md Open
//
// Both real abilities. The noncreature-cast trigger is an ordinary
// shape the AST pipeline may already cover; the {0} mana ability is
// not — "add X mana where X is this creature's power" is a
// power-scaled mana ability with a once-per-turn, your-turn-only
// restriction, and the restricted-mana enforcement gap noted in the
// r63 predefined-token audit applies to it. Neither is implemented
// here, and a handler that implements a different card is worse than
// no handler, so this is left empty rather than guessed at.
func registerViviOrnitier(r *Registry) {}

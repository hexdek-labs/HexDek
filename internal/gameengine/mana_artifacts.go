package gameengine

// CR §106 — artifact mana sources.
//
// This file handles the fast-mana staples that the previous engine
// silently ignored because fill_mana_pool only iterated creatures and
// lands. Every mana rock in common cEDH use lives here:
//
//   - Pure {T}: Add … — Sol Ring, Mana Crypt, Mana Vault, Grim
//     Monolith, Basalt Monolith, Thran Dynamo, Fellwar Stone, Signets
//     (Dimir/Izzet/Selesnya/etc.), Talismans, Arcane Signet, Moxes,
//     Chromatic Lantern, Commander's Sphere.
//
//   - {T}, Sacrifice: Add … — Lotus Petal, Jeweled Lotus, Treasure
//     tokens, Gold tokens.
//
//   - {T}, Discard your hand, Sacrifice: Add {3} — Lion's Eye Diamond.
//
// Token artifacts (Treasure / Powerstone / Gold / Food / Blood / Map /
// Clue / Junk) also flow through this path — they live on the
// battlefield as Permanents with minimal AST and we recognize them by
// name/type_line.

// IsArtifactOnly returns true if this permanent is a pure artifact
// (NOT an artifact creature — those route through the creature branch
// for summoning-sickness-respecting tap-mana; NOT an artifact land —
// those route through the land branch).
func IsArtifactOnly(p *Permanent) bool {
	if p == nil || p.Card == nil {
		return false
	}
	tl := p.Card.TypeLine
	// Token permanents may have TypeLine like "Token Artifact —
	// Treasure" — those count as artifacts for mana purposes.
	isArt := containsLower(tl, "artifact")
	// Tokens often have empty TypeLine — also check Card.Types.
	if !isArt {
		for _, t := range p.Card.Types {
			if t == "artifact" {
				isArt = true
				break
			}
		}
	}
	if !isArt {
		return false
	}
	if containsLower(tl, "creature") || containsLower(tl, "land") {
		return false
	}
	// Also exclude artifact creatures via Card.Types.
	for _, t := range p.Card.Types {
		lt := t
		if lt == "creature" || lt == "land" {
			return false
		}
	}
	return true
}

// ArtifactHasDestructiveCost returns true for artifacts whose tap-for-
// mana ability has a side effect harmful to the controller (Lion's Eye
// Diamond discards the hand). Auto-pay paths skip these.
func ArtifactHasDestructiveCost(p *Permanent) bool {
	if p == nil || p.Card == nil {
		return false
	}
	return p.Card.DisplayName() == "Lion's Eye Diamond"
}

// ArtifactManaPotential returns the expected pip count produced by
// tapping this artifact. Used by availability checks.
func ArtifactManaPotential(p *Permanent) int {
	if p == nil || p.Card == nil {
		return 0
	}
	name := p.Card.DisplayName()
	tl := p.Card.TypeLine
	switch {
	case containsLower(tl, "treasure"), name == "Treasure Token":
		return 1
	case containsLower(tl, "powerstone"), name == "Powerstone Token":
		return 1
	case name == "Gold Token":
		return 1
	case name == "Sol Ring":
		return 2
	case name == "Mana Crypt":
		return 2
	case name == "Mana Vault", name == "Grim Monolith":
		return 3
	case name == "Basalt Monolith":
		return 3
	case name == "Thran Dynamo":
		return 3
	case name == "Lotus Petal":
		return 1
	case name == "Jeweled Lotus":
		return 3
	case name == "Lion's Eye Diamond":
		return 3
	}
	// Generic signet / Talisman / Mox / etc.: approximate 1 pip — the
	// real pip count comes from the AST; an ordinary signet taps for 2
	// (Signet: "{T}: Add two mana of any one color."). For MVP we
	// return 1 as the conservative baseline; if the AST carries a
	// mana-add activated ability we defer to it in ApplyArtifactMana.
	return 1
}

// ApplyArtifactMana taps the artifact for mana, crediting the seat's
// pool and handling any sacrifice-as-cost. Returns (pips added, true)
// on success, (0, false) if this artifact can't be tapped here.
//
// Side effects:
//   - Tokens with "sacrifice" in their ability are removed from the
//     battlefield.
//   - Lion's Eye Diamond discards the controller's hand as a cost.
//   - Mana Vault / Grim Monolith / Basalt Monolith flag themselves as
//     "stuck-tapped" so untap_step can choose to leave them tapped.
func ApplyArtifactMana(gs *GameState, seat *Seat, p *Permanent) (int, bool) {
	pips, ok := applyArtifactManaImpl(gs, seat, p)
	if ok && gs != nil {
		FireCardTrigger(gs, "artifact_tapped_for_mana", map[string]interface{}{
			"perm":            p,
			"card":            p.Card,
			"controller_seat": seat.Idx,
			"pips":            pips,
			"artifact_name":   p.Card.DisplayName(),
		})
	}
	return pips, ok
}

func applyArtifactManaImpl(gs *GameState, seat *Seat, p *Permanent) (int, bool) {
	if gs == nil || seat == nil || p == nil || p.Card == nil || p.Tapped {
		return 0, false
	}
	name := p.Card.DisplayName()
	tl := p.Card.TypeLine

	// Treasure token — {T}, Sacrifice: add one mana of any color.
	if containsLower(tl, "treasure") || name == "Treasure Token" {
		p.Tapped = true
		AddMana(gs, seat, "any", 1, name)
		SacrificePermanent(gs, p, "treasure_tap")
		return 1, true
	}
	// Powerstone token — {T}: Add {C}; spend only on noncreature.
	if containsLower(tl, "powerstone") || name == "Powerstone Token" {
		p.Tapped = true
		AddRestrictedMana(gs, seat, 1, "C",
			"noncreature_or_artifact_activation", name)
		return 1, true
	}
	// Gold token — Sacrifice (no tap): add one mana of any color.
	if name == "Gold Token" {
		AddMana(gs, seat, "any", 1, name)
		SacrificePermanent(gs, p, "gold_sac")
		return 1, true
	}
	// Sol Ring: {T}: Add {C}{C}.
	if name == "Sol Ring" {
		p.Tapped = true
		AddMana(gs, seat, "C", 2, name)
		return 2, true
	}
	// Mana Crypt: {T}: Add {C}{C}.
	if name == "Mana Crypt" {
		p.Tapped = true
		AddMana(gs, seat, "C", 2, name)
		return 2, true
	}
	// Mana Vault: {T}: Add {C}{C}{C}; doesn't untap unless you pay {4}.
	if name == "Mana Vault" || name == "Grim Monolith" {
		p.Tapped = true
		// Mana Vault adds 3 colorless; Grim Monolith also 3.
		AddMana(gs, seat, "C", 3, name)
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		p.Flags["mana_vault_stuck"] = 1
		p.Flags["skip_untap"] = 1
		p.DoesNotUntap = true
		return 3, true
	}
	// Basalt Monolith: {T}: Add {C}{C}{C}; {3}: Untap.
	if name == "Basalt Monolith" {
		p.Tapped = true
		AddMana(gs, seat, "C", 3, name)
		return 3, true
	}
	// Thran Dynamo: {T}: Add {C}{C}{C}.
	if name == "Thran Dynamo" {
		p.Tapped = true
		AddMana(gs, seat, "C", 3, name)
		return 3, true
	}
	// Lotus Petal: {T}, Sacrifice: Add one mana of any color.
	if name == "Lotus Petal" {
		p.Tapped = true
		AddMana(gs, seat, "any", 1, name)
		SacrificePermanent(gs, p, "lotus_petal_tap")
		return 1, true
	}
	// Jeweled Lotus: {T}, Sacrifice: Add three mana of any one color
	// (commander-restricted; MVP: any).
	if name == "Jeweled Lotus" {
		p.Tapped = true
		AddMana(gs, seat, "any", 3, name)
		SacrificePermanent(gs, p, "jeweled_lotus_tap")
		return 3, true
	}
	// Lion's Eye Diamond: {T}, Discard your hand, Sacrifice: Add 3 of
	// one color. CAUTION: this discards the hand — only do it when
	// explicitly asked.
	if name == "Lion's Eye Diamond" {
		p.Tapped = true
		// Discard hand as cost.
		hand := append([]*Card(nil), seat.Hand...)
		seat.Hand = seat.Hand[:0]
		for _, c := range hand {
			DiscardCard(gs, c, seat.Idx)
		}
		AddMana(gs, seat, "any", 3, name)
		SacrificePermanent(gs, p, "lions_eye_diamond_tap")
		return 3, true
	}
	// Fellwar Stone: {T}: Add one mana of any color among lands your
	// opponents control. MVP: add 1 any.
	if name == "Fellwar Stone" {
		p.Tapped = true
		AddMana(gs, seat, "any", 1, name)
		return 1, true
	}
	// Arcane Signet: {T}: Add one mana of any color in your
	// commander's identity. MVP: add 1 any.
	if name == "Arcane Signet" {
		p.Tapped = true
		AddMana(gs, seat, "any", 1, name)
		return 1, true
	}
	// Chromatic Lantern: {T}: Add one mana of any color.
	if name == "Chromatic Lantern" {
		p.Tapped = true
		AddMana(gs, seat, "any", 1, name)
		return 1, true
	}
	// Commander's Sphere: {T}: Add one mana of any color in your
	// commander's identity. MVP: any.
	if name == "Commander's Sphere" {
		p.Tapped = true
		AddMana(gs, seat, "any", 1, name)
		return 1, true
	}
	// Signets — "{1}, {T}: Add {X}{Y}". The activated ability costs {1}
	// generic mana PLUS the tap; net output is +1 mana (pay 1, get 2).
	// If the seat can't pay {1}, the signet can't be activated.
	if isSignet(name) {
		if seat.ManaPool < 1 {
			return 0, false // can't pay the {1} activation cost
		}
		seat.ManaPool -= 1
		SyncManaAfterSpend(seat)
		p.Tapped = true
		AddMana(gs, seat, "any", 2, name)
		return 2, true
	}
	if isTalisman(name) || isMox(name) {
		p.Tapped = true
		AddMana(gs, seat, "any", 1, name)
		return 1, true
	}
	return 0, false
}

// isSignet recognizes the ten Ravnica-guild signets (Azorius, Dimir,
// Rakdos, Gruul, Selesnya, Orzhov, Izzet, Golgari, Boros, Simic).
func isSignet(name string) bool {
	signets := map[string]bool{
		"Azorius Signet":  true,
		"Dimir Signet":    true,
		"Rakdos Signet":   true,
		"Gruul Signet":    true,
		"Selesnya Signet": true,
		"Orzhov Signet":   true,
		"Izzet Signet":    true,
		"Golgari Signet":  true,
		"Boros Signet":    true,
		"Simic Signet":    true,
	}
	return signets[name]
}

func isTalisman(name string) bool {
	talismans := []string{
		"Talisman of Progress", "Talisman of Dominance", "Talisman of Indulgence",
		"Talisman of Impulse", "Talisman of Unity", "Talisman of Hierarchy",
		"Talisman of Creativity", "Talisman of Resilience", "Talisman of Conviction",
		"Talisman of Curiosity",
	}
	for _, t := range talismans {
		if name == t {
			return true
		}
	}
	return false
}

func isMox(name string) bool {
	moxes := []string{
		"Mox Pearl", "Mox Sapphire", "Mox Jet", "Mox Ruby", "Mox Emerald",
		"Mox Diamond", "Mox Opal", "Mox Amber", "Mox Tantalite", "Chrome Mox",
		"Mox Ruby", "Mox Pyrite", "Moxite",
	}
	for _, m := range moxes {
		if name == m {
			return true
		}
	}
	return false
}

// SacrificePermanent removes p from the battlefield via the proper §701.17
// sacrifice path: runs §614 replacement effects (Rest in Peace redirect,
// etc.), fires dies/LTB triggers, handles commander zone redirect, and
// emits a sacrifice event. Tokens cease to exist (§704.5d); non-token
// cards go to their owner's graveyard (or replacement destination).
// Sacrifice ignores indestructible per CR §701.17b.
func SacrificePermanent(gs *GameState, p *Permanent, reason string) {
	sacrificePermanentImpl(gs, p, nil, reason)
}

// --- Local string helpers (no external dep needed) ------------------------

func containsLower(haystack, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	if len(haystack) < len(needle) {
		return false
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := 0; j < len(needle); j++ {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

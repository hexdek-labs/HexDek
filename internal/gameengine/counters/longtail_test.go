package counters

import (
	"strings"
	"testing"
)

// phase5LongtailNames lists every counter type the Phase 5 long-tail
// population is expected to install. Used by property tests below to
// pin registry coverage and to defend against accidental removal
// during future refactors.
//
// The list is split by Probe F catalog section to make audit easier.
var phase5LongtailNames = []string{
	// Section A — §704.5r non-pairing P/T variants
	"+1/+0", "+2/+2", "+0/+1", "+1/+2", "+0/+2",
	"-0/-1", "-0/-2", "-1/-0", "-2/-1", "-2/-2",

	// Section B — resource-substance counters
	"defense",

	// Section C — remaining §122.1c keyword counters
	"haste", "shadow", "decayed", "phyresis", "strike",

	// Section E — countdown family (time is Phase 1)
	"fade", "age",

	// Section F — alt-win / threshold
	"quest", "level", "verse", "study", "tower",
	"intervention", "filibuster", "luck",

	// Section G — §614 replacement-effect counters (stun + shield are Phase 1)
	"finality", "mannequin", "incarnation", "isolation",
	"paralyzation", "pin", "echo",

	// Section H.1 — multi-card storage / resource
	"brick", "depletion", "storage", "hour", "dream", "coin", "eon",
	"point", "ticket", "wish", "stash", "book", "page", "ki", "soot",
	"gold", "treasure", "food", "clue", "map", "landmark", "component",
	"cube", "currency", "delay", "fuse", "doom", "omen", "flood", "tide",
	"wind", "flame", "growth", "pressure", "mining", "mire", "polyp",
	"slumber", "slime", "spore", "ice", "soul", "bounty", "corpse", "hit",
	"acorn", "egg", "hatchling", "shell", "pupa", "tribute", "divinity",
	"fate", "feather", "fetch", "dread", "void", "velocity", "infection",
	"plague", "fungus", "burden", "cage", "collection", "net", "death",
	"devotion", "aim", "blight", "blaze", "scream", "fire", "healing",
	"fellowship",

	// Section H.2 — single-card markers
	"petal", "knowledge", "eyeball", "eyestalk", "bloodline", "croak",
	"funk", "fury", "harmony", "hoofprint", "muster", "valor", "vitality",
	"vortex", "wage", "unity", "phylactery", "prey", "bait", "chip",
	"chorus", "credit", "crystal", "cell", "brain", "gem", "intel", "kick",
	"matrix", "memory", "nest", "night", "ore", "pain", "palliation",
	"pause", "petrification", "plan", "plot", "possession", "rally",
	"rejection", "release", "reprieve", "resonance", "rev", "revival",
	"ribbon", "ritual", "scroll", "shred", "silver", "skewer", "sleep",
	"spark", "spite", "stall", "story", "strife", "supply", "suspect",
	"takeover", "task", "taste", "theft", "unlock", "vow", "voyage",
	"winch", "wreck", "aegis", "arrow", "arrowhead", "awakening",
	"blessing", "bloodstain", "bore", "bribery", "carrion", "conqueror",
	"contested", "corruption", "day", "descent", "despair", "discovery",
	"duty", "elixir", "ember", "enlightened", "eruption", "exalted",
	"exposure", "fear", "feeding", "film", "foreshadow", "ghostform",
	"glyph", "hope", "hourglass", "hunger", "husk", "impostor",
	"incubation", "influence", "ingenuity", "invitation", "javelin",
	"judgment", "magnet", "manifestation", "mask", "mine", "hack", "hole",
	"knickknack", "rebuilding", "midway", "shy",
}

// TestPhase5LongtailRegistered pins that every counter type Phase 5 is
// expected to install is present, has non-empty Notes (CR §-citation),
// has non-empty ValidTargets, and resolves through Lookup.
func TestPhase5LongtailRegistered(t *testing.T) {
	for _, name := range phase5LongtailNames {
		def := Lookup(name)
		if def == nil {
			t.Errorf("Phase 5 registry missing %q", name)
			continue
		}
		if def.Name != name {
			t.Errorf("Phase 5 %q: def.Name = %q, want %q", name, def.Name, name)
		}
		if def.Notes == "" {
			t.Errorf("Phase 5 %q: empty Notes — every entry must carry CR §-citation", name)
		}
		if len(def.ValidTargets) == 0 {
			t.Errorf("Phase 5 %q: empty ValidTargets", name)
		}
	}
}

// TestPhase5KeywordGrantInvariant pins the §122.1c invariant: a counter
// whose Category is KeywordGrant MUST have a non-nil GrantedAbility, and
// a counter whose Category is NOT KeywordGrant MUST have a nil
// GrantedAbility. This protects against accidental category/grant drift
// during sweeps.
func TestPhase5KeywordGrantInvariant(t *testing.T) {
	for _, def := range All() {
		isKW := def.Category == KeywordGrant
		hasGrant := def.GrantedAbility != nil
		if isKW && !hasGrant {
			t.Errorf("%q Category=KeywordGrant but GrantedAbility=nil (violates §122.1c)", def.Name)
		}
		if !isKW && hasGrant {
			t.Errorf("%q Category=%v but GrantedAbility=%+v (only KeywordGrant should carry a granted ability)",
				def.Name, def.Category, def.GrantedAbility)
		}
	}
}

// TestPhase5ValidTargetsCoverage pins that every TargetType enum value
// is referenced by at least one registry entry — defends against
// orphaned enum values that silently never accept any counter.
func TestPhase5ValidTargetsCoverage(t *testing.T) {
	seen := map[TargetType]bool{}
	for _, def := range All() {
		for _, vt := range def.ValidTargets {
			seen[vt] = true
		}
	}
	allEnum := []TargetType{
		TargetCreature, TargetPlaneswalker, TargetArtifact,
		TargetEnchantment, TargetLand, TargetBattle, TargetSaga, TargetPlayer,
	}
	for _, vt := range allEnum {
		if !seen[vt] {
			t.Errorf("TargetType %q never appears in any ValidTargets — orphaned enum value", vt.String())
		}
	}
}

// TestPhase5NoDuplicateNames pins that no counter name is registered
// twice. (registerDefinition panics on duplicate, so this would surface
// at init() — the test exists as documentation + defense for future
// refactors that might move registration out of init.)
func TestPhase5NoDuplicateNames(t *testing.T) {
	seen := map[string]bool{}
	for _, def := range All() {
		if seen[def.Name] {
			t.Errorf("duplicate registration for %q", def.Name)
		}
		seen[def.Name] = true
	}
}

// TestPhase5JokeCountersSkipped pins the §10 out-of-scope exclusion
// list. Joke / un-set counters MUST NOT be in the registry so the
// engine returns ErrUnknownCounterType and per_card layers can no-op.
func TestPhase5JokeCountersSkipped(t *testing.T) {
	jokeSet := []string{
		"stroopwafel", "bargle", "shoe", "twenty", "glass", "traffic",
		"keyword", "token", "spooky", "milk", "third-degree-burn", "art",
		"curse", "everything", "sickness",
	}
	for _, name := range jokeSet {
		if def := Lookup(name); def != nil {
			t.Errorf("joke-set counter %q was registered (out-of-scope per design doc §10): %+v", name, def)
		}
	}
}

// TestPhase5EnergyStillExcluded pins the CR §106.11 ruling carried over
// from Phase 4: energy is a resource pool, not a §122 counter; cannot
// be proliferated. The registry MUST NOT carry an "energy" entry.
func TestPhase5EnergyStillExcluded(t *testing.T) {
	if def := Lookup("energy"); def != nil {
		t.Errorf("energy was registered: %+v (CR §106.11 — energy is a resource pool, not a counter)", def)
	}
}

// ---------------------------------------------------------------------------
// Per-type semantic tests for non-obvious counter types.
// ---------------------------------------------------------------------------

// TestStatModifierVariantsNoPair pins that none of the §704.5r non-pairing
// variants register a pair-cancel relationship. Only +1/+1 and -1/-1 do.
func TestStatModifierVariantsNoPair(t *testing.T) {
	variants := []string{
		"+1/+0", "+2/+2", "+0/+1", "+1/+2", "+0/+2",
		"-0/-1", "-0/-2", "-1/-0", "-2/-1", "-2/-2",
	}
	for _, name := range variants {
		def := Lookup(name)
		if def == nil {
			t.Errorf("variant %q not registered", name)
			continue
		}
		if def.Category != StatModifier {
			t.Errorf("variant %q Category = %v, want StatModifier", name, def.Category)
		}
		if def.StackingBehavior.PairsWith != "" {
			t.Errorf("variant %q PairsWith = %q, want empty (§704.5r only +1/+1 and -1/-1 cancel)",
				name, def.StackingBehavior.PairsWith)
		}
		if !def.DoublingApplies {
			t.Errorf("variant %q DoublingApplies = false, want true", name)
		}
		if !def.Proliferate {
			t.Errorf("variant %q Proliferate = false, want true", name)
		}
	}
}

// TestDefenseCounterTargetsBattle pins §310 — defense lives on battles,
// not creatures or planeswalkers.
func TestDefenseCounterTargetsBattle(t *testing.T) {
	def := Lookup("defense")
	if def == nil {
		t.Fatal("defense not registered")
	}
	if len(def.ValidTargets) != 1 || def.ValidTargets[0] != TargetBattle {
		t.Errorf("defense ValidTargets = %v, want [battle]", def.ValidTargets)
	}
	if def.Category != LoyaltyCounter {
		t.Errorf("defense Category = %v, want LoyaltyCounter (battle resource-substance)", def.Category)
	}
	if !def.Placement.Allows(PlaceEnterCondition) {
		t.Errorf("defense Placement must allow ETB — CR §310.5 (battles enter with printed defense)")
	}
	if !def.Placement.Allows(PlaceAbilityCost) {
		t.Errorf("defense Placement must allow PlaceAbilityCost — §310.7 damage removes 1:1")
	}
}

// TestLoyaltyCounterEnterAndCostShape pins §306 — loyalty ETB at printed
// value, removed/added as cost for ± abilities, 0 → graveyard SBA.
func TestLoyaltyCounterEnterAndCostShape(t *testing.T) {
	def := Lookup("loyalty")
	if def == nil {
		t.Fatal("loyalty not registered")
	}
	if len(def.ValidTargets) != 1 || def.ValidTargets[0] != TargetPlaneswalker {
		t.Errorf("loyalty ValidTargets = %v, want [planeswalker]", def.ValidTargets)
	}
	if !def.Placement.Allows(PlaceEnterCondition) {
		t.Errorf("loyalty must allow ETB placement — §306.5b ETB at printed value")
	}
	if !def.Placement.Allows(PlaceAbilityCost) {
		t.Errorf("loyalty must allow PlaceAbilityCost — ± loyalty abilities pay counters")
	}
}

// TestSagaLoreAutoUpkeepOnly pins §714: lore counters are auto-placed at
// precombat main, NOT at ETB (sagas ETB with 0).
func TestSagaLoreAutoUpkeepOnly(t *testing.T) {
	def := Lookup("lore")
	if def == nil {
		t.Fatal("lore not registered")
	}
	if !def.Placement.Allows(PlaceAutoUpkeep) {
		t.Error("lore must allow auto-upkeep — §714.2 chapter accumulator")
	}
	if def.Placement.Allows(PlaceEnterCondition) {
		t.Error("lore must NOT allow ETB — §714.2 sagas ETB with 0 lore counters")
	}
	if len(def.ValidTargets) != 1 || def.ValidTargets[0] != TargetSaga {
		t.Errorf("lore ValidTargets = %v, want [saga]", def.ValidTargets)
	}
}

// TestTimeCounterMultiTarget pins §702.61/§702.63 — time counters live
// on suspended cards (any card type, since exile cards lose creature/etc
// type while suspended), vanishing permanents (any type), and Tarrasque-
// style creatures. ValidTargets must cover the full permanent surface.
func TestTimeCounterMultiTarget(t *testing.T) {
	def := Lookup("time")
	if def == nil {
		t.Fatal("time not registered")
	}
	required := []TargetType{TargetCreature, TargetArtifact, TargetEnchantment, TargetLand, TargetPlaneswalker}
	have := map[TargetType]bool{}
	for _, vt := range def.ValidTargets {
		have[vt] = true
	}
	for _, vt := range required {
		if !have[vt] {
			t.Errorf("time ValidTargets missing %v — suspend/vanishing applies across permanent types", vt)
		}
	}
}

// TestFadeCountdownShape pins §702.32 — ETB with N, -1 each upkeep,
// sacrifice when can't remove. PlaceAutoUpkeep + PlaceEnterCondition.
func TestFadeCountdownShape(t *testing.T) {
	def := Lookup("fade")
	if def == nil {
		t.Fatal("fade not registered")
	}
	if def.Category != TimeCounter {
		t.Errorf("fade Category = %v, want TimeCounter (§702.32 countdown family)", def.Category)
	}
	if !def.Placement.Allows(PlaceEnterCondition) {
		t.Error("fade must allow ETB — §702.32 ETB with N counters")
	}
	if !def.Placement.Allows(PlaceAutoUpkeep) {
		t.Error("fade must allow auto-upkeep — §702.32 -1 each upkeep")
	}
}

// TestAgeAccumulatorShape pins §702.24 — same shape as fade but
// accumulates (+1 each upkeep, cost scales) instead of decrementing.
func TestAgeAccumulatorShape(t *testing.T) {
	def := Lookup("age")
	if def == nil {
		t.Fatal("age not registered")
	}
	if def.Category != TimeCounter {
		t.Errorf("age Category = %v, want TimeCounter (§702.24 cumulative upkeep family)", def.Category)
	}
	if !def.Placement.Allows(PlaceAutoUpkeep) {
		t.Error("age must allow auto-upkeep — §702.24 +1 each upkeep")
	}
}

// TestChargeCounterBroadTargets pins that charge counters land on the
// broad artifact/creature/land/enchantment surface they actually appear
// on (Aetherflux Reservoir = artifact, Magosi/eon-style would-be lands,
// Pemmin's Aura-style enchantments, charge-counter creatures). Keeps the
// engine from rejecting legitimate per_card placements.
func TestChargeCounterBroadTargets(t *testing.T) {
	def := Lookup("charge")
	if def == nil {
		t.Fatal("charge not registered")
	}
	required := []TargetType{TargetArtifact, TargetCreature, TargetLand, TargetEnchantment}
	have := map[TargetType]bool{}
	for _, vt := range def.ValidTargets {
		have[vt] = true
	}
	for _, vt := range required {
		if !have[vt] {
			t.Errorf("charge ValidTargets missing %v — restrictive target rejects legitimate per_card placements", vt)
		}
	}
}

// TestStunCounterReplacementShape pins §701.50 — would-untap → consume
// a stun counter. Category MUST be ResourceMarker so the §614 pipeline
// recognizes the counter as drive-a-replacement-effect.
func TestStunCounterReplacementShape(t *testing.T) {
	def := Lookup("stun")
	if def == nil {
		t.Fatal("stun not registered")
	}
	if def.Category != ResourceMarker {
		t.Errorf("stun Category = %v, want ResourceMarker (§614/§701.50 untap-replacement driver)", def.Category)
	}
}

// TestFinalityCounterDeathReplacement pins §614 — finality on a creature
// replaces would-die with exile. ResourceMarker like stun/shield.
func TestFinalityCounterDeathReplacement(t *testing.T) {
	def := Lookup("finality")
	if def == nil {
		t.Fatal("finality not registered")
	}
	if def.Category != ResourceMarker {
		t.Errorf("finality Category = %v, want ResourceMarker", def.Category)
	}
	if len(def.ValidTargets) != 1 || def.ValidTargets[0] != TargetCreature {
		t.Errorf("finality ValidTargets = %v, want [creature]", def.ValidTargets)
	}
}

// TestIncarnationCounterIsPlayerScoped pins the Nine Lives shape:
// incarnation counters accumulate on the PLAYER as the lose-condition
// tracker. ValidTargets must include TargetPlayer, not the permanent.
func TestIncarnationCounterIsPlayerScoped(t *testing.T) {
	def := Lookup("incarnation")
	if def == nil {
		t.Fatal("incarnation not registered")
	}
	have := map[TargetType]bool{}
	for _, vt := range def.ValidTargets {
		have[vt] = true
	}
	if !have[TargetPlayer] {
		t.Errorf("incarnation ValidTargets = %v, want to include TargetPlayer — Nine Lives accumulates on the player",
			def.ValidTargets)
	}
}

// TestLevelCounterRule711 pins §711 — level counters accumulate via
// level-up activation, drive per-bracket P/T + abilities. Must be
// PlaceAbilityCounter and proliferate-eligible.
func TestLevelCounterRule711(t *testing.T) {
	def := Lookup("level")
	if def == nil {
		t.Fatal("level not registered")
	}
	if !def.Placement.Allows(PlaceAbilityCounter) {
		t.Error("level must allow ability-counter placement — level-up activation is the source")
	}
	if !def.Proliferate {
		t.Error("level must be proliferate-eligible per §701.27")
	}
	if !def.DoublingApplies {
		t.Error("level must allow §122.1g doubling (Doubling Season + level up)")
	}
}

// TestAltWinCountersAreProliferateEligible — quest/tower/filibuster/luck
// are all alt-win threshold counters. All MUST be proliferate-eligible
// per §701.27 (Rimefeather Owl + Helix Pinnacle is a real engine).
func TestAltWinCountersAreProliferateEligible(t *testing.T) {
	for _, name := range []string{"quest", "tower", "filibuster", "luck"} {
		def := Lookup(name)
		if def == nil {
			t.Errorf("%q not registered", name)
			continue
		}
		if !def.Proliferate {
			t.Errorf("%q Proliferate = false — alt-win counters MUST proliferate per §701.27", name)
		}
	}
}

// TestPlayerCountersNoDoubling pins the §122.1g + engine ruling: player
// counters (poison/experience/rad) are NOT modified by Doubling Season
// because "permanent you control" excludes the player.
func TestPlayerCountersNoDoubling(t *testing.T) {
	for _, name := range []string{"poison", "experience", "rad"} {
		def := Lookup(name)
		if def == nil {
			t.Errorf("player counter %q not registered", name)
			continue
		}
		if def.DoublingApplies {
			t.Errorf("%q DoublingApplies = true — player counters must NOT double per engine ruling (Doubling Season scopes to permanents)", name)
		}
		// Player counters still proliferate per §701.27.
		if !def.Proliferate {
			t.Errorf("%q Proliferate = false — player counters MUST proliferate per §701.27", name)
		}
	}
}

// TestKeywordCounterStrikeAliases pins the legacy "strike counter" alias
// resolves to first strike per the WotC errata noted in §122.1c.
func TestKeywordCounterStrikeAliases(t *testing.T) {
	def := Lookup("strike")
	if def == nil {
		t.Fatal("strike not registered")
	}
	if def.GrantedAbility == nil || def.GrantedAbility.Keyword != "first strike" {
		t.Errorf("strike GrantedAbility = %+v, want Keyword=\"first strike\" (errata)", def.GrantedAbility)
	}
	// "strike counter" alias must resolve.
	via := Lookup("strike counter")
	if via == nil || via.Name != "strike" {
		t.Errorf("alias \"strike counter\" resolved to %+v, want canonical \"strike\"", via)
	}
}

// TestPhyresisGrantsInfect pins Weatherlight Compleated: a phyresis
// counter grants infect (not a "phyresis" keyword).
func TestPhyresisGrantsInfect(t *testing.T) {
	def := Lookup("phyresis")
	if def == nil {
		t.Fatal("phyresis not registered")
	}
	if def.GrantedAbility == nil || def.GrantedAbility.Keyword != "infect" {
		t.Errorf("phyresis GrantedAbility = %+v, want Keyword=\"infect\" (Weatherlight Compleated)", def.GrantedAbility)
	}
}

// TestDecayedCounterGrantsDecayed pins decayed counter behavior — the
// only counter form of the decayed keyword (Rot-Curse Rakshasa). The
// keyword grant must match the counter name.
func TestDecayedCounterGrantsDecayed(t *testing.T) {
	def := Lookup("decayed")
	if def == nil {
		t.Fatal("decayed not registered")
	}
	if def.GrantedAbility == nil || def.GrantedAbility.Keyword != "decayed" {
		t.Errorf("decayed GrantedAbility = %+v, want Keyword=\"decayed\"", def.GrantedAbility)
	}
}

// TestEveryKeywordCounterHasCRCitation pins that every keyword counter
// Notes string mentions CR §122.1c — the load-bearing rule grants the
// keyword while the counter is present.
func TestEveryKeywordCounterHasCRCitation(t *testing.T) {
	for _, def := range All() {
		if def.Category != KeywordGrant {
			continue
		}
		if !strings.Contains(def.Notes, "§122.1c") {
			t.Errorf("%q is KeywordGrant but Notes missing §122.1c citation: %q", def.Name, def.Notes)
		}
	}
}

// TestPhase5MinimumRegistrySize pins a floor — Phase 5 was expected to
// add ~230 long-tail entries. Allows growth, fails if a future refactor
// drops the long-tail in bulk.
func TestPhase5MinimumRegistrySize(t *testing.T) {
	if got := len(All()); got < 200 {
		t.Errorf("registry size = %d, want at least 200 after Phase 5 long-tail population", got)
	}
}

// TestPhase5LongtailCountMatchesExpected pins the exact count of Phase 5
// long-tail names declared at the top of this file is present in the
// registry. Defends against silent drop of entries in registry_longtail.go.
func TestPhase5LongtailCountMatchesExpected(t *testing.T) {
	registered := 0
	for _, name := range phase5LongtailNames {
		if Lookup(name) != nil {
			registered++
		}
	}
	if registered != len(phase5LongtailNames) {
		t.Errorf("Phase 5 longtail: %d/%d names present in registry", registered, len(phase5LongtailNames))
	}
}

// Package per_card holds the snowflake-handler registry for the Go engine,
// mirroring scripts/extensions/per_card_runtime.py. Each card whose oracle
// text cannot be expressed cleanly via the general gameast grammar gets a
// hand-rolled handler keyed on the card NAME (exact match on printed name).
//
// Wiring into the engine:
//
//   - gameengine exposes function-pointer hooks (ETBHook, CastHook,
//     ResolveHook, ActivatedHook, TriggerHook).
//   - This package's init() installs dispatch closures into those hooks.
//   - gameengine.stack.go / resolve.go call the hooks at the appropriate
//     moments; when the hook is nil (no per_card import), dispatch is a
//     no-op. This keeps gameengine dependency-free of per_card.
//
// All registrations are keyed by the card's printed name. Case-sensitive
// display, but we normalize for matching (lower-case, punctuation-free).
package per_card

import (
	"strings"
	"sync"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Registry is the per-card handler lookup table.
type Registry struct {
	mu        sync.RWMutex
	etb       map[string][]ETBHandler
	onCast    map[string][]CastHandler
	onResolve map[string][]ResolveHandler
	activated map[string][]ActivatedHandler
	onTrigger map[string]map[string][]TriggerHandler
}

// ETBHandler fires when a permanent with the registered name enters the
// battlefield, AFTER stock AST-driven ETB triggers have resolved.
type ETBHandler func(gs *gameengine.GameState, perm *gameengine.Permanent)

// CastHandler fires when a spell with the registered name is cast but
// before the stack item resolves.
type CastHandler func(gs *gameengine.GameState, item *gameengine.StackItem)

// ResolveHandler fires when a non-permanent spell with the registered
// name resolves. When any ResolveHandler is registered for a card, the
// stock Effect dispatch is SKIPPED — the handler is authoritative.
type ResolveHandler func(gs *gameengine.GameState, item *gameengine.StackItem)

// ActivatedHandler fires when an activated ability of the registered
// card is resolved. AbilityIdx is 0-based; ctx carries X, targets, etc.
type ActivatedHandler func(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{})

// TriggerHandler fires for a custom game-event trigger.
type TriggerHandler func(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{})

var (
	globalMu sync.RWMutex
	global   = newRegistry()
)

func newRegistry() *Registry {
	return &Registry{
		etb:       map[string][]ETBHandler{},
		onCast:    map[string][]CastHandler{},
		onResolve: map[string][]ResolveHandler{},
		activated: map[string][]ActivatedHandler{},
		onTrigger: map[string]map[string][]TriggerHandler{},
	}
}

// Global returns the process-wide registry.
func Global() *Registry {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return global
}

// Reset wipes the global registry and re-runs the batch-#1 default
// registration. Tests call this in t.Cleanup() to isolate runs.
func Reset() {
	globalMu.Lock()
	global = newRegistry()
	globalMu.Unlock()
	registerDefaults()
	installEngineHooks()
}

// NormalizeName lowercases, strips punctuation, and collapses whitespace.
func NormalizeName(name string) string {
	return normalizeName(name)
}

func normalizeName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	b.Grow(len(name))
	prevSpace := false
	for _, r := range name {
		switch r {
		case '\'', '\u2019', ',', '.', '!', '?', ':', ';', '-', '\u2014', '\u2013':
			continue
		case ' ', '\t':
			if !prevSpace {
				b.WriteRune(' ')
				prevSpace = true
			}
			continue
		}
		b.WriteRune(r)
		prevSpace = false
	}
	return b.String()
}

// -----------------------------------------------------------------------------
// Registration
// -----------------------------------------------------------------------------

// OnETB registers an ETB handler for cardName.
func (r *Registry) OnETB(cardName string, h ETBHandler) {
	if h == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	k := normalizeName(cardName)
	r.etb[k] = append(r.etb[k], h)
}

// OnCast registers a cast-time handler for cardName.
func (r *Registry) OnCast(cardName string, h CastHandler) {
	if h == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	k := normalizeName(cardName)
	r.onCast[k] = append(r.onCast[k], h)
}

// OnResolve registers a resolve-time handler for non-permanent spells.
func (r *Registry) OnResolve(cardName string, h ResolveHandler) {
	if h == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	k := normalizeName(cardName)
	r.onResolve[k] = append(r.onResolve[k], h)
}

// OnActivated registers an activated-ability handler.
func (r *Registry) OnActivated(cardName string, h ActivatedHandler) {
	if h == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	k := normalizeName(cardName)
	r.activated[k] = append(r.activated[k], h)
}

// OnTrigger registers a custom-event trigger handler.
func (r *Registry) OnTrigger(cardName, event string, h TriggerHandler) {
	if h == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	k := normalizeName(cardName)
	if r.onTrigger[k] == nil {
		r.onTrigger[k] = map[string][]TriggerHandler{}
	}
	canonical := gameengine.NormalizeEventSingle(event)
	r.onTrigger[k][canonical] = append(r.onTrigger[k][canonical], h)
}

// -----------------------------------------------------------------------------
// Dispatch helpers (called by the hook installers below)
// -----------------------------------------------------------------------------

// fireETB runs every ETB handler registered for perm's card name.
func fireETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if perm == nil || perm.Card == nil {
		return
	}
	name := perm.Card.DisplayName()
	reg := Global()
	reg.mu.RLock()
	nk := normalizeName(name)
	handlers := append([]ETBHandler(nil), reg.etb[nk]...)
	if len(handlers) == 0 {
		if idx := strings.Index(nk, " // "); idx >= 0 {
			handlers = append([]ETBHandler(nil), reg.etb[strings.TrimSpace(nk[:idx])]...)
		}
	}
	reg.mu.RUnlock()
	for _, h := range handlers {
		if gs != nil && gs.Flags != nil && gs.Flags["ended"] == 1 {
			break
		}
		h(gs, perm)
	}
}

// fireOnCast runs every OnCast handler.
func fireOnCast(gs *gameengine.GameState, item *gameengine.StackItem) {
	if item == nil || item.Card == nil {
		return
	}
	name := item.Card.DisplayName()
	reg := Global()
	reg.mu.RLock()
	handlers := append([]CastHandler(nil), reg.onCast[normalizeName(name)]...)
	reg.mu.RUnlock()
	for _, h := range handlers {
		h(gs, item)
	}
}

// fireOnResolve runs every OnResolve handler and returns the count fired.
// The caller (stack.go) uses the count to decide whether to skip stock
// effect dispatch.
func fireOnResolve(gs *gameengine.GameState, item *gameengine.StackItem) int {
	if item == nil || item.Card == nil {
		return 0
	}
	name := item.Card.DisplayName()
	reg := Global()
	reg.mu.RLock()
	handlers := append([]ResolveHandler(nil), reg.onResolve[normalizeName(name)]...)
	reg.mu.RUnlock()
	for _, h := range handlers {
		h(gs, item)
	}
	return len(handlers)
}

// fireActivated runs every activated-ability handler.
func fireActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if src == nil || src.Card == nil {
		return
	}
	name := src.Card.DisplayName()
	reg := Global()
	reg.mu.RLock()
	handlers := append([]ActivatedHandler(nil), reg.activated[normalizeName(name)]...)
	reg.mu.RUnlock()
	for _, h := range handlers {
		h(gs, src, abilityIdx, ctx)
	}
}

// isManaAbilityEvent returns true for events that correspond to mana
// abilities per CR §605.1b. Triggered abilities that trigger from
// activating a mana ability AND could produce mana are themselves mana
// abilities and do NOT use the stack (CR §605.3a). These events must
// resolve immediately, not be pushed to the stack.
func isManaAbilityEvent(event string) bool {
	switch event {
	case "mana_added_from_permanent":
		// Kinnan, Bonder Prodigy — adds one mana when a nonland permanent
		// is tapped for mana. This is a mana ability per CR §605.1b.
		return true
	}
	return false
}

// fireTrigger walks every permanent on the battlefield and fires
// matching (name, event) handlers. Used for Rhystic Study, Mystic Remora,
// Aetherflux Reservoir, Displacer Kitten, Hullbreaker Horror, Cloudstone
// Curio — all triggered-by-game-event cards.
//
// Per CR §603.3, triggered abilities are pushed to the stack via
// PushPerCardTrigger so players have priority to respond. Mana abilities
// (CR §605.1b / §605.3a) are exempt and resolve immediately.
//
// Per CR §101.4 (APNAP ordering), when multiple triggered abilities
// trigger simultaneously, the active player's triggers go on the stack
// FIRST (resolve LAST due to LIFO), then each other player in turn
// order. Within a single player's triggers, that player chooses the
// order (MVP: arbitrary/registration order).
func fireTrigger(gs *gameengine.GameState, event string, ctx map[string]interface{}) {
	if gs == nil || event == "" {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["trigger_depth"]++
	defer func() { gs.Flags["trigger_depth"]-- }()
	if gs.Flags["trigger_depth"] > 8 {
		return
	}
	gs.Flags["trigger_total"]++
	if gs.Flags["trigger_total"] > 2000 {
		return
	}
	canonical := gameengine.NormalizeEventSingle(event)

	// Collect hits grouped by controller seat.
	type hit struct {
		perm *gameengine.Permanent
		hs   []TriggerHandler
	}
	hitsBySeat := map[int][]hit{}

	reg := Global()
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for _, perm := range seat.Battlefield {
			if perm == nil || perm.Card == nil {
				continue
			}
			name := normalizeName(perm.Card.DisplayName())
			reg.mu.RLock()
			byEvent := reg.onTrigger[name]
			if byEvent == nil {
				if idx := strings.Index(name, " // "); idx >= 0 {
					byEvent = reg.onTrigger[strings.TrimSpace(name[:idx])]
				}
			}
			var handlers []TriggerHandler
			if byEvent != nil {
				handlers = append([]TriggerHandler(nil), byEvent[canonical]...)
			}
			reg.mu.RUnlock()
			if len(handlers) > 0 {
				hitsBySeat[seat.Idx] = append(hitsBySeat[seat.Idx], hit{perm: perm, hs: handlers})
			}
		}
	}

	if len(hitsBySeat) == 0 {
		return
	}

	// Push triggers in APNAP order (CR §101.4).
	// Active player's triggers go on stack FIRST (resolve LAST due to LIFO).
	apnap := gameengine.APNAPOrder(gs)
	usesStack := !isManaAbilityEvent(event)
	for _, seatIdx := range apnap {
		hits, ok := hitsBySeat[seatIdx]
		if !ok {
			continue
		}
		for _, h := range hits {
			// Log that trigger handlers were evaluated for this permanent.
			// Even if the handler's internal condition didn't fire (e.g.
			// Vindictive Vampire only fires on YOUR creatures dying),
			// the engine DID dispatch the trigger — TriggerCompleteness
			// needs to see this follow-up event to avoid false positives.
			gs.LogEvent(gameengine.Event{
				Kind:   "trigger_evaluated",
				Seat:   h.perm.Controller,
				Target: -1,
				Source: h.perm.Card.DisplayName(),
				Details: map[string]interface{}{
					"event":     event,
					"card_name": h.perm.Card.DisplayName(),
					"rule":      "603.3",
				},
			})
			for _, handler := range h.hs {
				if gs.Flags != nil && gs.Flags["ended"] == 1 {
					return
				}
				if usesStack {
					// Push to stack instead of resolving immediately (CR §603.3).
					// PushPerCardTrigger handles the priority round and resolution
					// inline, matching the PushTriggeredAbility pattern.
					gameengine.PushPerCardTrigger(gs, h.perm, handler, ctx)
				} else {
					// Mana ability — resolve immediately without using the stack.
					handler(gs, h.perm, ctx)
				}
			}
		}
	}
}

// HasETB reports whether any ETB handler is registered for cardName.
func HasETB(cardName string) bool {
	reg := Global()
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	return len(reg.etb[normalizeName(cardName)]) > 0
}

// HasResolve reports whether any Resolve handler is registered.
func HasResolve(cardName string) bool {
	reg := Global()
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	return len(reg.onResolve[normalizeName(cardName)]) > 0
}

// HasActivated reports whether any activated handler is registered.
func HasActivated(cardName string) bool {
	reg := Global()
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	return len(reg.activated[normalizeName(cardName)]) > 0
}

// RegisteredCardNames returns the deduplicated list of card names that have
// at least one handler registered (ETB, cast, resolve, activated, or trigger).
func (r *Registry) RegisteredCardNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	seen := map[string]bool{}
	for k := range r.etb {
		seen[k] = true
	}
	for k := range r.onCast {
		seen[k] = true
	}
	for k := range r.onResolve {
		seen[k] = true
	}
	for k := range r.activated {
		seen[k] = true
	}
	for k := range r.onTrigger {
		seen[k] = true
	}
	names := make([]string, 0, len(seen))
	for k := range seen {
		names = append(names, k)
	}
	return names
}

// HasCastAndTrigger reports whether cast or trigger handlers exist for a normalized name.
func (r *Registry) HasCastAndTrigger(normName string) (hasCast, hasTrigger bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	hasCast = len(r.onCast[normName]) > 0
	hasTrigger = len(r.onTrigger[normName]) > 0
	return
}

// HasTrigger reports whether a trigger handler is registered for cardName+event.
func HasTrigger(cardName, event string) bool {
	reg := Global()
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	byEvent := reg.onTrigger[normalizeName(cardName)]
	if byEvent == nil {
		return false
	}
	canonical := gameengine.NormalizeEventSingle(event)
	return len(byEvent[canonical]) > 0
}

// installEngineHooks wires our dispatch functions into gameengine's
// function-pointer hooks. Called from init() and Reset().
func installEngineHooks() {
	gameengine.ETBHook = fireETB
	gameengine.CastHook = fireOnCast
	gameengine.ResolveHook = fireOnResolve
	gameengine.ActivatedHook = fireActivated
	gameengine.TriggerHook = fireTrigger
	gameengine.HasTriggerHook = HasTrigger
}

// registerDefaults is populated by handlers.go — the card-specific
// Register* functions append to Global(). Declared here so init() can
// call it before handlers.go's init runs (Go guarantees init order
// within a package by file-lexical order — safer to have one entry
// point).
func registerDefaults() {
	// Batch #1 (landed 2026-04-16 earlier in session).
	registerThassasOracle(Global())
	registerDemonicConsultation(Global())
	registerTaintedPact(Global())
	registerUnderworldBreach(Global())
	registerAetherfluxReservoir(Global())
	registerFoodChain(Global())
	registerDoomsday(Global())
	registerDisplacerKitten(Global())
	registerRhysticStudy(Global())
	registerMysticRemora(Global())
	registerCloudstoneCurio(Global())
	registerHullbreakerHorror(Global())
	registerNecroticOoze(Global())
	registerHermitDruid(Global())
	registerWalkingBallista(Global())
	// Batch #2 (this session). 15 more handlers.
	registerLaboratoryManiac(Global())
	registerJaceWielderOfMysteries(Global())
	registerAdNauseam(Global())
	registerPeregrineDrake(Global())
	registerPalinchron(Global())
	registerDeadeyeNavigator(Global())
	registerBolassCitadel(Global())
	registerManaCrypt(Global())
	registerDocksideExtortionist(Global())
	registerUrzaLordHighArtificer(Global())
	registerEmryLurkerOfTheLoch(Global())
	registerSenseisDiviningTop(Global())
	registerAngelsGrace(Global())
	registerSundialOfTheInfinite(Global())
	registerGrandAbolisher(Global())
	// Batch #3 (this session). 15 more handlers — mana-rock combo
	// pieces, static hate, reanimator staples.
	registerKinnanBonderProdigy(Global())
	registerBasaltMonolith(Global())
	registerGrimMonolith(Global())
	registerIsochronScepter(Global())
	registerDramaticReversal(Global())
	registerNullRod(Global())
	registerCollectorOuphe(Global())
	registerCursedTotem(Global())
	registerDrannithMagistrate(Global())
	registerOppositionAgent(Global())
	registerNecropotence(Global())
	registerGriselbrand(Global())
	registerRazakethTheFoulblooded(Global())
	// Deadeye Navigator is already registered in batch #2; we
	// upgraded its ETB handler in-place (soulbond auto-pairing).
	registerPhantasmalImage(Global())
	// Batch #4 — nightmare verification fixes.
	registerChainsOfMephistopheles(Global())
	registerIxidron(Global())
	registerPanglacialWurm(Global())
	registerMindslaver(Global())
	registerPerplexingChimera(Global())
	// Batch #5 — nightmare verification PARTIAL/FAIL promotions.
	registerSneakAttack(Global())
	registerSwordOfFeastAndFamine(Global())
	registerPactOfNegation(Global())
	registerSanguineBond(Global())
	registerExquisiteBlood(Global())
	registerTormentOfHailfire(Global())
	registerAggravatedAssault(Global())
	registerStrionicResonator(Global())
	registerMirageMirror(Global())
	registerReleaseToTheWind(Global())
	// Batch #6 — Pact cycle: remaining 4 Pacts.
	registerPactOfTheTitan(Global())
	registerSlaughterPact(Global())
	registerInterventionPact(Global())
	registerSummonersPact(Global())
	// Batch #7 — portfolio staples: fetchlands, shocklands, tutors,
	// utility lands, removal, fast mana, card advantage.
	registerFetchlands(Global())
	registerShocklands(Global())
	registerDemonicTutor(Global())
	registerVampiricTutor(Global())
	registerMysticalTutor(Global())
	registerEnlightenedTutor(Global())
	registerWorldlyTutor(Global())
	registerRagavan(Global())
	registerTheOneRing(Global())
	registerEternalWitness(Global())
	registerSylvanLibrary(Global())
	registerChromeMox(Global())
	registerMoxDiamond(Global())
	registerMoxAmber(Global())
	registerMoxOpal(Global())
	registerGemstoneCaverns(Global())
	registerBojukaBog(Global())
	registerOtawara(Global())
	registerBoseiju(Global())
	registerRoguesPassage(Global())
	registerReliquaryTower(Global())
	registerAncientTomb(Global())
	registerUrzasSaga(Global())
	registerPathToExile(Global())
	registerSwordsToPlowshares(Global())
	registerCyclonicRift(Global())
	// Batch #8 — critical gap: sac outlets, board wipes, counterspells,
	// cantrips. The cards that make Commander PLAY like Commander.
	// Sac outlets:
	registerAshnodsAltar(Global())
	registerPhyrexianAltar(Global())
	registerVisceraSeer(Global())
	registerCarrionFeeder(Global())
	registerAltarOfDementia(Global())
	registerGoblinBombardment(Global())
	registerYahenni(Global())
	registerWoeStrider(Global())
	// Board wipes:
	registerWrathOfGod(Global())
	registerDamnation(Global())
	registerToxicDeluge(Global())
	registerBlasphemousAct(Global())
	registerVanquishTheHorde(Global())
	registerFarewell(Global())
	registerAustereCommand(Global())
	// Counterspells:
	registerNegate(Global())
	registerSwanSong(Global())
	registerDovinsVeto(Global())
	registerArcaneDenial(Global())
	registerDispel(Global())
	registerManaDrain(Global())
	// Cantrips:
	registerBrainstorm(Global())
	registerPonder(Global())
	registerPreordain(Global())
	registerGitaxianProbe(Global())
	registerOpt(Global())
	registerConsider(Global())

	// Kraum — draw on opponent's 2nd spell each turn
	registerKraum(Global())
	// Yarok — ETB trigger doubler (Panharmonicon for all permanents)
	registerYarok(Global())
	// Yuriko — ninja commander trigger (reveal top, drain opponents)
	registerYuriko(Global())
	// Simic Basilisk — grant basilisk combat-damage destroy ability
	registerSimicBasilisk(Global())
	// Fynn, the Fangbearer — deathtouch creatures give 2 poison on combat damage
	registerFynn(Global())
	// Davros, Dalek Creator — opponents get 3 rad counters when a player loses
	registerDavros(Global())

	// Batch #10 — Treasure Nabber, chaos/cascade cards, Phylactery Lich.
	registerTreasureNabber(Global())
	registerPossibilityStorm(Global())
	registerChaosWand(Global())
	registerThousandYearStorm(Global())
	registerPhylacteryLich(Global())
	registerEyeOfVecna(Global())
	registerHandOfVecna(Global())
	registerBookOfVileDarkness(Global())

	// Batch #11 — aristocrat death-trigger cards.
	registerBloodArtist(Global())
	registerZulaportCutthroat(Global())
	registerBastionOfRemembrance(Global())
	registerCruelCelebrant(Global())
	registerVindictiveVampire(Global())
	registerSyrKonrad(Global())

	// Batch #9 — 23 commander per-card handlers for the deck collection.
	registerArdenn(Global())
	registerCoram(Global())
	registerFireLordAzula(Global())
	registerKaust(Global())
	registerLordOfTheNazgul(Global())
	registerLumra(Global())
	registerMaja(Global())
	registerMoraug(Global())
	registerMuldrotha(Global())
	registerNarsetEnlightenedExile(Global())
	registerObeka(Global())
	registerOloro(Global())
	registerRagost(Global())
	registerRal(Global())
	registerRiku(Global())
	registerSatya(Global())
	registerSoraya(Global())
	registerTergrid(Global())
	registerUlrich(Global())
	registerVarina(Global())
	registerVoja(Global())
	registerYshtola(Global())
	registerYarus(Global())
	registerAshling(Global())

	// Ragost combo pieces — infinite damage via artifact/food sacrifice loop.
	registerCrimeNovelist(Global())
	registerNukaColaVendingMachine(Global())
	registerPenregonStrongbull(Global())

	// Batch #12 — forced-discard spells and payoffs (Tergrid ecosystem).
	registerHymnToTourach(Global())
	registerMindTwist(Global())
	registerDarkDeal(Global())
	registerDeliriumSkeins(Global())
	registerSyphonMind(Global())
	registerNecrogenMists(Global())
	registerBottomlessPit(Global())
	registerRankleMasterOfPranks(Global())
	registerOppression(Global())
	registerWasteNot(Global())
	registerLilianasCaress(Global())
	registerMegrim(Global())
	registerTinybones(Global())

	// Batch #13 — sacrifice-forcers and stax pieces (Tergrid ecosystem).
	registerFleshbagMarauder(Global())
	registerMercilessExecutioner(Global())
	registerPlaguecrafter(Global())
	registerInnocentBlood(Global())
	registerSmallpox(Global())
	registerPox(Global())
	registerGravePact(Global())
	registerGraveBetrayal(Global())
	registerLivingDeath(Global())
	registerArchfiendOfDepravity(Global())
	registerDeathCloud(Global())
	registerVictimize(Global())
	registerVonasHunger(Global())

	// Batch #14 — stax lock pieces (7174n1c request).
	registerDefenseGrid(Global())
	registerNotionThief(Global())
	// Trinisphere: already wired in ScanCostModifiers by name, no per-card handler needed.

	// Batch #15 — high-impact commander staples that were skewing ELO.
	registerSolRing(Global())
	registerManaVault(Global())
	registerDarkRitual(Global())
	registerSmotheringTithe(Global())
	registerWindfall(Global())
	registerWheelOfFortune(Global())
	registerEntomb(Global())
	registerReanimate(Global())
	registerAnimateDead(Global())
	registerForceOfWill(Global())
	registerForceOfNegation(Global())
	registerFierceGuardianship(Global())
	registerLionsEyeDiamond(Global())
	registerDauthiVoidwalker(Global())
	registerYawgmothThranPhysician(Global())
	registerGildedDrake(Global())
	registerSurvivalOfTheFittest(Global())

	// Batch #16 — WotC Game Changers: the 25 highest-impact cards in
	// Commander that were missing handlers, causing cEDH decks to
	// underperform against precons.
	registerHumility(Global())
	registerTeferisProtection(Global())
	registerConsecratedSphinx(Global())
	registerGiftsUngiven(Global())
	registerIntuition(Global())
	registerNarsetParterOfVeils(Global())
	registerBraidsCabalMinion(Global())
	registerImperialSeal(Global())
	registerOrcishBowmasters(Global())
	registerGamble(Global())
	registerJeskasWill(Global())
	registerBiorhythm(Global())
	registerCropRotation(Global())
	registerNaturalOrder(Global())
	registerSeedbornMuse(Global())
	registerAuraShards(Global())
	registerCoalitionVictory(Global())
	registerGrandArbiterAugustinIV(Global())
	registerFieldOfTheDead(Global())
	registerGaeasCradle(Global())
	registerGlacialChasm(Global())
	registerMishrasWorkshop(Global())
	registerPanopticMirror(Global())
	registerSerrasSanctum(Global())
	registerTabernacleAtPendrellVale(Global())

	// Batch #17 — token-creation triggers and aristocrat payoffs.
	// Anointed Procession's token_created trigger is upgraded in
	// batch17_sweep.go (registered via its own init()).
	registerChatterfang(Global())
	registerPitilessPlunderer(Global())

	// Batch #18 — drain commanders (lifegain→drain + ETB→drain).
	registerDinaSoulSteeper(Global())
	registerDinaEssenceBrewer(Global())
	registerVitoThornOfTheDuskRose(Global())
	registerVitoFanaticOfAclazotz(Global())
	registerMaraudingBlightPriest(Global())
	registerCorpseKnight(Global())

	// Batch #19 — enchantment toolbox commanders (attack-trigger tutors).
	registerZurTheEnchanter(Global())
	registerLightPawsEmperorsVoice(Global())
	// Batch #19 — artifact-commander synergy.
	registerDrafna(Global())
	registerJolene(Global())

	// Batch #20 — high-drift partner/toolbox commanders.
	registerThrasios(Global())
	registerVialSmasher(Global())

	// Batch #21 — Phase 5 drift commanders.
	registerAbdelAdrian(Global())
	registerMeria(Global())
	registerRaffine(Global())

	// Batch #22 — B5 partner commanders.
	registerTymna(Global())
	registerMalcolm(Global())
	registerEtali(Global())
	registerKrark(Global())

	// Batch #23 — B5 value commanders.
	registerTivit(Global())
	registerObNixilisCaptive(Global())
	registerMeren(Global())
	registerSelvala(Global())

	// Batch #24 — B5 combo/value commanders.
	registerElsha(Global())
	registerEmiel(Global())
	registerEdgarMarkov(Global())
	registerAtraxaGrandUnifier(Global())
	registerTayam(Global())
	registerArcumDagsson(Global())

	// Batch #25 — B5 commanders.
	registerAtraxaPraetorsVoice(Global())
	registerQueza(Global())
	registerFrancisco(Global())
	registerSidarKondo(Global())
	registerBladewing(Global())
	registerDihada(Global())
	registerRowanScion(Global())
	registerTeveshSzat(Global())
	registerOjerAxonil(Global())
	registerZurgoOjutai(Global())

	// Batch #26 — high-impact tutor / token / ninjutsu commanders.
	registerSisayWeatherlightCaptain(Global())
	registerKrenkoMobBoss(Global())
	registerSatoruUmezawa(Global())
	// Batch #26 — Spacecraft station, DFC saga commander, Merfolk tribal.
	registerHearthhull(Global())
	registerTerra(Global())
	registerHakbal(Global())

	// Batch #26 (cont) — newer commanders: death-trigger growth, discard
	// copy, graveyard recast.
	registerGorma(Global())
	registerHashaton(Global())
	registerShiko(Global())

	// Batch #26 (cont) — full-coverage push: Elder Dragon Cycle (B/G + U/R)
	// + planeswalker commander.
	registerWitherbloom(Global())
	registerPrismari(Global())
	registerQuintorius(Global())

	// Batch #26 (cont) — Elder Dragon historian, lifegain pest anthem,
	// -1/-1 counter trigger goblin.
	registerLorehold(Global())
	registerBlech(Global())
	registerAuntieOol(Global())

	// Batch #27 — staple commanders: attack-trigger cheat-in,
	// counter-scaling cost reduction, global P/T swap.
	registerKaaliaOfTheVast(Global())
	registerAnimarSoulOfElements(Global())
	registerDoranTheSiegeTower(Global())

	// Batch #27 (cont) — Dinosaur ETB discover, attack-life-gradient draw,
	// opp-creature exile-instead-of-die / end-step Rat tokens.
	registerPantlaza(Global())
	registerBreena(Global())
	registerVren(Global())

	// Batch #27 (cont) — high-impact ETB/activated/trigger commanders:
	// Sauron (amass + Ring tempt), Kenrith (5-mode activated),
	// Niv-Mizzet Reborn (guild-pair reveal-10).
	registerSauronDarkLord(Global())
	registerKenrithReturnedKing(Global())
	registerNivMizzetReborn(Global())

	// Batch #27 (cont) — extra-draw / DFC reveal-cheat / Eldrazi copy.
	registerNekusar(Global())
	registerEsika(Global())
	registerUlalek(Global())

	// Batch #27 (cont) — cost-reducer (creature-target), top-of-library
	// MV>=4 cast + upkeep surveil, X-spell counter scaler.
	registerKillianInkDuelist(Global())
	registerGlarbCalamitysAugur(Global())
	registerZimoneInfiniteAnalyst(Global())

	// Batch #28 — beginning-of-combat creature-count gradient, ETB-draw +
	// discard drain, graveyard/exile recast treasure engine.
	registerTeval(Global())
	registerMorcant(Global())
	registerGolbez(Global())

	// Batch #28 (cont) — {0}-cost partner stub, library-exile combat
	// trigger, copy-spell-and-bounce-self activated commander.
	registerRograkh(Global())
	registerAshlingTheLimitless(Global())
	registerRootha(Global())

	// Batch #28 (cont) — prowess team-anthem + cast-trigger unblockable,
	// Dragon-tribal ETB/attack reveal-cheat, life-delta end-step double
	// payoff (counter buff + reanimate).
	registerBriaRiptideRogue(Global())
	registerUreniOfTheUnwritten(Global())
	registerBetorAncestorsVoice(Global())

	// Batch #28 (cont) — combat-payoff Tarkir dragon-clan commander,
	// FF DFC drain/transform/sac-draw, Rat-tribal token swarm.
	registerEshkiDragonclaw(Global())
	registerSephiroth(Global())
	registerMarrowGnawer(Global())

	// Batch #28 (cont) — combat-damage free-cast from graveyard,
	// lifegain end-step draw + Cleric token activation, second-draw
	// Rabbit token + token-count draw activation.
	registerViviOrnitier(Global())
	registerBertaWiseExtrapolator(Global())
	registerMsBumbleflower(Global())

	// Batch #29 — spell-copy commanders + DFC discard/transform.
	// Alania (first instant/sorcery/Otter copy w/ opponent draw),
	// Mica (sac-artifact instant/sorcery copy),
	// Kefka DFC (ETB/attack wheel + transform + opp-life-loss draw).
	registerAlania(Global())
	registerMica(Global())
	registerKefka(Global())

	// Batch #29 (cont) — graveyard-recast token-swarm + ward + surveil
	// (Saruman), multicolored-cast WURG modal payoff (Aragorn),
	// noncreature-cast Spirit token + sac-for-red ritual (Kykar).
	registerSarumanTheWhiteHand(Global())
	registerAragornTheUniter(Global())
	registerKykarWindsFury(Global())

	// Batch #29 (cont) — token-on-attack + combat-death-draw soldier
	// commander (Caesar), ETB-draw + legendary-spell scry artificer
	// (Kilo), combat-damage-toughness lifegain partner snake (Ikra).
	registerCaesarLegionsEmperor(Global())
	registerKiloApogeeMind(Global())
	registerIkraShidiqi(Global())

	// Batch #29 (cont) — Jund blitz cost-reducer, mono-B life-for-mana
	// snowballer, GW counter-spreader attack trigger.
	registerHenzieToolboxTorre(Global())
	registerKrrikSonOfYawgmoth(Global())
	registerSovereignOkinecAhau(Global())

	// Batch #29 (cont) — creature-spell-cast scaler + power-X mana
	// (Helga), owner-bounce-on-death w/ pay-3-life (Athreos), first
	// life-loss-per-opponent-turn growth + draw (Valgavoth).
	registerHelgaSkittishSeer(Global())
	registerAthreosGodOfPassage(Global())
	registerValgavothHarrowerOfSouls(Global())

	// Batch #30 — high-deck-count commanders missing handlers:
	// Brago combat-blink (any number of nonland permanents you control),
	// Korvold ETB/attack sac + sacrifice payoff (counter + draw).
	// Prismari, the Inspiration storm grant is already wired in batch #26.
	registerBragoKingEternal(Global())
	registerKorvoldFaeCursedKing(Global())
	// Batch #30 (cont) — FF Alien combat-counter / Mutant-death draw,
	// Elder Dragon DFC ETB-discard / activate-transform-to-planeswalker,
	// Zombie tribal death-token-with-decayed / end-step sac-draw.
	registerJenovaAncientCalamity(Global())
	registerNicolBolas(Global())
	registerWilheltTheRotcleaver(Global())
	// Batch #30 (cont) — high-deck-count commanders missing handlers:
	// Jodah legendary-cast cascade-into-legendary-nonland, Umbris
	// nightmare/horror ETB exile-from-opp-library-until-land, The
	// Necrobloom landfall token (Plant or Zombie at 7+ distinct lands).
	registerJodahTheUnifier(Global())
	registerUmbrisFearManifest(Global())
	registerTheNecrobloom(Global())

	// Batch #30 (cont) — Fallout dice/treasure activated commander,
	// FF7 Melee + 7+-power damage untap & extra-combat trigger,
	// Ashling Rekindled // Rimebound modal DFC alternation loot/ramp.
	registerMrHousePresidentAndCEO(Global())
	registerTifaMartialArtist(Global())
	registerAshlingRekindled(Global())

	// Batch #30 (cont) — oracle-text correctness sweep for high-deck-count
	// commanders whose original handlers in batches #9 and #26 were built
	// from inaccurate descriptions. The register* calls themselves remain
	// in their batch-of-record above; the function bodies were rewritten in
	// dedicated files (fire_lord_azula.go, yshtola_nights_blessed.go,
	// witherbloom.go) and stubbed out of commanders_batch.go. Rewrites:
	//   - Fire Lord Azula: firebending 2 + spell-copy while attacking
	//     (was: noncreature spell during combat → 1/1 token).
	//   - Y'shtola, Night's Blessed: vigilance + end-step 4-life-loss
	//     draw + noncreature MV3+ ping-each-opp/gain-2 (was: ETB draw
	//     equal to power).
	//   - Witherbloom, the Balancer: affinity-for-creatures cost
	//     reduction wired through cost_modifiers.go for both Witherbloom
	//     herself and the granted instant/sorcery clause.

	// Batch #30 (cont) — Fallout rad-counter Insect Mutant (Mothman ETB +
	// attack rad-each-player), Final Fantasy equipment-attach + power-7
	// treasure attack (Cloud), and the classic 5C Dragon eminence
	// cost-reducer + group-attack draw + permanent-from-hand cheat
	// (The Ur-Dragon).
	registerTheWiseMothman(Global())
	registerCloudExSoldier(Global())
	registerTheUrDragon(Global())

	// Batch #31 — under-supported commanders surfaced by deck-coverage
	// audit: Phyrexian sacrifice growth (Szarel), mobilize attack-token
	// commander with combat LTB drain/draw (Zurgo Stormrender), AC
	// crossover treasure-on-tapped-tribe + Vehicle exile/play
	// (Edward Kenway).
	registerSzarelGenesisShepherd(Global())
	registerZurgoStormrender(Global())
	registerEdwardKenway(Global())

	// Batch #31 (cont) — Bloomburrow draw-or-discard frog with extra
	// land drop, Strixhaven prepare DFC bird bard (creature-cast →
	// +1/+1 counter via Heroic Stanza copy), Assassin's Creed attack-
	// trigger graveyard-exile-and-copy swarm.
	registerFlubsTheFool(Global())
	registerAbigalePoetLaureate(Global())
	registerAltairIbnLaAhad(Global())

	// Batch #32 — high-deck-count commanders missing handlers:
	// Najeela (Warrior tribal token spawn + WUBRG extra-combat untap),
	// Winota (non-Human attack → top-six Human cheat),
	// Prosper (ETB/attack impulse exile + end-step drain & treasure).
	registerNajeelaBladeBlossom(Global())
	registerWinotaJoinerOfForces(Global())
	registerProsperTomeBound(Global())

	// Batch #32 (cont) — three more commanders surfaced by the deck-
	// coverage audit: Scriv, the Obligator (W/B Aetherdrift Inkling — ETB
	// + attack Contract Aura token attached to opponent creature), Gonti,
	// Canny Acquisitor (Sultai cost-reduction for unowned spells +
	// combat-damage exile/play with any-mana clause), Gishath, Sun's
	// Avatar (Naya Dinosaur tribal — combat-damage reveal-N and put
	// dinosaur creature cards onto the battlefield).
	registerScrivTheObligator(Global())
	registerGontiCannyAcquisitor(Global())
	registerGishathSunsAvatar(Global())

	// Batch #32 — high-deck-count commander handlers (oracle-text-correct):
	// Killian, Decisive Mentor (NOT Killian, Ink Duelist) — enchantment ETB
	// tap+goad + aura-attacker draw; Doran, Besieged by Time (NOT Doran,
	// the Siege Tower) — toughness>power cost reduction + +X/+X attack
	// buff where X = T-P; Leonardo, the Balance (TMNT crossover) —
	// once-per-turn +1/+1 spread on token ETB + WUBRG menace/trample/
	// lifelink team grant.
	registerKillianDecisiveMentor(Global())
	registerDoranBesiegedByTime(Global())
	registerLeonardoTheBalance(Global())

	// Batch #32 (cont) — three more commanders surfaced by deck-coverage
	// audit: Urza, Chief Artificer (NOT Urza, Lord High Artificer) —
	// affinity-for-artifact-creatures cost reducer + artifact-creature
	// menace grant + end-step Construct token; Lathril, Blade of the Elves
	// (Elf tribal Combat-damage Elf Warrior tokens + tap-ten-Elves drain
	// activated); The Gitrog, Ravenous Ride (NOT The Gitrog Monster) —
	// Mount with Saddle 1, combat-damage sacrifice-saddler / draw X /
	// fetch X lands tapped.
	registerUrzaChiefArtificer(Global())
	registerLathrilBladeOfTheElves(Global())
	registerGitrogRavenousRide(Global())

	// Batch #32 (cont) — under-supported transforming commanders:
	// MOM saga DFC praetor with combat-damage punishment trigger and
	// {2}{W} + sac-three-creatures activated transform (Elesh Norn //
	// The Argent Etchings); Cat-tribal flip planeswalker — ETB token,
	// Cat-death may-transform, +2 counter spread / 0 token+damage /
	// -4 selective opponent sweep (Ajani, Nacatl Pariah // Ajani,
	// Nacatl Avenger); 4C cascade-on-combat-damage commander — grants
	// hand-cast cascade until end of turn (Yidris, Maelstrom Wielder).
	registerEleshNornArgentEtchings(Global())
	registerAjaniNacatlPariah(Global())
	registerYidrisMaelstromWielder(Global())

	// Batch #33 — high-deck-count commanders missing handlers:
	// Syr Gwyn, Hero of Ashvale (Mardu Knight/Equipment commander —
	// equip-{0} static while you control a Knight + equipped-attacker
	// draw card / drain each opponent 1), Riku of Two Reflections
	// (Temur copy commander — pay {2}{G/U} to token-copy a creature
	// spell or copy an instant/sorcery on the stack). Brago, King
	// Eternal already wired in Batch #30.
	registerSyrGwynHeroOfAshvale(Global())
	registerRikuOfTwoReflections(Global())

	// Batch #33 (cont) — three more commanders surfaced by the deck-
	// coverage audit: Lord Windgrace (BRG land-recursion planeswalker
	// commander — discard-for-land/draw, graveyard-land reanimate, and
	// nonland-wipe + library-land-fetch ultimate), Ghave, Guru of Spores
	// (Abzan +1/+1-counter & Saproling generator — ETB with five +1/+1
	// counters, counter-for-Saproling and sac-for-counter activations),
	// Jhoira, Weatherlight Captain (Esper artifact-historic cantrip
	// commander — draw on each historic spell cast).
	registerLordWindgrace(Global())
	registerGhaveGuruOfSpores(Global())
	registerJhoiraWeatherlightCaptain(Global())

	// Batch #33 (cont) — three more commanders surfaced by the deck-
	// coverage audit: Jin Sakai, Ghost of Tsushima (Esper Samurai —
	// combat-damage draw + lone-attacker modal Standoff/Ghost), Wan Shi
	// Tong, All-Knowing (mono-U Bird Spirit — ETB tuck nonland to bottom
	// + library-add Spirit token doubler), Ashnod, Flesh Mechanist
	// (mono-B Artificer — attack sac → tapped Powerstone + {5}+exile-
	// creature-card activated 3/3 Zombie artifact creature token).
	registerJinSakaiGhostOfTsushima(Global())
	registerWanShiTongAllKnowing(Global())
	registerAshnodFleshMechanist(Global())

	// Batch #33 (cont) — three more commanders surfaced by deck-coverage
	// audit: Zask, Skittering Swarmlord (mono-G/G Insect graveyard
	// recursion — graveyard land/Insect cast permission + another-Insect-
	// dies bottom-of-library + mill-2 trigger + {1}{B/G} +1/+0 deathtouch
	// activated), Kibo, Uktabi Prince (R/G Monkey Noble — {T}-create-Banana-
	// for-each-player + opponent-artifact-dies Ape/Monkey buff + attack-
	// force-sac-artifact), Frodo, Adventurous Hobbit (W/B Halfling Scout —
	// partner with Sam, vigilance, gained-3+-life attack-trigger Ring tempt
	// with Ring-bearer ≥2-tempt draw payoff).
	registerZaskSkitteringSwarmlord(Global())
	registerKiboUktabiPrince(Global())
	registerFrodoAdventurousHobbit(Global())

	// Batch #33 (cont) — Grist, the Hunger Tide (Golgari planeswalker/
	// insect commander — Insect token + mill-loop +1, sac-creature →
	// destroy -2, creature-graveyard drain -5), The Infamous Cruelclaw
	// (Rakdos Weasel Mercenary — combat-damage exile-until-nonland +
	// alt cost discard-instead-of-mana cast permission), Rivaz of the
	// Claw (Rakdos Lizard Warlock — typed-mana Dragon-only ramp,
	// once-per-turn Dragon graveyard cast permission with exile-on-die
	// rider).
	registerGristTheHungerTide(Global())
	registerInfamousCruelclaw(Global())
	registerRivazOfTheClaw(Global())

	// Batch #33 (cont) — Final Fantasy + March of the Machine commanders
	// surfaced by the deck-coverage audit: Ardbert, Warrior of Darkness
	// (FF14 Orzhov Spirit Warrior — color-cast +1/+1 spread on legendaries
	// + vigilance/menace UEOT), Terra, Herald of Hope (FF6 Mardu Wizard
	// Warrior — Trance combat-begin self-mill 2 + flying UEOT, combat-
	// damage may-pay {2} return ≤3-power creature card from graveyard
	// tapped), Sidar Jabari of Zhalfir (Esper Human Knight — Eminence
	// attack-with-Knight loot + combat-damage Knight reanimate).
	registerArdbertWarriorOfDarkness(Global())
	registerTerraHeraldOfHope(Global())
	registerSidarJabariOfZhalfir(Global())

	// Batch #33 (cont) — three more high-deck-count commanders surfaced
	// by deck-coverage scan: Primo, the Unbounded ({X}{G}{G}{U} Fractal
	// Wolf — ETB with twice-X +1/+1 counters + base-power-0 combat
	// damage spawns a counter-loaded Fractal token); Karlach, Fury of
	// Avernus ({4}{R} Tiefling Barbarian — first-combat untap + first-
	// strike grant + extra combat); Hinata, Dawn-Crowned ({1}{U}{R}{W}
	// Kirin Spirit — symmetric per-target cost reduction/tax wired into
	// ScanCostModifiers).
	registerPrimoTheUnbounded(Global())
	registerKarlachFuryOfAvernus(Global())
	registerHinataDawnCrowned(Global())

	// Auto-generated handlers (cmd/gen-handlers).
	registerGeneratedHandlers(Global())
}

func init() {
	registerDefaults()
	installEngineHooks()
}

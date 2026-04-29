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
}

func init() {
	registerDefaults()
	installEngineHooks()
}

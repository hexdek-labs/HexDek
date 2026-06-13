package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// damage_replacement_static_r63b.go — second wave of the CR §614 DAMAGE
// replacement backlog (replacement-damage-r63b). Wave 1
// (damage_scale_family_r63.go) wired the simple fixed mult/bonus
// source-filtered doublers. This wave covers the remaining STATIC
// (continuous, battlefield-resident) damage replacements that still fit
// the gs.DamageReplacements registry shape but need a richer closure:
//
//   - reductions / set-to-N: Ghosts of the Innocent (half), Benevolent
//     Unicorn (−1 spell), Lashknife Barrier (−1 to your creatures),
//     Divine Presence (≥4 → 3), Forethought Amulet (i/s ≥3 to you → 2)
//   - self / target-scoped doublers: Charging Tuskodon, Goldnight
//     Castigator, Tor Wauki the Younger
//   - attachment-scoped doublers: The Sound of Drums (aura),
//     Inquisitor's Flail (equipment)
//   - condition-gated doublers: Anthem of Rakdos (hellbent), Far Fortune
//     (max speed), The Rollercrusher Ride (delirium), Aether Revolt
//     (revolt), Fated Firepower (+fire-counter count)
//   - mixed prevent + bonus: Rem Karolus, Stalwart Slayer
//
// All were inert (their replacement parsed to a log-only
// if_intervening_tail / replacement_static scaffold). Each registers at
// ETB on gs.DamageReplacements (consulted by combat + spell + DealDamage
// before §615 prevention, per the verified damage-615 audit) and
// unregisters at permanent_ltb. ONE-SHOT "next time … this turn"
// redirects, chosen-player ETB picks, this-turn spell effects, and
// transformed-DFC back-face filters are NOT in scope — they need a
// next-damage shield abstraction / stored state and are logged as
// remaining backlog.

func init() {
	registerDamageReplStaticFamily(Global())
	AddResetHook(registerDamageReplStaticFamily)
}

// registerDamageRepl wires ETB-register / LTB-unregister boilerplate for an
// arbitrary §614 damage-replacement closure. makeFn receives the owning
// permanent (so the closure can capture self / self.AttachedTo / counters).
func registerDamageRepl(r *Registry, cardName, handlerID string, makeFn func(self *gameengine.Permanent) gameengine.DamageReplacementFn) {
	etb := func(gs *gameengine.GameState, perm *gameengine.Permanent) {
		if gs == nil || perm == nil || perm.Card == nil {
			return
		}
		gs.RegisterDamageReplacement(&gameengine.DamageReplacement{
			SourcePerm: perm,
			HandlerID:  handlerID,
			Fn:         makeFn(perm),
		})
		emit(gs, handlerID, perm.Card.DisplayName(), map[string]interface{}{
			"seat": perm.Controller,
		})
	}
	ltb := func(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
		if gs == nil || perm == nil || ctx == nil {
			return
		}
		if leaving, _ := ctx["perm"].(*gameengine.Permanent); leaving != perm {
			return
		}
		gs.UnregisterDamageReplacementsForPermanent(perm)
	}
	r.OnETB(cardName, etb)
	r.OnTrigger(cardName, "permanent_ltb", ltb)
}

// dsIsCombat reports whether the damage event is combat damage.
func dsIsCombat(ctx *gameengine.DamageContext) bool {
	return ctx.Kind == gameengine.DamageCombatPlayer || ctx.Kind == gameengine.DamageCombatCreature
}

// dsTargetIsYou: player-target damage aimed at self's controller.
func dsTargetIsYou(self *gameengine.Permanent, ctx *gameengine.DamageContext) bool {
	return ctx.TargetPerm == nil && ctx.TargetSeat == self.Controller
}

func registerDamageReplStaticFamily(r *Registry) {
	// ---- reductions / set-to-N (symmetric or target-scoped) ----

	// Ghosts of the Innocent: any source → half that damage, rounded down.
	registerDamageRepl(r, "Ghosts of the Innocent", "ghosts_of_innocent_halve",
		func(self *gameengine.Permanent) gameengine.DamageReplacementFn {
			return func(gs *gameengine.GameState, ctx *gameengine.DamageContext) {
				ctx.Amount = ctx.Amount / 2 // integer floor = rounded down
			}
		})

	// Benevolent Unicorn: a spell would deal damage → minus 1 (floor 0).
	registerDamageRepl(r, "Benevolent Unicorn", "benevolent_unicorn_minus1",
		func(self *gameengine.Permanent) gameengine.DamageReplacementFn {
			return func(gs *gameengine.GameState, ctx *gameengine.DamageContext) {
				if !dsInstantOrSorcery(ctx) {
					return
				}
				if ctx.Amount -= 1; ctx.Amount < 0 {
					ctx.Amount = 0
				}
			}
		})

	// Lashknife Barrier: source → a creature you control → minus 1 (floor 0).
	registerDamageRepl(r, "Lashknife Barrier", "lashknife_barrier_minus1",
		func(self *gameengine.Permanent) gameengine.DamageReplacementFn {
			return func(gs *gameengine.GameState, ctx *gameengine.DamageContext) {
				if ctx.TargetPerm == nil || !ctx.TargetPerm.IsCreature() {
					return
				}
				if ctx.TargetPerm.Controller != self.Controller {
					return
				}
				if ctx.Amount -= 1; ctx.Amount < 0 {
					ctx.Amount = 0
				}
			}
		})

	// Divine Presence: any source dealing 4+ → deals 3 instead.
	registerDamageRepl(r, "Divine Presence", "divine_presence_set3",
		func(self *gameengine.Permanent) gameengine.DamageReplacementFn {
			return func(gs *gameengine.GameState, ctx *gameengine.DamageContext) {
				if ctx.Amount >= 4 {
					ctx.Amount = 3
				}
			}
		})

	// Forethought Amulet: instant/sorcery source dealing 3+ to YOU → 2.
	registerDamageRepl(r, "Forethought Amulet", "forethought_amulet_set2",
		func(self *gameengine.Permanent) gameengine.DamageReplacementFn {
			return func(gs *gameengine.GameState, ctx *gameengine.DamageContext) {
				if !dsInstantOrSorcery(ctx) || !dsTargetIsYou(self, ctx) {
					return
				}
				if ctx.Amount >= 3 {
					ctx.Amount = 2
				}
			}
		})

	// ---- self / target-scoped doublers ----

	// Charging Tuskodon: this creature → combat damage to a player → ×2.
	registerDamageRepl(r, "Charging Tuskodon", "charging_tuskodon_double",
		func(self *gameengine.Permanent) gameengine.DamageReplacementFn {
			return func(gs *gameengine.GameState, ctx *gameengine.DamageContext) {
				if ctx.Source == self && ctx.Kind == gameengine.DamageCombatPlayer {
					ctx.Amount *= 2
				}
			}
		})

	// Goldnight Castigator: damage to you ×2; damage to this creature ×2.
	registerDamageRepl(r, "Goldnight Castigator", "goldnight_castigator_double",
		func(self *gameengine.Permanent) gameengine.DamageReplacementFn {
			return func(gs *gameengine.GameState, ctx *gameengine.DamageContext) {
				if dsTargetIsYou(self, ctx) || ctx.TargetPerm == self {
					ctx.Amount *= 2
				}
			}
		})

	// Tor Wauki the Younger: another source you control → NONcombat → +1.
	registerDamageRepl(r, "Tor Wauki the Younger", "tor_wauki_plus1",
		func(self *gameengine.Permanent) gameengine.DamageReplacementFn {
			return func(gs *gameengine.GameState, ctx *gameengine.DamageContext) {
				if !dsYouControl(self, ctx) || ctx.Source == self || dsIsCombat(ctx) {
					return
				}
				ctx.Amount += 1
			}
		})

	// ---- attachment-scoped doublers ----

	// The Sound of Drums (Aura): enchanted creature → combat damage → ×2.
	registerDamageRepl(r, "The Sound of Drums", "sound_of_drums_double",
		func(self *gameengine.Permanent) gameengine.DamageReplacementFn {
			return func(gs *gameengine.GameState, ctx *gameengine.DamageContext) {
				if self.AttachedTo != nil && ctx.Source == self.AttachedTo && dsIsCombat(ctx) {
					ctx.Amount *= 2
				}
			}
		})

	// Inquisitor's Flail (Equipment): equipped creature deals combat damage
	// → ×2; another creature deals combat damage TO equipped creature → ×2.
	registerDamageRepl(r, "Inquisitor's Flail", "inquisitors_flail_double",
		func(self *gameengine.Permanent) gameengine.DamageReplacementFn {
			return func(gs *gameengine.GameState, ctx *gameengine.DamageContext) {
				eq := self.AttachedTo
				if eq == nil || !dsIsCombat(ctx) {
					return
				}
				if ctx.Source == eq { // equipped creature dealing combat damage
					ctx.Amount *= 2
					return
				}
				if ctx.TargetPerm == eq && ctx.Source != eq { // combat damage to equipped
					ctx.Amount *= 2
				}
			}
		})

	// ---- condition-gated doublers / bonuses ----

	// Anthem of Rakdos — Hellbent: empty hand → source you control → ×2.
	registerDamageRepl(r, "Anthem of Rakdos", "anthem_of_rakdos_hellbent_double",
		func(self *gameengine.Permanent) gameengine.DamageReplacementFn {
			return func(gs *gameengine.GameState, ctx *gameengine.DamageContext) {
				if !dsYouControl(self, ctx) {
					return
				}
				if self.Controller < 0 || self.Controller >= len(gs.Seats) {
					return
				}
				if len(gs.Seats[self.Controller].Hand) != 0 {
					return
				}
				ctx.Amount *= 2
			}
		})

	// Far Fortune, End Boss — Max speed: source you control → opp → +1.
	registerDamageRepl(r, "Far Fortune, End Boss", "far_fortune_maxspeed_plus1",
		func(self *gameengine.Permanent) gameengine.DamageReplacementFn {
			return func(gs *gameengine.GameState, ctx *gameengine.DamageContext) {
				if !dsYouControl(self, ctx) || !dsOpponentTarget(self, ctx) {
					return
				}
				if !gameengine.MaxSpeedActive(gs, self.Controller) {
					return
				}
				ctx.Amount += 1
			}
		})

	// The Rollercrusher Ride — Delirium: source you control → NONcombat → ×2.
	registerDamageRepl(r, "The Rollercrusher Ride", "rollercrusher_delirium_double",
		func(self *gameengine.Permanent) gameengine.DamageReplacementFn {
			return func(gs *gameengine.GameState, ctx *gameengine.DamageContext) {
				if !dsYouControl(self, ctx) || dsIsCombat(ctx) {
					return
				}
				if !gameengine.DeliriumActive(gs, self.Controller) {
					return
				}
				ctx.Amount *= 2
			}
		})

	// Aether Revolt — Revolt: source you control → NONcombat → opp → +2.
	registerDamageRepl(r, "Aether Revolt", "aether_revolt_plus2",
		func(self *gameengine.Permanent) gameengine.DamageReplacementFn {
			return func(gs *gameengine.GameState, ctx *gameengine.DamageContext) {
				if !dsYouControl(self, ctx) || dsIsCombat(ctx) || !dsOpponentTarget(self, ctx) {
					return
				}
				if !gameengine.RevoltActive(gs, self.Controller) {
					return
				}
				ctx.Amount += 2
			}
		})

	// Fated Firepower: source you control → opp → +N (N = fire counters).
	registerDamageRepl(r, "Fated Firepower", "fated_firepower_plus_fire_counters",
		func(self *gameengine.Permanent) gameengine.DamageReplacementFn {
			return func(gs *gameengine.GameState, ctx *gameengine.DamageContext) {
				if !dsYouControl(self, ctx) || !dsOpponentTarget(self, ctx) {
					return
				}
				if self.Counters == nil {
					return
				}
				if n := self.Counters["fire"]; n > 0 {
					ctx.Amount += n
				}
			}
		})

	// ---- mixed prevent + bonus ----

	// Rem Karolus, Stalwart Slayer: a spell to you / your permanent →
	// prevent; a spell to an opponent / their permanent → +1.
	registerDamageRepl(r, "Rem Karolus, Stalwart Slayer", "rem_karolus_spell_prevent_or_plus1",
		func(self *gameengine.Permanent) gameengine.DamageReplacementFn {
			return func(gs *gameengine.GameState, ctx *gameengine.DamageContext) {
				if !dsInstantOrSorcery(ctx) {
					return
				}
				if ctx.TargetSeat == self.Controller {
					ctx.Prevented = true
					return
				}
				ctx.Amount += 1
			}
		})
}

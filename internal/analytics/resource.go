package analytics

import "github.com/hexdek/hexdek/internal/gameengine"

// Resource types aligned with Freya's ResourceType constants (which live in
// cmd/mtgsquad-freya and cannot be imported). These are simple strings so
// they match for causal-link comparison without coupling the packages.
const (
	ResMana      = "mana"
	ResToken     = "token"
	ResCard      = "card"
	ResLife      = "life"
	ResDamage    = "damage"
	ResPermanent = "permanent"
	ResLand      = "land"
	ResTrigger   = "trigger"
)

// EventResources maps a game event to the resources it produces and consumes.
// This is event-based (not oracle-text-based) so it works without Freya.
func EventResources(ev *gameengine.Event) (produces, consumes []string) {
	switch ev.Kind {
	case "create_token":
		produces = append(produces, ResToken)
	case "draw_card":
		produces = append(produces, ResCard)
	case "damage":
		produces = append(produces, ResDamage)
	case "life_change":
		if ev.Amount > 0 {
			produces = append(produces, ResLife)
		} else if ev.Amount < 0 {
			consumes = append(consumes, ResLife)
		}
	case "pay_mana":
		consumes = append(consumes, ResMana)
	case "pool_drain":
		// Mana was generated but wasted -- still produced.
		produces = append(produces, ResMana)
	case "enter_battlefield":
		produces = append(produces, ResPermanent)
	case "leave_battlefield":
		consumes = append(consumes, ResPermanent)
	case "play_land":
		produces = append(produces, ResLand)
	case "cast":
		consumes = append(consumes, ResMana)
	case "triggered_ability":
		produces = append(produces, ResTrigger)
	case "sacrifice":
		consumes = append(consumes, ResPermanent)
	case "destroy":
		consumes = append(consumes, ResPermanent)
	}
	return
}

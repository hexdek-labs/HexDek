package gameengine

import (
	"github.com/hexdek/hexdek/internal/gameengine/counters"
)

// Counter DB Phase 4 — gameengine-level Proliferate wrapper per CR §701.23.
//
// Lifts the primitive in internal/gameengine/counters/proliferate.go into
// engine context: adapts *Permanent and *Seat to counters.Target,
// mirrors legacy player-counter fields (Seat.PoisonCounters,
// Flags["experience_counters"], Flags["rad_counters"]) onto the new
// counters.CounterStack store so existing call sites that read the
// legacy fields keep seeing the post-proliferate value, emits the
// canonical "proliferate" event, and fires the "proliferate" trigger so
// per_card observers (Venser, Corpse Puppet etc.) react once per
// proliferate event regardless of choice count.

// ProliferateTarget is the engine-level choice shape — exactly one of
// Permanent or Player must be non-nil. CounterType names the counter
// kind the controller chose (must already be present on the target per
// CR §701.23).
type ProliferateTarget struct {
	Permanent   *Permanent
	Player      *Seat
	CounterType string
}

// Proliferate applies the targets list per CR §701.23. Returns the
// number of counters actually placed plus any error from the underlying
// counters primitive. Logs a single "proliferate" event with the total
// count and fires the "proliferate" trigger once at the end so observer
// cards see one notification per proliferate event (not one per choice).
//
// Empty targets is a clean (0, nil) no-op — matches the "choose any
// number" wording. Targets with both Permanent and Player set, or
// neither, are skipped silently (caller-side bug guard — keeps the
// orchestrator robust against per_card mistakes).
func Proliferate(gs *GameState, controller int, targets []ProliferateTarget) (int, error) {
	if gs == nil {
		return 0, nil
	}
	// Pre-validate + collect: drop ill-formed entries (both/neither
	// Permanent and Player set), seed view stacks on permanents and
	// players so the CounterStacks adapter can see legacy storage,
	// build the canonical choice list.
	var (
		choices         []counters.ProliferateChoice
		permTargets     []*Permanent
		permKinds       []string
		permsSeen       = map[*Permanent]bool{}
		playerSeats     []*Seat
		playerSeatsSeen = map[*Seat]bool{}
	)
	for _, t := range targets {
		if (t.Permanent == nil) == (t.Player == nil) {
			continue
		}
		if t.Permanent != nil {
			canon := counters.CanonicalName(t.CounterType)
			// Legacy-map gate: skip choices whose kind isn't actually
			// present on the legacy map.
			if t.Permanent.Counters == nil || t.Permanent.Counters[canon] <= 0 {
				continue
			}
			if !permsSeen[t.Permanent] {
				seedPermCounterView(t.Permanent)
				permsSeen[t.Permanent] = true
			}
			choices = append(choices, counters.ProliferateChoice{
				Target:      t.Permanent.AsCounterTarget(),
				CounterType: t.CounterType,
			})
			permTargets = append(permTargets, t.Permanent)
			permKinds = append(permKinds, canon)
			continue
		}
		// Player target.
		if !playerSeatsSeen[t.Player] {
			seedPlayerCounterView(t.Player)
			playerSeats = append(playerSeats, t.Player)
			playerSeatsSeen[t.Player] = true
		}
		choices = append(choices, counters.ProliferateChoice{
			Target:      t.Player.AsCounterTarget(),
			CounterType: t.CounterType,
		})
	}

	sourceIID := "p" + intToStr(controller) + "-proliferate"
	tick := 0
	if gs.EventLog != nil {
		tick = len(gs.EventLog)
	}
	applied, err := counters.Proliferate(choices, sourceIID, tick)

	// Mirror per-permanent placements back to the legacy Counters map
	// so per_card observers / invariant scans / AST-detect paths that
	// read `p.Counters[kind]` see the post-proliferate value.
	for i, perm := range permTargets {
		if i >= applied || perm == nil {
			continue
		}
		// Doubling pipeline result (Phase 4 identity; Phase 6 real walk).
		placed := counters.ApplyDoublingPipeline(perm.AsCounterTarget(), permKinds[i], 1)
		perm.AddCounter(permKinds[i], placed)
	}
	// Reconcile view stacks: drop the transient baseline entries while
	// preserving the proliferate-driven deltas on CounterStacks.
	for perm := range permsSeen {
		flushPermCounterView(perm)
	}
	for _, s := range playerSeats {
		flushPlayerCounterView(s)
	}

	if applied > 0 {
		gs.InvalidateCharacteristicsCache()
	}
	gs.LogEvent(Event{
		Kind:   "proliferate",
		Seat:   controller,
		Amount: applied,
		Details: map[string]interface{}{
			"rule":      "701.23",
			"choices":   len(choices),
			"source_id": sourceIID,
		},
	})
	FireCardTrigger(gs, "proliferate", map[string]interface{}{
		"seat":   controller,
		"amount": applied,
	})
	return applied, err
}

// seedPermCounterView projects the permanent's legacy Counters map into
// CounterStacks as transient view stacks so counters.Proliferate's
// CounterCount gate sees the kinds already present. Reset by
// flushPermCounterView after the primitive returns.
func seedPermCounterView(p *Permanent) {
	if p == nil || len(p.Counters) == 0 {
		return
	}
	// Drop any stale view stacks from a prior call window before
	// seeding fresh. Defends against re-entrant proliferates within
	// the same priority pass.
	keep := p.CounterStacks[:0]
	for _, st := range p.CounterStacks {
		if st.PlacedByInstanceID == playerCounterViewSrc {
			continue
		}
		keep = append(keep, st)
	}
	p.CounterStacks = keep
	for kind, n := range p.Counters {
		if n <= 0 {
			continue
		}
		p.CounterStacks = append(p.CounterStacks, counters.CounterStack{
			Type:               counters.CanonicalName(kind),
			Count:              n,
			PlacedByInstanceID: playerCounterViewSrc,
		})
	}
}

// flushPermCounterView strips the transient view stacks from a
// permanent's CounterStacks slice. The proliferate-driven delta stacks
// (non-view source-id) stay so InstanceID lineage is preserved on the
// new placement; the baseline view stacks are no longer needed because
// the legacy Counters map already holds the pre-proliferate counts and
// has been bumped by the wrapper's permTargets mirror loop above.
func flushPermCounterView(p *Permanent) {
	if p == nil || len(p.CounterStacks) == 0 {
		return
	}
	keep := p.CounterStacks[:0]
	for _, st := range p.CounterStacks {
		if st.PlacedByInstanceID == playerCounterViewSrc {
			continue
		}
		keep = append(keep, st)
	}
	p.CounterStacks = keep
}

// BuildGreedyProliferateTargets is the GreedyHat policy helper — assembles
// the legacy proliferate-all-eligible target list that resolve_helpers.go's
// inline case "proliferate" path expects. Per choice generated:
//   - For each permanent on the battlefield with counters: one choice
//     per counter kind. Opponent-controlled permanents skip "+1/+1"
//     (don't help opponents).
//   - For the controller: experience counters proliferated.
//   - For opponents: poison + rad counters proliferated.
//
// Energy is intentionally excluded (CR §106.11 resource pool, not a
// §122 counter — IsProliferateEligibleType("energy")=false).
func BuildGreedyProliferateTargets(gs *GameState, controller int) []ProliferateTarget {
	if gs == nil {
		return nil
	}
	var out []ProliferateTarget
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || len(p.Counters) == 0 {
				continue
			}
			isOurs := p.Controller == controller
			for kind, count := range p.Counters {
				if count <= 0 {
					continue
				}
				if !isOurs && kind == "+1/+1" {
					continue
				}
				out = append(out, ProliferateTarget{Permanent: p, CounterType: kind})
			}
		}
	}
	for i, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		if i == controller {
			if s.Flags != nil && s.Flags["experience_counters"] > 0 {
				out = append(out, ProliferateTarget{Player: s, CounterType: "experience"})
			}
		} else {
			if s.PoisonCounters > 0 {
				out = append(out, ProliferateTarget{Player: s, CounterType: "poison"})
			}
			if s.Flags != nil && s.Flags["rad_counters"] > 0 {
				out = append(out, ProliferateTarget{Player: s, CounterType: "rad"})
			}
		}
	}
	return out
}

// seedPlayerCounterView projects the seat's legacy counter fields into
// Seat.Counters as transient view stacks so the counters.Target adapter
// sees a unified picture. Called before Proliferate's CounterCount gate
// runs. Reset by flushPlayerCounterView once the primitive returns.
//
// Idempotent: removes any prior "view" stacks before re-seeding so
// repeated proliferates within the same priority window stay coherent.
func seedPlayerCounterView(s *Seat) {
	if s == nil {
		return
	}
	out := s.Counters[:0]
	for _, st := range s.Counters {
		if st.PlacedByInstanceID == playerCounterViewSrc {
			continue
		}
		out = append(out, st)
	}
	s.Counters = out
	if s.PoisonCounters > 0 {
		s.Counters = append(s.Counters, counters.CounterStack{
			Type:               "poison",
			Count:              s.PoisonCounters,
			PlacedByInstanceID: playerCounterViewSrc,
		})
	}
	if s.Flags != nil {
		if c := s.Flags["experience_counters"]; c > 0 {
			s.Counters = append(s.Counters, counters.CounterStack{
				Type:               "experience",
				Count:              c,
				PlacedByInstanceID: playerCounterViewSrc,
			})
		}
		if c := s.Flags["rad_counters"]; c > 0 {
			s.Counters = append(s.Counters, counters.CounterStack{
				Type:               "rad",
				Count:              c,
				PlacedByInstanceID: playerCounterViewSrc,
			})
		}
	}
}

// flushPlayerCounterView walks Seat.Counters and reconciles legacy
// player-counter kinds (poison / experience / rad) back to their
// canonical storage. View-source stacks are dropped (they represented
// the pre-call baseline already held in the legacy field); any
// non-view stack of the same kind is the proliferate-driven delta and
// gets added to the legacy field, then dropped. Other counter kinds
// (Phase 4 future player counters) stay in Seat.Counters as the
// canonical store.
func flushPlayerCounterView(s *Seat) {
	if s == nil {
		return
	}
	keep := s.Counters[:0]
	deltaPoison, deltaExp, deltaRad := 0, 0, 0
	for _, st := range s.Counters {
		if st.PlacedByInstanceID == playerCounterViewSrc {
			continue // baseline view — discard
		}
		switch st.Type {
		case "poison":
			deltaPoison += st.Count
		case "experience":
			deltaExp += st.Count
		case "rad":
			deltaRad += st.Count
		default:
			keep = append(keep, st)
		}
	}
	s.Counters = keep
	s.PoisonCounters += deltaPoison
	if deltaExp > 0 || deltaRad > 0 {
		if s.Flags == nil {
			s.Flags = map[string]int{}
		}
		s.Flags["experience_counters"] += deltaExp
		s.Flags["rad_counters"] += deltaRad
	}
}

// playerCounterViewSrc marks transient view stacks installed by
// seedPlayerCounterView. Distinct from any real PlacedByInstanceID so
// flushPlayerCounterView can tell view stacks from proliferate-driven
// stacks reliably.
const playerCounterViewSrc = "__player_counter_view__"

// seatCounterTarget wraps a *Seat so it satisfies counters.Target.
// HasCardType("player") is the convention that target-validation paths
// use to recognize player targets without poking at TargetPlayer
// directly. Phase 4 keeps the gameengine wrapper as the canonical
// player-counter entry point; the primitive doesn't need to know.
type seatCounterTarget struct{ s *Seat }

func (t seatCounterTarget) HasCardType(typ string) bool {
	return typ == "player"
}

func (t seatCounterTarget) HasSubtype(string) bool { return false }

func (t seatCounterTarget) CounterStacks() []counters.CounterStack {
	if t.s == nil {
		return nil
	}
	return t.s.Counters
}

func (t seatCounterTarget) SetCounterStacks(stacks []counters.CounterStack) {
	if t.s == nil {
		return
	}
	t.s.Counters = stacks
}

// AsCounterTarget returns a counters.Target view of this seat. Used by
// the Proliferate orchestrator and future Phase 4+ player-counter
// placement paths.
func (s *Seat) AsCounterTarget() counters.Target {
	return seatCounterTarget{s: s}
}

// intToStr is a tiny local stringifier to avoid pulling fmt/strconv
// into the file just for one source-id label.
func intToStr(i int) string {
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	var buf [20]byte
	n := len(buf)
	for i > 0 {
		n--
		buf[n] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		n--
		buf[n] = '-'
	}
	return string(buf[n:])
}

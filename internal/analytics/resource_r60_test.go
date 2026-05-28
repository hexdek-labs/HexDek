package analytics

import (
	"reflect"
	"sort"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// resource_r60_test pins EventResources's switch arms — the function
// is the load-bearing mapping that DetectCoTriggers uses to spot
// causal A-produces-X / B-consumes-X links between cards. Every event
// kind that downstream analytics inspects needs deterministic
// produces/consumes output here.

// TestEventResources_KindToResource pins the 1:1 event-kind → resource
// mapping. Each tracked Kind produces or consumes a specific resource
// string; the test covers every branch in the switch including the
// life_change conditional split.
func TestEventResources_KindToResource(t *testing.T) {
	cases := []struct {
		kind         string
		amount       int
		wantProduces []string
		wantConsumes []string
	}{
		{"create_token", 0, []string{ResToken}, nil},
		{"draw_card", 0, []string{ResCard}, nil},
		{"damage", 0, []string{ResDamage}, nil},
		{"pay_mana", 0, nil, []string{ResMana}},
		{"pool_drain", 0, []string{ResMana}, nil},
		{"enter_battlefield", 0, []string{ResPermanent}, nil},
		{"leave_battlefield", 0, nil, []string{ResPermanent}},
		{"play_land", 0, []string{ResLand}, nil},
		{"cast", 0, nil, []string{ResMana}},
		{"triggered_ability", 0, []string{ResTrigger}, nil},
		{"sacrifice", 0, nil, []string{ResPermanent}},
		{"destroy", 0, nil, []string{ResPermanent}},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			ev := &gameengine.Event{Kind: tc.kind, Amount: tc.amount}
			gotP, gotC := EventResources(ev)
			if !reflect.DeepEqual(gotP, tc.wantProduces) {
				t.Errorf("%s produces = %v, want %v", tc.kind, gotP, tc.wantProduces)
			}
			if !reflect.DeepEqual(gotC, tc.wantConsumes) {
				t.Errorf("%s consumes = %v, want %v", tc.kind, gotC, tc.wantConsumes)
			}
		})
	}
}

// TestEventResources_LifeChangeBranches pins the three life_change
// arms: positive Amount = produces life (gain), negative = consumes
// life (loss), zero = neither (no resource attributed). The
// conditional split is the only event in the switch that flips
// direction on the event payload — easy to regress when refactoring.
func TestEventResources_LifeChangeBranches(t *testing.T) {
	cases := []struct {
		amount       int
		wantProduces []string
		wantConsumes []string
	}{
		{+5, []string{ResLife}, nil},  // gain
		{-3, nil, []string{ResLife}},  // loss
		{0, nil, nil},                  // no-op
	}
	for _, tc := range cases {
		ev := &gameengine.Event{Kind: "life_change", Amount: tc.amount}
		gotP, gotC := EventResources(ev)
		if !reflect.DeepEqual(gotP, tc.wantProduces) {
			t.Errorf("life_change %+d produces = %v, want %v", tc.amount, gotP, tc.wantProduces)
		}
		if !reflect.DeepEqual(gotC, tc.wantConsumes) {
			t.Errorf("life_change %+d consumes = %v, want %v", tc.amount, gotC, tc.wantConsumes)
		}
	}
}

// TestEventResources_UnknownKindReturnsEmpty pins the default branch:
// an unhandled Kind produces and consumes nothing. Without this,
// DetectCoTriggers would crash trying to dereference a nil slice or
// build false co-trigger pairs from spurious resource attributions.
func TestEventResources_UnknownKindReturnsEmpty(t *testing.T) {
	ev := &gameengine.Event{Kind: "no_such_event_kind", Amount: 42}
	p, c := EventResources(ev)
	if p != nil || c != nil {
		t.Errorf("unknown kind should produce neither/consume nothing, got p=%v c=%v", p, c)
	}
}

// TestEventResources_ConstantSymbolsStable pins the resource-string
// constants since DetectCoTriggers does string comparison on these to
// find causal links. A drift in any constant would silently break
// pairing without producing a compile error.
func TestEventResources_ConstantSymbolsStable(t *testing.T) {
	want := []string{"mana", "token", "card", "life", "damage", "permanent", "land", "trigger"}
	got := []string{ResMana, ResToken, ResCard, ResLife, ResDamage, ResPermanent, ResLand, ResTrigger}
	// Order-insensitive comparison — the constants are an unordered set.
	sort.Strings(want)
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("resource constants drifted: got %v, want %v", got, want)
	}
}

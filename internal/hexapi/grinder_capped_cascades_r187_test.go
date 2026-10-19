package hexapi

import (
	"regexp"
	"strconv"
	"testing"
	"time"
)

// r187 — transport only, and the header of
// grinder_capped_game_r185_test.go says why that is all this package
// can hold: the fixture decks are synthetic 36-Forest piles with no
// triggered abilities, so trigger nesting never reaches 2 and NO
// cap-related mutation can go red here. Every driveable game reports
// cascades=0, so these tests pin that the field EXISTS, PARSES and
// sits in the agreed position; the value semantics are pinned at
// their source in internal/gameengine/cascade_count_r187_test.go,
// where r186's tests drive real cascades through the real
// fireTrigger. Both halves are covered; neither covers the other.
//
// This is recorded as a fixture limit rather than papered over with a
// hand-fed value at the destination, which is the mistake r167
// shipped and r185's M8 note re-states.

var r187CappedGame = regexp.MustCompile(
	`CAPPED GAME [0-9.]+s turns=\d+ seed=(-?\d+) decks=\[[^\]]*\] ` +
		`swallowed=(\d+) cascademax=(\d+) cascades=(\d+) shape=`)

// cascades= is present, parses, and sits between cascademax= and the
// r186 shape group.
func TestR187_CappedLineCarriesTheCascadeCount(t *testing.T) {
	out := r185Grind(t, maintenanceFixture(), time.Hour, 0)
	m := r187CappedGame.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("no CAPPED GAME line with cascades= between cascademax= "+
			"and shape=:\n%s", out)
	}
	n, err := strconv.Atoi(m[4])
	if err != nil {
		t.Fatalf("cascades=%q does not parse: %v", m[4], err)
	}
	total, err := strconv.Atoi(m[2])
	if err != nil {
		t.Fatalf("swallowed=%q does not parse: %v", m[2], err)
	}
	if n > total {
		t.Fatalf("cascades=%d exceeds swallowed=%d — every counted "+
			"cascade swallowed at least one dispatch, so the count can "+
			"never exceed the total: %s", n, total, m[0])
	}
}

// The r185 regex must keep matching: cascades= was appended after the
// run r185 pinned, not spliced into it.
func TestR187_DoesNotDisturbTheR185Run(t *testing.T) {
	out := r185Grind(t, maintenanceFixture(), time.Hour, 0)
	if r185CappedGame.FindStringSubmatch(out) == nil {
		t.Fatalf("r185's CAPPED GAME parse broke — the new field must "+
			"follow seed=/decks=/swallowed=/cascademax=, not reorder "+
			"them:\n%s", out)
	}
}

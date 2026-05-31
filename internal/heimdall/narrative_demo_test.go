package heimdall

import "testing"

// TestSummarizeGame_DemoOutput is a non-asserting eyeball test —
// emits the full happy-path narrative via t.Logf so reviewers see the
// real consumer-facing output when running `go test -v`. Not a
// regression pin (the other tests cover that); just a transparency
// surface so the PR reviewer can read the narrative end-to-end
// without instrumenting.
func TestSummarizeGame_DemoOutput(t *testing.T) {
	t.Logf("\n--- HAPPY PATH ---\n%s\n", SummarizeGame(mkBaselineSnap()))

	draw := mkBaselineSnap()
	draw.Seed.Winner = -1
	draw.Seed.KillMethod = "timeout"
	t.Logf("\n--- DRAW ---\n%s\n", SummarizeGame(draw))

	noMul := mkBaselineSnap()
	for i := range noMul.MulliganStats {
		noMul.MulliganStats[i].MulligansTaken = 0
		noMul.MulliganStats[i].OpeningHandSize = 7
	}
	t.Logf("\n--- NO MULLIGANS ---\n%s\n", SummarizeGame(noMul))
}

package judge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// r134 — a diagnostic must not tell its reader to do something the
// system cannot do.
//
// Every liveness/wall_clock record used to end "repro and
// sample-profile this seed". The seed fixes the shuffle, the mulligans
// and the opening seat, but each hat is built from Curse DNA drawn out
// of a mutable, evolving pool on a DIFFERENT RNG, so replaying a seed
// gives the same opening and a differently-played game. The instruction
// had been sitting in the violation records telling whoever read one to
// go and do an impossible thing.

func TestR134_WatchdogMessageDoesNotPromiseASeedRepro(t *testing.T) {
	v := WatchdogViolation(1789817169370804852, 42, 10*time.Minute)

	if strings.Contains(strings.ToLower(v.Message), "repro and sample-profile this seed") {
		t.Fatalf("the message must not instruct a repro the system cannot do: %s", v.Message)
	}
	for _, want := range []string{"SHUFFLE", "NOT the play", "deck_keys"} {
		if !strings.Contains(v.Message, want) {
			t.Errorf("the message must say what the seed IS good for and where to triage instead; missing %q in: %s", want, v.Message)
		}
	}
	if got := v.Context["seed_reproduces"]; got != seedReproducesShuffleOnly {
		t.Errorf("the caveat must be machine-readable too; seed_reproduces = %v, want %q", got, seedReproducesShuffleOnly)
	}
	// The identifying facts must survive the rewording — they are what
	// triage actually runs on.
	if v.Context["seed"] != int64(1789817169370804852) || v.Context["game_idx"] != 42 {
		t.Errorf("seed and game index must still be recorded: %+v", v.Context)
	}
}

// A DERIVED gate, not a listed one: nothing in this package may tell a
// reader to reproduce a run from its seed, however it is phrased. A
// listed check on one string would be re-broken by the next person who
// writes "replay this seed" in a different message.
func TestR134_NoDiagnosticInThisPackagePromisesASeedRepro(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	// Phrases that amount to "go and replay this seed". The caveat text
	// itself talks about what the seed does NOT do, so it is not caught.
	banned := []string{
		"repro and sample-profile this seed",
		"repro this seed",
		"replay this seed",
		"reproduce this seed",
		"rerun this seed",
		"re-run this seed",
	}
	var found []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		b, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		low := strings.ToLower(string(b))
		for _, phrase := range banned {
			if strings.Contains(low, phrase) {
				found = append(found, name+": "+phrase)
			}
		}
	}
	if len(found) > 0 {
		t.Fatalf("a seed does not reproduce a grinder game — hat DNA comes from a mutable pool on a different RNG.\n"+
			"These diagnostics instruct an impossible repro:\n  %s\n"+
			"Say what the seed IS good for (the shuffle) and point triage at deck_keys and the abort census.",
			strings.Join(found, "\n  "))
	}
}

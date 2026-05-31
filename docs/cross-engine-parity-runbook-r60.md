# Cross-Engine Parity — Operational Runbook (r60)

**Companion to `docs/cross-engine-parity-setup.md`** (which is the *design*
document for the parity system). This file is the *operator* document:
exact commands to run, machine prerequisites, expected output at each
step, and the diagnostic playbook for the common failure modes.

Use this when ops bandwidth becomes available to drive Verification Phase
4 to completion.

---

## 0. Why this exists

PR #954 shipped the parity scaffolding and 17 scenario JSONs. The current
worktree has no JVM, so this PR can't actually drive cross-engine runs.
This runbook is so the next operator (or future you) can pick up
exactly where the work stops, with no spelunking required.

**Current state** (verified on this branch):

```
$ scripts/parity-test.sh --tag combat-fundamentals
xmage adapter:     MISSING (skip-with-77 mode)
RESULT: SKIPPED (xmage adapter missing). Exit 77.
```

Exit 77 is the autotools "test was unable to run, don't count as failure"
convention. Once the steps below are completed, the same command exits 0
on parity, 1 on divergence.

---

## 1. Machine prerequisites

| Requirement | Minimum | Verified-on-this-machine? |
|---|---|---|
| Go | 1.22+ | yes (HexDek's existing build) |
| Python 3 | 3.10+ | yes |
| Java (JDK) | 17+ | **no — macOS java shim only** |
| Maven | 3.8+ | unknown |
| Git | 2.30+ | yes |
| Disk | ~5 GB free (xmage clone + build artifacts) | check before cloning |

### 1.1 JVM install

macOS — install OpenJDK 17 (the version xmage's pom.xml targets):

```
brew install openjdk@17
sudo ln -sfn /opt/homebrew/opt/openjdk@17/libexec/openjdk.jdk /Library/Java/JavaVirtualMachines/openjdk-17.jdk
export JAVA_HOME=/Library/Java/JavaVirtualMachines/openjdk-17.jdk/Contents/Home
export PATH="$JAVA_HOME/bin:$PATH"
java -version  # → openjdk 17.X.X
```

Linux:

```
sudo apt-get install -y openjdk-17-jdk maven
java -version
```

### 1.2 Sanity check before proceeding

```
$ scripts/parity-xmage-setup.sh --preflight
```

Expected output on a properly-configured machine:

```
[preflight] Go:     go version go1.22.X ...
[preflight] python3: Python 3.X.X
[preflight] java:    openjdk version "17.X.X" ...
[preflight] mvn:     Apache Maven 3.X.X
[preflight] disk:    XX GB free at $HOME
[preflight] OK — all prerequisites present.
```

If `java -version` errors with "Unable to locate a Java Runtime", you
hit the macOS shim — install OpenJDK per §1.1 first.

---

## 2. Build the xmage adapter

### 2.1 Clone xmage at a pinned commit

```
mkdir -p ~/src && cd ~/src
git clone https://github.com/magefree/mage.git
cd mage
git checkout xmage_1.4.55  # pin to a release tag, NOT main — see §6.1 for why
```

Tag `xmage_1.4.55` is the most recent release as of this runbook's
authoring; check `git tag --list 'xmage_*' --sort=-v:refname | head` for
newer. Update §2.1 here if you pin a newer tag.

### 2.2 Build xmage

```
cd ~/src/mage
mvn -DskipTests=true -pl Mage,Mage.Server,Mage.Sets package
```

Expected: ~10 minutes on first build (downloads ~500 MB of deps to
`~/.m2/repository/`). Watch for `BUILD SUCCESS`.

Resulting jars:
- `~/src/mage/Mage/target/mage-1.4.55.jar` (core engine)
- `~/src/mage/Mage.Sets/target/mage-sets-1.4.55.jar` (card definitions)

The `HEXDEK_XMAGE_JAR` env var must point to the FAT jar (with deps
bundled), so build it next:

```
mvn -DskipTests=true -pl Mage.Server package assembly:single
ls ~/src/mage/Mage.Server/target/mage-server-1.4.55-jar-with-dependencies.jar
```

Set the env var so `scripts/parity-test.sh` picks it up:

```
export HEXDEK_XMAGE_JAR="$HOME/src/mage/Mage.Server/target/mage-server-1.4.55-jar-with-dependencies.jar"
```

### 2.3 Write the adapter shim

The adapter is a tiny Java class HexDek calls via JNI-free
subprocess. Contract:

```
java -jar $HEXDEK_XMAGE_JAR \
  --adapter hexdek.parity.Adapter \
  --decks deck0.txt,deck1.txt,deck2.txt,deck3.txt \
  --seed 101 \
  --max-turns 30 \
  --output /tmp/xmage-events.jsonl
```

The adapter subscribes to `mage.game.events.GameEvent` and emits one
canonical `Event` per line, plus a final `{"_outcome": {...}}` line.
Mirror the Python harness contract documented in
`internal/paritycheck/paritycheck.go::RunPython`.

**Where to put the adapter source**: a separate repo
(`github.com/hexdek-labs/hexdek-xmage-adapter`) NOT in the main HexDek
tree — xmage is GPL and HexDek is MIT. The adapter source needs to live
under GPL to satisfy linking; keeping it out-of-tree keeps the MIT
boundary clean. The runbook step is: clone the adapter repo, build the
shaded jar, point `HEXDEK_XMAGE_JAR` at it.

Skeleton class (~150 LOC) is sketched in
`docs/cross-engine-parity-setup.md` §2.2 — copy from there as the
starting point.

### 2.4 Smoke test the adapter

```
java -jar $HEXDEK_XMAGE_JAR \
  --adapter hexdek.parity.Adapter \
  --decks data/decks/test/voja_wolf_elf_tribal_b4_jaws_of_the_conclave.txt \
  --seed 101 --max-turns 30 \
  --output /tmp/smoke.jsonl

wc -l /tmp/smoke.jsonl   # → expect 500-5000 lines depending on game length
tail -1 /tmp/smoke.jsonl # → expect {"_outcome": {...}}
```

If the jsonl is empty or the last line isn't `_outcome`, the adapter is
broken — check §6 for diagnostics.

---

## 3. Drive the parity gauntlet

### 3.1 Single scenario

```
scripts/parity-test.sh --tag combat-fundamentals
```

Expected output on a working setup:

```
── HexDek Cross-Engine Parity Scenario Run ──
  scenarios:         3
  tag filter:        combat-fundamentals
  xmage adapter:     CONFIGURED ($HEXDEK_XMAGE_JAR)
  skipped-as-fail:   no

Running scenario: combat-fundamentals-alpha-strike (seed 101)
  go:    150 events, winner=2, turns=19, end=life_zero
  xmage: 150 events, winner=2, turns=19, end=life_zero
  → PARITY

Running scenario: combat-fundamentals-commander-damage (seed 201)
  ...
  → PARITY

Running scenario: combat-fundamentals-trample-lethal (seed 301)
  ...
  → PARITY

RESULT: 3/3 scenarios match across engines. Exit 0.
```

### 3.2 Full corpus (all 17 scenarios)

```
scripts/parity-test.sh
```

Walltime expectation: ~30 minutes on first run (xmage cold-start is ~10s
per game; HexDek is ~1s). Subsequent runs are JVM-warm and faster.

### 3.3 Divergence handling

A divergence prints the first 10 differing events side-by-side:

```
Running scenario: replacement-effects-doubling-season (seed 501)
  go:    312 events, winner=1, turns=22, end=life_zero
  xmage: 312 events, winner=1, turns=22, end=life_zero
  → DIVERGENCE: 4 events differ
    [event 87] go=token_minted{count:4} xmage=token_minted{count:2}
    [event 88] go=triggered_ability{src:Anointed Procession} xmage=<absent>
    ...

RESULT: 0 parity / 1 divergence / 16 not-yet-run. Exit 1.
Report at: data/parity-reports/2026-05-31T16-30-00.md
```

The full ParityReport JSON is at `data/parity-reports/`. Use
`cmd/hexdek-parity --diff <go.jsonl> <xmage.jsonl>` to drill into a
single scenario's full event diff.

---

## 4. Expected divergence classes

These are NOT bugs in either engine but EXPECTED differences. The Diff
filter in `internal/paritycheck/paritycheck.go::Diff` should canonicalize
these away; if it doesn't, the canonicalization layer needs extending
(this is the most common source of false-positive divergences during
adapter bring-up).

| Class | Example | Resolution |
|---|---|---|
| **RNG non-identity** | Hand order after shuffle | Diff strips library/hand orderings; only the multiset of cards matters |
| **Internal timestamp** | Layer recalc timestamps | Diff strips `_ts` fields |
| **Object-ID stamps** | xmage uses UUIDs, HexDek uses InstanceIDs | Diff strips both; matches by (card_name, controller, turn) tuple instead |
| **Trigger-ordering** | CR §603.3b stack push order | Diff sorts triggers within a single batch before compare |
| **Event-grouping** | xmage batches "deal 2 damage to each opponent" as 1 event, HexDek as 3 | Diff explodes into per-target events on both sides |
| **Phase-step naming** | "precombat_main" vs "main1" | Diff applies the alias map in `paritycheck.go::aliasPhase` |

When a NEW divergence class shows up (likely during the first run), add
its alias/strip rule to the Diff layer rather than declaring an engine
bug — bugs are the small residual after the canonicalization is right.

---

## 5. Expected real bugs to find

The point of cross-engine parity is to catch ENGINE bugs that one engine
hides via implementation accident. Anticipated finds, ordered by likelihood:

1. **Commander damage tracking** — different engines disagree about
   whether unblocked-then-prevented commander damage counts toward the
   21-point threshold. Per CR §903.10a, damage that's PREVENTED is not
   DEALT, so it shouldn't count. The `combat-fundamentals-commander-damage`
   scenario is set up to surface this if either engine gets it wrong.
2. **Replacement-effect ordering** — CR §616.1 says affected player
   chooses the order. xmage and HexDek may have different default
   orderings when no player input is available (Loki / Hat decisions).
3. **State-based action timing** — CR §704.3 the SBA pass repeats until
   nothing changes. Order of checks within one pass is engine-defined;
   some scenarios in `mass-effects` will stress this.
4. **Replacement vs. trigger** — CR §616.1 vs §603.6c on shapes like
   "deals damage instead". xmage and HexDek take different fast paths.

Document each find as a `data/parity-reports/divergence-NNN.md` postmortem
with: scenario, divergence summary, CR ruling, which engine matches the
ruling, and the bug-or-feature verdict.

---

## 6. Diagnostic playbook

### 6.1 xmage adapter returns 0 events

```
wc -l /tmp/smoke.jsonl   # → 0
```

Cause: adapter not subscribing to `GameEvent` before game start, OR
game ended at turn 0 (deck load failure). Check `~/src/mage/logs/server.log`
for `MalformedCard` exceptions — usually a card in the deck list isn't
in the xmage corpus.

If pinning to an older xmage tag, some new cards (post-r60-corpus) won't
exist. Either pin a NEWER tag (per §2.1), or strip the missing cards
from the scenario decks. Don't run with mismatched corpora.

### 6.2 `BUILD FAILURE` during xmage build

```
mvn -DskipTests=true ... package
[ERROR] Failed to execute goal ... compilation failure
```

Most common: JDK version mismatch. xmage 1.4.55 needs JDK 17; check
`mvn --version` shows "Java version: 17.X". If it shows 11 or 21, set
`JAVA_HOME` correctly per §1.1 and re-run.

Second most common: stale `~/.m2/repository/`. `rm -rf ~/.m2/repository/`
and re-run the mvn package step.

### 6.3 Divergence in event 0

```
[event 0] go=phase_start{phase:upkeep} xmage=phase_start{phase:untap}
```

Phase-step ordering disagreement at game start. CR §500.1: untap → upkeep
→ draw. HexDek skips untap on turn 1 per CR §103.7a; xmage doesn't (or
vice versa). Whoever matches §103.7a wins.

If both are right by CR but the engines disagree about the FIRST player's
turn-1 draw, that's the §103.7a multiplayer carve-out — different
engines apply it to different seat counts. Document and proceed.

### 6.4 Parity holds locally, breaks in CI

CI is JVM-warm differently than local. Cause is usually `JAVA_HOME` not
set in the CI runner's env. Either set it explicitly in the job, or
have `scripts/parity-test.sh` probe for it and fail-fast with a clear
error if absent.

---

## 7. Wiring to CI

After the adapter is verified locally on the 17-scenario corpus, wire
to CI per PR #954 doc §6 item 5:

1. Add a GitHub Actions workflow at `.github/workflows/parity.yml` (the
   repo has no `.github/` directory yet — create it).
2. Pin the runner to `ubuntu-22.04` (consistent JDK availability).
3. Cache `~/.m2/repository/` to keep xmage build under 2 min after first
   run.
4. Run on PRs touching `internal/gameengine/`, `internal/paritycheck/`,
   or `data/parity-scenarios/`. Don't run on every PR — too slow for the
   hot path.

Skeleton:

```yaml
name: cross-engine-parity
on:
  pull_request:
    paths:
      - 'internal/gameengine/**'
      - 'internal/paritycheck/**'
      - 'data/parity-scenarios/**'
jobs:
  parity:
    runs-on: ubuntu-22.04
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.22' }
      - uses: actions/setup-java@v4
        with: { distribution: 'temurin', java-version: '17' }
      - uses: actions/cache@v4
        with:
          path: ~/.m2/repository
          key: m2-${{ hashFiles('docs/cross-engine-parity-runbook-r60.md') }}
      - run: scripts/parity-xmage-setup.sh --preflight
      - run: scripts/parity-xmage-setup.sh --build-and-cache
      - run: scripts/parity-test.sh
```

---

## 8. Handoff checklist

When delivering the working parity system, the next operator should be
able to run:

```
$ scripts/parity-xmage-setup.sh --preflight
[preflight] OK

$ scripts/parity-xmage-setup.sh --build-and-cache
[build] xmage 1.4.55 built and cached at ~/.cache/hexdek/xmage-1.4.55.jar
[build] HEXDEK_XMAGE_JAR set in ~/.zshrc

$ scripts/parity-test.sh --tag combat-fundamentals
RESULT: 3/3 scenarios match across engines. Exit 0.

$ scripts/parity-test.sh
RESULT: 17/17 scenarios match across engines. Exit 0.
```

…and have a markdown report at `data/parity-reports/<timestamp>.md` for
any divergence.

If any of the above commands errors, the runbook is incomplete —
update it and re-PR before declaring done.

# r145 — the 12:37 outage: the deck finally named, and the token reading killed

**Status: narrowed and filed. No code.** The repro was NOT run — see "Why
I did not run it here".

## What happened

```
12:36:27  heap=997MB    normal
12:37:02  heap=12275MB  sys=16018MB  turns/game=31.7
12:37:19  aborting game at turn 10 — process heap 18452MB exceeds the 6442MB ceiling
12:49:13  (my restart)
```

**~1GB to 12GB in 35 seconds, then 12 to 18.4 in 17 more — about 300MB/s.**
An allocation rate, not a leak. 18.4GB on a 15.8GB box. Prod stayed down
12 minutes because `run-server.sh` is a `while true` *inside the process
group that dies*; nothing outside the process was watching.

## The deck, and what is actually in it

The largest board in the event was `s2=[life=38 bf=121(tok=118)] perms=133`
on `moxfield/sorin_of_house_markov_..._khazad_doom_JfUAlsri` — the **third**
time the census has named this deck (r130: 797 of 963 slow-game lines;
r138: 277 tokens).

Reading the actual decklist for the first time, it holds **all three parts
of a known unbounded life-drain trigger loop**:

| card | text |
|---|---|
| **Bloodthirsty Conqueror** | "Whenever an opponent loses life, you gain that much life." |
| **Marauding Blight-Priest** | "Whenever you gain life, each opponent loses 1 life." |
| **Vito, Thorn of the Dusk Rose** | the same closer, also present |

Conqueror + either closer loops: opponent loses life → you gain that much
→ each opponent loses 1 → you gain that much → …

**We had already done the CR research on exactly this pair and never
connected it to this deck.** The standing note reads: *"Bloodthirsty
Conqueror + Vito — the loop TERMINATES and is NOT a draw (§104.4b needs
'no way to stop'; life strictly DECREASES so §704.5a removes the player).
An engine ending this as a §104.4b draw is WRONG."* That research was
abstract. It has been sitting in the brief while the same deck was named
three times by the census.

## Killing my own reading of the census

**118 token objects cannot be 18GB.** 133 permanents is nothing. I have
been treating the token count as if it located the memory, and it does
not — it only indicates the deck goes off.

This agrees with evidence we already had and I under-weighted:

- **r136**: the census *eliminated* the game object graph as the resident
  location — 3GB attributed to a game holding fourteen permanents.
- **This event**: the 18,452MB abort was a **newborn with perms=7 and zero
  tokens**. The memory was not in the game being aborted.

So the tokens are a marker for "this deck does something explosive", not
for where the bytes are. **Anointed Procession + Kambal remains a
plausible story for the TOKEN COUNT and is not a story for the MEMORY.**

## And the trigger cascade does not obviously explain it either

The obvious next guess is that the loop's trigger cascade is the
allocator. The guards argue against it being that simple:

- `drainPendingTriggers` counts **total resolutions across both loops**
  (`resolves`, capped at `maxTriggerDrainIterations` = 500) and drops the
  stack and pending queue on trip. That is a real bound on resolutions,
  not a per-pass bound — the comment explains it was widened for exactly
  this class after the r63 game-216 stall.
- `pushTriggeredAbility` counts every fire against
  `maxTriggerFiresPerTurn` = 1000 and calls `EndGameAsLoopDraw` on trip.

1,000 fires reaching 18GB would need ~18MB allocated per fire, which is
not credible. **So the bytes are somewhere these counters do not look.**
That is the finding: not "the loop allocates", but "the loop is in the
deck AND the counted paths cannot account for the volume".

**Unexamined candidate, explicitly not claimed:** the AI's search. The hat
runs rollouts at budget 200+, and a rollout over a 121-permanent board
clones state. Transient clone churn is invisible to both counters above
and to the per-game heap attribution (which measures the game's own
graph, not copies made during search). **This has not been measured and
must not be repeated as if it had been.**

## Why I did not run the repro here

The next step is to drive those four decks directly. **I did not do it on
.206.** The box has 15.8GB, the live server holds ~1.3GB, and the
workload is known to reach 18GB. `systemd-run --user` has no bus on that
host, so there is no cgroup cap available, and `ulimit -v` is unreliable
against Go's address-space reservation. Deliberately running a known-OOM
workload on the production box twelve minutes after it last went down is
not covered by standing deploy authority.

**What is needed:** a non-production host, or a memory-capped cgroup on
.206, or root to create one. Either unblocks the repro immediately.

## The three standing asks, now with a cost attached

1. **A supervisor outside the process group.** `run-server.sh` cannot
   survive its own process group dying. Twelve minutes of downtime today.
   A systemd unit fixes it.
2. **Kernel log access on .206.** No dmesg, no journal, so an OOM kill
   cannot be confirmed — only inferred from arithmetic.
3. **Liveness monitoring.** Nothing was watching. The outage was found
   because an unrelated stale job happened to die and prompt a look.

# HexDek docs — start here

This folder holds ~260 working documents — audit reports, gauntlet
baselines, design notes, forensic write-ups. Most are point-in-time
artifacts named by round (`-r60`, `-r63`). The curated reference set:

## Reference

- **[CHANGELOG-r60-r63.md](CHANGELOG-r60-r63.md)** — the consolidation
  era, PR by PR, translated for humans: the Hex Judge build-out, the
  engine-correctness campaign, the cleanup rounds, frontend, tooling.
- **[LEGACY-EOL.md](LEGACY-EOL.md)** — every retired tool and feature:
  what it was, when and why it went, what replaced it. Check here
  before resurrecting or re-deleting anything.
- **[FEATURES.md](FEATURES.md)** — a readable, feature-by-feature tour
  of what HexDek actually does today (engine, Judge, AI, Freya, tooling,
  platform), each grounded in the package that implements it.
- **[THESIS-SNAPSHOT.md](THESIS-SNAPSHOT.md)** — what HexDek is and
  where the project stands: the perfect-accuracy thesis, the Judge, the
  learning loops, current scores.
- **[deck-lifecycle.md](deck-lifecycle.md)** — the flowchart of a
  deck's journey: import → Freya analysis → strategy profile → hat play
  → Heimdall analytics → Huginn/Muninn learning → back into analysis.
- **[BUG-LEDGER.md](BUG-LEDGER.md)** — the known-bug ledger: fixed
  (root cause + fix), worked-around (mitigated, root cause still open),
  was-a-bug-then-became-intended, and the current open set.

## Architecture

- **[ARCHITECTURE.md](ARCHITECTURE.md)** + the
  [architecture/](architecture/) folder — system layout.
- **[architecture-system-map.md](architecture-system-map.md)** — the
  tool fleet and how data flows between Thor / Freya / Loki / Heimdall
  / Huginn / Muninn and the engine.
- **[architecture-learning-loop.md](architecture-learning-loop.md)** —
  the play → observe → learn → feed-back loop (closed as of r63).
- **[API.md](API.md)** — the HTTP/WS surface.

## Everything else

Round-stamped reports (`*-r60.md`, `loki-r41-report.md`, gauntlet
baselines, forensics) are historical evidence: trust the conclusions
that made it into the changelog, the EOL log, or CLAUDE.md's Issue Log
over any single stale report.

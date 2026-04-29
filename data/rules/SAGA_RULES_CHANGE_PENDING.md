# Saga Rules Change — PENDING (Do Not Implement Yet)

**Flagged:** 2026-04-18
**Source:** 7174n1c — official MTG rules update incoming
**Status:** DEFERRED — wait for final rules text before touching

## What's Changing

The MTG Comprehensive Rules are updating how Sagas handle lore counter entry mechanics, specifically:
- How lore counters interact with Doubling Season and similar counter-doubling effects
- How counter-moving abilities/spells interact with Saga chapter triggers
- Possible changes to the ETB counter placement (currently §714.2a)

## Current Engine State

- 216 Sagas in corpus, all currently pass Thor (100% verified)
- Saga SBA §704.5s (final chapter sacrifice) is implemented
- Doubling Season + Saga ETB counter doubling is tested in Thor deep-rules
- Lore counter advancement triggers chapter abilities

## What NOT To Do

- Do NOT update Saga mechanics until the official rules text is published
- Do NOT implement speculative changes based on previews/leaks
- Do NOT refactor the Saga subframe — it works for current rules

## When To Revisit

- After official CR update is published
- Re-run Thor on all 216 Sagas
- Check Doubling Season interaction specifically
- Update deep-rules Pack 4 (ETBReplacement) Saga tests
- Update ENGINE_RULES_COVERAGE.md

## Cards to Retest

All 216 Sagas plus:
- Doubling Season
- Vorinclex, Monstrous Raider
- Proliferate effects
- Hex Parasite / Chisei, Heart of Oceans (counter removal)
- Power Conduit (counter conversion)
- Nesting Grounds (counter moving)

# Set Ingestion Queue

## Queued Sets

### Strixhaven: Secrets of Strixhaven (SOS) — April 2026
**Deadline:** 2026-04-28 (10 days from flag date)
**Flagged:** 2026-04-18 by 7174n1c
**Status:** QUEUED — do not start until current unverified cleanup is complete

- 305 cards main set
- 362 cards with collector booster treatments
- 65 Mystical Archive cards (reprints with special frame)
- Total new cards to parse: ~305 unique + ~65 archive

**Ingestion Steps:**
1. Download updated oracle-cards.json from Scryfall after release
2. Re-run parser on full corpus (new cards auto-included)
3. Run Thor on all new SOS cards specifically
4. Flag any new UnknownEffect / parsed_effect_residual
5. Check for new keywords or mechanics requiring engine support
6. Add per-card handlers for any new staples
7. Re-run Loki chaos gauntlet with new cards in pool
8. Update ENGINE_RULES_COVERAGE.md

**Prerequisites before starting:**
- Current 8,896 unverified count reduced to acceptable level
- Saga rules change NOT yet applied (separate track)
- Thor claim-verifier at 100/100

## Completed Sets
(none yet)

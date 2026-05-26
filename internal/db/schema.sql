-- hexdek ephemeral game state schema (SQLite).
--
-- Persistent identity layer: device, deck, friend.
-- Ephemeral game layer: party, party_member, game, game_player, game_card,
-- action_log.
--
-- Devices are long-lived (persistent identity). Parties are short-lived
-- (one per game session). Games are even shorter-lived (one per match).
-- All ephemeral data can be wiped on server restart without breaking the
-- persistent identity layer.

-- ===== PERSISTENT IDENTITY =====

CREATE TABLE IF NOT EXISTS device (
    id            TEXT PRIMARY KEY,        -- UUID v4
    display_name  TEXT NOT NULL,
    created_at    INTEGER NOT NULL,        -- unix epoch seconds
    last_seen_at  INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS session (
    token        TEXT PRIMARY KEY,         -- opaque random hex string
    device_id    TEXT NOT NULL REFERENCES device(id) ON DELETE CASCADE,
    created_at   INTEGER NOT NULL,
    expires_at   INTEGER NOT NULL,         -- unix epoch seconds; 0 = no expiry
    last_used_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_session_device ON session(device_id);
CREATE INDEX IF NOT EXISTS idx_session_expires ON session(expires_at);

-- API keys for programmatic access alongside sessions. Sessions are
-- short-lived (30-day default) browser bearer tokens minted at
-- device registration; API keys are user-issued, long-lived (or
-- explicit-expiry), individually revocable credentials for CI /
-- scripts / external integrations.
--
-- Storage shape:
--   id        — short hex handle (6 bytes / 12 hex chars). Public,
--               used as the revoke path component. NOT a secret.
--   key_hash  — SHA-256 hex of the plaintext key. Plaintext is
--               shown to the user once at issuance and never stored.
--               Hash is unique so a leaked storage layer can't
--               trivially reconstruct credentials.
--   name      — user-supplied display string for "which key is this".
--   expires_at / revoked_at — both 0 means active. Either non-zero
--               means inactive (expires_at = lifecycle cap; revoked_at
--               = user explicitly revoked from the management UI).
CREATE TABLE IF NOT EXISTS api_key (
    id            TEXT PRIMARY KEY,
    device_id     TEXT NOT NULL REFERENCES device(id) ON DELETE CASCADE,
    name          TEXT NOT NULL DEFAULT '',
    key_hash      TEXT NOT NULL UNIQUE,
    created_at    INTEGER NOT NULL,
    last_used_at  INTEGER NOT NULL DEFAULT 0,
    expires_at    INTEGER NOT NULL DEFAULT 0,   -- 0 = no expiry
    revoked_at    INTEGER NOT NULL DEFAULT 0    -- 0 = active
);
CREATE INDEX IF NOT EXISTS idx_api_key_device ON api_key(device_id);
CREATE INDEX IF NOT EXISTS idx_api_key_hash ON api_key(key_hash);

-- TOTP (RFC 6238) enrollment state per device. Two-row lifecycle:
--   1. EnrollTOTP inserts with status='pending' + a fresh secret.
--   2. ConfirmTOTP transitions to status='active' after the user
--      submits a sample code that verifies — until then the secret
--      MUST NOT gate any auth (the user might typo the QR scan and
--      need to retry).
-- DisableTOTP removes the row entirely; a fresh enrollment then
-- starts at pending again.
--
-- See docs/auth-2fa-survey.md for the full scope rationale + the
-- pieces deferred to follow-up PRs (HTTP endpoints, session
-- elevation, backup codes, rate-limit on verify).
CREATE TABLE IF NOT EXISTS device_totp (
    device_id     TEXT PRIMARY KEY REFERENCES device(id) ON DELETE CASCADE,
    secret        TEXT NOT NULL,                  -- base32-encoded, no padding
    status        TEXT NOT NULL,                  -- 'pending' | 'active'
    created_at    INTEGER NOT NULL,               -- enrollment start
    confirmed_at  INTEGER NOT NULL DEFAULT 0      -- 0 = unconfirmed
);
CREATE INDEX IF NOT EXISTS idx_device_totp_status ON device_totp(status);

CREATE TABLE IF NOT EXISTS deck (
    id                TEXT PRIMARY KEY,
    owner_device_id   TEXT NOT NULL REFERENCES device(id),
    name              TEXT NOT NULL,
    commander_name    TEXT,                 -- may be NULL if not commander format
    format            TEXT NOT NULL DEFAULT 'commander',
    moxfield_url      TEXT,
    imported_at       INTEGER NOT NULL,
    raw_json          TEXT NOT NULL         -- the full deck JSON for re-shuffling
);

CREATE INDEX IF NOT EXISTS idx_deck_owner ON deck(owner_device_id);

CREATE TABLE IF NOT EXISTS friend (
    device_id        TEXT NOT NULL REFERENCES device(id),
    friend_device_id TEXT NOT NULL REFERENCES device(id),
    created_at       INTEGER NOT NULL,
    PRIMARY KEY (device_id, friend_device_id)
);

-- ===== EPHEMERAL GAME STATE =====

CREATE TABLE IF NOT EXISTS party (
    id              TEXT PRIMARY KEY,        -- 6-char join code (e.g. "K3F2X9")
    host_device_id  TEXT NOT NULL REFERENCES device(id),
    state           TEXT NOT NULL DEFAULT 'lobby', -- 'lobby' | 'playing' | 'finished'
    max_players     INTEGER NOT NULL DEFAULT 4,
    created_at      INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS party_member (
    party_id      TEXT NOT NULL REFERENCES party(id) ON DELETE CASCADE,
    device_id     TEXT NOT NULL REFERENCES device(id),
    deck_id       TEXT REFERENCES deck(id),    -- nullable until selected
    seat_position INTEGER NOT NULL,
    is_ai         INTEGER NOT NULL DEFAULT 0,  -- bool: 0/1
    joined_at     INTEGER NOT NULL,
    PRIMARY KEY (party_id, device_id)
);

CREATE INDEX IF NOT EXISTS idx_party_member_party ON party_member(party_id);

CREATE TABLE IF NOT EXISTS game (
    id                TEXT PRIMARY KEY,
    party_id          TEXT NOT NULL REFERENCES party(id),
    started_at        INTEGER NOT NULL,
    finished_at       INTEGER,
    winner_device_id  TEXT REFERENCES device(id),
    shuffle_seed_hash TEXT NOT NULL,        -- commit phase
    shuffle_seed      TEXT                  -- reveal phase, after game ends
);

CREATE TABLE IF NOT EXISTS game_player (
    game_id            TEXT NOT NULL REFERENCES game(id) ON DELETE CASCADE,
    seat_position      INTEGER NOT NULL,
    device_id          TEXT NOT NULL REFERENCES device(id),
    deck_id            TEXT NOT NULL REFERENCES deck(id),
    life               INTEGER NOT NULL DEFAULT 40,    -- commander default
    poison_counters    INTEGER NOT NULL DEFAULT 0,
    mana_pool_w        INTEGER NOT NULL DEFAULT 0,
    mana_pool_u        INTEGER NOT NULL DEFAULT 0,
    mana_pool_b        INTEGER NOT NULL DEFAULT 0,
    mana_pool_r        INTEGER NOT NULL DEFAULT 0,
    mana_pool_g        INTEGER NOT NULL DEFAULT 0,
    mana_pool_c        INTEGER NOT NULL DEFAULT 0,
    lands_played_turn  INTEGER NOT NULL DEFAULT 0, -- reset to 0 at untap step
    attempted_empty_draw INTEGER NOT NULL DEFAULT 0, -- CR §119.5: set when player draws from empty library; CheckGameEnd eliminates flagged seats
    PRIMARY KEY (game_id, seat_position)
);

CREATE TABLE IF NOT EXISTS game_card (
    game_id        TEXT NOT NULL REFERENCES game(id) ON DELETE CASCADE,
    instance_id    TEXT NOT NULL,            -- UUID per card instance
    card_name      TEXT NOT NULL,
    card_data      TEXT NOT NULL,            -- JSON snapshot of card oracle data
    owner_seat     INTEGER NOT NULL,
    zone           TEXT NOT NULL,            -- library | hand | battlefield | graveyard | exile | command | stack
    zone_position  INTEGER NOT NULL,         -- 0 = top of zone, increasing
    tapped         INTEGER NOT NULL DEFAULT 0,
    tapped_for_mana_this_turn INTEGER NOT NULL DEFAULT 0, -- 1 if this card has produced mana already this turn
    revealed_to    TEXT NOT NULL DEFAULT '', -- comma-separated seat positions that have seen this card
    PRIMARY KEY (game_id, instance_id)
);

CREATE INDEX IF NOT EXISTS idx_game_card_zone ON game_card(game_id, owner_seat, zone, zone_position);

CREATE TABLE IF NOT EXISTS game_turn (
    game_id        TEXT PRIMARY KEY REFERENCES game(id) ON DELETE CASCADE,
    active_seat    INTEGER NOT NULL,
    phase          TEXT NOT NULL,            -- untap | upkeep | draw | main1 | combat | main2 | end | cleanup
    priority_seat  INTEGER NOT NULL,
    turn_number    INTEGER NOT NULL DEFAULT 1
);

-- Combat tracking: while in combat phase, we record pending attackers
-- (one row per attacking creature) and blockers (one row per blocker
-- with the attacker it blocks). Cleared at combat end.
CREATE TABLE IF NOT EXISTS combat_attacker (
    game_id       TEXT NOT NULL REFERENCES game(id) ON DELETE CASCADE,
    instance_id   TEXT NOT NULL,            -- attacking creature's instance id
    target_seat   INTEGER NOT NULL,         -- player being attacked
    declared_at   INTEGER NOT NULL,
    PRIMARY KEY (game_id, instance_id)
);

CREATE TABLE IF NOT EXISTS combat_blocker (
    game_id          TEXT NOT NULL REFERENCES game(id) ON DELETE CASCADE,
    blocker_id       TEXT NOT NULL,         -- blocking creature's instance id
    attacker_id      TEXT NOT NULL,         -- which attacker it blocks
    declared_at      INTEGER NOT NULL,
    PRIMARY KEY (game_id, blocker_id)
);

CREATE TABLE IF NOT EXISTS action_log (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    game_id       TEXT NOT NULL REFERENCES game(id) ON DELETE CASCADE,
    seat_position INTEGER,                   -- nullable for system actions
    timestamp     INTEGER NOT NULL,
    action_type   TEXT NOT NULL,             -- play_card | activate | attack | block | pass_priority | trigger | etc.
    payload       TEXT NOT NULL              -- JSON
);

CREATE INDEX IF NOT EXISTS idx_action_log_game ON action_log(game_id, id);

-- ===== CARD ORACLE CACHE =====
-- Cached Scryfall card data so we don't hammer their API on every lookup.

CREATE TABLE IF NOT EXISTS card_oracle (
    name             TEXT PRIMARY KEY,        -- canonical card name (lowercased for matching)
    display_name     TEXT NOT NULL,           -- proper-cased name as returned by Scryfall
    scryfall_id      TEXT NOT NULL,
    mana_cost        TEXT,
    cmc              INTEGER NOT NULL DEFAULT 0,
    type_line        TEXT,
    oracle_text      TEXT,
    image_uri_normal TEXT,                    -- Scryfall normal-size image URL
    image_uri_art    TEXT,                    -- art-crop image URL
    set_code         TEXT,
    cached_at        INTEGER NOT NULL,
    legalities       TEXT NOT NULL DEFAULT '' -- JSON {format: "legal|not_legal|banned|restricted"}
);

CREATE INDEX IF NOT EXISTS idx_card_oracle_name ON card_oracle(name);

-- ===== SHOWMATCH PERSISTENT STATE =====
-- ELO ratings and game history that survive server restarts.

CREATE TABLE IF NOT EXISTS showmatch_elo (
    deck_key     TEXT PRIMARY KEY,
    commander    TEXT NOT NULL DEFAULT '',
    owner        TEXT NOT NULL DEFAULT '',
    rating       REAL NOT NULL DEFAULT 1500.0,
    hex_rating   REAL NOT NULL DEFAULT 0.0,
    games        INTEGER NOT NULL DEFAULT 0,
    wins         INTEGER NOT NULL DEFAULT 0,
    losses       INTEGER NOT NULL DEFAULT 0,
    delta        REAL NOT NULL DEFAULT 0.0,
    hex_delta    REAL NOT NULL DEFAULT 0.0,
    bracket      INTEGER NOT NULL DEFAULT 0,
    updated_at   INTEGER NOT NULL
);

-- r60: per-deck Freya profile scalars co-keyed by deck_key. Lets
-- analytics queries (e.g. bracket-vs-ELO correlation) join rating
-- against archetype / synergy / power-tier distribution without
-- re-running Freya on the raw deck list. Populated by the
-- `runFreya` callsite in hexapi/handler.go after every Freya
-- analysis completes — INSERT OR REPLACE semantics, so each deck
-- carries the latest analysis (no version history). See
-- docs/bracket-elo-distribution-r60.md "Freya synergy
-- cross-reference" section for the motivation; this is the
-- snapshot-schema follow-up that makes the synergy correlation
-- runnable end-to-end against the snapshot DB.
--
-- Sibling table (rather than additional columns on showmatch_elo)
-- so the rating-update hot path stays narrow and Freya re-runs
-- don't have to touch the ELO row. The pair is joined on deck_key
-- for any analytics query that needs both signals.
CREATE TABLE IF NOT EXISTS deck_freya_profile (
    deck_key            TEXT PRIMARY KEY,
    commander           TEXT NOT NULL DEFAULT '',
    owner               TEXT NOT NULL DEFAULT '',
    primary_archetype   TEXT NOT NULL DEFAULT '',
    secondary_archetype TEXT NOT NULL DEFAULT '',
    bracket             INTEGER NOT NULL DEFAULT 0,
    -- 0.0-100.0 percentage form of DeckProfile.CommanderSynergy
    -- (which Freya stores as 0.0-1.0). Stored as percent so the
    -- column reads naturally in ad-hoc SQL ("synergy_pct > 50").
    synergy_pct         REAL NOT NULL DEFAULT 0.0,
    power_percentile    INTEGER NOT NULL DEFAULT 0,
    -- JSON map: {"S": int, "A": int, "B": int, "C": int, "D": int}.
    -- Mirrors DeckProfile.PowerTierCounts. Empty-object default so
    -- ad-hoc json_extract queries don't NULL out.
    power_tier_counts   TEXT NOT NULL DEFAULT '{}',
    -- JSON array of {"role": string, "count": int} ordered by
    -- count desc, capped at the top 3 roles (DeckProfile.TopRoles).
    primary_roles       TEXT NOT NULL DEFAULT '[]',
    updated_at          INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_deck_freya_profile_owner
    ON deck_freya_profile(owner);
CREATE INDEX IF NOT EXISTS idx_deck_freya_profile_archetype
    ON deck_freya_profile(primary_archetype);

CREATE TABLE IF NOT EXISTS showmatch_game (
    game_id      INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at   INTEGER NOT NULL,
    finished_at  INTEGER NOT NULL,
    turns        INTEGER NOT NULL,
    winner       INTEGER NOT NULL DEFAULT -1,
    winner_name  TEXT NOT NULL DEFAULT 'DRAW',
    end_reason   TEXT NOT NULL DEFAULT 'unknown',
    rng_seed     INTEGER NOT NULL DEFAULT 0  -- engine RNG seed; 0 = unknown
);

CREATE TABLE IF NOT EXISTS showmatch_game_seat (
    game_id           INTEGER NOT NULL REFERENCES showmatch_game(game_id) ON DELETE CASCADE,
    seat              INTEGER NOT NULL,
    commander         TEXT NOT NULL,
    life              INTEGER NOT NULL,
    hand_size         INTEGER NOT NULL DEFAULT 0,
    library_size      INTEGER NOT NULL DEFAULT 0,
    gy_size           INTEGER NOT NULL DEFAULT 0,
    bf_size           INTEGER NOT NULL DEFAULT 0,
    lost              INTEGER NOT NULL DEFAULT 0,
    -- deck_key + battlefield_cards are also added by applyMigrations for
    -- old DBs that pre-date PR #78 — declared here so fresh DBs see
    -- them at CREATE TABLE time and CREATE INDEX on deck_key (below)
    -- doesn't fail on first boot.
    deck_key          TEXT NOT NULL DEFAULT '',
    battlefield_cards TEXT NOT NULL DEFAULT '[]',
    PRIMARY KEY (game_id, seat)
);

CREATE INDEX IF NOT EXISTS idx_showmatch_game_finished ON showmatch_game(finished_at);
CREATE INDEX IF NOT EXISTS idx_showmatch_seat_commander ON showmatch_game_seat(commander);

-- r60: heimdall observation snapshot persistence. One row per game
-- holding the JSON-serialized GameObservationSnapshot
-- (CommanderZoneVisits / RegretCards / MVPCards / CardFirstPlayed +
-- per-seat commander names). Drives the "rich path" of the
-- /api/games/{id}/summary endpoint — without this row the endpoint
-- falls back to the "db_only" summary built from game metadata.
-- ON DELETE CASCADE so snapshots clean up when the parent game row
-- is purged.
CREATE TABLE IF NOT EXISTS showmatch_game_observation (
    game_id    INTEGER PRIMARY KEY REFERENCES showmatch_game(game_id) ON DELETE CASCADE,
    payload    TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT 0
);

-- r60: LoadOwnerGames is the per-owner game-history query that
-- powers heimdall / the dashboard "your recent games" panel. It
-- joins three tables and filters with `e.owner = ?`. Without these
-- two indexes the planner is forced into a "scan all seats, look up
-- each one's elo row by deck_key, discard rows where owner doesn't
-- match" plan — every per-owner query touches every seat row, so
-- latency scales linearly with TOTAL games in the DB rather than
-- with the requested owner's history.
--
-- idx_showmatch_elo_owner gives the planner a starting point: search
-- elo by owner first (small result set), then join the seats by
-- deck_key, then join the games by id. idx_showmatch_seat_deck_key
-- is the second half — without an index on game_seat.deck_key the
-- planner can't follow the new join order and falls back to the old
-- scan-seats plan. The pair is needed; the index on elo.owner alone
-- doesn't change the plan at this dataset size.
CREATE INDEX IF NOT EXISTS idx_showmatch_elo_owner ON showmatch_elo(owner);
CREATE INDEX IF NOT EXISTS idx_showmatch_seat_deck_key ON showmatch_game_seat(deck_key);

-- Per-gauntlet-run ELO snapshot. One row per completed gauntlet,
-- captures the rating trajectory for the deck under test (seat 0).
-- Drives the deck-page ELO history chart — without this table, the
-- only visible signal is the current rating, hiding the calibration
-- arc players see over multiple runs.
CREATE TABLE IF NOT EXISTS gauntlet_runs (
    run_id       INTEGER PRIMARY KEY AUTOINCREMENT,
    deck_key     TEXT NOT NULL,
    commander    TEXT NOT NULL DEFAULT '',
    started_at   INTEGER NOT NULL,
    finished_at  INTEGER NOT NULL,
    games        INTEGER NOT NULL,
    wins         INTEGER NOT NULL,
    losses       INTEGER NOT NULL,
    win_rate     REAL NOT NULL DEFAULT 0.0,
    elo_start    REAL NOT NULL DEFAULT 0.0,
    elo_end      REAL NOT NULL DEFAULT 0.0,
    elo_delta    REAL NOT NULL DEFAULT 0.0,
    avg_turns    REAL NOT NULL DEFAULT 0.0,
    -- Per-position counts (1st/2nd/3rd/4th) — populated when the
    -- gauntlet uses the placements tracking added in PR #78. Older
    -- runs leave these zeroed.
    place_1st    INTEGER NOT NULL DEFAULT 0,
    place_2nd    INTEGER NOT NULL DEFAULT 0,
    place_3rd    INTEGER NOT NULL DEFAULT 0,
    place_4th    INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_gauntlet_runs_deck     ON gauntlet_runs(deck_key, finished_at DESC);
CREATE INDEX IF NOT EXISTS idx_gauntlet_runs_finished ON gauntlet_runs(finished_at);

-- r60 gauntlet replay archive: explicit junction between a
-- gauntlet_runs row and the showmatch_game rows it produced.
-- Without this junction the only way to recover a gauntlet's
-- per-game data is the (deck_key, time-window) heuristic the
-- tournament-stats endpoint uses today — which mis-attributes
-- games when two gauntlets for the same deck overlap.
--
-- game_index is the 0-based ordinal of the game within the
-- gauntlet, so consumers can replay games in their original
-- sequence regardless of database insertion order (which can
-- reorder slightly under concurrent writes).
--
-- ON DELETE CASCADE both directions: dropping a gauntlet_runs
-- row drops its junction rows; dropping a showmatch_game row
-- drops the corresponding junction entry (the game is gone, the
-- link to it shouldn't dangle).
CREATE TABLE IF NOT EXISTS gauntlet_run_game (
    gauntlet_id INTEGER NOT NULL REFERENCES gauntlet_runs(run_id) ON DELETE CASCADE,
    game_id     INTEGER NOT NULL REFERENCES showmatch_game(game_id) ON DELETE CASCADE,
    game_index  INTEGER NOT NULL,
    PRIMARY KEY (gauntlet_id, game_id)
);
CREATE INDEX IF NOT EXISTS idx_gauntlet_run_game_gauntlet ON gauntlet_run_game(gauntlet_id, game_index);
CREATE INDEX IF NOT EXISTS idx_gauntlet_run_game_game ON gauntlet_run_game(game_id);

CREATE TABLE IF NOT EXISTS card_win_stats (
    card_name    TEXT NOT NULL,
    commander    TEXT NOT NULL,
    games        INTEGER NOT NULL DEFAULT 0,
    wins         INTEGER NOT NULL DEFAULT 0,
    on_board_at_win INTEGER NOT NULL DEFAULT 0,
    avg_turn_played REAL NOT NULL DEFAULT 0.0,
    updated_at   INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (card_name, commander)
);

CREATE INDEX IF NOT EXISTS idx_card_win_stats_commander ON card_win_stats(commander);
CREATE INDEX IF NOT EXISTS idx_card_win_stats_winrate ON card_win_stats(wins, games);

-- ===== DECK VERSIONING =====

CREATE TABLE IF NOT EXISTS deck_version (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    deck_owner   TEXT NOT NULL,
    deck_name    TEXT NOT NULL,
    commander    TEXT,
    version      INTEGER NOT NULL DEFAULT 1,
    card_list    TEXT NOT NULL,
    card_count   INTEGER NOT NULL DEFAULT 0,
    is_main      INTEGER NOT NULL DEFAULT 1,
    created_at   INTEGER NOT NULL,
    notes        TEXT
);

CREATE INDEX IF NOT EXISTS idx_deck_version_owner ON deck_version(deck_owner, deck_name);
CREATE INDEX IF NOT EXISTS idx_deck_version_main ON deck_version(deck_owner, deck_name, is_main);

-- ===== KEY-VALUE STORE =====
-- Simple key-value store for aggregate counters that survive restarts.

CREATE TABLE IF NOT EXISTS kv_store (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at INTEGER NOT NULL DEFAULT 0
);

-- ===== DECK IMPORT LOG =====
-- One row per deck import event (paste or Moxfield URL). Used to surface a
-- "Recent Imports" log on the dashboard and to audit deck provenance.

CREATE TABLE IF NOT EXISTS import_log (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    owner        TEXT NOT NULL,
    deck_key     TEXT NOT NULL,           -- "owner/id" of the saved deck file
    deck_name    TEXT NOT NULL DEFAULT '',
    commander    TEXT NOT NULL DEFAULT '',
    source       TEXT NOT NULL,           -- 'paste' | 'moxfield'
    source_url   TEXT NOT NULL DEFAULT '',
    card_count   INTEGER NOT NULL DEFAULT 0,
    imported_at  INTEGER NOT NULL         -- unix epoch seconds
);

CREATE INDEX IF NOT EXISTS idx_import_log_owner ON import_log(owner, imported_at DESC);

-- ===== TEMPORAL PINCER — ANONYMOUS PAGEVIEW + STITCH =====
-- pageviews is append-only and stores no PII. anon_id is a client-generated
-- UUID kept in localStorage; owner is filled in lazily when the visitor logs
-- in (see session_stitch + the backfill UPDATE in handleStitch).

CREATE TABLE IF NOT EXISTS pageviews (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    anon_id   TEXT NOT NULL,
    owner     TEXT,
    path      TEXT NOT NULL,
    ts        INTEGER NOT NULL,           -- unix epoch milliseconds (client-supplied)
    referrer  TEXT
);

CREATE INDEX IF NOT EXISTS idx_pageviews_anon  ON pageviews(anon_id);
CREATE INDEX IF NOT EXISTS idx_pageviews_owner ON pageviews(owner) WHERE owner IS NOT NULL;

-- session_stitch records the moment an anonymous session was linked to an
-- authenticated owner. Re-stitching the same pair is a no-op via INSERT OR REPLACE.

CREATE TABLE IF NOT EXISTS session_stitch (
    anon_id     TEXT NOT NULL,
    owner       TEXT NOT NULL,
    stitched_at INTEGER NOT NULL,         -- unix epoch milliseconds
    PRIMARY KEY (anon_id, owner)
);

-- ===== CARD PERFORMANCE =====
-- One row per card. Updated when a game ends if any seat had the card
-- in battlefield/hand/graveyard. games_included counts those games;
-- wins_when_included counts the subset where the holding seat won.
-- avg_turn_played + avg_battlefield_time are running means; the
-- internal *_count columns hold the denominators so means stay correct
-- across thousands of upserts. See internal/db/card_performance.go.

CREATE TABLE IF NOT EXISTS card_performance (
    card_name             TEXT PRIMARY KEY,
    games_included        INTEGER NOT NULL DEFAULT 0,
    wins_when_included    INTEGER NOT NULL DEFAULT 0,
    avg_turn_played       REAL    NOT NULL DEFAULT 0,
    avg_battlefield_time  REAL    NOT NULL DEFAULT 0,
    turn_play_count       INTEGER NOT NULL DEFAULT 0,
    bf_obs_count          INTEGER NOT NULL DEFAULT 0,
    updated_at            INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_card_performance_winrate
    ON card_performance(wins_when_included, games_included);

-- ===== BOINC CONTRIBUTOR =====
-- Distributed-compute (BOINC-style) tables. The hexdek-contrib client
-- earns credits by running game simulations on behalf of the server.
--
-- contributor_credits: one row per contributor (owner slug). Tracks
-- lifetime credits, chunk counts, validation stats, and anomaly state.
-- A frozen contributor's submissions are still received but credit
-- accrual is paused until manual review.
--
-- contrib_chunk: append-only log of every chunk that was assigned and
-- (eventually) returned. Used for spot-check selection, anomaly stats,
-- and operator-visible recent-activity.

CREATE TABLE IF NOT EXISTS contributor_credits (
    owner             TEXT PRIMARY KEY,
    credits_total     INTEGER NOT NULL DEFAULT 0,
    chunks_completed  INTEGER NOT NULL DEFAULT 0,
    chunks_rejected   INTEGER NOT NULL DEFAULT 0,
    games_simulated   INTEGER NOT NULL DEFAULT 0,
    -- Running stats for the 3-sigma anomaly detector. We track the
    -- mean and the second moment (M2) using Welford's algorithm so we
    -- can compute variance online without storing the full sample.
    elapsed_ms_n      INTEGER NOT NULL DEFAULT 0,
    elapsed_ms_mean   REAL    NOT NULL DEFAULT 0.0,
    elapsed_ms_m2     REAL    NOT NULL DEFAULT 0.0,
    last_z_score      REAL    NOT NULL DEFAULT 0.0,
    frozen            INTEGER NOT NULL DEFAULT 0,    -- 1 = credits paused
    frozen_reason     TEXT    NOT NULL DEFAULT '',
    first_seen_at     INTEGER NOT NULL DEFAULT 0,
    last_active_at    INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS contrib_chunk (
    chunk_id          TEXT PRIMARY KEY,
    owner             TEXT NOT NULL,
    issued_at         INTEGER NOT NULL,
    returned_at       INTEGER NOT NULL DEFAULT 0,
    games_count       INTEGER NOT NULL DEFAULT 0,
    n_seats           INTEGER NOT NULL DEFAULT 0,
    elapsed_ms        INTEGER NOT NULL DEFAULT 0,
    outcome_hash      TEXT    NOT NULL DEFAULT '',
    accepted          INTEGER NOT NULL DEFAULT 0,    -- 0 = pending, 1 = ok, -1 = rejected
    spot_checked      INTEGER NOT NULL DEFAULT 0,
    spot_check_passed INTEGER NOT NULL DEFAULT 0,
    credits_awarded   INTEGER NOT NULL DEFAULT 0,
    reason            TEXT    NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_contrib_chunk_owner
    ON contrib_chunk(owner, returned_at DESC);
CREATE INDEX IF NOT EXISTS idx_contrib_chunk_issued
    ON contrib_chunk(issued_at);

-- ===== ANTI-CHEAT PHASE 2: SPOT-CHECK + CAUTERIZE =====
-- verification_queue: pending replays for game outcomes selected by
-- the spot-check scheduler. Each row pins the inputs needed to
-- deterministically re-execute the game (rng seed, deck keys per
-- seat) and the claim being verified (winner, turns). The worker
-- transitions rows pending → running → passed | failed | error.
CREATE TABLE IF NOT EXISTS verification_queue (
    queue_id         INTEGER PRIMARY KEY AUTOINCREMENT,
    game_id          INTEGER NOT NULL,
    deck_key         TEXT NOT NULL,                    -- contributor under review
    enqueued_at      INTEGER NOT NULL,
    status           TEXT NOT NULL DEFAULT 'pending',  -- pending|running|passed|failed|error
    started_at       INTEGER,
    finished_at      INTEGER,
    detail           TEXT NOT NULL DEFAULT '',
    rng_seed         INTEGER NOT NULL DEFAULT 0,
    n_seats          INTEGER NOT NULL DEFAULT 0,
    deck_keys_json   TEXT NOT NULL DEFAULT '[]',
    claimed_winner   INTEGER NOT NULL DEFAULT -1,
    claimed_turns    INTEGER NOT NULL DEFAULT 0,
    replayed_winner  INTEGER NOT NULL DEFAULT -1,
    replayed_turns   INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_verqueue_status ON verification_queue(status, enqueued_at);
CREATE INDEX IF NOT EXISTS idx_verqueue_deck ON verification_queue(deck_key, enqueued_at DESC);
CREATE INDEX IF NOT EXISTS idx_verqueue_game ON verification_queue(game_id);

-- contributor_sanctions: warnings + bans issued by the cauterize
-- service when a verification fails. Escalation: 1st = warning,
-- 2nd = 24-hour ban, 3rd+ = permanent ban. expires_at is NULL for
-- warnings (no expiry) and permanent bans (never expire); set to a
-- future unix timestamp for temp bans.
CREATE TABLE IF NOT EXISTS contributor_sanctions (
    sanction_id  INTEGER PRIMARY KEY AUTOINCREMENT,
    deck_key     TEXT NOT NULL,
    owner        TEXT NOT NULL DEFAULT '',
    offense_num  INTEGER NOT NULL,                     -- 1, 2, 3, ...
    severity     TEXT NOT NULL,                        -- warning|temp_ban|permanent_ban
    issued_at    INTEGER NOT NULL,
    expires_at   INTEGER,                              -- NULL for warnings + permanent
    reason       TEXT NOT NULL DEFAULT '',
    queue_id     INTEGER,                              -- triggering verification, if any
    reviewed     INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_sanctions_deck ON contributor_sanctions(deck_key, issued_at DESC);
CREATE INDEX IF NOT EXISTS idx_sanctions_active ON contributor_sanctions(deck_key, severity, expires_at);

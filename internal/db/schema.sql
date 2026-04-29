-- mtgsquad ephemeral game state schema (SQLite).
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
    cached_at        INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_card_oracle_name ON card_oracle(name);

-- ===== SHOWMATCH PERSISTENT STATE =====
-- ELO ratings and game history that survive server restarts.

CREATE TABLE IF NOT EXISTS showmatch_elo (
    deck_key     TEXT PRIMARY KEY,
    commander    TEXT NOT NULL DEFAULT '',
    owner        TEXT NOT NULL DEFAULT '',
    rating       REAL NOT NULL DEFAULT 1500.0,
    games        INTEGER NOT NULL DEFAULT 0,
    wins         INTEGER NOT NULL DEFAULT 0,
    losses       INTEGER NOT NULL DEFAULT 0,
    delta        REAL NOT NULL DEFAULT 0.0,
    updated_at   INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS showmatch_game (
    game_id      INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at   INTEGER NOT NULL,
    finished_at  INTEGER NOT NULL,
    turns        INTEGER NOT NULL,
    winner       INTEGER NOT NULL DEFAULT -1,
    winner_name  TEXT NOT NULL DEFAULT 'DRAW',
    end_reason   TEXT NOT NULL DEFAULT 'unknown'
);

CREATE TABLE IF NOT EXISTS showmatch_game_seat (
    game_id      INTEGER NOT NULL REFERENCES showmatch_game(game_id) ON DELETE CASCADE,
    seat         INTEGER NOT NULL,
    commander    TEXT NOT NULL,
    life         INTEGER NOT NULL,
    hand_size    INTEGER NOT NULL DEFAULT 0,
    library_size INTEGER NOT NULL DEFAULT 0,
    gy_size      INTEGER NOT NULL DEFAULT 0,
    bf_size      INTEGER NOT NULL DEFAULT 0,
    lost         INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (game_id, seat)
);

CREATE INDEX IF NOT EXISTS idx_showmatch_game_finished ON showmatch_game(finished_at);
CREATE INDEX IF NOT EXISTS idx_showmatch_seat_commander ON showmatch_game_seat(commander);

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

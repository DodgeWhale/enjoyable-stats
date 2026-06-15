CREATE TABLE IF NOT EXISTS players (
    discord_user_id TEXT NOT NULL PRIMARY KEY,
    steam_id        TEXT NOT NULL,
    guild_id        TEXT NOT NULL,
    linked_at       TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS demo_insights (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    demo_file    TEXT NOT NULL,
    steam_id     TEXT NOT NULL,
    trigger_type TEXT NOT NULL,
    round_number INTEGER NOT NULL,
    detail_json  TEXT NOT NULL,
    created_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

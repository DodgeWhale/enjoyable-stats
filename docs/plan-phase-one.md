You are helping implement a Go application called enjoyable-stats.

## Project Overview
A CLI tool that downloads a CS2 demo file, parses it for custom game events,
and posts insights to a Discord channel — but only for players who have linked
their Steam ID via a Discord bot slash command. Built in Go 1.26.4.

## Status
Phase 1 is implemented and running. This document has been updated to reflect
the actual implementation, including deviations from the original pseudocode
and post-plan additions made during development.

## Notes for the Implementing Agent
You are implementing Phase 1 of this plan. Read the whole document first, then
build it.

Priorities (in strict order — when they conflict, the higher one wins):
1. Simplicity — prefer the smallest solution that works. Avoid overengineering;
   follow YAGNI. Do not add abstraction, config, interfaces, or indirection that
   this plan does not call for. The capability-interface design in the rule
   engine IS the intended amount of structure — don't add more.
2. Correctness — the code must compile, run, and behave as specified. Verify the
   real demoinfocs v5 / discordgo / modernc APIs (signatures, event types,
   field names) rather than guessing; adjust to the actual API where this plan's
   pseudocode differs, keeping the described behaviour.
3. Performance — only optimize where there is clear evidence it's needed. No
   premature optimization.

Working approach:
- Build bottom-up so each package compiles before the next: config → db →
  downloader → analyser → bot → cmd, then tests, then README.
- Run `go build ./...` and `go vet ./...` as you go; run `go test ./...` at the
  end. Fix what you break.
- This plan's code blocks are pseudocode/intent, not literal source — match the
  actual library APIs. If something in the plan is impossible or clearly wrong
  against the real API, prefer the simplest correct alternative and note it.
- Follow the Coding Standards and Package Organisation sections exactly.
- Stay within scope: implement only Phase 1; respect the "Do not implement"
  list. Do not add dependencies beyond those in the Tech Stack.

## Tech Stack
- Go 1.26.4
- github.com/bwmarrin/discordgo — Discord bot
- github.com/markus-wa/demoinfocs-golang/v5 — CS2 demo parser
- modernc.org/sqlite — SQLite driver (pure Go, no cgo)
- github.com/joho/godotenv — load .env into the process environment
- SQLite database file: cs2analyser.db

## Project Structure
enjoyable-stats/
├── main.go                — thin entrypoint, calls cmd.Execute()
├── config/config.go
├── cmd/root.go            — subcommand dispatch (stdlib flag, no cobra)
├── cmd/bot.go             — `bot` subcommand: persistent, handles slash commands
├── cmd/analyse.go         — `analyse` subcommand: one-shot download/parse/post/exit
├── bot/bot.go
├── bot/commands.go
├── analyser/analyser.go
├── analyser/triggers.go
├── analyser/events.go
├── analyser/state.go      — per-round game state shared across triggers
├── db/db.go
├── db/queries.go
├── db/schema.sql
└── downloader/downloader.go

## Package Organisation (group by domain)
Code is grouped by domain, one package per concern (config, db, downloader,
analyser, bot, cmd) — not by technical layer. This is already the case; keep it.
A few conventions to stay organised and extensible without going full DDD:

- Each domain package owns its own types, logic, and clean public API; internals
  stay unexported. Cross-domain types are passed explicitly (no shared globals).
- A domain may grow subpackages under its own directory as it expands, e.g.
  analyser/services/<name>.go, analyser/triggers/<name>.go, or
  db/migrations/. Add these only when a domain gets large enough to warrant it —
  don't pre-create empty subpackages in Phase 1.
- Keep dependencies pointing inward toward plain data: cmd wires everything;
  domain packages should not import cmd, and lower-level packages (db,
  downloader) should not import higher-level ones (bot, analyser).
- Avoid a catch-all "utils"/"helpers" package; put helpers in the domain that
  owns them.

## Run Modes (two subcommands)
The bot's slash commands are interactive and require a long-lived process, while
demo analysis is a run-once pipeline. These two lifecycles are split into
separate subcommands dispatched in cmd/root.go using os.Args[1] and a
per-command flag.FlagSet (standard library only — do NOT add cobra):

- `enjoyable-stats bot`
  - Starts the Discord session, registers /link-steam and /unlink-steam,
    and blocks (listening for interactions) until SIGINT/SIGTERM.
- `enjoyable-stats analyse -demo <url> | -file <path> [-channel <id>] [--debug]`
  - Runs the one-shot pipeline (download/prepare → parse → post) and exits.
  - -demo: download and decompress from a URL (supports .dem and .dem.bz2).
  - -file: use a local .dem or .dem.bz2 file, skipping download entirely;
    .dem.bz2 is decompressed alongside the source to .dem on first use.
  - -demo and -file are mutually exclusive.
  - --debug: after parsing, write a JSON array of StateSnapshot values to
    <demoname>.state.json for inspection (one snapshot per tracked event).
  - Opens its own short-lived Discord session only to post insights.
  - Logs per-phase timing (prepare, parse, save, post, total) and demo size
    on every run via slog.

## Database Schema
Two tables:

players — stores Discord user ID, Steam ID, guild ID, linked_at timestamp
demo_insights — stores demo filename, steam_id, trigger_type, round_number,
                detail_json (JSON blob), created_at timestamp

## Phase 1 Scope (implement this only)
1. config/config.go
   - Call godotenv.Load() to read the .env file into the process environment,
     then read DISCORD_TOKEN and DISCORD_CHANNEL_ID via os.Getenv
     (os.Getenv alone does NOT read .env files — godotenv populates them first)
   - A missing .env is not fatal (real env vars may already be set); a missing
     DISCORD_TOKEN IS fatal
   - Provide a simple Config struct: { DiscordToken, DiscordChannelID string }
   - Precedence: DISCORD_TOKEN and DISCORD_CHANNEL_ID from config take effect.
     Only the channel ID may be overridden by the analyse -channel flag; if
     -channel is empty, fall back to Config.DiscordChannelID. The token is never
     overridable via CLI.

2. db/db.go + db/queries.go + db/schema.sql
   - Connect to SQLite using modernc.org/sqlite (driver name is "sqlite",
     NOT "sqlite3")
   - Embed schema.sql with //go:embed and execute it on startup to create
     tables if they don't exist (CREATE TABLE IF NOT EXISTS ...). Embedding
     keeps the binary self-contained and independent of the working directory.
   - Steam IDs and Discord IDs are stored as TEXT (see "SteamID handling" note
     below).
   - Implement these queries:
     - InsertPlayer(discordUserID, steamID, guildID string) error
     - DeletePlayer(discordUserID string) error
     - GetTrackedSteamIDs() (map[string]bool, error)
     - GetPlayerMentions() (map[string]string, error)
         — returns steamID → discordUserID, used by PostInsights for @mentions
     - InsertInsight(demoFile, steamID, triggerType string, round int, detail map[string]any) error
         — json.Marshal the detail map into the detail_json TEXT column

3. downloader/downloader.go
   - Download(url, destDir): downloads from a URL into destDir (e.g. demos/).
     - os.MkdirAll the destination directory before writing.
     - Use an *http.Client with a timeout (not the default http.Get).
     - Check resp.StatusCode == http.StatusOK; otherwise return a wrapped error.
     - If the URL ends in ".bz2", wrap resp.Body in bzip2.NewReader and strip
       ".bz2" from the output filename so the final file is a raw .dem.
       (compress/bzip2 stdlib — decompression only, no new dependency.)
     - Otherwise stream resp.Body directly.
     - Return the local .dem file path.
   - PrepareLocal(path): accepts an existing local file path.
     - Returns plain .dem paths as-is.
     - For .dem.bz2: decompresses to a sibling .dem file (reuses it if already
       present), then returns the .dem path.
   - Demos are stored in the project-local demos/ directory (gitignored),
     not /tmp/demos/.
   - .gz is out of scope for Phase 1.

4. analyser/events.go
   - Define an Insight struct:
       SteamID     string          // converted from uint64 at emit time
       TriggerType string
       Round       int
       Detail      map[string]any

### SteamID handling (conversion boundary)
- demoinfocs exposes player IDs as uint64 (player.SteamID64).
- The DB layer and public APIs use string IDs everywhere.
- Conversion happens at exactly two edges, both inside the analyser:
  1. On startup, convert the incoming map[string]bool of tracked IDs into an
     internal map[uint64]bool (tracked set) for fast per-event comparison.
  2. When emitting an Insight, convert the uint64 ID to string with
     strconv.FormatUint(id, 10).
- Nothing outside the analyser deals in uint64.

## Rule Engine Design — scope item 5 (analyser/state.go + triggers.go + analyser.go)

### How Go handles this kind of logic
There is no built-in "rules engine"; the idiomatic Go approach is small
composable interfaces + a central dispatcher that owns shared state. We use:

- A single authoritative `State` struct holding per-round game state (current
  round, kills-per-player this round, alive counts per team, clutch candidate).
  Triggers never mutate it; the analyser owns and updates it.
- A set of small "capability" interfaces. Each trigger implements ONLY the
  event hooks it cares about (this mirrors how the stdlib uses optional
  interfaces like http.Flusher/http.Hijacker). The analyser type-asserts each
  trigger to discover which hooks it supports.
- The demo parser is already an event bus (RegisterEventHandler), so the
  analyser registers ONE handler per relevant demo event, updates `State`, then
  fans the event out to the triggers that implement the matching hook,
  collecting any returned Insights.

### Warmup filtering and round numbering
- Ignore ALL events while GameState().IsWarmupPeriod() is true. The analyser
  checks this at the top of each event handler and returns early during warmup,
  so warmup kills never reach state updates or triggers (prevents false
  aces/team-kills and bad round counts).
- Use the parser's own round number (GameState().TotalRoundsPlayed()) as the
  authoritative round for Insights rather than a hand-rolled counter. The
  per-round resets (kills/alive/clutch) still key off events.RoundStart.

### analyser/state.go
    type State struct {
        Round   int                    // set from GameState().TotalRoundsPlayed()
        Tracked map[uint64]bool        // tracked players (internal uint64 set)

        kills    map[uint64]int        // kills this round, per steamID
        mvps     map[uint64]int        // MVP count this match, per steamID
        prevMVPs map[uint64]int        // MVP count before last RoundEnd update
        alive    map[common.Team]int   // alive count per team (T / CT)

        // clutch tracking: when a team drops to exactly 1 alive, we snapshot
        // the lone survivor and how many enemies were alive at that instant.
        clutcher        uint64         // 0 = none this round
        clutchTeam      common.Team
        clutchVsEnemies int
    }
    - ResetRound(players []*common.Player) clears kills/alive/clutch state and
      recomputes alive counts from the provided participant slice.
    - mvps and prevMVPs are NOT reset per round (they are match-level counters).
    - StateSnapshot is a JSON-serialisable view of State at a point in time,
      with uint64 IDs converted to strings and Team values to "T"/"CT".
    - WriteStateLog(path string, states []StateSnapshot) error writes the
      snapshot slice as indented JSON to a file.

### analyser/triggers.go
    // Capability interfaces — triggers implement only what they need.
    type Trigger interface { Name() string }

    type RoundStartHook interface { OnRoundStart(s *State) }
    type KillHook       interface { OnKill(s *State, e events.Kill) []Insight }
    type RoundEndHook   interface { OnRoundEnd(s *State, e events.RoundEnd) []Insight }

    Triggers (each a struct in this file):

    - TeamKill (KillHook):
        if killer != nil && victim != nil && killer != victim &&
           killer.Team == victim.Team && tracked(killer):
            emit Insight{TriggerType:"team_kill", SteamID:killer,
                         Round:s.Round, Detail:{victim, weapon}}

    - Ace (KillHook):
        state.kills[killer]++ is done by the analyser before fanout.
        if tracked(killer) && s.kills[killer] == 5:   // == 5, fires once
            emit Insight{TriggerType:"ace", Round:s.Round,
                         Detail:{kills:5}}

    - Clutch (RoundEndHook):
        The analyser maintains the clutch snapshot in State as deaths occur
        (see dispatch below). On round end:
        if s.clutcher != 0 && tracked(s.clutcher) &&
           s.clutchVsEnemies >= 2 && winningTeam == s.clutchTeam:
            emit Insight{TriggerType:"clutch", SteamID:s.clutcher,
                         Round:s.Round, Detail:{vs:s.clutchVsEnemies}}

    - MVP (RoundEndHook):
        NOTE: events.RoundMVPAnnouncement does NOT fire in CS2 demos processed
        by demoinfocs v5. MVP counts are tracked instead via Player.MVPs() (the
        m_iMVPs scoreboard property), read at each RoundEnd for all tracked
        players. The analyser saves the previous count in prevMVPs before
        updating mvps, so the trigger can fire exactly once when a player
        crosses 3 MVPs:
        if mvps[id] >= 3 && prevMVPs[id] < 3:
            emit Insight{TriggerType:"mvp", SteamID:id,
                         Round:s.Round, Detail:{mvps:count}}

### analyser/analyser.go (the dispatcher)
    type Analyser struct { triggers []Trigger }

    func New() *Analyser  // initialises with triggers: TeamKill, Ace, Clutch, MVP

    func (a *Analyser) Analyse(path string, tracked map[string]bool, debug bool) ([]Insight, []StateSnapshot, error)
    - Open the demo file, create the demoinfocs parser.
    - Build State with Tracked converted to map[uint64]bool.
    - When debug is true, append a StateSnapshot to stateLog after each
      handler completes; otherwise record() is a no-op.
    - Register handlers ONCE with the parser. EVERY handler returns early if
      GameState().IsWarmupPeriod() is true (warmup events are fully ignored).
    - At the top of each (non-warmup) handler, refresh
      state.Round = GameState().TotalRoundsPlayed().

        events.RoundStart:
            state.ResetRound(parser.GameState().Participants().Playing())
            for each trigger implementing RoundStartHook: t.OnRoundStart(state)
            record("round_start")

        events.Kill:
            // analyser updates shared state FIRST
            if killer tracked: state.kills[killer.SteamID64]++
            for each trigger implementing KillHook:
                insights = append(insights, t.OnKill(state, e)...)
            // clutch tracking reuses Kill (no separate PlayerDeath handler):
            decrement state.alive[victim.Team]
            if state.alive[victim.Team] == 1:
                lone := the remaining alive player on that team
                state.clutcher = lone.SteamID64
                state.clutchTeam = lone.Team
                state.clutchVsEnemies = state.alive[enemyTeam]
            record("kill")

        events.RoundEnd:
            // sync MVP counts from scoreboard before fanout
            for each tracked participant: state.prevMVPs[id] = state.mvps[id]
                                          state.mvps[id] = player.MVPs()
            for each trigger implementing RoundEndHook:
                insights = append(insights, t.OnRoundEnd(state, e)...)
            record("round_end")

    - parser.ParseToEnd(); return collected insights, stateLog, error.

    Note: demoinfocs invokes event handlers sequentially on one goroutine, so
    State needs no mutex.

6. bot/bot.go + bot/commands.go
   - Create a Discord session with discordgo
   - Set intents: discordgo.IntentGuilds (note: singular, not IntentsGuilds —
     that constant does not exist in discordgo v0.29.0)
   - Register the slash commands as GUILD-scoped (per guild from the ready
     event's session state), not global — guild commands register instantly,
     whereas global commands can take up to ~1 hour to propagate. Pass the
     guild ID to ApplicationCommandCreate (empty string = global). Easy to
     switch to global later if needed.
   - Register two slash commands on startup:
     - /link-steam [steam-id] — validate it's a 17-digit numeric string,
       insert into players table, respond with confirmation
     - /unlink-steam — remove from players table, respond with confirmation
   - Implement PostInsights(channelID string, insights []Insight, players map[string]string)
     - players is a map of steamID → discordUserID for @mentions, obtained from
       db.GetPlayerMentions()
     - Group insights by player, format a readable embed or message per player
     - Post to the channel
   - PostInsights is the only bot capability the `analyse` subcommand uses (via
     a short-lived session); slash commands are served only by `bot`.
   - bot.New(token string, database *db.DB) — when database is nil, no slash
     command handlers are registered (post-only mode used by `analyse`).

7. main.go + cmd/
   - main.go is a thin entrypoint: call cmd.Execute() and exit non-zero on error.
   - cmd/root.go: inspect os.Args[1] to dispatch to "bot" or "analyse"; print
     usage if missing/unknown. Each subcommand defines its own flag.FlagSet.

   - cmd/bot.go — `enjoyable-stats bot` (persistent):
     1. Load config
     2. Connect DB
     3. Start Discord session, register slash commands
     4. Block until SIGINT/SIGTERM (signal.NotifyContext)
     5. Gracefully close the session on shutdown

   - cmd/analyse.go — `enjoyable-stats analyse -demo <url> | -file <path> [-channel <id>] [--debug]`
     (one-shot pipeline, in this order):
     1. Load config
     2. Resolve channel ID: -channel flag if set, else Config.DiscordChannelID
     3. Connect DB
     4. Prepare demo: if -file, call downloader.PrepareLocal; if -demo, call
        downloader.Download into demos/  (changed from /tmp/demos/)
     5. Fetch tracked Steam IDs (GetTrackedSteamIDs) + mentions (GetPlayerMentions)
     6. Parse demo, get insights (analyser.Analyse with debug flag)
     7. If --debug, write state log to <demo>.state.json via analyser.WriteStateLog
     8. Save insights to DB (InsertInsight per insight)
     9. Open a short-lived Discord session and PostInsights to the channel
     10. Close the session and exit
     - Per-phase durations (prepare, parse, save, post, total) and demo size
       are logged at INFO level on completion.

## Coding Standards
- Errors must be wrapped with fmt.Errorf("context: %w", err) and handled
  with if err != nil — no panics except in main for fatal startup errors
- No global variables — pass dependencies explicitly via structs
- Each package should expose a clean public API; keep internals unexported
- Use log/slog for structured logging throughout
- Add a README.md with:
  - Project description
  - Setup steps: prerequisites (Go 1.26.4), .env config with DISCORD_TOKEN and
    DISCORD_CHANNEL_ID
  - How to run the app: both `go run . <subcommand> ...` during development and
    `go build` then `./enjoyable-stats <subcommand> ...`
   - The available commands and their flags:
    - `bot` — run the persistent Discord bot (handles /link-steam, /unlink-steam)
    - `analyse -demo <url> | -file <path> [-channel <id>] [--debug]`
  - The /link-steam and /unlink-steam slash command usage
  - A note that .dem and .dem.bz2 URLs and local files are supported
    (.gz is not handled in Phase 1)
  - How to run the tests: `go test ./...`

## Tests (scope item 8)
Keep tests minimal — happy paths only, no edge-case or error-path coverage.
Use the standard library `testing` package only (no testify or other deps).
Prefer table-driven tests with descriptive case names (the name should read as
the behaviour under test, e.g. "ace fires at fifth kill in round"). Do NOT add
narration comments — let the test/case names document intent.

Cover these (pure logic, no network or real demo files required):
- config: Load returns the token and channel ID from a populated environment.
- db: round-trip against a temporary SQLite file —
  InsertPlayer then GetTrackedSteamIDs / GetPlayerMentions return the row;
  InsertInsight persists and the detail_json is valid JSON.
- analyser triggers: construct a State plus synthetic events and assert each
  trigger emits the expected Insight on its happy path —
  team_kill on same-team kill, ace at the 5th kill, clutch on last-alive win
  vs ≥2 enemies, mvp when mvps crosses 3 at round-end.
  (Tests the trigger logic directly; no demo parsing.)
- bot: the Steam ID validation accepts a valid 17-digit ID.
- downloader: the output-filename helper strips a .bz2 suffix to .dem;
  PrepareLocal returns a .dem path as-is.

## Do not implement
- Auto polling / Phase 2 automation
- Any web interface
- Integration tests, mocks for Discord/HTTP, or error-path/edge-case tests
  (happy-path unit tests only for Phase 1)
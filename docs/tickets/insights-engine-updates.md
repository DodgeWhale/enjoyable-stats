---
title: Insights Engine Updates and Debug Output Spec
status: DONE
---

## Purpose

This document defines new work for the existing `enjoyable-stats` Go project: a ranking, selection, name-aware rendering, and debug-output layer that turns the insights the analyser already produces into a short, ranked, banter-style recap for a single CS2 demo.

Existing capabilities are already implemented and out of scope: CLI command structure (`cmd/`), demo download and local file handling (`downloader/`), Discord posting (`bot/`), tracked-player filtering, bot slash commands (`bot/commands.go`), config loading (`config/`), and persistence (`db/`).

### Important constraint: tracked players only

Every detector in `analyser/triggers.go` gates on `s.Tracked[steamID]`, and `Tracked` is populated only from the `players` table (Steam IDs linked via `/link-steam`). The analyser therefore only ever produces insights for linked players. The product goal below is scoped accordingly: the recap covers the most talked-about moments **involving tracked players**, not all ten players. Opponents appear only as victims inside `Insight.Detail` (for example `team_kill` and `knife_kill` store `victim`).

The parser (`demoinfocs-golang/v5`) exposes `common.Player.Name` and `common.Player.SteamID64`, so names can be rendered once they are captured during parsing. Today they are discarded: `Insight` stores only `SteamID` as a string.

## Product Goal

For a single demo, answer:

> What are the 3 to 5 moments involving our tracked players that people would actually talk about afterward?

A short post-game recap with personality, replacing the current flat "group every insight by player and dump them" output in `bot.FormatInsights`.

## In Scope

### Included

- Capture player names during parsing and attach them to insights.
- Deterministic scoring of the insights the analyser already emits.
- Ranking, per-player caps, deduplication, and an overall public cap.
- A few high-signal new detectors that are cheaply achievable with the current parser data.
- Name-first Discord rendering (reusing the existing `@mention` path for linked players).
- A structured debug JSON describing candidates, scores, and drop reasons, plus richer `-debug` console output.

### Excluded

- CLI flag changes. `analyse` already has `-debug`; reuse it. No new flags are required.
- Bot command changes (`bot/commands.go` is untouched).
- Steam linking/unlinking, demo download/decompression.
- Database schema changes. Insights are still saved through the existing `db.InsertInsight`. Scoring and recap selection happen in memory and are not persisted (the debug JSON is written to a file, not the DB).
- Cross-demo aggregation, history, or career stats.
- Whole-match (all ten players) analysis. The pipeline only sees tracked players; do not pretend otherwise.

## Current Pipeline (for reference)

`analyser.Analyse` (`analyser/analyser.go`) registers parser event handlers, runs detectors that implement the hook interfaces in `analyser/triggers.go`, and returns `([]Insight, []StateSnapshot, error)`. `cmd/analyse.go` and `bot/pipeline.go` then save each insight via `db.InsertInsight` and post them with `bot.FormatInsights` / `bot.PostInsights`.

```go
// analyser/events.go (current)
type Insight struct {
    SteamID     string
    TriggerType string
    Round       int
    Detail      map[string]any
}
```

Detectors already implemented (do not rebuild these): `team_kill`, `ace`, `clutch`, `mvp`, `lurker_tax`, `bomb_god`, `entry_king`, `refund_request`, `entry_victim`, `instant_trade`, `bomb_mule`, `defuse_interrupted`, `flash_tax`, `kit_dodger`, `economy_terrorist`, `knife_kill`, `knife_team_kill`.

## New Pipeline

The new layer sits **after** detection and **before** rendering. Reuse the existing detectors as the candidate source rather than writing a parallel detection system.

1. **Detection (existing)** - `Analyse` runs the current triggers and collects `[]Insight`.
2. **Name enrichment (new)** - fill `Insight.PlayerName` (and victim names in `Detail`) from names captured during parsing.
3. **Scoring (new)** - assign a deterministic `Score` to each insight from package constants plus context bonuses.
4. **Selection (new)** - rank, dedupe, apply per-player and overall caps, choose one headline.
5. **Rendering (new)** - produce a name-first banter summary in `bot`.
6. **Debug export (new)** - when `-debug` is set, write a recap debug JSON next to the existing state log and print ranked plus dropped candidates to the console.

Suggested file layout (keep the Insight type in `analyser` to avoid an import cycle; rendering stays in `bot` where Discord formatting already lives):

- `analyser/events.go` - extend `Insight`, add the recap result types.
- `analyser/scoring.go` - base weights and `Score(...)`.
- `analyser/recap.go` - `BuildRecap(...)`: ranking, dedupe, caps, debug trace.
- `bot/render.go` - templates and the name-first renderer.

## Data Model

Extend the existing `Insight` rather than introducing a parallel type. Use plain fields and the existing `Detail map[string]any`; match the current style (string Steam IDs, snake_case `Detail` keys).

```go
// analyser/events.go (extended)
type Insight struct {
    SteamID     string
    PlayerName  string         // captured from common.Player.Name; "" if unknown
    TriggerType string
    Round       int
    Score       int            // assigned by the scoring layer; 0 before scoring
    Detail      map[string]any
}
```

Recap result and debug types:

```go
// analyser/recap.go
type Recap struct {
    DemoID   string             // demo file path/base, same value used for db rows
    MapName  string             // parser.Header().MapName, "" if unavailable
    Rounds   int                // final TotalRoundsPlayed
    Headline *Insight           // single highest-priority moment, nil if none
    Public   []Insight          // ranked and capped, includes the headline
    Dropped  []DroppedInsight   // everything detected but not surfaced, for debug
    Trace    []DebugEvent       // ordered scoring/selection decisions
}

type DroppedInsight struct {
    Insight Insight
    Reason  string // e.g. "per_player_cap", "below_min_score", "duplicate", "public_cap"
}

type DebugEvent struct {
    Stage   string         // see Debug Stage Events
    SteamID string
    Trigger string
    Round   int
    Message string
    Fields  map[string]any
}
```

## Name Capture and Rendering

### Capturing names during parsing

`common.Player.Name` is available on every event but is currently dropped. Add a names map to `State` and populate it from the players seen in the existing handlers:

```go
// analyser/state.go
names map[uint64]string // steamID64 -> latest known name
```

Populate it opportunistically wherever players are already in hand (the `Kill` handler's killer/victim, `RoundEnd`'s `Participants().Playing()`, `BombPlanted`, etc.). After `ParseToEnd`, before returning, enrich each collected insight: set `Insight.PlayerName = names[steamID]`, and resolve any `Detail["victim"]` Steam ID to a `Detail["victim_name"]` when present.

### Render-time resolution order

The renderer in `bot` chooses a display string per insight in this order:

1. Discord mention. If `players[steamID]` (the existing steamID -> discordUserID map from `db.GetPlayerMentions`) has an entry, use `<@id>`. This is the current behaviour and must be preserved for linked players.
2. `Insight.PlayerName` if non-empty (covers victims and any unlinked names captured from the demo).
3. The raw Steam ID string.
4. Generic `someone` if even the Steam ID is missing.

Add one helper in `bot/render.go`, for example `displayName(steamID, name string, mentions map[string]string) string`, and route all rendering through it. Do not create a second naming path; extend the existing mention logic in `bot.formatPlayerInsights`.

## New Detectors

Only add detectors that are cheap and deterministic with data already available in the parser. Each follows the existing `KillHook` / `RoundEndHook` pattern in `analyser/triggers.go`, gates on `s.Tracked`, and is registered in `analyser.New()`.

Achievable and worth adding:

- `multi_kill` (4k). Emit at round end when `s.kills[killer] == 4`. Check at round end rather than on the 4th kill: a 5-kill round passes through 4 kills, so firing on the 4th kill would emit both a `multi_kill` and an `ace` for the same player and round. Carry `Detail["kills"]`. Ace already covers 5k.
- `flash_assist`. In the `Kill` handler, when `e.AssistedFlash` is true and the flasher is tracked, credit a support moment. The flasher is not on `events.Kill` directly; reuse the existing `PlayerFlashed` tracking (`state.flashBlinds` already records blinds) to attribute, or record the most recent enemy flasher per victim during `PlayerFlashed` and match it on the kill. Keep it simple: if exact attribution is awkward, emit per round when a tracked player blinded an enemy who then died within a short window.
- `team_flash`. Negative. In `PlayerFlashed`, the current `flash_tax` detector only counts enemies blinding the tracked player. Add the inverse: a tracked player blinding a **teammate** for a long duration (`e.Attacker.Team == e.Player.Team`, `FlashDuration() >= 1s`). Track count in `State`, emit at match end above a threshold.
- `zeus_kill`. Mirror `KnifeKill` exactly, swapping `isKnife` for a `isZeus` check (`w.Type == common.EqZeus`).
- `style_kill`. In the `Kill` handler for a tracked enemy kill, flag when `e.IsWallBang()` or `e.NoScope` (with an AWP) is true. Carry the flavour in `Detail` (`wallbang`, `noscope`). Through-smoke kills are intentionally excluded: they are too common to read as a highlight.

Explicitly dropped from the original draft as vague or not deterministically achievable with current data:

- "Strong site entry" and "high-impact post-plant hold" - no clean signal; entry is already covered by `entry_king` / first-kill tracking.
- "High-leverage misplay that flips a winning round" and any "conversion bonus that correlates with round won/lost" - requires a win-probability model the project does not have. Keep scoring to local, observable facts.
- "Man-advantage untraded death", "anti-eco loss participation with clear error indicators", "low-impact survival / passive save" - too noisy to detect reliably; skip. `lurker_tax`, `instant_trade`, and `economy_terrorist` already cover the achievable parts of this space.
- "Repeated bad luck" - already approximated by `instant_trade`.

## Scoring Model

Scoring runs over the collected `[]Insight`. Use package-level constants and a pure function, matching how the existing triggers use hardcoded thresholds (`instantTradeWindow`, `< 2000`, `>= 4500`, etc.). Do **not** introduce a config subsystem for weights; the project config (`config/config.go`) is env-only (`DISCORD_TOKEN`, `DISCORD_CHANNEL_ID`) and must not carry tuning knobs.

```go
// analyser/scoring.go
var baseScores = map[string]int{
    "ace":            100,
    "clutch":          70, // plus leverage from Detail["vs"]
    "multi_kill":      55,
    "knife_kill":      60,
    "zeus_kill":       60,
    "style_kill":      40,
    "entry_king":      35,
    "bomb_god":        30,
    "instant_trade":   25,
    "mvp":             50, // plus a bonus per MVP beyond the threshold from Detail["mvps"]
    "flash_assist":    20,
    // negatives
    "knife_team_kill": 55, // funny, scores high as a lowlight
    "team_kill":       45,
    "team_flash":      30,
    "lurker_tax":      30,
    "refund_request":  25,
    "entry_victim":    25,
    "bomb_mule":       20,
    "kit_dodger":      20,
    "defuse_interrupted": 20,
    "economy_terrorist": 25,
    "flash_tax":       15,
}
```

`Score(ins Insight) int` starts from `baseScores[ins.TriggerType]` and applies deterministic adjustments derived only from `Detail`:

- Leverage bonus: for `clutch` / `lurker_tax`, add a per-enemy bonus from `Detail["vs"]` (a 1v3 beats a 1v2).
- Rarity bonus: `knife_kill`, `zeus_kill`, `knife_team_kill` get a flat boost (rare and banter-worthy).
- Style bonus: `style_kill` adds per-flavour points (noscope > wallbang, pick fixed values).
- MVP bonus: `mvp` adds points per MVP beyond the threshold from `Detail["mvps"]`, so a bigger MVP haul ranks higher.
- No "conversion bonus" (would need round win attribution; out of scope).

Penalties are applied in the selection layer, not in `Score`, because they depend on what else was already chosen (see below).

## Selection: Ranking, Dedupe, and Caps

`BuildRecap(insights []Insight, demoID, mapName string, rounds int) Recap` performs deterministic selection:

1. Score every insight.
2. Sort by score descending; break ties by `Round` then `TriggerType` then `SteamID` so output is stable for a given demo.
3. Deduplicate near-identical candidates for the same player and trigger in the same round (record dropped ones with reason `duplicate`).
4. Apply a minimum score threshold (`minPublicScore`); below it goes to `Dropped` with reason `below_min_score`.
5. Apply a per-player cap (`maxPerPlayer`, default 3). Extra moments for that player are dropped with reason `per_player_cap`. When a player already has a stronger lowlight, drop weaker lowlights first.
6. Apply the overall public cap (`maxPublic`, default 5). Overflow is dropped with reason `public_cap`.
7. The first entry in `Public` is the `Headline`.

Defaults as package constants:

```go
const (
    maxPublic      = 5
    maxPerPlayer   = 3
    minPublicScore = 15
)
```

### Priority order (rationale behind the score weights)

`Score` (from `baseScores` plus bonuses) is the single source of trigger priority. The ordering below is the design rationale that informs those weights; it is **not** a second runtime map. Do not add a parallel `triggerPriority` table that has to be kept in sync with `baseScores`. Ranking is by score descending, and the highest-ranked public insight is the headline. Ties are broken deterministically by `Round`, then `TriggerType`, then `SteamID` (see selection step 2 above), not by a separate priority lookup.

When tuning `baseScores`, keep the weights roughly consistent with this intent:

1. Round-winning/losing swing moments that we can actually observe: `clutch`, `ace`.
2. Rare spectacle: `knife_kill`, `zeus_kill`, `knife_team_kill`, `style_kill`.
3. Large multi-kills: `multi_kill`.
4. Entry and support: `entry_king`, `flash_assist`, `instant_trade`.
5. Soft banter and lowlights: everything else.

## Discord Rendering

Replace the per-player dump in `bot.FormatInsights` with a recap renderer that consumes `Recap.Public` and `Recap.Headline`. Keep the existing emoji/copy in `bot.formatInsight` as the per-line template source; do not scatter new string building through scoring.

### Output shape

- One headline line (the `Recap.Headline`).
- Two to four supporting lines from the rest of `Recap.Public`.
- Optional closing jab.

### Rendering rules

- Resolve names via the resolution order above; prefer `@mentions` for linked players, then captured names.
- Keep each line short.
- Do not print raw `Score` values in normal mode (debug only).
- Do not print dropped candidates in normal mode.
- Degrade gracefully when `MapName`, `PlayerName`, or `Detail` fields are missing.

### Templates

Move the copy into `bot/render.go` as a small set of per-trigger templates. The existing strings in `bot.formatInsight` are the starting point; example patterns:

- `{player} dragged round {round} over the line in a 1v{vs}.`
- `{player} committed a war crime on {victim} with the knife.`
- `{player} was technically in the server.`

No tone-profile storage. A single default tone is enough for handoff; do not build a tone-selection subsystem.

## Debug Specification

The recap debug output is a first-class deliverable and should make tuning easy. It reuses the existing `-debug` flag on `analyse` (`cmd/analyse.go`); no new flag.

### Console output (extend the existing `-debug` branch)

When `-debug` is set, `cmd/analyse.go` currently prints `bot.FormatInsights`. Extend it to print, from the `Recap`:

- Ranked public moments with their scores.
- All dropped candidates with their drop reason.
- The score breakdown per candidate.
- The headline choice.

### Structured debug JSON (new file)

`cmd/analyse.go` already writes a state log to `<demo>.state.json` via `analyser.WriteStateLog`. Add a second writer that emits the recap to `<demo>.recap.json` using the same `os.WriteFile(..., 0o644)` + `json.MarshalIndent` style as `WriteStateLog`. Keep the existing `.state.json` as-is.

Top-level shape:

```json
{
  "demo_id": "...",
  "map": "...",
  "rounds": 0,
  "headline": {},
  "public": [],
  "dropped": [],
  "trace": []
}
```

Each public/dropped entry includes: Steam ID, player name, trigger type, round, score, `Detail`, and (for dropped) the drop reason.

### Debug stage events (`DebugEvent.Stage` values)

Log at least: `scored`, `deduped`, `selected_public`, `dropped_public`, `name_fallback_used`. (Detection-time stages from the original draft such as `candidate_detected`/`candidate_enriched` are redundant here because detection already happens in the existing trigger layer.)

### Debug design rules

- Deterministic for the same demo input (enforced by the stable sort above).
- Independent of Discord formatting (operates on `Recap`, not rendered strings).
- Useful even when `Public` is empty (still emits candidates and drop reasons).

## Integration Touchpoints

Concrete files and functions to change:

- `analyser/events.go` - extend `Insight`; add `Recap`, `DroppedInsight`, `DebugEvent`.
- `analyser/state.go` - add `names` map and initialise it in `Analyse` / `ResetRound` as appropriate; add a recap JSON writer alongside `WriteStateLog`.
- `analyser/analyser.go` - populate `names` in the existing handlers; new detectors registered in `New()`; enrich insight names after `ParseToEnd`. Expose `MapName` via `parser.Header().MapName` and `Rounds` via the final `TotalRoundsPlayed()`.
- `analyser/scoring.go`, `analyser/recap.go` - new.
- `bot/bot.go` / `bot/render.go` - replace `FormatInsights` internals with the recap renderer; keep `PostInsights` posting one or more messages.
- `cmd/analyse.go` - build the `Recap`, print debug console output, write `<demo>.recap.json` when `-debug`.
- `bot/pipeline.go` - the bot path (`RunAnalysis`) should build the recap and post it too, so live posting matches `analyse`.

Database (`db/`) is unchanged: individual insights are still saved via `InsertInsight`; the recap is in-memory plus the debug file.

## Configuration

No new configuration subsystem. All thresholds and weights live as package-level constants in `analyser/scoring.go` and `analyser/recap.go` (`maxPublic`, `maxPerPlayer`, `minPublicScore`, `baseScores`, bonus values). This matches the existing constant-driven detectors and keeps `config/config.go` env-only.

## Testing

Follow the existing table-light, constructed-input style in `analyser/triggers_test.go` (build structs directly, assert on returned values). No real-demo fixtures are required.

### Unit tests

- Name resolution prefers `@mention`, then `PlayerName`, then Steam ID, then `someone` (`bot/render_test.go`).
- `Score` ordering: ace > knife > multi_kill > flash_assist; clutch leverage increases with `Detail["vs"]`.
- `BuildRecap` enforces `maxPerPlayer` and `maxPublic`, records correct drop reasons, and picks the highest-priority headline.
- Deduplication collapses same player/trigger/round duplicates.
- Renderer degrades gracefully when optional `Detail` fields are absent.

### Golden test

One golden test on the **recap debug JSON** built from a hand-constructed `[]Insight` (not a parsed demo). Assert the JSON contains scores, the public list stays capped, and dropped entries carry reasons. This keeps the test fast and deterministic without checking a 25MB demo into the repo.

## Acceptance Criteria

- The existing detectors feed into a new scoring + selection layer; no parallel detection system is added.
- Player names are captured during parsing and attached to insights; rendering prefers `@mentions`, then names, then Steam IDs.
- Public output is ranked and capped (default 5, max 3 per player) with a single headline.
- A few new achievable detectors (`multi_kill`, `flash_assist`, `team_flash`, `zeus_kill`, `style_kill`) are added; vague/unachievable ones are not.
- `analyse -debug` prints ranked and dropped candidates with score breakdowns and writes `<demo>.recap.json` alongside the existing `<demo>.state.json`.
- The recap debug JSON captures candidates, ranked public moments, and drop reasons, and is deterministic for the same input.
- Public output reads as a narrative recap rather than a flat per-player dump.
- No config subsystem, schema change, or new CLI flag is introduced.

## Recommended Implementation Order

1. Extend `Insight` (`PlayerName`, `Score`) and add the recap/debug types.
2. Capture names in `State` during parsing and enrich insights after `ParseToEnd`.
3. Add `scoring.go` (base weights + `Score`) and `recap.go` (ranking, dedupe, caps, trace).
4. Add the new detectors (`multi_kill`, `zeus_kill`, `style_kill`, `team_flash`, `flash_assist`) and register them in `New()`.
5. Add the name-first renderer in `bot/render.go` and switch `FormatInsights` / `PostInsights` to consume the recap.
6. Extend `cmd/analyse.go` debug output and write `<demo>.recap.json`; mirror in `bot/pipeline.go`.
7. Add unit and golden tests; tune constants against the demos already in `demos/`.

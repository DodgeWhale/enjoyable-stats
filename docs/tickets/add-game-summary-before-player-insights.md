---
title: Add Game Summary Before Player Insights
status: PLAN
---

## Purpose

The recap currently opens with a bare `**{map}** recap` line and jumps straight into ranked player moments (`bot.FormatRecap` in `bot/render.go`). It never states how the match actually went. Add a one-line game summary (map, final scoreline, and the tracked team's result) that renders **above** the player insights, so a reader sees the outcome before the banter.

The match result data already exists in the parser; today it is discarded. The analyser only exposes `MapName` and `Rounds` from a parse.

## In Scope / Excluded

### Included

- Capture final team scores and clan names from the parser at match end.
- Derive the tracked team's outcome (won / lost / tie) from the final round's player teams.
- Carry this through a new `analyser.Summary` value into `Recap`.
- Render a summary header line in `bot.FormatRecap` before the headline.
- Include the summary in the `<demo>.recap.json` debug output and the `-debug` console output.

### Excluded

- No new CLI flag, config, or DB schema change. The summary is in-memory plus the existing debug file.
- No per-player scoreboards, KDA tables, or per-round breakdowns. One summary line only.
- No new detectors or scoring changes.
- No change to how individual insights are saved (`db.InsertInsight` is untouched).

## Existing Implementation

- `analyser/analyser.go` `Analyse` parses the demo and returns `Result{Insights, MapName, Rounds, StateLog, NameTrace}`. `MapName` comes from the `CDemoFileHeader` net message; `Rounds` from `parser.GameState().TotalRoundsPlayed()` after `ParseToEnd`.
- `analyser/events.go` defines `Result` and `Recap` (`Recap` holds `MapName string`, `Rounds int`).
- `analyser/recap.go` `BuildRecap(insights []Insight, demoID, mapName string, rounds int) Recap` scores, ranks, and caps insights.
- `analyser/state.go` `recapToJSON` / `WriteRecapLog` serialise the recap to `<demo>.recap.json`; `State.roundEndPlayers` holds `[]*common.Player` captured at the last `RoundEnd`, each with its final `.Team`.
- `bot/render.go` `FormatRecap` builds the message, opening with the `**{map}** recap` header. `FormatRecapDebug` prints the ranked/dropped lists.
- `bot/bot.go` `FormatInsights` / `PostInsights` and `cmd/analyse.go` / `bot/pipeline.go` thread `mapName`/`rounds` scalars into `BuildRecap` and posting.

The parser exposes final scores: `parser.GameState().TeamCounterTerrorists()` and `TeamTerrorists()` return `*common.TeamState`, which has `Score()` and `ClanName()`. `TeamState` scores accumulate across the halftime side swap, so the final CT/T scores are the true totals for whichever group is on each side at the end.

## Design

### Summary type (`analyser/events.go`)

Introduce one value object holding the match facts. It replaces the standalone `MapName`/`Rounds` fields on `Result` and `Recap`.

```go
type Summary struct {
    MapName       string
    Rounds        int
    CTScore       int
    TScore        int
    CTClan        string // "" if unavailable
    TClan         string // "" if unavailable
    TrackedSide   string // "CT", "T", or "" when undeterminable or mixed
    TrackedScore  int    // 0 when TrackedSide == ""
    OpponentScore int    // 0 when TrackedSide == ""
    Outcome       string // "won", "lost", "tie", or "" when TrackedSide == ""
}
```

Update `Result` to carry `Summary Summary` (drop `MapName`/`Rounds`). Update `Recap` to carry `Summary Summary` (drop `MapName`/`Rounds`); keep `DemoID` as-is.

### Capturing scores (`analyser/analyser.go`)

After `ParseToEnd()` (where `Rounds` is read today), read final scores and clan names:

```go
ct := parser.GameState().TeamCounterTerrorists()
t := parser.GameState().TeamTerrorists()
```

Determine the tracked side from `state.roundEndPlayers` (the final round's players, already captured with their `.Team`): if every tracked player among them is on one side, that side is `TrackedSide`; if tracked players appear on both sides, or none are present, `TrackedSide` is `""`. Then build the summary via a pure helper so the win/loss math is unit-testable without a parser:

```go
// analyser/recap.go (or analyser/summary.go)
func makeSummary(mapName string, rounds, ctScore, tScore int, ctClan, tClan, trackedSide string) Summary
```

`makeSummary` fills `TrackedScore`/`OpponentScore` from the matching side and sets `Outcome`: `"won"` if tracked > opponent, `"lost"` if tracked < opponent, `"tie"` if equal. When `trackedSide == ""` it leaves `TrackedScore`, `OpponentScore`, and `Outcome` zero/empty.

`Analyse` resolves `trackedSide` (a `"CT"`/`"T"`/`""` string) from `state.roundEndPlayers` and `state.Tracked`, then returns `Result{..., Summary: makeSummary(...)}`.

Guard against nil `TeamState` (defensive: treat missing as score 0, clan ""). Empty `roundEndPlayers` yields `TrackedSide == ""`.

### Selection (`analyser/recap.go`)

Change the signature to take the summary instead of the two scalars:

```go
func BuildRecap(insights []Insight, demoID string, summary Summary) Recap
```

Set `recap.DemoID = demoID` and `recap.Summary = summary`. The scoring, dedupe, and cap logic is otherwise unchanged.

### Debug JSON (`analyser/state.go`)

Replace the top-level `map`/`rounds` fields in `recapJSON` with a nested `summary` object so the score data has a home:

```go
type recapJSON struct {
    DemoID   string               `json:"demo_id"`
    Summary  summaryJSON          `json:"summary"`
    Headline *insightJSON         `json:"headline"`
    Public   []insightJSON        `json:"public"`
    Dropped  []droppedInsightJSON `json:"dropped"`
    Trace    []DebugEvent         `json:"trace"`
}

type summaryJSON struct {
    Map           string `json:"map"`
    Rounds        int    `json:"rounds"`
    CTScore       int    `json:"ct_score"`
    TScore        int    `json:"t_score"`
    CTClan        string `json:"ct_clan,omitempty"`
    TClan         string `json:"t_clan,omitempty"`
    TrackedSide   string `json:"tracked_side,omitempty"`
    TrackedScore  int    `json:"tracked_score"`
    OpponentScore int    `json:"opponent_score"`
    Outcome       string `json:"outcome,omitempty"`
}
```

`recapToJSON` maps `recap.Summary` into `summaryJSON`.

### Rendering (`bot/render.go`)

Replace the opening header in `FormatRecap` with a summary line built from `recap.Summary`. Keep it short and in the existing banter register. Do not use em dashes.

Header rules (degrade gracefully when fields are missing):

- Map prefix: `**{map}** ` when `MapName != ""`, otherwise start with `Match`.
- When `Summary.Outcome != ""`: append the result from the tracked team's view, for example `**de_dust2** recap: won 16:13.` (`won`/`lost`/`tied` from `Outcome`, using `TrackedScore:OpponentScore`).
- When `Outcome == ""` but a scoreline exists (`CTScore` or `TScore` non-zero): show the neutral scoreline, for example `**de_dust2** recap: 16:13 (CT:T).`
- When no scores at all: fall back to the current `**{map}** recap` / `Match recap` wording.

Then render the headline and supporting lines exactly as today.

Decision: render the summary header even when `recap.Public` is empty, as long as the summary has data (map name set or a non-zero scoreline), so a quiet match still posts its result. Return `nil` only when both `Public` is empty and the summary is empty. (Today `FormatRecap` returns `nil` on empty `Public`; this is the one behavioural change.)

`FormatRecapDebug`: prepend a single summary line (map, scoreline, outcome) above the existing `Headline:` line.

## Integration Touchpoints

- `analyser/events.go` - add `Summary`; replace `MapName`/`Rounds` on `Result` and `Recap` with `Summary Summary`.
- `analyser/analyser.go` - read final CT/T scores and clan names after `ParseToEnd`; resolve tracked side from `state.roundEndPlayers`; populate `Result.Summary` via `makeSummary`.
- `analyser/recap.go` - `BuildRecap(insights, demoID, summary)`; add `makeSummary`.
- `analyser/state.go` - `recapJSON` gains a nested `summaryJSON`; `recapToJSON` maps it.
- `bot/render.go` - summary header in `FormatRecap`; summary line in `FormatRecapDebug`.
- `bot/bot.go` - `FormatInsights` / `PostInsights` take `(demoID string, summary analyser.Summary)` instead of `(demoID, mapName string, rounds int)`.
- `cmd/analyse.go` - pass `result.Summary` to `BuildRecap` and `PostInsights`; logging that referenced `result.MapName`/`result.Rounds` reads `result.Summary.*`.
- `bot/pipeline.go` - pass `result.Summary` to `PostInsights`.

`db/` is unchanged.

## Testing

Match the constructed-input style in `analyser/recap_test.go` and `bot/render_test.go`. No real demo fixtures.

- `makeSummary`: tracked side won / lost / tie sets the right `Outcome`, `TrackedScore`, `OpponentScore`; `trackedSide == ""` leaves outcome empty and scores zero.
- `BuildRecap`: still enforces caps and drop reasons with the new signature; `recap.Summary` round-trips.
- Golden recap JSON (`TestWriteRecapLog_goldenJSON`): update to the nested `summary` object; assert `"ct_score"`, `"t_score"`, and `"outcome"` are present alongside the existing `"map"`/`"rounds"` fragments.
- `FormatRecap`: renders the outcome scoreline when `Summary.Outcome` is set; renders the neutral scoreline when only scores are present; falls back to plain wording when scores are absent; renders a summary-only message when `Public` is empty but the summary has data.
- `FormatRecapDebug`: includes the summary line.

## Acceptance Criteria

- Final team scores and clan names are captured from the parser; the tracked team's outcome is derived from the final round's player teams.
- The recap message opens with a game summary line (map, scoreline, won/lost/tie) above the player insights, degrading gracefully when data is missing.
- `<demo>.recap.json` contains a `summary` object with scores and outcome; `analyse -debug` prints the summary in the console output.
- A quiet match (no public moments) still posts its game summary.
- No new CLI flag, config subsystem, or DB schema change is introduced; em dashes are not used in rendered copy.

## Implementation Order

1. Add the `Summary` type and swap `MapName`/`Rounds` for `Summary` on `Result` and `Recap` (`analyser/events.go`).
2. Capture scores and tracked side in `Analyse`; add `makeSummary` (`analyser/analyser.go`, `analyser/recap.go`).
3. Update `BuildRecap` signature and `recapJSON`/`recapToJSON` (`analyser/recap.go`, `analyser/state.go`).
4. Update `bot` signatures and render the summary header (`bot/bot.go`, `bot/render.go`).
5. Update callers (`cmd/analyse.go`, `bot/pipeline.go`).
6. Update and add tests; run `go build ./...` and `go test ./...`.

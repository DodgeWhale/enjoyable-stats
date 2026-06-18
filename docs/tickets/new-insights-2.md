---
title: Add more insights to the analysis engine
status: DONE
---

## Ideas

- **Kit Dodger** - competitive demos only; CT side, had ≥$400 at `RoundFreezetimeEnd` but no defuse kit; bomb was planted that round (re-check `!HasDefuseKit()` at plant). "Had the money. Skipped the kit. Paid in full."

- **Entry Victim** - died as your team's first death the most times in the match (inverse of Entry King). "Opened the site - for the other team."

- **Bomb Mule** - died while carrying the bomb 3+ times in one match. "Reliable courier, unreliable survivor."

- **Instant Trade** - killed an enemy within 3s of them killing your teammate, 3+ times in one match. "Refrag speed: professional."

- **Flash Tax** - blinded by enemy flashes (`Attacker.Team != Player.Team`, `FlashDuration() >= 1s`) the most in the match, or 8+ qualifying blinds. "Consider playing anti-flash next match."

- **Economy Terrorist** - `MoneySpentThisRound() >= 4500` on a round where ≥3 teammates have `EquipmentValueFreezeTimeEnd() < 2000`. "Single-handedly wrecked the team economy."

- **Defuse Interrupted** - killed while defusing 2+ times in one match. "Almost had it. Twice."

- **Knife Kill** - tracked player kills an enemy with a knife. "Brought a knife to a gunfight. Somehow it worked."

- **Knife Team Kill** - tracked player team-kills with a knife. "Backstabbed a teammate. Peak teamwork."

### Bomb God message (existing insight)

- **TriggerType:** `bomb_god` (no trigger logic change — threshold stays 3+ objective rounds)
- **Discord (updated):** `"Actually played the objective unlike everyone else. (%d plants, %d defuses)"`

**Problem:** Discord line has no counts; other insights include context in parentheses.

**State fields (add):**
- `bombPlants map[uint64]int` — match-level; increment on each `BombPlanted` for tracked player
- `bombDefuses map[uint64]int` — match-level; increment on each `BombDefused` for tracked player

Keep existing `bombObjectiveRounds` for the 3-round threshold (deduped per round). Plants and defuses are separate totals — a player with 2 plants and 1 defuse in 3 rounds shows `(2 plants, 1 defuse)`.

**Logic:**
1. In existing `BombPlanted` handler: after `recordBombObjective`, increment `state.bombPlants[player.SteamID64]`.
2. In existing `BombDefused` handler: after `recordBombObjective`, increment `state.bombDefuses[player.SteamID64]`.
3. `BombGod` trigger `Detail`: add `"plants"` and `"defuses"` alongside existing `"rounds"` / `"bomb_rounds"`.
4. `bot/bot.go` `formatInsight` case `bomb_god`: read counts from `Detail` and append `(X plants, Y defuses)`.

**Tests:** extend `TestBombGod_firesWhenCrossingThreeObjectiveRounds` to assert `Detail["plants"]` / `Detail["defuses"]`; add unit test or handler test if plant/defuse increments live in analyser.

## Implementation

### Handoff context

This ticket adds **9 new triggers** to the existing analysis engine. Read `docs/plan-phase-one.md` (analyser section) and `docs/tickets/new-insights.md` (completed prior batch) for background.

**Architecture (do not reinvent):**
- `analyser/analyser.go` — demo parser dispatcher. Registers demoinfocs event handlers, owns shared `State`, fans out to triggers. All handlers skip warmup via `parser.GameState().IsWarmupPeriod()`.
- `analyser/state.go` — per-round and per-match counters. `ResetRound()` clears round-scoped maps; match-level maps (e.g. `firstKills`, `bombObjectiveRounds`) persist across rounds.
- `analyser/triggers.go` — one struct per insight, implementing capability interfaces (`KillHook`, `RoundEndHook`, `MatchEndHook`, etc.). Triggers read state; they do not register parser handlers themselves.
- `analyser/events.go` — `Insight{SteamID, TriggerType, Round, Detail}`.
- `bot/bot.go` — `formatInsight()` switch maps `TriggerType` → Discord message string.
- `analyser/triggers_test.go` — unit tests construct `State` + event, call trigger hook directly (no demo parse).

**Conventions:**
- `TriggerType` values are snake_case (e.g. `entry_victim`, `kit_dodger`).
- Only **tracked** players (`state.Tracked`) emit insights.
- Round-level insights set `Round` to the current round; match-level insights (Entry Victim, Instant Trade, Flash Tax) use `s.Round` at match end (same as Entry King).
- `Detail` map holds structured extras for Discord formatting and debugging.
- Register new triggers in `analyser.New()` trigger slice.
- Add table-driven or focused unit tests per trigger (positive + negative cases), mirroring existing tests.

**Files to touch:**
| File | Changes |
|---|---|
| `analyser/state.go` | New match/round state fields; extend `ResetRound()` |
| `analyser/analyser.go` | Register new demoinfocs handlers; update shared state before fanout |
| `analyser/triggers.go` | 9 new trigger structs |
| `analyser/triggers_test.go` | Tests for each trigger |
| `bot/bot.go` | `formatInsight()` cases + emoji lines from Ideas section |

**New demoinfocs events to register** (none exist in analyser today):
- `events.RoundFreezetimeEnd` — Kit Dodger snapshot, Economy Terrorist eco count
- `events.PlayerFlashed` — Flash Tax counting
- `events.BombPickup` / `events.BombDropped` — Bomb Mule carrier tracking
- `events.BombDefuseStart` / `events.BombDefuseAborted` / `events.BombDefused` — Defuse Interrupted

**Existing events to extend:**
- `events.Kill` — add `firstDeaths` tally (Entry Victim), bomb-carrier death check (Bomb Mule), teammate-death log + trade detection (Instant Trade), defuser death check (Defuse Interrupted). Set `state.currentTime = parser.CurrentTime()` at top of handler for timestamp-based logic.
- `events.BombPlanted` — Kit Dodger fire (already has handler for Bomb God)

**Suggested implementation order:** Entry Victim → Instant Trade → Bomb Mule → Defuse Interrupted → Flash Tax → Kit Dodger → Economy Terrorist → Knife Kill → Knife Team Kill.

**Verification:** Run `go test ./analyser/...`. Optionally parse `demos/003825697182816665754_1712708442.dem` with `--debug` and inspect state log.

---

### Kit Dodger
- **TriggerType:** `kit_dodger`
- **Type:** round-level lowlight
- **Discord:** `"Had the money. Skipped the kit. Paid in full."`

**Scope:** competitive matchmaking demos only. We assume all ingested demos are comp — no game-mode detection needed. In comp, `RoundFreezetimeEnd` = buy end.

**State fields:**
- `kitDodgerCandidates map[uint64]bool` — tracked CTs snapshotted at freezetime end (round-scoped; reset in `ResetRound()`)

**Logic:**
1. On `RoundFreezetimeEnd`: for each tracked player on `common.TeamCounterTerrorists` where `Money() >= 400 && !HasDefuseKit()`, set `kitDodgerCandidates[id] = true`.
2. On `BombPlanted`: for each id in `kitDodgerCandidates`, if player still has `!HasDefuseKit()`, emit insight. Use `BombPlantedHook` or inline check in existing `BombPlanted` handler after `recordBomb`.
3. Clear candidates after firing (or on `ResetRound`).

**Detail:** `{ "round": N }` (round is already on Insight; optional).

**Edge cases:**
- Player buys/picks up kit after snapshot → `HasDefuseKit()` at plant suppresses fire (correct).
- Bomb not planted → no fire (correct).
- T-side tracked players → never snapshotted.

---

### Entry Victim
- **TriggerType:** `entry_victim`
- **Type:** match-level lowlight
- **Discord:** `"Opened the site - for the other team."`

**State fields:**
- `firstDeaths map[uint64]int` — match-level; do NOT reset in `ResetRound()`

**Logic:**
1. In `analyser.go` `Kill` handler: mirror existing `firstKills` logic. When `!state.roundHasKill` and victim is non-nil, increment `state.firstDeaths[e.Victim.SteamID64]`. Do this alongside the existing first-kill increment (first kill and first death are the same event).
2. `EntryVictim` trigger implements `MatchEndHook`: identical structure to `EntryKing` but reads `firstDeaths`. Only consider tracked players. Emit for player(s) with max count; require `max > 0`.

**Detail:** `{ "first_deaths": N }`

**Edge cases:**
- Team kill as first death counts — consistent with Entry King not filtering team kills.
- Ties among tracked players → emit for all tied (same as Entry King).

---

### Bomb Mule
- **TriggerType:** `bomb_mule`
- **Type:** match-level lowlight (threshold)
- **Discord:** `"Reliable courier, unreliable survivor."`

**State fields:**
- `bombCarrier uint64` — steam ID of current C4 carrier (0 if dropped/planted)
- `bombMuleDeaths map[uint64]int` — match-level death count while carrying

**Logic:**
1. On `BombPickup`: set `bombCarrier = e.Player.SteamID64`.
2. On `BombDropped`: if `e.Player != nil`, check if dropped due to death (carrier matches victim on same tick) — simpler approach: on `Kill`, before updating carrier, if `e.Victim.SteamID64 == state.bombCarrier` and tracked, increment `bombMuleDeaths[victim]`. Then let drop event clear carrier.
3. On `BombPlanted`: set `bombCarrier = 0`.
4. `BombMule` implements `MatchEndHook`: for tracked players with `bombMuleDeaths[id] >= 3`, emit once.

**Alternative (recommended):** on `Kill`, check `e.Victim` had C4 in inventory via iterating `e.Victim.Weapons()` for `common.EqBomb`, OR `state.bombCarrier == e.Victim.SteamID64`. Maintain carrier via pickup/drop/plant events regardless.

**Detail:** `{ "deaths": N }`

**Edge cases:**
- Post-plant deaths don't count (carrier cleared on plant).
- Intentional drop then death → only counts if still carrier at death tick.

---

### Instant Trade
- **TriggerType:** `instant_trade`
- **Type:** match-level highlight (threshold)
- **Discord:** `"Refrag speed: professional."`

**State fields:**
- `currentTime time.Duration` — set from `parser.CurrentTime()` each kill handler invocation
- `recentTeammateDeaths []teammateDeath` — slice of `{enemyKiller uint64, at time.Duration}`; prune entries older than 3s on each kill
- `instantTrades map[uint64]int` — match-level count per tracked player

**Logic:**
1. On `Kill` where `e.Victim != nil && e.Killer != nil && e.Killer.Team != e.Victim.Team` (enemy kill): if victim is on a tracked player's team (or any teammate of a tracked player), append `{enemyKiller: e.Killer.SteamID64, at: state.currentTime}`.
2. On same `Kill` where killer is tracked: prune deaths where `currentTime - death.at > 3s`. If `e.Victim.SteamID64` matches any recent `enemyKiller`, increment `instantTrades[killer]`.
3. `InstantTrade` `MatchEndHook`: emit for tracked players with `instantTrades[id] >= 3`.

**Detail:** `{ "trades": N }`

**Edge cases:**
- Same enemy kills two teammates; one trade counts once per kill event (correct).
- Killer/victim nil (world damage) → skip.
- No proximity check by design.

---

### Flash Tax
- **TriggerType:** `flash_tax`
- **Type:** match-level lowlight
- **Discord:** `"Consider playing anti-flash next match."`

**State fields:**
- `flashBlinds map[uint64]int` — match-level qualifying blind count

**Logic:**
1. Register `events.PlayerFlashed` handler. demoinfocs delays this event until `FlashDuration` is populated — safe to read `e.FlashDuration()` immediately.
2. Qualifying blind: `e.Player != nil`, tracked, `e.Attacker != nil`, `e.Attacker.Team != e.Player.Team`, `e.FlashDuration() >= 1*time.Second`.
3. Increment `flashBlinds[player.SteamID64]`.
4. `FlashTax` `MatchEndHook`: find max among tracked players. Emit if (a) player has max AND max is strictly greater than all other tracked players, OR (b) player has `>= 8` blinds (fallback when no clear leader or solo tracked player). If multiple tracked tie for max, emit for all tied.

**Detail:** `{ "blinds": N }`

**Edge cases:**
- Team flashes excluded by team check.
- Sub-1s blinds ignored.

---

### Economy Terrorist
- **TriggerType:** `economy_terrorist`
- **Type:** round-level lowlight
- **Discord:** `"Single-handedly wrecked the team economy."`

**State fields:**
- `ecoTeammateCount map[common.Team]int` — teammates (excluding self) with `EquipmentValueFreezeTimeEnd() < 2000`, snapshotted at freezetime end (round-scoped; reset in `ResetRound()`)

**Logic:**
1. On `RoundFreezetimeEnd`: for each team, count players where `EquipmentValueFreezeTimeEnd() < 2000`. Store per-team count in `ecoTeammateCount[team]`. When evaluating a tracked player, use `ecoTeammateCount[player.Team] - 1` to exclude self (or count explicitly excluding the player).
2. On `RoundEnd`: for each tracked player where `MoneySpentThisRound() >= 4500` AND eco teammate count (excluding self) `>= 3`, emit insight.

**Detail:** `{ "spent": N, "eco_teammates": N }`

**Edge cases:**
- Player saved gear from prior round → low `MoneySpentThisRound()` despite full loadout → won't fire (acceptable).
- Exactly 3 eco teammates required, not "most of team".
- Count teammates only, not enemies.

---

### Defuse Interrupted
- **TriggerType:** `defuse_interrupted`
- **Type:** match-level lowlight (threshold)
- **Discord:** `"Almost had it. Twice."`

**State fields:**
- `activeDefusers map[uint64]bool` — players currently defusing (round-scoped)
- `defuseInterrupted map[uint64]int` — match-level count

**Logic:**
1. On `BombDefuseStart`: set `activeDefusers[e.Player.SteamID64] = true`.
2. On `BombDefuseAborted` or `BombDefused`: delete from `activeDefusers`.
3. On `Kill`: if `e.Victim != nil && activeDefusers[e.Victim.SteamID64]` and tracked, increment `defuseInterrupted[victim]`, delete from active set.
4. `DefuseInterrupted` `MatchEndHook`: emit for tracked players with count `>= 2`.

**Detail:** `{ "interruptions": N }`

**Edge cases:**
- Voluntary abort then death later → not in active set, won't fire (correct).
- Dying same tick defuse completes → rare ordering issue; acceptable.

---

### Knife Kill
- **TriggerType:** `knife_kill`
- **Type:** round-level highlight
- **Discord:** `"Brought a knife to a gunfight. Somehow it worked."`

**State fields:** none (pure `KillHook`, same pattern as `TeamKill` / `Ace`).

**Logic:**
1. `KnifeKill` implements `KillHook`.
2. On `Kill` where `e.Killer != nil`, `e.Victim != nil`, killer is tracked, `e.Killer.Team != e.Victim.Team`, and `e.Weapon.Type == common.EqKnife`, emit insight.

**Detail:** `{ "victim": "<steam_id>" }`

**Edge cases:**
- Killer/victim nil → skip.
- Self-damage / suicide (`Killer.SteamID64 == Victim.SteamID64`) → skip.
- Zeus (`EqZeus`) → not a knife; skip.
- All knife skins map to `EqKnife` in demoinfocs.

---

### Knife Team Kill
- **TriggerType:** `knife_team_kill`
- **Type:** round-level lowlight
- **Discord:** `"Backstabbed a teammate. Peak teamwork."`

**State fields:** none (pure `KillHook`).

**Logic:**
1. `KnifeTeamKill` implements `KillHook`.
2. On `Kill` where `e.Killer != nil`, `e.Victim != nil`, killer is tracked, `e.Killer.Team == e.Victim.Team`, `e.Killer.SteamID64 != e.Victim.SteamID64`, and `e.Weapon.Type == common.EqKnife`, emit insight.

**Detail:** `{ "victim": "<steam_id>" }`

**Edge cases:**
- May co-fire with existing `team_kill` on the same event (both are round-level; `team_kill` includes weapon in detail). Do not change `TeamKill` behaviour.
- Friendly fire from non-tracked players → skip (tracked killer only).

---

### Discord formatting (`bot/bot.go`)

Add cases to `formatInsight()`:

| TriggerType | Suggested line |
|---|---|
| `kit_dodger` | `💸 Had the money. Skipped the kit. Paid in full. (round %d)` |
| `entry_victim` | `🚪 Opened the site - for the other team. (%d first deaths)` |
| `bomb_mule` | `💣 Reliable courier, unreliable survivor. (%d bomb deaths)` |
| `instant_trade` | `⚡ Refrag speed: professional. (%d instant trades)` |
| `flash_tax` | `😵 Consider playing anti-flash next match. (%d blinds)` |
| `economy_terrorist` | `💸 Single-handedly wrecked the team economy. (round %d)` |
| `defuse_interrupted` | `🔧 Almost had it. Twice.` (or include count if >2) |
| `bomb_god` | `💣 Actually played the objective unlike everyone else. (%d plants, %d defuses)` — see **Bomb God message** above |
| `knife_kill` | `🔪 Brought a knife to a gunfight. Somehow it worked. (round %d)` |
| `knife_team_kill` | `🔪 Backstabbed a teammate. Peak teamwork. (round %d)` |

Exact wording can match Ideas section quotes; include `Detail` counts where useful.

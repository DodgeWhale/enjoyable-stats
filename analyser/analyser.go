package analyser

import (
	"fmt"
	"os"
	"strconv"

	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
	common "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
)

type Analyser struct {
	triggers []Trigger
}

func New() *Analyser {
	return &Analyser{
		triggers: []Trigger{
			TeamKill{}, Ace{}, Clutch{}, MVP{},
			LurkerTax{}, BombGod{}, EntryKing{}, RefundRequest{},
		},
	}
}

func (a *Analyser) Analyse(path string, tracked map[string]bool, debug bool) ([]Insight, []StateSnapshot, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("analyser: open demo: %w", err)
	}
	defer f.Close()

	parser := demoinfocs.NewParser(f)
	defer parser.Close()

	state := State{
		Tracked:             make(map[uint64]bool, len(tracked)),
		kills:               make(map[uint64]int),
		mvps:                make(map[uint64]int),
		prevMVPs:            make(map[uint64]int),
		mvpRounds:           make(map[uint64][]int),
		alive:               make(map[common.Team]int),
		firstKills:          make(map[uint64]int),
		bombObjectiveRounds: make(map[uint64][]int),
		prevBombGod:         make(map[uint64]int),
	}
	for id := range tracked {
		uid, err := strconv.ParseUint(id, 10, 64)
		if err == nil {
			state.Tracked[uid] = true
		}
	}

	var insights []Insight
	var stateLog []StateSnapshot
	record := func(event string) {
		if debug {
			stateLog = append(stateLog, state.snapshot(event))
		}
	}

	parser.RegisterEventHandler(func(e events.RoundStart) {
		if parser.GameState().IsWarmupPeriod() {
			return
		}
		state.Round = parser.GameState().TotalRoundsPlayed()
		state.ResetRound(parser.GameState().Participants().Playing())
		for _, t := range a.triggers {
			if h, ok := t.(RoundStartHook); ok {
				h.OnRoundStart(&state)
			}
		}
		record("round_start")
	})

	parser.RegisterEventHandler(func(e events.Kill) {
		if parser.GameState().IsWarmupPeriod() {
			return
		}
		state.Round = parser.GameState().TotalRoundsPlayed()

		if e.Killer != nil {
			if !state.roundHasKill {
				state.roundHasKill = true
				state.firstKills[e.Killer.SteamID64]++
			}
			if state.Tracked[e.Killer.SteamID64] {
				state.kills[e.Killer.SteamID64]++
				if isAWP(e.Weapon) {
					if purchaseID, ok := state.awpPurchaseWeapon[e.Killer.SteamID64]; ok && e.Weapon != nil && e.Weapon.UniqueID2() == purchaseID {
						state.awpKillWithOwn[e.Killer.SteamID64] = true
					}
				}
			}
		}

		for _, t := range a.triggers {
			if h, ok := t.(KillHook); ok {
				insights = append(insights, h.OnKill(&state, e)...)
			}
		}

		if e.Victim == nil {
			record("kill")
			return
		}
		if state.Tracked[e.Victim.SteamID64] {
			state.diedThisRound[e.Victim.SteamID64] = true
		}
		state.alive[e.Victim.Team]--
		if state.alive[e.Victim.Team] != 1 {
			record("kill")
			return
		}
		var enemyTeam common.Team
		if e.Victim.Team == common.TeamTerrorists {
			enemyTeam = common.TeamCounterTerrorists
		} else {
			enemyTeam = common.TeamTerrorists
		}
		for _, p := range parser.GameState().Participants().Playing() {
			if p.Team == e.Victim.Team && p.SteamID64 != e.Victim.SteamID64 {
				state.clutcher = p.SteamID64
				state.clutchTeam = p.Team
				state.clutchVsEnemies = state.alive[enemyTeam]
				break
			}
		}
		record("kill")
	})

	parser.RegisterEventHandler(func(e events.ItemPickup) {
		if parser.GameState().IsWarmupPeriod() {
			return
		}
		state.Round = parser.GameState().TotalRoundsPlayed()
		if e.Player == nil || !state.Tracked[e.Player.SteamID64] || !isAWP(e.Weapon) {
			return
		}
		if !parser.GameState().IsFreezetimePeriod() && !e.Player.IsInBuyZone() {
			return
		}
		state.awpPurchaseWeapon[e.Player.SteamID64] = e.Weapon.UniqueID2()
		record("item_pickup")
	})

	recordBomb := func(player *common.Player) {
		if player == nil || !state.Tracked[player.SteamID64] {
			return
		}
		state.recordBombObjective(player.SteamID64)
	}

	parser.RegisterEventHandler(func(e events.BombPlanted) {
		if parser.GameState().IsWarmupPeriod() {
			return
		}
		state.Round = parser.GameState().TotalRoundsPlayed()
		recordBomb(e.Player)
		record("bomb_planted")
	})

	parser.RegisterEventHandler(func(e events.BombDefused) {
		if parser.GameState().IsWarmupPeriod() {
			return
		}
		state.Round = parser.GameState().TotalRoundsPlayed()
		recordBomb(e.Player)
		record("bomb_defused")
	})

	parser.RegisterEventHandler(func(e events.RoundEnd) {
		if parser.GameState().IsWarmupPeriod() {
			return
		}
		state.Round = parser.GameState().TotalRoundsPlayed()
		for _, p := range parser.GameState().Participants().Playing() {
			if !state.Tracked[p.SteamID64] {
				continue
			}
			prev := state.mvps[p.SteamID64]
			state.prevMVPs[p.SteamID64] = prev
			state.mvps[p.SteamID64] = p.MVPs()
			if p.MVPs() > prev {
				state.mvpRounds[p.SteamID64] = append(state.mvpRounds[p.SteamID64], state.Round)
			}
		}
		for _, t := range a.triggers {
			if h, ok := t.(RoundEndHook); ok {
				insights = append(insights, h.OnRoundEnd(&state, e)...)
			}
		}
		for id := range state.Tracked {
			state.prevBombGod[id] = state.bombObjectiveCount(id)
		}
		record("round_end")
	})

	if err := parser.ParseToEnd(); err != nil {
		return nil, nil, fmt.Errorf("analyser: parse: %w", err)
	}

	for _, t := range a.triggers {
		if h, ok := t.(MatchEndHook); ok {
			insights = append(insights, h.OnMatchEnd(&state)...)
		}
	}

	return insights, stateLog, nil
}

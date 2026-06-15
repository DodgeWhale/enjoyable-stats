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
		triggers: []Trigger{TeamKill{}, Ace{}, Clutch{}, MVP{}},
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
		Tracked:  make(map[uint64]bool, len(tracked)),
		kills:    make(map[uint64]int),
		mvps:     make(map[uint64]int),
		prevMVPs: make(map[uint64]int),
		alive:    make(map[common.Team]int),
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

		if e.Killer != nil && state.Tracked[e.Killer.SteamID64] {
			state.kills[e.Killer.SteamID64]++
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
		}
		for _, t := range a.triggers {
			if h, ok := t.(RoundEndHook); ok {
				insights = append(insights, h.OnRoundEnd(&state, e)...)
			}
		}
		record("round_end")
	})

	if err := parser.ParseToEnd(); err != nil {
		return nil, nil, fmt.Errorf("analyser: parse: %w", err)
	}

	return insights, stateLog, nil
}

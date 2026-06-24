package analyser

import (
	"fmt"
	"os"
	"strconv"
	"time"

	demoinfocs "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
	common "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/msg"
)

type Analyser struct {
	triggers []Trigger
}

func New() *Analyser {
	return &Analyser{
		triggers: []Trigger{
			TeamKill{}, Ace{}, MultiKill{}, Clutch{}, MVP{},
			LurkerTax{}, BombGod{}, EntryKing{}, RefundRequest{},
			EntryVictim{}, InstantTrade{}, BombMule{}, DefuseInterrupted{},
			FlashTax{}, FlashAssist{}, TeamFlash{}, KitDodger{}, EconomyTerrorist{},
			KnifeKill{}, KnifeTeamKill{}, ZeusKill{}, StyleKill{},
		},
	}
}

func (a *Analyser) Analyse(path string, tracked map[string]bool, debug bool) (Result, error) {
	f, err := os.Open(path)
	if err != nil {
		return Result{}, fmt.Errorf("analyser: open demo: %w", err)
	}
	defer f.Close()

	parser := demoinfocs.NewParser(f)
	defer parser.Close()

	state := State{
		Tracked:             make(map[uint64]bool, len(tracked)),
		names:               make(map[uint64]string),
		kills:               make(map[uint64]int),
		mvps:                make(map[uint64]int),
		prevMVPs:            make(map[uint64]int),
		mvpRounds:           make(map[uint64][]int),
		alive:               make(map[common.Team]int),
		firstKills:          make(map[uint64]int),
		firstDeaths:         make(map[uint64]int),
		bombObjectiveRounds: make(map[uint64][]int),
		bombPlants:          make(map[uint64]int),
		bombDefuses:         make(map[uint64]int),
		prevBombGod:         make(map[uint64]int),
		bombMuleDeaths:      make(map[uint64]int),
		instantTrades:       make(map[uint64]int),
		flashBlinds:         make(map[uint64]int),
		teamFlashBlinds:     make(map[uint64]int),
		recentEnemyFlashes:  make(map[uint64]recentFlash),
		defuseInterrupted:   make(map[uint64]int),
	}
	for id := range tracked {
		uid, err := strconv.ParseUint(id, 10, 64)
		if err == nil {
			state.Tracked[uid] = true
		}
	}

	var insights []Insight
	var stateLog []StateSnapshot
	var mapName string
	record := func(event string) {
		if debug {
			stateLog = append(stateLog, state.snapshot(event))
		}
	}

	parser.RegisterNetMessageHandler(func(m *msg.CDemoFileHeader) {
		mapName = m.GetMapName()
	})

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
		state.currentTime = parser.CurrentTime()

		if e.Killer != nil {
			state.recordName(e.Killer)
		}
		if e.Victim != nil {
			state.recordName(e.Victim)
		}

		if e.Killer != nil {
			if !state.roundHasKill {
				state.roundHasKill = true
				state.firstKills[e.Killer.SteamID64]++
				if e.Victim != nil {
					state.firstDeaths[e.Victim.SteamID64]++
				}
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

		recordInstantTrade(&state, e, parser.GameState().Participants().Playing())
		recordBombMuleDeath(&state, e)
		recordDefuseInterrupted(&state, e)

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

	parser.RegisterEventHandler(func(e events.RoundFreezetimeEnd) {
		if parser.GameState().IsWarmupPeriod() {
			return
		}
		state.Round = parser.GameState().TotalRoundsPlayed()
		playing := parser.GameState().Participants().Playing()
		state.ecoTeammateCount = make(map[common.Team]int)
		for _, p := range playing {
			if p.EquipmentValueFreezeTimeEnd() < 2000 {
				state.ecoTeammateCount[p.Team]++
			}
		}
		for _, p := range playing {
			if !state.Tracked[p.SteamID64] {
				continue
			}
			if p.Team != common.TeamCounterTerrorists {
				continue
			}
			if p.Money() >= 400 && !p.HasDefuseKit() {
				state.kitDodgerCandidates[p.SteamID64] = true
			}
		}
		record("round_freezetime_end")
	})

	parser.RegisterEventHandler(func(e events.PlayerFlashed) {
		if parser.GameState().IsWarmupPeriod() {
			return
		}
		state.Round = parser.GameState().TotalRoundsPlayed()
		state.currentTime = parser.CurrentTime()
		if e.Player == nil || e.Attacker == nil {
			return
		}
		state.recordName(e.Player)
		state.recordName(e.Attacker)

		if e.Attacker.Team != e.Player.Team && e.FlashDuration() >= time.Second {
			state.recentEnemyFlashes[e.Player.SteamID64] = recentFlash{
				flasher: e.Attacker.SteamID64,
				at:      state.currentTime,
			}
		}

		if state.Tracked[e.Attacker.SteamID64] && e.Attacker.Team == e.Player.Team && e.FlashDuration() >= time.Second {
			state.teamFlashBlinds[e.Attacker.SteamID64]++
		}

		if !state.Tracked[e.Player.SteamID64] {
			return
		}
		if e.Attacker.Team == e.Player.Team {
			return
		}
		if e.FlashDuration() < time.Second {
			return
		}
		state.flashBlinds[e.Player.SteamID64]++
		record("player_flashed")
	})

	parser.RegisterEventHandler(func(e events.BombPickup) {
		if parser.GameState().IsWarmupPeriod() {
			return
		}
		state.Round = parser.GameState().TotalRoundsPlayed()
		if e.Player != nil {
			state.bombCarrier = e.Player.SteamID64
		}
		record("bomb_pickup")
	})

	parser.RegisterEventHandler(func(e events.BombDropped) {
		if parser.GameState().IsWarmupPeriod() {
			return
		}
		state.Round = parser.GameState().TotalRoundsPlayed()
		if e.Player != nil && e.Player.SteamID64 == state.bombCarrier {
			state.bombCarrier = 0
		}
		record("bomb_dropped")
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
		state.bombCarrier = 0
		if e.Player != nil {
			state.recordName(e.Player)
		}
		if e.Player != nil && state.Tracked[e.Player.SteamID64] {
			state.bombPlants[e.Player.SteamID64]++
		}
		recordBomb(e.Player)
		for _, t := range a.triggers {
			if h, ok := t.(BombPlantedHook); ok {
				insights = append(insights, h.OnBombPlanted(&state, parser.GameState().Participants().Playing())...)
			}
		}
		record("bomb_planted")
	})

	parser.RegisterEventHandler(func(e events.BombDefuseStart) {
		if parser.GameState().IsWarmupPeriod() {
			return
		}
		state.Round = parser.GameState().TotalRoundsPlayed()
		if e.Player != nil {
			state.activeDefusers[e.Player.SteamID64] = true
		}
		record("bomb_defuse_start")
	})

	parser.RegisterEventHandler(func(e events.BombDefuseAborted) {
		if parser.GameState().IsWarmupPeriod() {
			return
		}
		state.Round = parser.GameState().TotalRoundsPlayed()
		if e.Player != nil {
			delete(state.activeDefusers, e.Player.SteamID64)
		}
		record("bomb_defuse_aborted")
	})

	parser.RegisterEventHandler(func(e events.BombDefused) {
		if parser.GameState().IsWarmupPeriod() {
			return
		}
		state.Round = parser.GameState().TotalRoundsPlayed()
		if e.Player != nil {
			delete(state.activeDefusers, e.Player.SteamID64)
			if state.Tracked[e.Player.SteamID64] {
				state.bombDefuses[e.Player.SteamID64]++
			}
		}
		recordBomb(e.Player)
		record("bomb_defused")
	})

	parser.RegisterEventHandler(func(e events.AnnouncementLastRoundHalf) {
		if parser.GameState().IsWarmupPeriod() {
			return
		}
		state.markLastRoundOfFirstHalf()
	})

	parser.RegisterEventHandler(func(e events.RoundEnd) {
		if parser.GameState().IsWarmupPeriod() {
			return
		}
		state.Round = parser.GameState().TotalRoundsPlayed()
		if state.lastRoundOfFirstHalf {
			ctScore, tScore := 0, 0
			if ct := parser.GameState().TeamCounterTerrorists(); ct != nil {
				ctScore = ct.Score()
			}
			if t := parser.GameState().TeamTerrorists(); t != nil {
				tScore = t.Score()
			}
			state.finishFirstHalf(ctScore, tScore)
		}
		state.roundEndPlayers = parser.GameState().Participants().Playing()
		for _, p := range state.roundEndPlayers {
			state.recordName(p)
		}
		state.roundMoneySpent = make(map[uint64]int)
		for _, p := range state.roundEndPlayers {
			if !state.Tracked[p.SteamID64] {
				continue
			}
			state.roundMoneySpent[p.SteamID64] = p.MoneySpentThisRound()
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
		return Result{}, fmt.Errorf("analyser: parse: %w", err)
	}

	for _, t := range a.triggers {
		if h, ok := t.(MatchEndHook); ok {
			insights = append(insights, h.OnMatchEnd(&state)...)
		}
	}

	nameTrace := enrichInsights(insights, state.names)

	ctScore, tScore := 0, 0
	ctClan, tClan := "", ""
	if ct := parser.GameState().TeamCounterTerrorists(); ct != nil {
		ctScore = ct.Score()
		ctClan = ct.ClanName()
	}
	if t := parser.GameState().TeamTerrorists(); t != nil {
		tScore = t.Score()
		tClan = t.ClanName()
	}

	firstHalfCT, firstHalfT, secondHalfCT, secondHalfT := state.halfScores(ctScore, tScore)
	summary := makeSummary(
		mapName,
		parser.GameState().TotalRoundsPlayed(),
		ctScore, tScore,
		ctClan, tClan,
		trackedSide(&state),
		firstHalfCT, firstHalfT, secondHalfCT, secondHalfT,
	)

	return Result{
		Insights:  insights,
		Summary:   summary,
		StateLog:  stateLog,
		NameTrace: nameTrace,
	}, nil
}

// trackedSide returns the final side ("CT" or "T") that the tracked players are
// on at match end, or "" when tracked players span both sides or none are
// present in the final round.
func trackedSide(s *State) string {
	ctTracked, tTracked := false, false
	for _, p := range s.roundEndPlayers {
		if p == nil || !s.Tracked[p.SteamID64] {
			continue
		}
		switch p.Team {
		case common.TeamCounterTerrorists:
			ctTracked = true
		case common.TeamTerrorists:
			tTracked = true
		}
	}
	switch {
	case ctTracked && !tTracked:
		return "CT"
	case tTracked && !ctTracked:
		return "T"
	default:
		return ""
	}
}

const instantTradeWindow = 3 * time.Second
const flashAssistWindow = 5 * time.Second

func teamHasTrackedPlayer(s *State, team common.Team, playing []*common.Player) bool {
	for _, p := range playing {
		if p.Team == team && s.Tracked[p.SteamID64] {
			return true
		}
	}
	return false
}

func recordInstantTrade(s *State, e events.Kill, playing []*common.Player) {
	if e.Victim == nil || e.Killer == nil || e.Killer.Team == e.Victim.Team {
		return
	}
	if teamHasTrackedPlayer(s, e.Victim.Team, playing) {
		s.recentTeammateDeaths = append(s.recentTeammateDeaths, teammateDeath{
			enemyKiller: e.Killer.SteamID64,
			at:          s.currentTime,
		})
	}
	if !s.Tracked[e.Killer.SteamID64] {
		return
	}
	pruned := s.recentTeammateDeaths[:0]
	for _, d := range s.recentTeammateDeaths {
		if s.currentTime-d.at <= instantTradeWindow {
			pruned = append(pruned, d)
		}
	}
	s.recentTeammateDeaths = pruned
	for _, d := range s.recentTeammateDeaths {
		if d.enemyKiller == e.Victim.SteamID64 {
			s.instantTrades[e.Killer.SteamID64]++
			break
		}
	}
}

func playerHasBomb(p *common.Player) bool {
	if p == nil {
		return false
	}
	for _, w := range p.Weapons() {
		if w.Type == common.EqBomb {
			return true
		}
	}
	return false
}

func recordBombMuleDeath(s *State, e events.Kill) {
	if e.Victim == nil || !s.Tracked[e.Victim.SteamID64] {
		return
	}
	if s.bombCarrier == e.Victim.SteamID64 || playerHasBomb(e.Victim) {
		s.bombMuleDeaths[e.Victim.SteamID64]++
	}
}

func recordDefuseInterrupted(s *State, e events.Kill) {
	if e.Victim == nil || !s.activeDefusers[e.Victim.SteamID64] {
		return
	}
	if !s.Tracked[e.Victim.SteamID64] {
		delete(s.activeDefusers, e.Victim.SteamID64)
		return
	}
	s.defuseInterrupted[e.Victim.SteamID64]++
	delete(s.activeDefusers, e.Victim.SteamID64)
}

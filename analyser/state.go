package analyser

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	common "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	"github.com/oklog/ulid/v2"
)

type teammateDeath struct {
	enemyKiller uint64
	at          time.Duration
}

type State struct {
	Round   int
	Tracked map[uint64]bool

	kills           map[uint64]int
	mvps            map[uint64]int
	prevMVPs        map[uint64]int
	mvpRounds       map[uint64][]int
	alive           map[common.Team]int
	clutcher        uint64
	clutchTeam      common.Team
	clutchVsEnemies int

	roundHasKill        bool
	awpPurchaseWeapon   map[uint64]ulid.ULID
	awpKillWithOwn      map[uint64]bool
	diedThisRound       map[uint64]bool
	firstKills          map[uint64]int
	firstDeaths         map[uint64]int
	bombObjectiveRounds map[uint64][]int
	bombPlants          map[uint64]int
	bombDefuses         map[uint64]int
	prevBombGod         map[uint64]int

	bombCarrier          uint64
	bombMuleDeaths       map[uint64]int
	currentTime          time.Duration
	recentTeammateDeaths []teammateDeath
	instantTrades        map[uint64]int
	flashBlinds          map[uint64]int
	kitDodgerCandidates  map[uint64]bool
	ecoTeammateCount     map[common.Team]int
	activeDefusers       map[uint64]bool
	defuseInterrupted    map[uint64]int

	roundEndPlayers []*common.Player
	roundMoneySpent map[uint64]int
}

type StateSnapshot struct {
	Event           string         `json:"event"`
	Round           int            `json:"round"`
	Tracked         []string       `json:"tracked,omitempty"`
	Kills           map[string]int `json:"kills,omitempty"`
	MVPs            map[string]int `json:"mvps,omitempty"`
	PrevMVPs        map[string]int `json:"prev_mvps,omitempty"`
	Alive           map[string]int `json:"alive,omitempty"`
	Clutcher        string         `json:"clutcher,omitempty"`
	ClutchTeam      string         `json:"clutch_team,omitempty"`
	ClutchVsEnemies int            `json:"clutch_vs_enemies,omitempty"`
}

func (s *State) snapshot(event string) StateSnapshot {
	snap := StateSnapshot{
		Event:           event,
		Round:           s.Round,
		ClutchVsEnemies: s.clutchVsEnemies,
	}
	for id := range s.Tracked {
		snap.Tracked = append(snap.Tracked, strconv.FormatUint(id, 10))
	}
	if len(s.kills) > 0 {
		snap.Kills = steamIDMap(s.kills)
	}
	if len(s.mvps) > 0 {
		snap.MVPs = steamIDMap(s.mvps)
	}
	if len(s.prevMVPs) > 0 {
		snap.PrevMVPs = steamIDMap(s.prevMVPs)
	}
	if len(s.alive) > 0 {
		snap.Alive = teamMap(s.alive)
	}
	if s.clutcher != 0 {
		snap.Clutcher = strconv.FormatUint(s.clutcher, 10)
	}
	if s.clutchTeam != 0 {
		snap.ClutchTeam = teamName(s.clutchTeam)
	}
	return snap
}

func WriteStateLog(path string, states []StateSnapshot) error {
	data, err := json.MarshalIndent(states, "", "  ")
	if err != nil {
		return fmt.Errorf("analyser: marshal state log: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("analyser: write state log: %w", err)
	}
	return nil
}

func steamIDMap(m map[uint64]int) map[string]int {
	out := make(map[string]int, len(m))
	for id, v := range m {
		out[strconv.FormatUint(id, 10)] = v
	}
	return out
}

func teamMap(m map[common.Team]int) map[string]int {
	out := make(map[string]int, len(m))
	for team, v := range m {
		out[teamName(team)] = v
	}
	return out
}

func teamName(team common.Team) string {
	switch team {
	case common.TeamTerrorists:
		return "T"
	case common.TeamCounterTerrorists:
		return "CT"
	default:
		return fmt.Sprintf("team_%d", team)
	}
}

func (s *State) ResetRound(players []*common.Player) {
	s.kills = make(map[uint64]int)
	s.alive = make(map[common.Team]int)
	s.clutcher = 0
	s.clutchTeam = 0
	s.clutchVsEnemies = 0
	s.roundHasKill = false
	s.awpPurchaseWeapon = make(map[uint64]ulid.ULID)
	s.awpKillWithOwn = make(map[uint64]bool)
	s.diedThisRound = make(map[uint64]bool)
	s.kitDodgerCandidates = make(map[uint64]bool)
	s.ecoTeammateCount = make(map[common.Team]int)
	s.activeDefusers = make(map[uint64]bool)
	s.roundEndPlayers = nil
	s.roundMoneySpent = make(map[uint64]int)
	for _, p := range players {
		if p.IsAlive() {
			s.alive[p.Team]++
		}
	}
}

func (s *State) recordBombObjective(id uint64) {
	for _, r := range s.bombObjectiveRounds[id] {
		if r == s.Round {
			return
		}
	}
	s.bombObjectiveRounds[id] = append(s.bombObjectiveRounds[id], s.Round)
}

func (s *State) bombObjectiveCount(id uint64) int {
	return len(s.bombObjectiveRounds[id])
}

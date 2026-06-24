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

type recentFlash struct {
	flasher uint64
	at      time.Duration
}

type State struct {
	Round   int
	Tracked map[uint64]bool
	names   map[uint64]string

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
	teamFlashBlinds      map[uint64]int
	recentEnemyFlashes   map[uint64]recentFlash
	kitDodgerCandidates  map[uint64]bool
	ecoTeammateCount     map[common.Team]int
	activeDefusers       map[uint64]bool
	defuseInterrupted    map[uint64]int

	roundEndPlayers []*common.Player
	roundMoneySpent map[uint64]int

	lastRoundOfFirstHalf bool
	halftimeCT           int
	halftimeT            int
	halftimeSeen         bool
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
	FirstHalfCT   int    `json:"first_half_ct"`
	FirstHalfT    int    `json:"first_half_t"`
	SecondHalfCT  int    `json:"second_half_ct"`
	SecondHalfT   int    `json:"second_half_t"`
	CTClan        string `json:"ct_clan,omitempty"`
	TClan         string `json:"t_clan,omitempty"`
	TrackedSide   string `json:"tracked_side,omitempty"`
	TrackedScore  int    `json:"tracked_score"`
	OpponentScore int    `json:"opponent_score"`
	Outcome       string `json:"outcome,omitempty"`
}

type insightJSON struct {
	SteamID     string         `json:"steam_id"`
	PlayerName  string         `json:"player_name"`
	TriggerType string         `json:"trigger_type"`
	Round       int            `json:"round"`
	Score       int            `json:"score"`
	Detail      map[string]any `json:"detail,omitempty"`
}

type droppedInsightJSON struct {
	insightJSON
	Reason string `json:"reason"`
}

func WriteRecapLog(path string, recap Recap) error {
	data, err := json.MarshalIndent(recapToJSON(recap), "", "  ")
	if err != nil {
		return fmt.Errorf("analyser: marshal recap log: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("analyser: write recap log: %w", err)
	}
	return nil
}

func recapToJSON(recap Recap) recapJSON {
	out := recapJSON{
		DemoID: recap.DemoID,
		Summary: summaryJSON{
			Map:           recap.Summary.MapName,
			Rounds:        recap.Summary.Rounds,
			CTScore:       recap.Summary.CTScore,
			TScore:        recap.Summary.TScore,
			FirstHalfCT:   recap.Summary.FirstHalfCT,
			FirstHalfT:    recap.Summary.FirstHalfT,
			SecondHalfCT:  recap.Summary.SecondHalfCT,
			SecondHalfT:   recap.Summary.SecondHalfT,
			CTClan:        recap.Summary.CTClan,
			TClan:         recap.Summary.TClan,
			TrackedSide:   recap.Summary.TrackedSide,
			TrackedScore:  recap.Summary.TrackedScore,
			OpponentScore: recap.Summary.OpponentScore,
			Outcome:       recap.Summary.Outcome,
		},
		Trace: recap.Trace,
	}
	if recap.Headline != nil {
		ins := insightToJSON(*recap.Headline)
		out.Headline = &ins
	}
	for _, ins := range recap.Public {
		out.Public = append(out.Public, insightToJSON(ins))
	}
	for _, d := range recap.Dropped {
		out.Dropped = append(out.Dropped, droppedInsightJSON{
			insightJSON: insightToJSON(d.Insight),
			Reason:      d.Reason,
		})
	}
	return out
}

func insightToJSON(ins Insight) insightJSON {
	return insightJSON{
		SteamID:     ins.SteamID,
		PlayerName:  ins.PlayerName,
		TriggerType: ins.TriggerType,
		Round:       ins.Round,
		Score:       ins.Score,
		Detail:      ins.Detail,
	}
}

func (s *State) markLastRoundOfFirstHalf() {
	s.lastRoundOfFirstHalf = true
}

func (s *State) finishFirstHalf(ctScore, tScore int) {
	s.halftimeCT = ctScore
	s.halftimeT = tScore
	s.halftimeSeen = true
	s.lastRoundOfFirstHalf = false
}

func (s *State) halfScores(finalCT, finalT int) (firstHalfCT, firstHalfT, secondHalfCT, secondHalfT int) {
	if !s.halftimeSeen {
		return 0, 0, 0, 0
	}
	return s.halftimeCT, s.halftimeT, finalCT - s.halftimeCT, finalT - s.halftimeT
}

func (s *State) recordName(p *common.Player) {
	if p == nil || p.SteamID64 == 0 {
		return
	}
	if s.names == nil {
		s.names = make(map[uint64]string)
	}
	if p.Name != "" {
		s.names[p.SteamID64] = p.Name
	}
}

func enrichInsights(insights []Insight, names map[uint64]string) []DebugEvent {
	var trace []DebugEvent
	for i := range insights {
		if id, err := strconv.ParseUint(insights[i].SteamID, 10, 64); err == nil {
			if name, ok := names[id]; ok {
				insights[i].PlayerName = name
			} else if insights[i].SteamID != "" {
				trace = append(trace, DebugEvent{
					Stage:   "name_fallback_used",
					SteamID: insights[i].SteamID,
					Trigger: insights[i].TriggerType,
					Round:   insights[i].Round,
					Message: "no captured name for player",
				})
			}
		}
		if victim, ok := insights[i].Detail["victim"].(string); ok {
			if id, err := strconv.ParseUint(victim, 10, 64); err == nil {
				if name, ok := names[id]; ok {
					insights[i].Detail["victim_name"] = name
				}
			}
		}
	}
	return trace
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
	s.recentEnemyFlashes = make(map[uint64]recentFlash)
	for _, p := range players {
		s.recordName(p)
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

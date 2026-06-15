package analyser

import (
	"strconv"

	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
)

type Trigger interface {
	Name() string
}

type RoundStartHook interface {
	OnRoundStart(s *State)
}

type KillHook interface {
	OnKill(s *State, e events.Kill) []Insight
}

type RoundEndHook interface {
	OnRoundEnd(s *State, e events.RoundEnd) []Insight
}

// TeamKill fires when a tracked player kills a teammate.
type TeamKill struct{}

func (TeamKill) Name() string { return "team_kill" }

func (TeamKill) OnKill(s *State, e events.Kill) []Insight {
	if e.Killer == nil || e.Victim == nil {
		return nil
	}
	if e.Killer.SteamID64 == e.Victim.SteamID64 {
		return nil
	}
	if e.Killer.Team != e.Victim.Team {
		return nil
	}
	if !s.Tracked[e.Killer.SteamID64] {
		return nil
	}
	weapon := ""
	if e.Weapon != nil {
		weapon = e.Weapon.String()
	}
	return []Insight{{
		SteamID:     strconv.FormatUint(e.Killer.SteamID64, 10),
		TriggerType: "team_kill",
		Round:       s.Round,
		Detail: map[string]any{
			"victim": strconv.FormatUint(e.Victim.SteamID64, 10),
			"weapon": weapon,
		},
	}}
}

// Ace fires when a tracked player reaches 5 kills in a round.
type Ace struct{}

func (Ace) Name() string { return "ace" }

func (Ace) OnKill(s *State, e events.Kill) []Insight {
	if e.Killer == nil {
		return nil
	}
	if !s.Tracked[e.Killer.SteamID64] {
		return nil
	}
	if s.kills[e.Killer.SteamID64] == 5 {
		return []Insight{{
			SteamID:     strconv.FormatUint(e.Killer.SteamID64, 10),
			TriggerType: "ace",
			Round:       s.Round,
			Detail:      map[string]any{"kills": 5},
		}}
	}
	return nil
}

// Clutch fires when a tracked player wins a 1vN (N≥2) situation.
type Clutch struct{}

func (Clutch) Name() string { return "clutch" }

func (Clutch) OnRoundEnd(s *State, e events.RoundEnd) []Insight {
	if s.clutcher == 0 {
		return nil
	}
	if !s.Tracked[s.clutcher] {
		return nil
	}
	if s.clutchVsEnemies < 2 {
		return nil
	}
	if e.Winner != s.clutchTeam {
		return nil
	}
	return []Insight{{
		SteamID:     strconv.FormatUint(s.clutcher, 10),
		TriggerType: "clutch",
		Round:       s.Round,
		Detail:      map[string]any{"vs": s.clutchVsEnemies},
	}}
}

// MVP fires each time a tracked player earns a new MVP once they have reached 3.
type MVP struct{}

func (MVP) Name() string { return "mvp" }

func (MVP) OnRoundEnd(s *State, e events.RoundEnd) []Insight {
	var out []Insight
	for id, count := range s.mvps {
		if !s.Tracked[id] {
			continue
		}
		if count < 3 || count <= s.prevMVPs[id] {
			continue
		}
		rounds := make([]int, len(s.mvpRounds[id]))
		copy(rounds, s.mvpRounds[id])
		out = append(out, Insight{
			SteamID:     strconv.FormatUint(id, 10),
			TriggerType: "mvp",
			Round:       s.Round,
			Detail:      map[string]any{"mvps": count, "rounds": rounds},
		})
	}
	return out
}

package analyser

import (
	"strconv"

	common "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
	"github.com/oklog/ulid/v2"
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

type MatchEndHook interface {
	OnMatchEnd(s *State) []Insight
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

// LurkerTax fires when a tracked player is last alive with 3+ enemies up and loses.
type LurkerTax struct{}

func (LurkerTax) Name() string { return "lurker_tax" }

func (LurkerTax) OnRoundEnd(s *State, e events.RoundEnd) []Insight {
	if s.clutcher == 0 {
		return nil
	}
	if !s.Tracked[s.clutcher] {
		return nil
	}
	if s.clutchVsEnemies < 3 {
		return nil
	}
	if e.Winner == s.clutchTeam {
		return nil
	}
	return []Insight{{
		SteamID:     strconv.FormatUint(s.clutcher, 10),
		TriggerType: "lurker_tax",
		Round:       s.Round,
		Detail:      map[string]any{"vs": s.clutchVsEnemies},
	}}
}

// BombGod fires when a tracked player plants or defuses in 3+ rounds in one match.
type BombGod struct{}

func (BombGod) Name() string { return "bomb_god" }

func (BombGod) OnRoundEnd(s *State, e events.RoundEnd) []Insight {
	var out []Insight
	for id := range s.Tracked {
		count := s.bombObjectiveCount(id)
		if count < 3 || s.prevBombGod[id] >= 3 {
			continue
		}
		rounds := make([]int, len(s.bombObjectiveRounds[id]))
		copy(rounds, s.bombObjectiveRounds[id])
		out = append(out, Insight{
			SteamID:     strconv.FormatUint(id, 10),
			TriggerType: "bomb_god",
			Round:       s.Round,
			Detail:      map[string]any{"rounds": count, "bomb_rounds": rounds},
		})
	}
	return out
}

// EntryKing fires at match end for tracked player(s) with the most opening kills.
type EntryKing struct{}

func (EntryKing) Name() string { return "entry_king" }

func (EntryKing) OnMatchEnd(s *State) []Insight {
	max := 0
	for id, count := range s.firstKills {
		if !s.Tracked[id] {
			continue
		}
		if count > max {
			max = count
		}
	}
	if max == 0 {
		return nil
	}

	var out []Insight
	for id, count := range s.firstKills {
		if !s.Tracked[id] || count != max {
			continue
		}
		out = append(out, Insight{
			SteamID:     strconv.FormatUint(id, 10),
			TriggerType: "entry_king",
			Round:       s.Round,
			Detail:      map[string]any{"first_kills": count},
		})
	}
	return out
}

// RefundRequest fires when a tracked player buys an AWP, dies without a kill with it.
type RefundRequest struct{}

func (RefundRequest) Name() string { return "refund_request" }

func (RefundRequest) OnRoundEnd(s *State, e events.RoundEnd) []Insight {
	var out []Insight
	for id, weaponID := range s.awpPurchaseWeapon {
		if weaponID == (ulid.ULID{}) {
			continue
		}
		if !s.Tracked[id] {
			continue
		}
		if s.awpKillWithOwn[id] {
			continue
		}
		if !s.diedThisRound[id] {
			continue
		}
		out = append(out, Insight{
			SteamID:     strconv.FormatUint(id, 10),
			TriggerType: "refund_request",
			Round:       s.Round,
		})
	}
	return out
}

func isAWP(w *common.Equipment) bool {
	return w != nil && w.Type == common.EqAWP
}

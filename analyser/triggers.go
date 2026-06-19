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

type BombPlantedHook interface {
	OnBombPlanted(s *State, players []*common.Player) []Insight
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

// MultiKill fires when a tracked player ends the round on exactly 4 kills.
// It checks at round end rather than on the 4th kill so a 5-kill round emits
// only an Ace, never a redundant 4k for the same player and round.
type MultiKill struct{}

func (MultiKill) Name() string { return "multi_kill" }

func (MultiKill) OnRoundEnd(s *State, e events.RoundEnd) []Insight {
	var out []Insight
	for id, kills := range s.kills {
		if !s.Tracked[id] || kills != 4 {
			continue
		}
		out = append(out, Insight{
			SteamID:     strconv.FormatUint(id, 10),
			TriggerType: "multi_kill",
			Round:       s.Round,
			Detail:      map[string]any{"kills": 4},
		})
	}
	return out
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
			Detail: map[string]any{
				"rounds":      count,
				"bomb_rounds": rounds,
				"plants":      s.bombPlants[id],
				"defuses":     s.bombDefuses[id],
			},
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

// KnifeKill fires when a tracked player kills an enemy with a knife.
type KnifeKill struct{}

func (KnifeKill) Name() string { return "knife_kill" }

func (KnifeKill) OnKill(s *State, e events.Kill) []Insight {
	if e.Killer == nil || e.Victim == nil {
		return nil
	}
	if e.Killer.SteamID64 == e.Victim.SteamID64 {
		return nil
	}
	if e.Killer.Team == e.Victim.Team {
		return nil
	}
	if !s.Tracked[e.Killer.SteamID64] {
		return nil
	}
	if !isKnife(e.Weapon) {
		return nil
	}
	return []Insight{{
		SteamID:     strconv.FormatUint(e.Killer.SteamID64, 10),
		TriggerType: "knife_kill",
		Round:       s.Round,
		Detail: map[string]any{
			"victim": strconv.FormatUint(e.Victim.SteamID64, 10),
		},
	}}
}

// KnifeTeamKill fires when a tracked player team-kills with a knife.
type KnifeTeamKill struct{}

func (KnifeTeamKill) Name() string { return "knife_team_kill" }

func (KnifeTeamKill) OnKill(s *State, e events.Kill) []Insight {
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
	if !isKnife(e.Weapon) {
		return nil
	}
	return []Insight{{
		SteamID:     strconv.FormatUint(e.Killer.SteamID64, 10),
		TriggerType: "knife_team_kill",
		Round:       s.Round,
		Detail: map[string]any{
			"victim": strconv.FormatUint(e.Victim.SteamID64, 10),
		},
	}}
}

func isAWP(w *common.Equipment) bool {
	return w != nil && w.Type == common.EqAWP
}

func isKnife(w *common.Equipment) bool {
	return w != nil && w.Type == common.EqKnife
}

func isZeus(w *common.Equipment) bool {
	return w != nil && w.Type == common.EqZeus
}

// ZeusKill fires when a tracked player kills an enemy with a Zeus.
type ZeusKill struct{}

func (ZeusKill) Name() string { return "zeus_kill" }

func (ZeusKill) OnKill(s *State, e events.Kill) []Insight {
	if e.Killer == nil || e.Victim == nil {
		return nil
	}
	if e.Killer.SteamID64 == e.Victim.SteamID64 {
		return nil
	}
	if e.Killer.Team == e.Victim.Team {
		return nil
	}
	if !s.Tracked[e.Killer.SteamID64] {
		return nil
	}
	if !isZeus(e.Weapon) {
		return nil
	}
	return []Insight{{
		SteamID:     strconv.FormatUint(e.Killer.SteamID64, 10),
		TriggerType: "zeus_kill",
		Round:       s.Round,
		Detail: map[string]any{
			"victim": strconv.FormatUint(e.Victim.SteamID64, 10),
		},
	}}
}

// StyleKill fires when a tracked player gets a flashy enemy kill.
type StyleKill struct{}

func (StyleKill) Name() string { return "style_kill" }

func (StyleKill) OnKill(s *State, e events.Kill) []Insight {
	if e.Killer == nil || e.Victim == nil {
		return nil
	}
	if e.Killer.SteamID64 == e.Victim.SteamID64 {
		return nil
	}
	if e.Killer.Team == e.Victim.Team {
		return nil
	}
	if !s.Tracked[e.Killer.SteamID64] {
		return nil
	}

	detail := make(map[string]any)
	if e.IsWallBang() {
		detail["wallbang"] = true
	}
	if e.NoScope && isAWP(e.Weapon) {
		detail["noscope"] = true
	}
	if len(detail) == 0 {
		return nil
	}
	return []Insight{{
		SteamID:     strconv.FormatUint(e.Killer.SteamID64, 10),
		TriggerType: "style_kill",
		Round:       s.Round,
		Detail:      detail,
	}}
}

// FlashAssist fires when a tracked player flash-assists an enemy kill.
type FlashAssist struct{}

func (FlashAssist) Name() string { return "flash_assist" }

func (FlashAssist) OnKill(s *State, e events.Kill) []Insight {
	if e.Victim == nil || e.Killer == nil || e.Killer.Team == e.Victim.Team {
		return nil
	}
	flash, ok := s.recentEnemyFlashes[e.Victim.SteamID64]
	if !ok {
		return nil
	}
	if s.currentTime-flash.at > flashAssistWindow {
		return nil
	}
	if !s.Tracked[flash.flasher] {
		return nil
	}
	return []Insight{{
		SteamID:     strconv.FormatUint(flash.flasher, 10),
		TriggerType: "flash_assist",
		Round:       s.Round,
		Detail: map[string]any{
			"victim": strconv.FormatUint(e.Victim.SteamID64, 10),
		},
	}}
}

// TeamFlash fires at match end when a tracked player blinds teammates repeatedly.
type TeamFlash struct{}

func (TeamFlash) Name() string { return "team_flash" }

func (TeamFlash) OnMatchEnd(s *State) []Insight {
	const threshold = 3
	var out []Insight
	for id, count := range s.teamFlashBlinds {
		if !s.Tracked[id] || count < threshold {
			continue
		}
		out = append(out, Insight{
			SteamID:     strconv.FormatUint(id, 10),
			TriggerType: "team_flash",
			Round:       s.Round,
			Detail:      map[string]any{"blinds": count},
		})
	}
	return out
}

// EntryVictim fires at match end for tracked player(s) with the most opening deaths.
type EntryVictim struct{}

func (EntryVictim) Name() string { return "entry_victim" }

func (EntryVictim) OnMatchEnd(s *State) []Insight {
	max := 0
	for id, count := range s.firstDeaths {
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
	for id, count := range s.firstDeaths {
		if !s.Tracked[id] || count != max {
			continue
		}
		out = append(out, Insight{
			SteamID:     strconv.FormatUint(id, 10),
			TriggerType: "entry_victim",
			Round:       s.Round,
			Detail:      map[string]any{"first_deaths": count},
		})
	}
	return out
}

// InstantTrade fires at match end when a tracked player gets 3+ instant trades.
type InstantTrade struct{}

func (InstantTrade) Name() string { return "instant_trade" }

func (InstantTrade) OnMatchEnd(s *State) []Insight {
	var out []Insight
	for id, count := range s.instantTrades {
		if !s.Tracked[id] || count < 3 {
			continue
		}
		out = append(out, Insight{
			SteamID:     strconv.FormatUint(id, 10),
			TriggerType: "instant_trade",
			Round:       s.Round,
			Detail:      map[string]any{"trades": count},
		})
	}
	return out
}

// BombMule fires at match end when a tracked player dies with the bomb 3+ times.
type BombMule struct{}

func (BombMule) Name() string { return "bomb_mule" }

func (BombMule) OnMatchEnd(s *State) []Insight {
	var out []Insight
	for id, count := range s.bombMuleDeaths {
		if !s.Tracked[id] || count < 3 {
			continue
		}
		out = append(out, Insight{
			SteamID:     strconv.FormatUint(id, 10),
			TriggerType: "bomb_mule",
			Round:       s.Round,
			Detail:      map[string]any{"deaths": count},
		})
	}
	return out
}

// DefuseInterrupted fires at match end when a tracked player is killed while defusing 2+ times.
type DefuseInterrupted struct{}

func (DefuseInterrupted) Name() string { return "defuse_interrupted" }

func (DefuseInterrupted) OnMatchEnd(s *State) []Insight {
	var out []Insight
	for id, count := range s.defuseInterrupted {
		if !s.Tracked[id] || count < 2 {
			continue
		}
		out = append(out, Insight{
			SteamID:     strconv.FormatUint(id, 10),
			TriggerType: "defuse_interrupted",
			Round:       s.Round,
			Detail:      map[string]any{"interruptions": count},
		})
	}
	return out
}

// FlashTax fires at match end for the most flashed tracked player(s), or anyone with 8+ blinds.
type FlashTax struct{}

func (FlashTax) Name() string { return "flash_tax" }

func (FlashTax) OnMatchEnd(s *State) []Insight {
	max := 0
	for id, count := range s.flashBlinds {
		if !s.Tracked[id] {
			continue
		}
		if count > max {
			max = count
		}
	}

	emitted := make(map[uint64]bool)
	var out []Insight

	if max > 0 {
		for id, count := range s.flashBlinds {
			if !s.Tracked[id] || count != max {
				continue
			}
			emitted[id] = true
			out = append(out, Insight{
				SteamID:     strconv.FormatUint(id, 10),
				TriggerType: "flash_tax",
				Round:       s.Round,
				Detail:      map[string]any{"blinds": count},
			})
		}
	}

	for id, count := range s.flashBlinds {
		if !s.Tracked[id] || count < 8 || emitted[id] {
			continue
		}
		out = append(out, Insight{
			SteamID:     strconv.FormatUint(id, 10),
			TriggerType: "flash_tax",
			Round:       s.Round,
			Detail:      map[string]any{"blinds": count},
		})
	}
	return out
}

// KitDodger fires when a tracked CT had kit money but no defuse kit when the bomb was planted.
type KitDodger struct{}

func (KitDodger) Name() string { return "kit_dodger" }

func (KitDodger) OnBombPlanted(s *State, players []*common.Player) []Insight {
	if len(s.kitDodgerCandidates) == 0 {
		return nil
	}
	byID := make(map[uint64]*common.Player, len(players))
	for _, p := range players {
		byID[p.SteamID64] = p
	}

	var out []Insight
	for id := range s.kitDodgerCandidates {
		p, ok := byID[id]
		if !ok || p.HasDefuseKit() {
			continue
		}
		out = append(out, Insight{
			SteamID:     strconv.FormatUint(id, 10),
			TriggerType: "kit_dodger",
			Round:       s.Round,
		})
	}
	s.kitDodgerCandidates = make(map[uint64]bool)
	return out
}

// EconomyTerrorist fires when a tracked player overspends while teammates are on eco.
type EconomyTerrorist struct{}

func (EconomyTerrorist) Name() string { return "economy_terrorist" }

func (EconomyTerrorist) OnRoundEnd(s *State, e events.RoundEnd) []Insight {
	var out []Insight
	for _, p := range s.roundEndPlayers {
		if !s.Tracked[p.SteamID64] {
			continue
		}
		spent := s.roundMoneySpent[p.SteamID64]
		if spent < 4500 {
			continue
		}
		ecoTeammates := s.ecoTeammateCount[p.Team] - 1
		if ecoTeammates < 3 {
			continue
		}
		out = append(out, Insight{
			SteamID:     strconv.FormatUint(p.SteamID64, 10),
			TriggerType: "economy_terrorist",
			Round:       s.Round,
			Detail: map[string]any{
				"spent":         spent,
				"eco_teammates": ecoTeammates,
			},
		})
	}
	return out
}

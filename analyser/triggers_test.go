package analyser

import (
	"testing"

	common "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
)

func TestTeamKill_firesOnSameTeamKillByTrackedPlayer(t *testing.T) {
	s := &State{
		Round:   2,
		Tracked: map[uint64]bool{1: true},
		kills:   make(map[uint64]int),
		alive:   make(map[common.Team]int),
	}
	e := events.Kill{
		Killer: &common.Player{SteamID64: 1, Team: common.TeamTerrorists},
		Victim: &common.Player{SteamID64: 2, Team: common.TeamTerrorists},
	}
	got := TeamKill{}.OnKill(s, e)
	if len(got) != 1 {
		t.Fatalf("len(insights) = %d, want 1", len(got))
	}
	if got[0].TriggerType != "team_kill" {
		t.Errorf("TriggerType = %q, want %q", got[0].TriggerType, "team_kill")
	}
	if got[0].Round != 2 {
		t.Errorf("Round = %d, want 2", got[0].Round)
	}
}

func TestAce_firesAtFifthKillInRound(t *testing.T) {
	s := &State{
		Round:   4,
		Tracked: map[uint64]bool{10: true},
		kills:   map[uint64]int{10: 5},
		alive:   make(map[common.Team]int),
	}
	e := events.Kill{
		Killer: &common.Player{SteamID64: 10, Team: common.TeamCounterTerrorists},
		Victim: &common.Player{SteamID64: 20, Team: common.TeamTerrorists},
	}
	got := Ace{}.OnKill(s, e)
	if len(got) != 1 {
		t.Fatalf("len(insights) = %d, want 1", len(got))
	}
	if got[0].TriggerType != "ace" {
		t.Errorf("TriggerType = %q, want %q", got[0].TriggerType, "ace")
	}
	if got[0].Round != 4 {
		t.Errorf("Round = %d, want 4", got[0].Round)
	}
}

func TestClutch_firesOnLastAliveWinVsTwoOrMoreEnemies(t *testing.T) {
	s := &State{
		Round:           7,
		Tracked:         map[uint64]bool{99: true},
		kills:           make(map[uint64]int),
		alive:           make(map[common.Team]int),
		clutcher:        99,
		clutchTeam:      common.TeamCounterTerrorists,
		clutchVsEnemies: 3,
	}
	e := events.RoundEnd{
		Winner: common.TeamCounterTerrorists,
	}
	got := Clutch{}.OnRoundEnd(s, e)
	if len(got) != 1 {
		t.Fatalf("len(insights) = %d, want 1", len(got))
	}
	if got[0].TriggerType != "clutch" {
		t.Errorf("TriggerType = %q, want %q", got[0].TriggerType, "clutch")
	}
	if got[0].Round != 7 {
		t.Errorf("Round = %d, want 7", got[0].Round)
	}
	vs, _ := got[0].Detail["vs"].(int)
	if vs != 3 {
		t.Errorf("Detail[vs] = %d, want 3", vs)
	}
}

func TestMVP_firesWhenCrossingThreeMVPsAtRoundEnd(t *testing.T) {
	s := &State{
		Round:    23,
		Tracked:  map[uint64]bool{42: true},
		mvps:     map[uint64]int{42: 3},
		prevMVPs: map[uint64]int{42: 2},
		kills:    make(map[uint64]int),
		alive:    make(map[common.Team]int),
	}
	got := MVP{}.OnRoundEnd(s, events.RoundEnd{})
	if len(got) != 1 {
		t.Fatalf("len(insights) = %d, want 1", len(got))
	}
	if got[0].TriggerType != "mvp" {
		t.Errorf("TriggerType = %q, want %q", got[0].TriggerType, "mvp")
	}
	if got[0].Round != 23 {
		t.Errorf("Round = %d, want 23", got[0].Round)
	}
	mvps, _ := got[0].Detail["mvps"].(int)
	if mvps != 3 {
		t.Errorf("Detail[mvps] = %d, want 3", mvps)
	}
}

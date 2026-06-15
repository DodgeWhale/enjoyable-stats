package analyser

import (
	"testing"

	common "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
	"github.com/oklog/ulid/v2"
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

func TestMVP_reportsCurrentCountNotJustThreshold(t *testing.T) {
	count := 7
	s := &State{
		Round:    27,
		Tracked:  map[uint64]bool{42: true},
		mvps:     map[uint64]int{42: count},
		prevMVPs: map[uint64]int{42: count - 1},
		kills:    make(map[uint64]int),
		alive:    make(map[common.Team]int),
	}
	got := MVP{}.OnRoundEnd(s, events.RoundEnd{})
	if len(got) != 1 {
		t.Fatalf("len(insights) = %d, want 1", len(got))
	}
	mvps, _ := got[0].Detail["mvps"].(int)
	if mvps != count {
		t.Errorf("Detail[mvps] = %d, want %d", mvps, count)
	}
}

func TestMVP_doesNotFireWhenNoNewMVPAboveThreshold(t *testing.T) {
	s := &State{
		Round:    28,
		Tracked:  map[uint64]bool{42: true},
		mvps:     map[uint64]int{42: 4},
		prevMVPs: map[uint64]int{42: 4},
		kills:    make(map[uint64]int),
		alive:    make(map[common.Team]int),
	}
	got := MVP{}.OnRoundEnd(s, events.RoundEnd{})
	if len(got) != 0 {
		t.Fatalf("len(insights) = %d, want 0", len(got))
	}
}

func TestLurkerTax_firesWhenLastAliveLosesVsThreeOrMore(t *testing.T) {
	s := &State{
		Round:           5,
		Tracked:         map[uint64]bool{7: true},
		clutcher:        7,
		clutchTeam:      common.TeamTerrorists,
		clutchVsEnemies: 3,
	}
	got := LurkerTax{}.OnRoundEnd(s, events.RoundEnd{Winner: common.TeamCounterTerrorists})
	if len(got) != 1 {
		t.Fatalf("len(insights) = %d, want 1", len(got))
	}
	if got[0].TriggerType != "lurker_tax" {
		t.Errorf("TriggerType = %q, want %q", got[0].TriggerType, "lurker_tax")
	}
}

func TestLurkerTax_doesNotFireWhenLastAliveWins(t *testing.T) {
	s := &State{
		Round:           5,
		Tracked:         map[uint64]bool{7: true},
		clutcher:        7,
		clutchTeam:      common.TeamTerrorists,
		clutchVsEnemies: 3,
	}
	got := LurkerTax{}.OnRoundEnd(s, events.RoundEnd{Winner: common.TeamTerrorists})
	if len(got) != 0 {
		t.Fatalf("len(insights) = %d, want 0", len(got))
	}
}

func TestBombGod_firesWhenCrossingThreeObjectiveRounds(t *testing.T) {
	s := &State{
		Round:               12,
		Tracked:             map[uint64]bool{55: true},
		bombObjectiveRounds: map[uint64][]int{55: {2, 7, 12}},
		prevBombGod:         map[uint64]int{55: 2},
	}
	got := BombGod{}.OnRoundEnd(s, events.RoundEnd{})
	if len(got) != 1 {
		t.Fatalf("len(insights) = %d, want 1", len(got))
	}
	if got[0].TriggerType != "bomb_god" {
		t.Errorf("TriggerType = %q, want %q", got[0].TriggerType, "bomb_god")
	}
}

func TestBombGod_doesNotFireBeforeThreshold(t *testing.T) {
	s := &State{
		Round:               8,
		Tracked:             map[uint64]bool{55: true},
		bombObjectiveRounds: map[uint64][]int{55: {2, 7}},
		prevBombGod:         map[uint64]int{55: 1},
	}
	got := BombGod{}.OnRoundEnd(s, events.RoundEnd{})
	if len(got) != 0 {
		t.Fatalf("len(insights) = %d, want 0", len(got))
	}
}

func TestEntryKing_firesForTrackedLeaderAtMatchEnd(t *testing.T) {
	s := &State{
		Round:      30,
		Tracked:    map[uint64]bool{11: true, 22: true},
		firstKills: map[uint64]int{11: 4, 22: 2, 33: 10},
	}
	got := EntryKing{}.OnMatchEnd(s)
	if len(got) != 1 {
		t.Fatalf("len(insights) = %d, want 1", len(got))
	}
	if got[0].SteamID != "11" {
		t.Errorf("SteamID = %q, want %q", got[0].SteamID, "11")
	}
	firstKills, _ := got[0].Detail["first_kills"].(int)
	if firstKills != 4 {
		t.Errorf("Detail[first_kills] = %d, want 4", firstKills)
	}
}

func TestRefundRequest_firesWhenAWPBoughtDiedWithNoKill(t *testing.T) {
	s := &State{
		Round:             9,
		Tracked:           map[uint64]bool{88: true},
		awpPurchaseWeapon: map[uint64]ulid.ULID{88: ulid.Make()},
		diedThisRound:     map[uint64]bool{88: true},
	}
	got := RefundRequest{}.OnRoundEnd(s, events.RoundEnd{})
	if len(got) != 1 {
		t.Fatalf("len(insights) = %d, want 1", len(got))
	}
	if got[0].TriggerType != "refund_request" {
		t.Errorf("TriggerType = %q, want %q", got[0].TriggerType, "refund_request")
	}
}

func TestRefundRequest_doesNotFireWhenAWPGetsAKill(t *testing.T) {
	s := &State{
		Round:             9,
		Tracked:           map[uint64]bool{88: true},
		awpPurchaseWeapon: map[uint64]ulid.ULID{88: ulid.Make()},
		awpKillWithOwn:    map[uint64]bool{88: true},
		diedThisRound:     map[uint64]bool{88: true},
	}
	got := RefundRequest{}.OnRoundEnd(s, events.RoundEnd{})
	if len(got) != 0 {
		t.Fatalf("len(insights) = %d, want 0", len(got))
	}
}

func TestRefundRequest_doesNotFireWhenPlayerSurvives(t *testing.T) {
	s := &State{
		Round:             9,
		Tracked:           map[uint64]bool{88: true},
		awpPurchaseWeapon: map[uint64]ulid.ULID{88: ulid.Make()},
	}
	got := RefundRequest{}.OnRoundEnd(s, events.RoundEnd{})
	if len(got) != 0 {
		t.Fatalf("len(insights) = %d, want 0", len(got))
	}
}

package analyser

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestBuildRecap_enforcesCapsAndDropReasons(t *testing.T) {
	insights := []Insight{
		{SteamID: "1", TriggerType: "ace", Round: 1},
		{SteamID: "1", TriggerType: "clutch", Round: 2, Detail: map[string]any{"vs": 2}},
		{SteamID: "1", TriggerType: "knife_kill", Round: 11},
		{SteamID: "1", TriggerType: "mvp", Round: 3, Detail: map[string]any{"mvps": 3}},
		{SteamID: "2", TriggerType: "flash_tax", Round: 4, Detail: map[string]any{"blinds": 10}},
		{SteamID: "3", TriggerType: "kit_dodger", Round: 5},
		{SteamID: "4", TriggerType: "entry_king", Round: 6, Detail: map[string]any{"first_kills": 4}},
		{SteamID: "5", TriggerType: "bomb_mule", Round: 7, Detail: map[string]any{"deaths": 3}},
		{SteamID: "6", TriggerType: "instant_trade", Round: 8, Detail: map[string]any{"trades": 4}},
		{SteamID: "7", TriggerType: "refund_request", Round: 9},
		{SteamID: "8", TriggerType: "economy_terrorist", Round: 10},
		{SteamID: "9", TriggerType: "entry_victim", Round: 11, Detail: map[string]any{"first_deaths": 4}},
		{SteamID: "10", TriggerType: "bomb_god", Round: 12, Detail: map[string]any{"plants": 3}},
		{SteamID: "11", TriggerType: "team_flash", Round: 13, Detail: map[string]any{"blinds": 4}},
		{SteamID: "1", TriggerType: "ace", Round: 1},
		{SteamID: "12", TriggerType: "unknown_trigger", Round: 14},
	}

	recap := BuildRecap(insights, "demo.dem", Summary{MapName: "de_mirage", Rounds: 30})

	if len(recap.Public) > maxPublic {
		t.Errorf("public len = %d, want <= %d", len(recap.Public), maxPublic)
	}

	playerCounts := make(map[string]int)
	for _, ins := range recap.Public {
		playerCounts[ins.SteamID]++
	}
	for id, count := range playerCounts {
		if count > maxPerPlayer {
			t.Errorf("player %s has %d public moments, want <= %d", id, count, maxPerPlayer)
		}
	}

	reasons := make(map[string]bool)
	for _, d := range recap.Dropped {
		reasons[d.Reason] = true
	}
	for _, want := range []string{"duplicate", "per_player_cap", "public_cap", "below_min_score"} {
		if !reasons[want] {
			t.Errorf("expected drop reason %q", want)
		}
	}

	if recap.Headline == nil {
		t.Fatal("expected headline")
	}
	if recap.Headline.TriggerType != "ace" {
		t.Errorf("headline = %q, want ace", recap.Headline.TriggerType)
	}
}

func TestBuildRecap_samePlayerEqualScoreTieBreakIsStable(t *testing.T) {
	insights := []Insight{
		{SteamID: "1", TriggerType: "instant_trade", Round: 1, Detail: map[string]any{"trades": 3}},
		{SteamID: "1", TriggerType: "refund_request", Round: 2},
		{SteamID: "1", TriggerType: "entry_victim", Round: 3, Detail: map[string]any{"first_deaths": 3}},
		{SteamID: "1", TriggerType: "economy_terrorist", Round: 4},
	}

	r1 := BuildRecap(insights, "demo.dem", Summary{Rounds: 10})
	r2 := BuildRecap(insights, "demo.dem", Summary{Rounds: 10})

	if len(r1.Public) != maxPerPlayer {
		t.Fatalf("public len = %d, want %d", len(r1.Public), maxPerPlayer)
	}
	if publicTriggerKey(r1) != publicTriggerKey(r2) {
		t.Errorf("same demo should pick the same equal-score moments")
	}
}

func TestBuildRecap_samePlayerEqualScoreTieBreakVariesByDemo(t *testing.T) {
	insights := []Insight{
		{SteamID: "1", TriggerType: "instant_trade", Round: 1, Detail: map[string]any{"trades": 3}},
		{SteamID: "1", TriggerType: "refund_request", Round: 2},
		{SteamID: "1", TriggerType: "entry_victim", Round: 3, Detail: map[string]any{"first_deaths": 3}},
		{SteamID: "1", TriggerType: "economy_terrorist", Round: 4},
	}

	seen := make(map[string]bool)
	for i := range 50 {
		recap := BuildRecap(insights, fmt.Sprintf("demo-%d", i), Summary{Rounds: 10})
		seen[publicTriggerKey(recap)] = true
	}
	if len(seen) < 2 {
		t.Fatalf("expected different tie-breaks across demo IDs, got %d unique orderings", len(seen))
	}
}

func publicTriggerKey(recap Recap) string {
	var parts []string
	for _, ins := range recap.Public {
		parts = append(parts, ins.TriggerType)
	}
	return strings.Join(parts, ",")
}

func TestBuildRecap_deduplicatesSamePlayerTriggerRound(t *testing.T) {
	insights := []Insight{
		{SteamID: "1", TriggerType: "team_kill", Round: 3, Detail: map[string]any{"victim": "2"}},
		{SteamID: "1", TriggerType: "team_kill", Round: 3, Detail: map[string]any{"victim": "2"}},
	}
	recap := BuildRecap(insights, "demo.dem", Summary{Rounds: 10})
	if len(recap.Dropped) != 1 || recap.Dropped[0].Reason != "duplicate" {
		t.Fatalf("expected one duplicate drop, got %+v", recap.Dropped)
	}
}

func TestWriteRecapLog_goldenJSON(t *testing.T) {
	insights := []Insight{
		{SteamID: "100", PlayerName: "Alpha", TriggerType: "ace", Round: 12},
		{SteamID: "100", PlayerName: "Alpha", TriggerType: "clutch", Round: 14, Detail: map[string]any{"vs": 2}},
		{SteamID: "100", PlayerName: "Alpha", TriggerType: "knife_kill", Round: 16},
		{SteamID: "100", PlayerName: "Alpha", TriggerType: "mvp", Round: 20, Detail: map[string]any{"mvps": 4}},
		{SteamID: "200", PlayerName: "Bravo", TriggerType: "flash_tax", Round: 8, Detail: map[string]any{"blinds": 2}},
		{SteamID: "100", PlayerName: "Alpha", TriggerType: "ace", Round: 12},
	}
	summary := makeSummary("de_inferno", 24, 16, 14, "", "", "CT", 3, 9, 13, 5)
	recap := BuildRecap(insights, "fixtures/demo.dem", summary)

	data, err := json.MarshalIndent(recapToJSON(recap), "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	jsonStr := string(data)

	for _, fragment := range []string{
		`"demo_id": "fixtures/demo.dem"`,
		`"map": "de_inferno"`,
		`"rounds": 24`,
		`"ct_score": 16`,
		`"t_score": 14`,
		`"first_half_ct": 3`,
		`"first_half_t": 9`,
		`"second_half_ct": 13`,
		`"second_half_t": 5`,
		`"outcome": "won"`,
		`"trigger_type": "ace"`,
		`"reason": "duplicate"`,
		`"reason": "per_player_cap"`,
	} {
		if !strings.Contains(jsonStr, fragment) {
			t.Errorf("JSON missing %q\n%s", fragment, jsonStr)
		}
	}

	publicCount := strings.Count(jsonStr, `"trigger_type"`)
	if publicCount < len(recap.Public)+1 {
		t.Errorf("expected score fields in public entries, got trigger_type count %d", publicCount)
	}
}

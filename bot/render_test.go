package bot

import (
	"strings"
	"testing"

	"github.com/DodgeWhale/enjoyable-stats/analyser"
)

func TestDisplayName_prefersMentionThenNameThenSteamID(t *testing.T) {
	mentions := map[string]string{"111": "999"}

	cases := []struct {
		name    string
		steamID string
		player  string
		want    string
	}{
		{"mention", "111", "Alice", "<@999>"},
		{"captured name", "222", "Bob", "Bob"},
		{"steam id", "333", "", "333"},
		{"someone", "", "", "someone"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := displayName(tc.steamID, tc.player, mentions); got != tc.want {
				t.Errorf("displayName() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRenderInsightLine_degradesWithoutOptionalDetail(t *testing.T) {
	ins := analyser.Insight{TriggerType: "bomb_god", Round: 1}
	got := renderInsightLine(ins, "Player", nil, false)
	if got == "" {
		t.Fatal("expected non-empty render")
	}
	if got != "Player actually played the objective." {
		t.Errorf("render = %q", got)
	}
}

func TestFormatRecap_rendersHeadlineAndSupportingLines(t *testing.T) {
	recap := analyser.Recap{
		Summary: analyser.Summary{
			MapName:      "de_dust2",
			CTScore:      16,
			TScore:       14,
			FirstHalfCT:  3,
			FirstHalfT:   9,
			SecondHalfCT: 13,
			SecondHalfT:  5,
		},
		Public: []analyser.Insight{
			{SteamID: "1", PlayerName: "AcePlayer", TriggerType: "ace", Round: 10, Score: 100},
			{SteamID: "2", PlayerName: "Flashy", TriggerType: "flash_tax", Round: 5, Score: 15, Detail: map[string]any{"blinds": 8}},
		},
	}
	recap.Headline = &recap.Public[0]

	msgs := FormatRecap(recap, nil, false)
	if len(msgs) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(msgs))
	}
	if msgs[0] == "" {
		t.Fatal("expected non-empty recap message")
	}
	if !strings.Contains(msgs[0], "CT 16 - 14 T") {
		t.Errorf("expected final score in recap, got %q", msgs[0])
	}
	if !strings.Contains(msgs[0], "(CT 3-9, 13-5 T)") {
		t.Errorf("expected half scoreline in recap, got %q", msgs[0])
	}
}

func TestFormatRecap_showScoresInDebugMode(t *testing.T) {
	recap := analyser.Recap{
		Public: []analyser.Insight{
			{SteamID: "1", PlayerName: "AcePlayer", TriggerType: "ace", Round: 10, Score: 100},
		},
	}
	recap.Headline = &recap.Public[0]

	msgs := FormatRecap(recap, nil, true)
	if len(msgs) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(msgs))
	}
	if !strings.Contains(msgs[0], "(score 100)") {
		t.Errorf("expected score on debug recap line, got %q", msgs[0])
	}
}

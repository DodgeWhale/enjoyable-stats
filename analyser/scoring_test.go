package analyser

import "testing"

func TestScore_orderingAndBonuses(t *testing.T) {
	ace := Insight{TriggerType: "ace"}
	knife := Insight{TriggerType: "knife_kill"}
	multi := Insight{TriggerType: "multi_kill"}
	flash := Insight{TriggerType: "flash_assist"}

	if Score(ace) <= Score(knife) {
		t.Errorf("ace (%d) should beat knife (%d)", Score(ace), Score(knife))
	}
	if Score(knife) <= Score(multi) {
		t.Errorf("knife (%d) should beat multi_kill (%d)", Score(knife), Score(multi))
	}
	if Score(multi) <= Score(flash) {
		t.Errorf("multi_kill (%d) should beat flash_assist (%d)", Score(multi), Score(flash))
	}

	clutch2 := Insight{TriggerType: "clutch", Detail: map[string]any{"vs": 2}}
	clutch3 := Insight{TriggerType: "clutch", Detail: map[string]any{"vs": 3}}
	if Score(clutch3) <= Score(clutch2) {
		t.Errorf("1v3 clutch (%d) should beat 1v2 (%d)", Score(clutch3), Score(clutch2))
	}
}

func TestScore_styleKillAddsFlavourBonuses(t *testing.T) {
	base := Insight{TriggerType: "style_kill"}
	noscope := Insight{TriggerType: "style_kill", Detail: map[string]any{"noscope": true}}
	if Score(noscope) <= Score(base) {
		t.Errorf("noscope style kill should score higher")
	}
}

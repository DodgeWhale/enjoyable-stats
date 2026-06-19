package analyser

var baseScores = map[string]int{
	"ace":                100,
	"clutch":             70,
	"multi_kill":         55,
	"knife_kill":         60,
	"zeus_kill":          60,
	"style_kill":         40,
	"entry_king":         35,
	"bomb_god":           30,
	"instant_trade":      25,
	"mvp":                50,
	"flash_assist":       20,
	"knife_team_kill":    55,
	"team_kill":          45,
	"team_flash":         30,
	"lurker_tax":         30,
	"refund_request":     25,
	"entry_victim":       25,
	"bomb_mule":          20,
	"kit_dodger":         20,
	"defuse_interrupted": 20,
	"economy_terrorist":  25,
	"flash_tax":          15,
}

const (
	leverageBonusPerEnemy = 5
	rarityBonus           = 10
	styleNoScopeBonus     = 15
	styleWallbangBonus    = 10
	mvpThreshold          = 3
	mvpBonusPerExtra      = 5
)

func Score(ins Insight) int {
	score, ok := baseScores[ins.TriggerType]
	if !ok {
		return 0
	}

	switch ins.TriggerType {
	case "clutch", "lurker_tax":
		if vs, ok := ins.Detail["vs"].(int); ok {
			score += vs * leverageBonusPerEnemy
		}
	case "mvp":
		if mvps, ok := ins.Detail["mvps"].(int); ok && mvps > mvpThreshold {
			score += (mvps - mvpThreshold) * mvpBonusPerExtra
		}
	case "knife_kill", "zeus_kill", "knife_team_kill":
		score += rarityBonus
	case "style_kill":
		if noscope, _ := ins.Detail["noscope"].(bool); noscope {
			score += styleNoScopeBonus
		}
		if wallbang, _ := ins.Detail["wallbang"].(bool); wallbang {
			score += styleWallbangBonus
		}
	}

	return score
}

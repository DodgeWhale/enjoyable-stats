package bot

import (
	"fmt"
	"strings"

	"github.com/DodgeWhale/enjoyable-stats/analyser"
)

func displayName(steamID, name string, mentions map[string]string) string {
	if discordID, ok := mentions[steamID]; ok && discordID != "" {
		return "<@" + discordID + ">"
	}
	if name != "" {
		return name
	}
	if steamID != "" {
		return steamID
	}
	return "someone"
}

func victimName(ins analyser.Insight, mentions map[string]string) string {
	if name, ok := ins.Detail["victim_name"].(string); ok && name != "" {
		return name
	}
	if victim, ok := ins.Detail["victim"].(string); ok {
		return displayName(victim, "", mentions)
	}
	return "someone"
}

// FormatRecap renders a ranked recap for Discord or debug console output.
// When showScores is true, each selected line includes its score (debug only).
func FormatRecap(recap analyser.Recap, mentions map[string]string, showScores bool) []string {
	if len(recap.Public) == 0 {
		return nil
	}

	var sb strings.Builder
	if recap.MapName != "" {
		sb.WriteString(fmt.Sprintf("**%s** recap\n", recap.MapName))
	} else {
		sb.WriteString("Match recap\n")
	}

	if recap.Headline != nil {
		sb.WriteString(renderHeadline(*recap.Headline, mentions, showScores))
		sb.WriteString("\n")
	}

	supporting := 0
	for _, ins := range recap.Public {
		if recap.Headline != nil && ins.SteamID == recap.Headline.SteamID &&
			ins.TriggerType == recap.Headline.TriggerType && ins.Round == recap.Headline.Round {
			continue
		}
		if supporting >= 4 {
			break
		}
		sb.WriteString(renderSupporting(ins, mentions, showScores))
		sb.WriteString("\n")
		supporting++
	}

	if len(recap.Public) > 1 {
		sb.WriteString("Hard to argue with the tape.")
	}

	return []string{sb.String()}
}

func renderHeadline(ins analyser.Insight, mentions map[string]string, showScores bool) string {
	player := displayName(ins.SteamID, ins.PlayerName, mentions)
	line := "🔥 " + renderInsightLine(ins, player, mentions, true)
	if showScores {
		line += fmt.Sprintf(" (score %d)", ins.Score)
	}
	return line
}

func renderSupporting(ins analyser.Insight, mentions map[string]string, showScores bool) string {
	player := displayName(ins.SteamID, ins.PlayerName, mentions)
	line := "• " + renderInsightLine(ins, player, mentions, false)
	if showScores {
		line += fmt.Sprintf(" (score %d)", ins.Score)
	}
	return line
}

func renderInsightLine(ins analyser.Insight, player string, mentions map[string]string, headline bool) string {
	victim := victimName(ins, mentions)

	switch ins.TriggerType {
	case "ace":
		return fmt.Sprintf("%s dropped an ace in round %d.", player, ins.Round)
	case "multi_kill":
		kills, _ := ins.Detail["kills"].(int)
		return fmt.Sprintf("%s took a %dk in round %d.", player, kills, ins.Round)
	case "clutch":
		vs, _ := ins.Detail["vs"].(int)
		if headline {
			return fmt.Sprintf("%s dragged round %d over the line in a 1v%d.", player, ins.Round, vs)
		}
		return fmt.Sprintf("%s clutched a 1v%d in round %d.", player, vs, ins.Round)
	case "knife_kill":
		return fmt.Sprintf("%s committed a war crime on %s with the knife (round %d).", player, victim, ins.Round)
	case "zeus_kill":
		return fmt.Sprintf("%s tased %s into next week (round %d).", player, victim, ins.Round)
	case "knife_team_kill":
		return fmt.Sprintf("%s backstabbed %s. Peak teamwork (round %d).", player, victim, ins.Round)
	case "style_kill":
		return renderStyleKill(player, ins)
	case "team_kill":
		weapon, _ := ins.Detail["weapon"].(string)
		return fmt.Sprintf("%s team-killed %s in round %d (%s).", player, victim, ins.Round, weapon)
	case "entry_king":
		count, _ := ins.Detail["first_kills"].(int)
		return fmt.Sprintf("%s opened more doors than an estate agent (%d entry frags).", player, count)
	case "flash_assist":
		return fmt.Sprintf("%s set up %s with a full white (round %d).", player, victim, ins.Round)
	case "instant_trade":
		count, _ := ins.Detail["trades"].(int)
		return fmt.Sprintf("%s refragged at professional speed (%d instant trades).", player, count)
	case "bomb_god":
		plants, _ := ins.Detail["plants"].(int)
		defuses, _ := ins.Detail["defuses"].(int)
		switch {
		case plants > 0 && defuses > 0:
			return fmt.Sprintf("%s actually played the objective (%d plants, %d defuses).", player, plants, defuses)
		case plants > 0:
			return fmt.Sprintf("%s actually played the objective (%d plants).", player, plants)
		case defuses > 0:
			return fmt.Sprintf("%s actually played the objective (%d defuses).", player, defuses)
		default:
			return fmt.Sprintf("%s actually played the objective.", player)
		}
	case "mvp":
		count, _ := ins.Detail["mvps"].(int)
		return fmt.Sprintf("%s stacked %d MVPs this match.", player, count)
	case "lurker_tax":
		vs, _ := ins.Detail["vs"].(int)
		return fmt.Sprintf("%s lurked into a 1v%d and chose death (round %d).", player, vs, ins.Round)
	case "refund_request":
		return fmt.Sprintf("%s bought an AWP and donated it to the other team (round %d).", player, ins.Round)
	case "entry_victim":
		count, _ := ins.Detail["first_deaths"].(int)
		return fmt.Sprintf("%s opened the site for the other team (%d first deaths).", player, count)
	case "bomb_mule":
		count, _ := ins.Detail["deaths"].(int)
		return fmt.Sprintf("%s died with the bomb %d times. Reliable courier, unreliable survivor.", player, count)
	case "flash_tax":
		count, _ := ins.Detail["blinds"].(int)
		return fmt.Sprintf("%s paid the flash tax %d times.", player, count)
	case "team_flash":
		count, _ := ins.Detail["blinds"].(int)
		return fmt.Sprintf("%s blinded teammates %d times. Vision optional.", player, count)
	case "kit_dodger":
		return fmt.Sprintf("%s had kit money and skipped the kit (round %d).", player, ins.Round)
	case "economy_terrorist":
		return fmt.Sprintf("%s single-handedly wrecked the team economy (round %d).", player, ins.Round)
	case "defuse_interrupted":
		count, _ := ins.Detail["interruptions"].(int)
		if count > 2 {
			return fmt.Sprintf("%s almost had it. %d times.", player, count)
		}
		return fmt.Sprintf("%s almost had it. Twice.", player)
	default:
		return fmt.Sprintf("%s was technically in the server (round %d).", player, ins.Round)
	}
}

func renderStyleKill(player string, ins analyser.Insight) string {
	var parts []string
	if noscope, _ := ins.Detail["noscope"].(bool); noscope {
		parts = append(parts, "no-scope")
	}
	if wallbang, _ := ins.Detail["wallbang"].(bool); wallbang {
		parts = append(parts, "wallbang")
	}
	if len(parts) == 0 {
		return fmt.Sprintf("%s had style points in round %d.", player, ins.Round)
	}
	return fmt.Sprintf("%s hit a %s in round %d.", player, strings.Join(parts, " + "), ins.Round)
}

// FormatRecapDebug prints ranked and dropped candidates with scores for -debug mode.
func FormatRecapDebug(recap analyser.Recap) string {
	var sb strings.Builder

	if recap.Headline != nil {
		sb.WriteString(fmt.Sprintf("Headline: %s round %d (score %d)\n",
			recap.Headline.TriggerType, recap.Headline.Round, recap.Headline.Score))
	} else {
		sb.WriteString("Headline: none\n")
	}

	sb.WriteString("\nPublic moments:\n")
	if len(recap.Public) == 0 {
		sb.WriteString("  (none)\n")
	}
	for i, ins := range recap.Public {
		sb.WriteString(fmt.Sprintf("  %d. %s %s round %d score=%d\n",
			i+1, ins.PlayerName, ins.TriggerType, ins.Round, ins.Score))
	}

	sb.WriteString("\nDropped candidates:\n")
	if len(recap.Dropped) == 0 {
		sb.WriteString("  (none)\n")
	}
	for _, d := range recap.Dropped {
		sb.WriteString(fmt.Sprintf("  %s %s round %d score=%d reason=%s\n",
			d.Insight.PlayerName, d.Insight.TriggerType, d.Insight.Round, d.Insight.Score, d.Reason))
	}

	return sb.String()
}

package bot

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/DodgeWhale/enjoyable-stats/analyser"
	"github.com/DodgeWhale/enjoyable-stats/db"
	"github.com/bwmarrin/discordgo"
)

type Bot struct {
	session *discordgo.Session
	db      *db.DB
}

// New creates a Bot. When database is non-nil the bot registers slash-command
// handlers on the session (used by the `bot` subcommand). When database is nil
// the Bot is suitable for posting insights only (used by `analyse`).
func New(token string, database *db.DB) (*Bot, error) {
	s, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("bot: create session: %w", err)
	}
	b := &Bot{session: s, db: database}
	if database != nil {
		s.Identify.Intents = discordgo.IntentGuilds
		s.AddHandler(b.onReady)
		s.AddHandler(b.onInteraction)
	}
	return b, nil
}

func (b *Bot) Open() error {
	if err := b.session.Open(); err != nil {
		return fmt.Errorf("bot: open session: %w", err)
	}
	return nil
}

func (b *Bot) Close() error {
	return b.session.Close()
}

func (b *Bot) onReady(s *discordgo.Session, r *discordgo.Ready) {
	for _, g := range r.Guilds {
		if err := registerCommands(s, r.User.ID, g.ID); err != nil {
			slog.Error("failed to register commands", "guild", g.ID, "err", err)
		}
	}
	slog.Info("bot ready", "user", r.User.Username)
}

func (b *Bot) onInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}
	data := i.ApplicationCommandData()
	switch data.Name {
	case "link-steam":
		b.handleLinkSteam(s, i, data)
	case "unlink-steam":
		b.handleUnlinkSteam(s, i)
	case "analyse-demo":
		b.handleAnalyseDemo(s, i, data)
	}
}

// PostInsights formats and sends insights to the given channel.
// players maps steamID → discordUserID for @mentions.
func (b *Bot) PostInsights(channelID string, insights []analyser.Insight, players map[string]string) error {
	if len(insights) == 0 {
		return nil
	}

	grouped := make(map[string][]analyser.Insight)
	for _, ins := range insights {
		grouped[ins.SteamID] = append(grouped[ins.SteamID], ins)
	}

	for steamID, playerInsights := range grouped {
		mention := steamID
		if discordID, ok := players[steamID]; ok {
			mention = "<@" + discordID + ">"
		}

		var sb strings.Builder
		sb.WriteString(mention)
		sb.WriteString(" highlights:\n")

		var mvpIns []analyser.Insight
		for _, ins := range playerInsights {
			if ins.TriggerType == "mvp" {
				mvpIns = append(mvpIns, ins)
				continue
			}
			sb.WriteString(formatInsight(ins))
			sb.WriteString("\n")
		}
		if len(mvpIns) > 0 {
			sb.WriteString(formatMVPInsights(mvpIns))
			sb.WriteString("\n")
		}

		if _, err := b.session.ChannelMessageSend(channelID, sb.String()); err != nil {
			return fmt.Errorf("bot: post insights: %w", err)
		}
	}
	return nil
}

func formatInsight(ins analyser.Insight) string {
	switch ins.TriggerType {
	case "ace":
		return fmt.Sprintf("  🎯 ACE in round %d", ins.Round)
	case "team_kill":
		victim, _ := ins.Detail["victim"].(string)
		weapon, _ := ins.Detail["weapon"].(string)
		return fmt.Sprintf("  💀 Team kill in round %d (victim: %s, weapon: %s)", ins.Round, victim, weapon)
	case "clutch":
		vs, _ := ins.Detail["vs"].(int)
		return fmt.Sprintf("  🏆 1v%d clutch in round %d", vs, ins.Round)
	case "lurker_tax":
		vs, _ := ins.Detail["vs"].(int)
		return fmt.Sprintf("  🐌 Lurked into a 1v%d and chose death (round %d)", vs, ins.Round)
	case "bomb_god":
		return "  💣 Actually played the objective unlike everyone else."
	case "entry_king":
		count, _ := ins.Detail["first_kills"].(int)
		return fmt.Sprintf("  🚪 Opened more doors than an estate agent (%d entry frags)", count)
	case "refund_request":
		return fmt.Sprintf("  💸 £4,750 decoy grenade (round %d)", ins.Round)
	default:
		return fmt.Sprintf("  [%s] round %d", ins.TriggerType, ins.Round)
	}
}

// formatMVPInsights consolidates one or more MVP insights for a single player
// into one line listing every round they earned an MVP.
func formatMVPInsights(ins []analyser.Insight) string {
	last := ins[len(ins)-1]
	count, _ := last.Detail["mvps"].(int)
	rounds, _ := last.Detail["rounds"].([]int)

	roundStrs := make([]string, len(rounds))
	for i, r := range rounds {
		roundStrs[i] = fmt.Sprintf("%d", r)
	}

	label := "round"
	if len(roundStrs) > 1 {
		label = "rounds"
	}
	return fmt.Sprintf("  ⭐ %d MVPs (%s %s)", count, label, strings.Join(roundStrs, ", "))
}

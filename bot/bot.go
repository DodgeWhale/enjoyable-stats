package bot

import (
	"fmt"
	"log/slog"

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

// FormatInsights builds a ranked recap from raw insights.
// players maps steamID → discordUserID for @mentions.
func FormatInsights(insights []analyser.Insight, players map[string]string, demoID, mapName string, rounds int) []string {
	recap := analyser.BuildRecap(insights, demoID, mapName, rounds)
	return FormatRecap(recap, players, false)
}

// PostInsights formats and sends the recap to the given channel.
// players maps steamID → discordUserID for @mentions.
func (b *Bot) PostInsights(channelID string, insights []analyser.Insight, players map[string]string, demoID, mapName string, rounds int) error {
	for _, msg := range FormatInsights(insights, players, demoID, mapName, rounds) {
		if _, err := b.session.ChannelMessageSend(channelID, msg); err != nil {
			return fmt.Errorf("bot: post insights: %w", err)
		}
	}
	return nil
}

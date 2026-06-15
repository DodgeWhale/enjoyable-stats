package bot

import (
	"fmt"
	"log/slog"
	"regexp"

	"github.com/bwmarrin/discordgo"
)

var steamIDRe = regexp.MustCompile(`^\d{17}$`)

var slashCommands = []*discordgo.ApplicationCommand{
	{
		Name:        "link-steam",
		Description: "Link your Steam ID to receive CS2 demo insights",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "steam-id",
				Description: "Your 17-digit Steam ID (SteamID64)",
				Required:    true,
			},
		},
	},
	{
		Name:        "unlink-steam",
		Description: "Unlink your Steam ID from this server",
	},
}

func registerCommands(s *discordgo.Session, appID, guildID string) error {
	for _, cmd := range slashCommands {
		if _, err := s.ApplicationCommandCreate(appID, guildID, cmd); err != nil {
			return fmt.Errorf("commands: register %s: %w", cmd.Name, err)
		}
	}
	return nil
}

func (b *Bot) handleLinkSteam(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	opt := data.Options[0]
	steamID := opt.StringValue()

	if !steamIDRe.MatchString(steamID) {
		respond(s, i, "Invalid Steam ID. Please provide a 17-digit numeric Steam ID (SteamID64).")
		return
	}

	guildID := i.GuildID
	discordUserID := i.Member.User.ID

	if err := b.db.InsertPlayer(discordUserID, steamID, guildID); err != nil {
		slog.Error("link-steam: insert player", "err", err)
		respond(s, i, "Failed to link your Steam ID. Please try again.")
		return
	}

	respond(s, i, fmt.Sprintf("Successfully linked Steam ID `%s` to your account.", steamID))
}

func (b *Bot) handleUnlinkSteam(s *discordgo.Session, i *discordgo.InteractionCreate) {
	discordUserID := i.Member.User.ID

	if err := b.db.DeletePlayer(discordUserID); err != nil {
		slog.Error("unlink-steam: delete player", "err", err)
		respond(s, i, "Failed to unlink your Steam ID. Please try again.")
		return
	}

	respond(s, i, "Successfully unlinked your Steam ID.")
}

func respond(s *discordgo.Session, i *discordgo.InteractionCreate, msg string) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: msg,
		},
	})
	if err != nil {
		slog.Error("interaction respond failed", "err", err)
	}
}

// ValidateSteamID returns true if s is a 17-digit numeric string.
func ValidateSteamID(s string) bool {
	return steamIDRe.MatchString(s)
}

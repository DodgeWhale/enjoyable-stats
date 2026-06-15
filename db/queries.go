package db

import (
	"encoding/json"
	"fmt"
)

func (d *DB) InsertPlayer(discordUserID, steamID, guildID string) error {
	_, err := d.conn.Exec(
		`INSERT OR REPLACE INTO players (discord_user_id, steam_id, guild_id) VALUES (?, ?, ?)`,
		discordUserID, steamID, guildID,
	)
	if err != nil {
		return fmt.Errorf("db: InsertPlayer: %w", err)
	}
	return nil
}

func (d *DB) DeletePlayer(discordUserID string) error {
	_, err := d.conn.Exec(
		`DELETE FROM players WHERE discord_user_id = ?`,
		discordUserID,
	)
	if err != nil {
		return fmt.Errorf("db: DeletePlayer: %w", err)
	}
	return nil
}

func (d *DB) GetTrackedSteamIDs() (map[string]bool, error) {
	rows, err := d.conn.Query(`SELECT steam_id FROM players`)
	if err != nil {
		return nil, fmt.Errorf("db: GetTrackedSteamIDs: %w", err)
	}
	defer rows.Close()

	result := make(map[string]bool)
	for rows.Next() {
		var steamID string
		if err := rows.Scan(&steamID); err != nil {
			return nil, fmt.Errorf("db: GetTrackedSteamIDs scan: %w", err)
		}
		result[steamID] = true
	}
	return result, rows.Err()
}

func (d *DB) GetPlayerMentions() (map[string]string, error) {
	rows, err := d.conn.Query(`SELECT steam_id, discord_user_id FROM players`)
	if err != nil {
		return nil, fmt.Errorf("db: GetPlayerMentions: %w", err)
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var steamID, discordUserID string
		if err := rows.Scan(&steamID, &discordUserID); err != nil {
			return nil, fmt.Errorf("db: GetPlayerMentions scan: %w", err)
		}
		result[steamID] = discordUserID
	}
	return result, rows.Err()
}

func (d *DB) InsertInsight(demoFile, steamID, triggerType string, round int, detail map[string]any) error {
	detailJSON, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("db: InsertInsight marshal: %w", err)
	}
	_, err = d.conn.Exec(
		`INSERT INTO demo_insights (demo_file, steam_id, trigger_type, round_number, detail_json) VALUES (?, ?, ?, ?, ?)`,
		demoFile, steamID, triggerType, round, string(detailJSON),
	)
	if err != nil {
		return fmt.Errorf("db: InsertInsight: %w", err)
	}
	return nil
}

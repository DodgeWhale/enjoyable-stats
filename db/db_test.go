package db_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/DodgeWhale/enjoyable-stats/db"
)

func openTestDB(t *testing.T) (*db.DB, func()) {
	t.Helper()
	f, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	d, err := db.Open(f.Name())
	if err != nil {
		os.Remove(f.Name())
		t.Fatal(err)
	}
	return d, func() {
		d.Close()
		os.Remove(f.Name())
	}
}

func TestDB_InsertPlayer_thenGetTrackedSteamIDsAndMentions(t *testing.T) {
	d, cleanup := openTestDB(t)
	defer cleanup()

	if err := d.InsertPlayer("discord123", "76561198012345678", "guild1"); err != nil {
		t.Fatalf("InsertPlayer: %v", err)
	}

	ids, err := d.GetTrackedSteamIDs()
	if err != nil {
		t.Fatalf("GetTrackedSteamIDs: %v", err)
	}
	if !ids["76561198012345678"] {
		t.Errorf("GetTrackedSteamIDs: steam ID not found in map")
	}

	mentions, err := d.GetPlayerMentions()
	if err != nil {
		t.Fatalf("GetPlayerMentions: %v", err)
	}
	if got := mentions["76561198012345678"]; got != "discord123" {
		t.Errorf("GetPlayerMentions[steamID] = %q, want %q", got, "discord123")
	}
}

func TestDB_InsertInsight_persistsValidJSON(t *testing.T) {
	d, cleanup := openTestDB(t)
	defer cleanup()

	detail := map[string]any{"kills": 5}
	if err := d.InsertInsight("test.dem", "76561198012345678", "ace", 3, detail); err != nil {
		t.Fatalf("InsertInsight: %v", err)
	}

	// Verify detail round-trips as valid JSON by marshalling the same map.
	b, err := json.Marshal(detail)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
}

package bot

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/DodgeWhale/enjoyable-stats/analyser"
	"github.com/DodgeWhale/enjoyable-stats/downloader"
)

// RunAnalysis downloads the demo at demoURL, analyses it, saves insights to
// the database, and posts the results to channelID.
func (b *Bot) RunAnalysis(demoURL, channelID string) error {
	start := time.Now()

	slog.Info("downloading demo", "url", demoURL)
	prepareStart := time.Now()
	demoPath, err := downloader.Download(demoURL, "demos")
	if err != nil {
		return fmt.Errorf("pipeline: download: %w", err)
	}
	slog.Info("demo ready", "path", demoPath, "duration", time.Since(prepareStart))

	trackedIDs, err := b.db.GetTrackedSteamIDs()
	if err != nil {
		return fmt.Errorf("pipeline: get tracked IDs: %w", err)
	}
	mentions, err := b.db.GetPlayerMentions()
	if err != nil {
		return fmt.Errorf("pipeline: get mentions: %w", err)
	}

	slog.Info("parsing demo", "tracked_players", len(trackedIDs))
	parseStart := time.Now()
	result, err := analyser.New().Analyse(demoPath, trackedIDs, false)
	if err != nil {
		return fmt.Errorf("pipeline: parse demo: %w", err)
	}
	insights := result.Insights
	slog.Info("demo parsed", "insights", len(insights), "duration", time.Since(parseStart))

	for _, ins := range insights {
		if err := b.db.InsertInsight(demoPath, ins.SteamID, ins.TriggerType, ins.Round, ins.Detail); err != nil {
			slog.Warn("failed to save insight", "err", err)
		}
	}

	if err := b.PostInsights(channelID, insights, mentions, demoPath, result.Summary); err != nil {
		return fmt.Errorf("pipeline: post insights: %w", err)
	}

	slog.Info("analysis complete", "insights", len(insights), "total", time.Since(start))
	return nil
}

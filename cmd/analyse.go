package cmd

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/DodgeWhale/enjoyable-stats/analyser"
	"github.com/DodgeWhale/enjoyable-stats/bot"
	"github.com/DodgeWhale/enjoyable-stats/config"
	"github.com/DodgeWhale/enjoyable-stats/db"
	"github.com/DodgeWhale/enjoyable-stats/downloader"
)

func runAnalyse(args []string) error {
	fs := flag.NewFlagSet("analyse", flag.ContinueOnError)
	demoURL := fs.String("demo", "", "URL of the demo file to download and analyse")
	demoFile := fs.String("file", "", "Path to a local .dem or .dem.bz2 file")
	channelFlag := fs.String("channel", "", "Discord channel ID to post insights (overrides config)")
	debugFlag := fs.Bool("debug", false, "Print insights to the console instead of Discord, and write per-event state snapshots to a JSON file")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("analyse: parse flags: %w", err)
	}
	if *demoURL == "" && *demoFile == "" {
		fs.Usage()
		return fmt.Errorf("analyse: -demo or -file is required")
	}
	if *demoURL != "" && *demoFile != "" {
		return fmt.Errorf("analyse: -demo and -file are mutually exclusive")
	}

	var cfg *config.Config
	var err error
	if *debugFlag {
		cfg, err = config.LoadOptional()
	} else {
		cfg, err = config.Load()
	}
	if err != nil {
		return err
	}

	channelID := cfg.DiscordChannelID
	if *channelFlag != "" {
		channelID = *channelFlag
	}
	if !*debugFlag && channelID == "" {
		return fmt.Errorf("analyse: no channel ID provided (set DISCORD_CHANNEL_ID or use -channel)")
	}

	database, err := db.Open("cs2analyser.db")
	if err != nil {
		return err
	}
	defer database.Close()

	start := time.Now()

	var demoPath string
	prepareStart := time.Now()
	if *demoFile != "" {
		slog.Info("using local demo", "path", *demoFile)
		demoPath, err = downloader.PrepareLocal(*demoFile)
		if err != nil {
			return fmt.Errorf("analyse: prepare local demo: %w", err)
		}
	} else {
		slog.Info("downloading demo", "url", *demoURL)
		demoPath, err = downloader.Download(*demoURL, "demos")
		if err != nil {
			return fmt.Errorf("analyse: download: %w", err)
		}
	}
	prepareDur := time.Since(prepareStart)

	demoInfo, err := os.Stat(demoPath)
	if err != nil {
		return fmt.Errorf("analyse: stat demo: %w", err)
	}
	slog.Info("demo ready", "path", demoPath, "size_bytes", demoInfo.Size(), "duration", prepareDur)

	trackedIDs, err := database.GetTrackedSteamIDs()
	if err != nil {
		return fmt.Errorf("analyse: get tracked IDs: %w", err)
	}
	mentions, err := database.GetPlayerMentions()
	if err != nil {
		return fmt.Errorf("analyse: get mentions: %w", err)
	}

	slog.Info("parsing demo", "tracked_players", len(trackedIDs), "debug", *debugFlag)
	parseStart := time.Now()
	a := analyser.New()
	result, err := a.Analyse(demoPath, trackedIDs, *debugFlag)
	if err != nil {
		return fmt.Errorf("analyse: parse demo: %w", err)
	}
	insights := result.Insights
	parseDur := time.Since(parseStart)
	slog.Info("demo parsed", "insights", len(insights), "duration", parseDur)

	recap := analyser.BuildRecap(insights, demoPath, result.MapName, result.Rounds)
	recap.Trace = append(result.NameTrace, recap.Trace...)

	if *debugFlag {
		statePath := strings.TrimSuffix(demoPath, ".dem") + ".state.json"
		if err := analyser.WriteStateLog(statePath, result.StateLog); err != nil {
			return fmt.Errorf("analyse: write state log: %w", err)
		}
		slog.Info("state log written", "path", statePath, "snapshots", len(result.StateLog))

		recapPath := strings.TrimSuffix(demoPath, ".dem") + ".recap.json"
		if err := analyser.WriteRecapLog(recapPath, recap); err != nil {
			return fmt.Errorf("analyse: write recap log: %w", err)
		}
		slog.Info("recap log written", "path", recapPath)
	}

	saveStart := time.Now()
	saved := 0
	for _, ins := range insights {
		if err := database.InsertInsight(demoPath, ins.SteamID, ins.TriggerType, ins.Round, ins.Detail); err != nil {
			slog.Warn("failed to save insight", "err", err)
			continue
		}
		saved++
	}
	saveDur := time.Since(saveStart)

	var postDur time.Duration
	if *debugFlag {
		messages := bot.FormatRecap(recap, mentions, true)
		if len(messages) == 0 {
			fmt.Println("No insights.")
		} else {
			for _, msg := range messages {
				fmt.Println(msg)
			}
		}
		fmt.Print(bot.FormatRecapDebug(recap))
	} else {
		b, err := bot.New(cfg.DiscordToken, nil)
		if err != nil {
			return fmt.Errorf("analyse: create discord session: %w", err)
		}
		postStart := time.Now()
		if err := b.PostInsights(channelID, insights, mentions, demoPath, result.MapName, result.Rounds); err != nil {
			return fmt.Errorf("analyse: post insights: %w", err)
		}
		postDur = time.Since(postStart)
		if err := b.Close(); err != nil {
			slog.Warn("failed to close discord session", "err", err)
		}
	}

	totalDur := time.Since(start)
	logArgs := []any{
		"insights", len(insights),
		"insights_saved", saved,
		"tracked_players", len(trackedIDs),
		"demo_size_bytes", demoInfo.Size(),
		"prepare", prepareDur,
		"parse", parseDur,
		"save", saveDur,
		"total", totalDur,
	}
	if !*debugFlag {
		logArgs = append(logArgs, "post", postDur)
	}
	slog.Info("analysis complete", logArgs...)
	return nil
}

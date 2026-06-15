package cmd

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os/signal"
	"syscall"

	"github.com/DodgeWhale/enjoyable-stats/bot"
	"github.com/DodgeWhale/enjoyable-stats/config"
	"github.com/DodgeWhale/enjoyable-stats/db"
)

func runBot(args []string) error {
	fs := flag.NewFlagSet("bot", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("bot: parse flags: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	database, err := db.Open("cs2analyser.db")
	if err != nil {
		return err
	}
	defer database.Close()

	b, err := bot.New(cfg.DiscordToken, database)
	if err != nil {
		return err
	}

	if err := b.Open(); err != nil {
		return err
	}
	defer b.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	slog.Info("bot running — press Ctrl+C to stop")
	<-ctx.Done()
	slog.Info("shutting down")
	return nil
}

package main

import (
	"log/slog"
	"os"

	"github.com/DodgeWhale/enjoyable-stats/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

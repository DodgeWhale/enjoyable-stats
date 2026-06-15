package cmd

import (
	"fmt"
	"os"
)

func Execute() error {
	if len(os.Args) < 2 {
		printUsage()
		return fmt.Errorf("no subcommand provided")
	}

	switch os.Args[1] {
	case "bot":
		return runBot(os.Args[2:])
	case "analyse":
		return runAnalyse(os.Args[2:])
	default:
		printUsage()
		return fmt.Errorf("unknown subcommand: %s", os.Args[1])
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  enjoyable-stats bot")
	fmt.Fprintln(os.Stderr, "  enjoyable-stats analyse -demo <url> | -file <path> [-channel <id>] [-debug]")
}

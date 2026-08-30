package main

import (
	"os"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/subcommands/account"
	"github.com/oskarhane/google-cli/internal/subcommands/calendar"
	"github.com/oskarhane/google-cli/internal/subcommands/gmail"
)

func main() {
	cfg := app.NewConfig()
	root := app.NewRootCommand(cfg)
	root.AddCommand(
		account.NewCmd(cfg),
		gmail.NewCmd(cfg),
		calendar.NewCmd(cfg),
	)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

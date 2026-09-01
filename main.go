package main

import (
	"os"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/subcommands/account"
	"github.com/oskarhane/google-cli/internal/subcommands/calendar"
	"github.com/oskarhane/google-cli/internal/subcommands/docs"
	"github.com/oskarhane/google-cli/internal/subcommands/drive"
	"github.com/oskarhane/google-cli/internal/subcommands/gmail"
	"github.com/oskarhane/google-cli/internal/subcommands/sheets"
	"github.com/oskarhane/google-cli/internal/subcommands/skill"
	"github.com/oskarhane/google-cli/internal/subcommands/slides"
	"github.com/oskarhane/google-cli/internal/subcommands/update"
)

func main() {
	cfg := app.NewConfig()
	root := app.NewRootCommand(cfg)
	root.AddCommand(
		account.NewCmd(cfg),
		gmail.NewCmd(cfg),
		calendar.NewCmd(cfg),
		drive.NewCmd(cfg),
		docs.NewCmd(cfg),
		sheets.NewCmd(cfg),
		slides.NewCmd(cfg),
		skill.NewCmd(cfg),
		update.NewCmd(cfg),
	)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

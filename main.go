package main

import (
	"os"

	"github.com/oskarhane/google-cli/internal/app"
)

func main() {
	root := app.NewRootCommand(app.NewConfig())
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

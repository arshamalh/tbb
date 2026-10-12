// Command basic is a small botbase bot showing the main pieces: config,
// translations, the language picker, inline-button callbacks, a multi-step
// scene with app-specific session state, and a localized reply-keyboard
// cancel button.
//
//	TELEGRAM__TOKEN=123:abc go run ./examples/basic
//
// With no DB__DSN set, data goes to ./bot.db (SQLite).
package main

import (
	"log/slog"
	"os"

	"github.com/arshamalh/botbase"
)

// State is this bot's own session data, persisted with every session.
type State struct {
	Rating string `json:"rating,omitempty"`
}

// Config embeds the framework config; the bot could add its own fields.
type Config struct {
	botbase.Config
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	var cfg Config
	if err := botbase.LoadConfig(&cfg); err != nil {
		logger.Error("load config", "err", err)
		os.Exit(1)
	}

	app, err := botbase.New[State](cfg.Config, botbase.WithLogger[State](logger))
	if err != nil {
		logger.Error("create app", "err", err)
		os.Exit(1)
	}

	registerLanguages(app.I18n())

	h := &handlers{app: app}
	h.register()

	if err := app.Start(); err != nil {
		logger.Error("stopped with error", "err", err)
		os.Exit(1)
	}
}

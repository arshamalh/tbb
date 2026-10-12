package tbb

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	"github.com/uptrace/bun"
)

// Handler handles an update with the framework context.
type Handler[T any] func(c Ctx[T]) error

// Middleware wraps a Handler, e.g. to restrict access or add logging.
type Middleware[T any] func(next Handler[T]) Handler[T]

// Config is the framework configuration. It can be embedded in an app's own
// config struct and filled by LoadConfig.
type Config struct {
	Telegram TelegramConfig `json:"telegram"`
	DB       DBConfig       `json:"db"`

	// DefaultLanguage is used for chats that haven't chosen a language and
	// for keys missing in the chosen one.
	DefaultLanguage string `json:"default_language" default:"en"`
}

// TelegramConfig configures the Bot API connection.
type TelegramConfig struct {
	Token string `json:"token" validate:"required"`
	// URL overrides the Bot API server, e.g. a local telegram-bot-api.
	URL string `json:"url"`
	// PollerTimeout is the long-polling timeout.
	PollerTimeout time.Duration `json:"poller_timeout" default:"10s"`
}

// App is a Telegram bot with sessions, scenes, localization and persistence.
// T is the app-specific session state; use struct{} when there is none.
type App[T any] struct {
	bot      *Bot
	i18n     *I18n
	store    Store
	sessions *SessionStore[T]
	logger   *slog.Logger

	db     *bun.DB
	ownsDB bool

	settings        Settings
	middlewares     []Middleware[T]
	literalTexts    map[string]Handler[T]
	replyKeys       []replyRoute[T]
	scenes          map[Scene]*SceneRouter[T]
	defaultText     Handler[T]
	defaultCallback Handler[T]

	shutdownTimeout time.Duration
	inflight        sync.WaitGroup
}

type replyRoute[T any] struct {
	key     string
	handler Handler[T]
}

// AppOption customizes an App. (Option is telebot's send option.)
type AppOption[T any] func(*App[T])

// WithLogger sets the logger (slog.Default() otherwise).
func WithLogger[T any](l *slog.Logger) AppOption[T] {
	return func(a *App[T]) { a.logger = l }
}

// WithI18n sets the translation registry. By default an empty one with the
// configured DefaultLanguage is created; register languages with
// app.I18n().Register.
func WithI18n[T any](i *I18n) AppOption[T] {
	return func(a *App[T]) { a.i18n = i }
}

// WithStore replaces the default SQL store, e.g. with NewMemoryStore() in
// tests. No database is opened when a store is given.
func WithStore[T any](s Store) AppOption[T] {
	return func(a *App[T]) { a.store = s }
}

// WithDB uses an already open database instead of opening one from
// Config.DB. The framework migrates its tables in it but does not close it.
func WithDB[T any](db *bun.DB) AppOption[T] {
	return func(a *App[T]) { a.db = db }
}

// WithSettings adjusts the telebot Settings before the bot is created,
// e.g. to use a webhook poller, set Offline for tests, or change ParseMode
// (ModeMarkdownV2 by default).
func WithSettings[T any](fn func(*Settings)) AppOption[T] {
	return func(a *App[T]) { fn(&a.settings) }
}

// WithShutdownTimeout bounds how long Run waits for running handlers after
// polling stops (10s by default).
func WithShutdownTimeout[T any](d time.Duration) AppOption[T] {
	return func(a *App[T]) { a.shutdownTimeout = d }
}

// New creates the bot, opens and migrates the database (SQLite by default)
// and installs the text and callback routers.
func New[T any](cfg Config, opts ...AppOption[T]) (*App[T], error) {
	a := &App[T]{
		literalTexts:    make(map[string]Handler[T]),
		scenes:          make(map[Scene]*SceneRouter[T]),
		shutdownTimeout: 10 * time.Second,
		settings: Settings{
			URL:       cfg.Telegram.URL,
			Token:     cfg.Telegram.Token,
			Poller:    &LongPoller{Timeout: cfg.Telegram.PollerTimeout},
			ParseMode: ModeMarkdownV2,
		},
	}
	if a.settings.Poller.(*LongPoller).Timeout == 0 {
		a.settings.Poller = &LongPoller{Timeout: 10 * time.Second}
	}
	for _, opt := range opts {
		opt(a)
	}

	if a.logger == nil {
		a.logger = slog.Default()
	}
	if a.i18n == nil {
		a.i18n = NewI18n(cfg.DefaultLanguage)
		a.i18n.SetLogger(a.logger)
	}

	if a.store == nil {
		if a.db == nil {
			db, err := OpenDB(cfg.DB)
			if err != nil {
				return nil, err
			}
			a.db, a.ownsDB = db, true
		}
		sqlStore := NewSQLStore(a.db)
		if err := sqlStore.Migrate(context.Background()); err != nil {
			a.Close()
			return nil, fmt.Errorf("migrate: %w", err)
		}
		a.store = sqlStore
	}
	a.sessions = NewSessionStore[T](a.store)

	if a.settings.OnError == nil {
		a.settings.OnError = a.onError
	}
	bot, err := NewBot(a.settings)
	if err != nil {
		a.Close()
		return nil, fmt.Errorf("create bot: %w", err)
	}
	a.bot = bot

	a.defaultText = func(c Ctx[T]) error {
		c.Logger().Debug("no handler for text", "scene", c.GetScene(), "question", c.GetCurrentQuestion())
		return nil
	}
	a.defaultCallback = func(c Ctx[T]) error {
		cb := c.Callback()
		c.Logger().Warn("no handler for callback", "unique", cb.Unique, "data", cb.Data)
		return c.Respond()
	}
	bot.Handle(OnText, a.wrap(a.routeText))
	bot.Handle(OnCallback, a.wrap(func(c Ctx[T]) error { return a.defaultCallback(c) }))

	return a, nil
}

// Bot returns the underlying bot for anything the framework doesn't cover.
func (a *App[T]) Bot() *Bot { return a.bot }

// I18n returns the translation registry.
func (a *App[T]) I18n() *I18n { return a.i18n }

// Store returns the store holding accounts and sessions.
func (a *App[T]) Store() Store { return a.store }

// Sessions returns the session cache, e.g. to update a session from outside
// a handler (a scheduler, a webhook). Hold Lock while doing so and Save after.
func (a *App[T]) Sessions() *SessionStore[T] { return a.sessions }

// DB returns the database handle so the app can keep its own tables in the
// same database. It is nil when a custom Store was given.
func (a *App[T]) DB() *bun.DB { return a.db }

// Logger returns the app's logger.
func (a *App[T]) Logger() *slog.Logger { return a.logger }

// Use adds middlewares that run around every framework handler, in order.
func (a *App[T]) Use(mw ...Middleware[T]) {
	a.middlewares = append(a.middlewares, mw...)
}

// Command registers a handler for a command such as "/start".
func (a *App[T]) Command(command string, h Handler[T]) {
	a.bot.Handle(command, a.wrap(h))
}

// Callback registers the handler of an inline button.
func (a *App[T]) Callback(key BtnKey, h Handler[T]) {
	a.bot.Handle(key.Endpoint(), a.wrap(h))
}

// Reply registers the handler of a reply-keyboard button whose label is the
// translation of i18nKey: a message equal to the key's text in any
// registered language triggers it, whatever the chat's current scene.
func (a *App[T]) Reply(i18nKey string, h Handler[T]) {
	a.replyKeys = append(a.replyKeys, replyRoute[T]{key: i18nKey, handler: h})
}

// Text registers a handler for messages exactly equal to text, whatever the
// chat's current scene.
func (a *App[T]) Text(text string, h Handler[T]) {
	a.literalTexts[text] = h
}

// Scene returns the router of a scene, creating it on first use. Text
// messages of chats in that scene are dispatched by their current question.
func (a *App[T]) Scene(scene Scene) *SceneRouter[T] {
	r, ok := a.scenes[scene]
	if !ok {
		r = &SceneRouter[T]{scene: scene, handlers: make(map[Question]Handler[T])}
		a.scenes[scene] = r
	}
	return r
}

// DefaultText handles text that no Text, Reply or Scene route matched.
// By default such messages are ignored.
func (a *App[T]) DefaultText(h Handler[T]) { a.defaultText = h }

// DefaultCallback handles button presses without a registered Callback.
// By default the spinner is dismissed and a warning is logged. As for every
// callback, button data is already split: Callback().Unique holds the
// unregistered unique (empty for data not produced by a BtnKey), Data the
// payload and RawData the original string.
func (a *App[T]) DefaultCallback(h Handler[T]) { a.defaultCallback = h }

// Handle registers a handler for any other telebot endpoint, such as
// OnDocument, OnPhoto, OnCheckout or OnPayment, with the framework context.
// OnText and OnCallback are owned by the routers: use DefaultText and
// DefaultCallback instead.
func (a *App[T]) Handle(endpoint any, h Handler[T]) {
	switch endpoint {
	case OnText:
		a.DefaultText(h)
	case OnCallback:
		a.DefaultCallback(h)
	default:
		a.bot.Handle(endpoint, a.wrap(h))
	}
}

// SceneRouter dispatches the text messages of one scene by question.
type SceneRouter[T any] struct {
	scene       Scene
	handlers    map[Question]Handler[T]
	defaultFunc Handler[T]
}

// On registers the handler answering question q.
func (r *SceneRouter[T]) On(q Question, h Handler[T]) *SceneRouter[T] {
	r.handlers[q] = h
	return r
}

// Default handles the scene's questions that have no handler of their own.
func (r *SceneRouter[T]) Default(h Handler[T]) *SceneRouter[T] {
	r.defaultFunc = h
	return r
}

func (a *App[T]) routeText(c Ctx[T]) error {
	text := c.Text()
	if h, ok := a.literalTexts[text]; ok {
		return h(c)
	}
	for _, route := range a.replyKeys {
		for _, label := range a.i18n.Texts(route.key) {
			if label == text {
				return route.handler(c)
			}
		}
	}
	if scene := c.GetScene(); scene != 0 {
		if r, ok := a.scenes[scene]; ok {
			if h, ok := r.handlers[c.GetCurrentQuestion()]; ok {
				return h(c)
			}
			if r.defaultFunc != nil {
				return r.defaultFunc(c)
			}
		} else {
			c.Logger().Warn("no router for scene", "scene", scene)
		}
	}
	return a.defaultText(c)
}

// wrap turns a framework handler into a telebot handler: it serializes the
// chat, loads its session, links the account, runs the middlewares and the
// handler, and persists the session if the handler changed it.
func (a *App[T]) wrap(h Handler[T]) HandlerFunc {
	return func(tc Context) (err error) {
		a.inflight.Add(1)
		defer a.inflight.Done()

		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("tbb: handler panic: %v\n%s", r, debug.Stack())
			}
		}()

		ctx := context.Background()
		chatID := chatIDOf(tc)

		var sess *Session[T]
		if chatID == 0 {
			// Updates without a chat or sender: run with a throwaway session.
			sess = newSession[T](0)
		} else {
			unlock := a.sessions.Lock(chatID)
			defer unlock()
			sess, err = a.sessions.Get(ctx, chatID)
			if err != nil {
				return err
			}
		}

		c := &botCtx[T]{Context: tc, sessionState: sess, app: a}
		before := a.sessions.Snapshot(sess)

		if chatID != 0 {
			a.linkAccount(ctx, c)
		}

		final := h
		for i := len(a.middlewares) - 1; i >= 0; i-- {
			final = a.middlewares[i](final)
		}
		err = final(c)

		if chatID != 0 {
			if after := a.sessions.Snapshot(sess); string(after) != string(before) {
				if saveErr := a.sessions.Save(ctx, sess); saveErr != nil {
					a.logger.Error("save session", "chat_id", chatID, "err", saveErr)
				}
			}
		}
		return err
	}
}

// linkAccount makes sure a private chat's session points at its account and,
// when the session has no language yet, restores the one stored on it.
func (a *App[T]) linkAccount(ctx context.Context, c *botCtx[T]) {
	if c.UID != 0 {
		return
	}
	sender, chat := c.Sender(), c.Chat()
	if sender == nil || chat == nil || chat.ID != sender.ID {
		return // only private chats map 1:1 to a user
	}
	acc := accountFromUser(sender)
	if err := a.store.UpsertAccount(ctx, acc); err != nil {
		a.logger.Error("upsert account", "chat_id", chat.ID, "err", err)
		return
	}
	c.UID = acc.ID
	c.account = acc
	if c.sessionState.Language == "" && acc.Language != "" {
		c.sessionState.Language = acc.Language
	}
}

func chatIDOf(c Context) int64 {
	if chat := c.Chat(); chat != nil {
		return chat.ID
	}
	if sender := c.Sender(); sender != nil {
		return sender.ID
	}
	return 0
}

func (a *App[T]) onError(err error, c Context) {
	attrs := []any{"err", err}
	if c != nil {
		if id := chatIDOf(c); id != 0 {
			attrs = append(attrs, "chat_id", id)
		}
	}
	a.logger.Error("handler failed", attrs...)
}

// Run polls for updates until ctx is cancelled, then stops polling, waits
// (up to the shutdown timeout) for running handlers to finish, and closes
// the database if the app opened it.
func (a *App[T]) Run(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		a.bot.Start()
		close(done)
	}()
	a.logger.Info("bot started", "username", a.bot.Me.Username)

	<-ctx.Done()
	a.logger.Info("shutting down")
	a.bot.Stop()
	<-done

	waited := make(chan struct{})
	go func() {
		a.inflight.Wait()
		close(waited)
	}()
	var err error
	select {
	case <-waited:
	case <-time.After(a.shutdownTimeout):
		err = errors.New("tbb: timed out waiting for handlers to finish")
	}
	return errors.Join(err, a.Close())
}

// Start runs the bot until SIGINT or SIGTERM.
func (a *App[T]) Start() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return a.Run(ctx)
}

// Close releases the database if the app opened it. Run calls it on exit.
func (a *App[T]) Close() error {
	if a.ownsDB && a.db != nil {
		a.ownsDB = false
		return a.db.Close()
	}
	return nil
}

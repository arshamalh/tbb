<h1 align="center">tbb (TBotBase)</h1>

<p align="center">
  <b>A batteries-included Telegram bot framework for Go.</b><br>
  Sessions, scenes, i18n, persistence and clean keyboards, built on a fork of telebot v4.
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/arshamalh/tbb"><img src="https://pkg.go.dev/badge/github.com/arshamalh/tbb.svg" alt="Go Reference"></a>
  <a href="https://github.com/arshamalh/tbb/actions/workflows/go.yml"><img src="https://github.com/arshamalh/tbb/actions/workflows/go.yml/badge.svg" alt="CI"></a>
  <img src="https://img.shields.io/badge/go-%E2%89%A51.25-00ADD8?logo=go" alt="Go 1.25+">
  <img src="https://img.shields.io/badge/Bot%20API-10.1-26A5E4?logo=telegram" alt="Bot API 10.1">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-green" alt="MIT"></a>
</p>

---

Telegram libraries hand you updates. Everything a real bot needs on top of
them gets rewritten in every project: remembering where each user is in a
conversation, translations, persisting state, keeping chats free of dead
buttons, and escaping MarkdownV2 so Telegram doesn't reject the message.

**tbb** (short for **TBotBase**) is that layer, distilled from several production bots over the
years (most recently [Boteland](https://github.com/arshamalh/boteland)) and
merged with a fork of [telebot v4](https://github.com/tucnak/telebot) into
**one package**.

```go
app, _ := tbb.New[State](cfg) // SQLite, sessions, i18n: ready

app.Callback(TopUp, func(c tbb.Ctx[State]) error {
	c.Enter(SceneTopUp, AskAmount)                  // multi-step flow
	return c.CleanEdit(c.GetL8d("topup.ask"), kb)   // localized, escaped, no stale buttons
})

app.Scene(SceneTopUp).On(AskAmount, func(c tbb.Ctx[State]) error {
	c.State().Amount = c.Text()                     // typed state, persisted automatically
	c.ResetScene()
	return c.SendL8d("topup.done")
})
```

## Contents

- [Features](#features)
- [Install](#install)
- [Quick start](#quick-start)
- [Tutorial: a two-step flow in 5 minutes](#tutorial-a-two-step-flow-in-5-minutes)
- [How it works](#how-it-works)
- [Guide](#guide)
  - [App and routing](#app-and-routing)
  - [The context: `Ctx[T]`](#the-context-ctxt)
  - [Buttons and callback data](#buttons-and-callback-data)
  - [Sessions and scenes](#sessions-and-scenes)
  - [Localization](#localization)
  - [Parse modes and escaping](#parse-modes-and-escaping)
  - [Clean keyboards](#clean-keyboards)
  - [Database and accounts](#database-and-accounts)
  - [Configuration](#configuration)
  - [Middleware](#middleware)
  - [Long polling, webhooks and shutdown](#long-polling-webhooks-and-shutdown)
  - [Files](#files)
  - [Testing your bot](#testing-your-bot)
  - [Deployment](#deployment)
- [What the fork changes in telebot](#what-the-fork-changes-in-telebot)
- [Project layout](#project-layout)
- [FAQ](#faq)
- [Roadmap](#roadmap)
- [Contributing](#contributing)
- [Credits and license](#credits-and-license)

## Features

| | |
|---|---|
| 🧭 **Scenes and questions** | Route free text by where the user is in a flow, with no hand-rolled state machines. |
| 🧠 **Typed sessions** | `Session[T]` carries the framework's state plus your own struct `T`, saved after every handler that changes it. |
| 🗄 **Database included** | Zero-config SQLite (pure Go, no CGO); Postgres via a DSN. The framework migrates its own tables, and you get the `*bun.DB` for yours. |
| 🌍 **i18n** | Translations as Go maps, `{placeholder}` templates, fallback language, a built-in language picker, reply buttons matched in every language. |
| 🧹 **Clean keyboards** | `CleanEdit`, `CleanSend` and `CleanSendDelete` keep a single "control panel" message instead of a trail of dead buttons. |
| 🛡 **Safe formatting** | Escaping helpers for every parse mode and context. Markup errors come back typed, with the offending spot highlighted. |
| 🔒 **Correct under concurrency** | Updates from one chat run one at a time; different chats run in parallel. Panics are recovered. |
| ⚙️ **Ops-ready** | `slog` logging, env/.env config with validation, graceful shutdown that waits for in-flight handlers. |
| 📦 **All of telebot** | The full Bot API 10.1 client, webhooks, middleware, payments, stars, rich messages. Same package, no adapter layer. |

## Install

```bash
go get github.com/arshamalh/tbb
```

Requires **Go 1.25+**. No C toolchain is needed: SQLite is `modernc.org/sqlite`.

## Quick start

```go
package main

import "github.com/arshamalh/tbb"

type State struct{} // your per-chat data; empty for now

func main() {
	var cfg tbb.Config
	if err := tbb.LoadConfig(&cfg); err != nil { // reads TELEGRAM__TOKEN, .env, ...
		panic(err)
	}

	app, err := tbb.New[State](cfg) // opens ./bot.db and migrates it
	if err != nil {
		panic(err)
	}

	app.I18n().Register("en", "English 🇬🇧", tbb.Translations{
		"hello": "Hi {name}! 👋",
	})

	app.Command("/start", func(c tbb.Ctx[State]) error {
		return c.Send(c.GetL8d("hello", "name", c.Sender().FirstName))
	})

	if err := app.Start(); err != nil { // polls until SIGINT/SIGTERM
		panic(err)
	}
}
```

```bash
TELEGRAM__TOKEN=123456:ABC go run .
```

A complete bot with a language picker, menus, a two-step scene and a
localized Cancel button is in [`examples/basic`](examples/basic):

```bash
TELEGRAM__TOKEN=123456:ABC go run ./examples/basic
```

## Tutorial: a two-step flow in 5 minutes

We'll build a feedback flow: the user taps **Feedback**, sends a rating,
then a comment, and can cancel at any point.

**1. Declare your state, scenes and buttons.** They're plain constants, so
the compiler catches typos.

```go
type State struct {
	Rating string `json:"rating,omitempty"` // saved with the session
}

const (
	SceneFeedback tbb.Scene = iota + 1
)

const (
	AskRating tbb.Question = iota + 1
	AskComment
)

const BtnFeedback tbb.BtnKey = "feedback"
```

**2. Write translations.** One map per language; keys are yours.

```go
var en = tbb.Translations{
	tbb.LangNameKey: "English 🇬🇧",
	"menu":        "What would you like to do?",
	"btn.feedback": "📝 Feedback",
	"btn.cancel":  "Cancel ❌",
	"ask.rating":  "Rate us from 1 to 5.",
	"ask.comment": "Thanks! Any comment? (you gave {rating})",
	"thanks":      "✅ Saved: {rating}/5, \"{comment}\".",
}
```

**3. Wire the handlers.**

```go
type Ctx = tbb.Ctx[State]

func register(app *tbb.App[State]) {
	app.I18n().Register("en", "", en)

	app.Command("/start", menu)
	app.Callback(BtnFeedback, func(c Ctx) error {
		_ = c.Respond()
		c.Enter(SceneFeedback, AskRating)
		return c.CleanEdit(c.GetL8d("ask.rating"))
	})

	app.Scene(SceneFeedback).
		On(AskRating, func(c Ctx) error {
			c.State().Rating = c.Text()
			c.IncQuestion() // → AskComment
			return c.Send(c.GetL8d("ask.comment", "rating", c.Text()))
		}).
		On(AskComment, func(c Ctx) error {
			text := c.GetL8d("thanks", "rating", c.State().Rating, "comment", c.Text())
			c.ResetScene()
			*c.State() = State{}
			if err := c.Send(text); err != nil {
				return err
			}
			return menu(c)
		})

	// A reply-keyboard button, matched by its label in every language and
	// from inside any scene.
	app.Reply("btn.cancel", func(c Ctx) error {
		c.ResetScene()
		return menu(c)
	})
}

func menu(c Ctx) error {
	kb := &tbb.ReplyMarkup{}
	kb.Inline(BtnFeedback.AsRow(c.GetL8dBtn("btn.feedback")))
	return c.CleanEdit(c.GetL8d("menu"), kb)
}
```

**What you didn't write:** session storage, loading, saving, per-chat locking,
callback-data parsing, the scene router, language fallback, MarkdownV2 escaping
of `"`, `!`, `.` and the user's comment, or the cleanup of old keyboards.

## How it works

```mermaid
flowchart LR
    TG[Telegram] -->|update| P[Poller<br/>long polling / webhook]
    P --> R{telebot router}
    R -->|/command| W[wrap]
    R -->|\f unique| W
    R -->|text| TR[text router]
    TR --> W
    R -->|document, payment, ...| W
    W --> L[per-chat lock]
    L --> S[(session cache)]
    S -. miss .-> DB[(SQLite / Postgres)]
    L --> A[link account]
    A --> MW[your middleware]
    MW --> H[your handler]
    H --> D{session changed?}
    D -- yes --> DB
```

For every update, the framework does the following around your handler:

1. **Routes it.** Commands, buttons (`\f<unique>`), reply buttons, scenes and
   raw events. Callback data is split exactly once, before routing.
2. **Locks the chat.** Updates from one chat are processed in order;
   different chats run in parallel.
3. **Loads the session** from memory, or from the database on first contact
   or after a restart.
4. **Links the account.** In private chats, the Telegram user gets an
   `accounts` row; the language stored there is restored into a cold session.
5. **Runs your middleware and handler**, recovering from panics.
6. **Persists the session** if (and only if) the handler changed it, including
   your `State`.

## Guide

### App and routing

```go
app, err := tbb.New[State](cfg, opts...)
```

`T` is your session state (use `struct{}` if you have none). Register handlers
with:

| Method | Fires on |
|---|---|
| `app.Command("/start", h)` | a command (`/start`, `/start@yourbot`, `/start payload`) |
| `app.Callback(btnKey, h)` | an inline button built from that `BtnKey` |
| `app.Reply("btn.cancel", h)` | a reply-keyboard button whose label is that **translation key**, in any language |
| `app.Text("exact text", h)` | a text message equal to that string |
| `app.Scene(s).On(q, h)` | text while the chat is in scene `s` at question `q` |
| `app.Scene(s).Default(h)` | text in scene `s` at a question without its own handler |
| `app.DefaultText(h)` | any other text |
| `app.DefaultCallback(h)` | buttons without a registered `Callback` |
| `app.Handle(tbb.OnDocument, h)` | any other telebot event: `OnPhoto`, `OnCheckout`, `OnPayment`, `OnLocation`, … |
| `app.OnLanguageSelected(h)` | the built-in language picker (see [Localization](#localization)) |
| `app.Use(mw...)` | middleware around all of the above |

**Text routing order:** `Text` → `Reply` → current scene's question → scene's
`Default` → `DefaultText`. Because `Text` and `Reply` come first, a "Cancel"
or "Main menu" button works from inside any flow.

**Options:**

| Option | Purpose |
|---|---|
| `WithLogger[T](*slog.Logger)` | logger (default `slog.Default()`) |
| `WithI18n[T](*I18n)` | bring your own translation registry |
| `WithDB[T](*bun.DB)` | share an existing database pool (tbb migrates its tables in it, never closes it) |
| `WithStore[T](Store)` | replace persistence entirely, e.g. `NewMemoryStore()` in tests |
| `WithSettings[T](func(*Settings))` | tweak telebot settings: poller, parse mode, HTTP client, `Synchronous`, `Offline`… |
| `WithShutdownTimeout[T](d)` | how long `Run` waits for in-flight handlers (default 10s) |

Escape hatches: `app.Bot()` (the telebot `*Bot`), `app.DB()`, `app.Store()`,
`app.Sessions()`, `app.I18n()`, `app.Logger()`.

### The context: `Ctx[T]`

`Ctx[T]` **is** a telebot `Context` (`Send`, `Edit`, `Respond`, `Message`,
`Callback`, `Sender`, `Args`, …) plus:

| Group | Methods |
|---|---|
| Session | `Session()`, `State() *T`, `ID()` (chat), `UserID()` (account) |
| Scenes | `Enter(s, q)`, `SetScene(s)`, `GetScene()`, `SetNextQuestion(q)`, `IncQuestion()`, `GetCurrentQuestion()`, `ResetScene()` |
| Language | `GetLanguage()`, `SetLanguage(code) error` |
| i18n | `GetL8d(key, kv...)`, `GetL8dRaw(key, kv...)`, `GetL8dBtn(key, kv...)`, `SendL8d(key, opts...)`, `RespondL8d(key, kv...)` |
| Keyboards | `CleanSend`, `CleanEdit`, `CleanSendDelete` |
| Misc | `Account()`, `I18n()`, `Logger()` (pre-tagged with `chat_id`) |

### Buttons and callback data

Give every inline button a typed key and use it both to **build** and to
**route**, so the two can't drift apart:

```go
const TopUpAmount tbb.BtnKey = "tuAmt"

kb := &tbb.ReplyMarkup{}
kb.Inline(
	tbb.Row{
		TopUpAmount.Build("$5", "ton", "500"),   // callback data: "\ftuAmt|ton|500"
		TopUpAmount.Build("$10", "ton", "1000"),
	},
)

app.Callback(TopUpAmount, func(c tbb.Ctx[State]) error {
	args := c.Args()            // ["ton", "500"]
	_ = c.Callback().Unique     // "tuAmt"
	_ = c.Callback().Data       // "ton|500"
	_ = c.Callback().RawData    // "\ftuAmt|ton|500"
	return c.Respond()
})
```

- Keys must match `[A-Za-z0-9_-]+`, and unique plus data must fit Telegram's
  **64-byte** `callback_data` limit.
- Data is split **once, before routing**, for every handler including
  `DefaultCallback`. Never parse `\f…` yourself; use `ParseCallbackData` if you
  need it outside a handler.
- **Renaming a key?** Messages already in users' chats still carry the old one.
  Keep the old key registered (`app.Callback(old, h)`) for a while, or switch
  on `c.Callback().Unique` in `DefaultCallback`.

### Sessions and scenes

```go
type Session[T any] struct {
	ChatID        int64    // Telegram chat
	UID           int64    // accounts.id (private chats)
	Scene         Scene    // 0 = main menu
	Question      Question // step inside the scene
	Language      string
	KeyboardMsgID int      // used by the Clean* helpers
	State         T        // yours
}
```

- **Your state** is any JSON-serializable struct. Mutate it through
  `c.State()`; it's saved when the handler returns, and only if something
  changed (the framework compares JSON snapshots).
- **Lazy loading.** Nothing is preloaded at startup. A chat's session is read
  on its first update and then served from memory.
- **`SetScene(s)` starts at question 0.** Use `Enter(s, q)` to set both.
- **Group chats** get one session per group; account linking only happens in
  private chats, where a chat is a user.

Changing a session **outside** a handler (a scheduler, a webhook, an admin
command acting on someone else):

```go
unlock := app.Sessions().Lock(chatID) // same lock the handlers use
s, err := app.Sessions().Get(ctx, chatID)
if err == nil {
	s.State.PendingInvoice = 0
	err = app.Sessions().Save(ctx, s)
}
unlock()
```

### Localization

```go
app.I18n().Register("en", "English 🇬🇧", en) // order = order in the picker
app.I18n().Register("fa", "پارسی 🇮🇷", fa)
```

| Call | Returns |
|---|---|
| `c.GetL8d("welcome", "name", n)` | translated, `{name}` filled, **escaped for the bot's parse mode** |
| `c.GetL8dRaw(key, kv...)` / `c.GetL8dBtn(...)` | translated and filled, no escaping (for button labels, toasts, plain text) |
| `c.SendL8d(key, opts...)` | sends `GetL8d(key)` |
| `c.RespondL8d(key)` | answers a callback with a toast |
| `app.I18n().Get(lang, key)` | raw template, for use outside handlers |
| `tbb.Replace(tpl, "k", v)` | fills placeholders in any string |

**Fallback:** a missing key or language falls back to `Config.DefaultLanguage`
(`"en"`), then renders as `[missing: key]`, so gaps are visible rather than
blank.

**Language picker:**

```go
c.CleanSend(c.GetL8d("choose_language"), app.LanguageKeyboard())

app.OnLanguageSelected(showMainMenu)              // sets session + account language
app.OnLanguageSelected(showMainMenu, "setLang")   // also accept an older button key
```

**Localized reply buttons:** `app.Reply("btn.cancel", h)` matches the label
of `btn.cancel` in every registered language, so a Farsi user's "انصراف ❌"
and an English user's "Cancel ❌" reach the same handler.

### Parse modes and escaping

Bots send with **MarkdownV2** by default (change it with
`WithSettings(func(s *Settings){ s.ParseMode = tbb.ModeHTML })`). In
MarkdownV2, `_ * [ ] ( ) ~ \` > # + - = | { } . !` are all reserved. One
unescaped `.` and Telegram rejects the whole message
([telebot#553](https://github.com/tucnak/telebot/issues/553)).

**With `GetL8d` you rarely think about it:** templates keep their `*bold*`,
`_italic_` and `` `code` ``, everything else is escaped, and placeholder
values (user input!) are escaped completely.

For hand-built text, pick the helper by **where the text goes**:

| Context | Helper | Escapes |
|---|---|---|
| plain text, any mode | `tbb.Escape(mode, s)` | per mode |
| MarkdownV2 text | `EscapeMarkdownV2(s)` | every reserved char |
| your own MarkdownV2 template | `EscapeMarkdownV2Basics(s)` | all but `* _ \`` |
| inside `` `code` `` / ```` ```pre``` ```` | `EscapeMarkdownV2Code(s)` | only `` ` `` and `\` |
| inside `[text](url)` | `EscapeMarkdownV2URL(s)` | only `)` and `\` |
| legacy Markdown | `EscapeMarkdown(s)` | `_ * \` [` |
| HTML | `EscapeHTML(s)` | `& < > "` |

```go
msg := fmt.Sprintf("Login with `%s`", tbb.EscapeMarkdownV2Code(cmd))
c.Send("Plain text, no formatting at all.", tbb.ModeDefault) // one message without parse mode
```

**When Telegram still says no,** the error tells you where:

```go
_, err := c.Bot().Send(to, text)
if errors.Is(err, tbb.ErrCantParseEntities) {
	// telegram: can't parse entities: Can't find end of the entity starting
	// at byte offset 7 near "Hi john▶_doe!" (400)
	var pe *tbb.ParseEntitiesError
	errors.As(err, &pe) // pe.Offset, pe.Text, pe.Excerpt()
}
```

### Clean keyboards

Menus built from inline keyboards quickly leave a trail of old messages whose
buttons still look clickable. tbb remembers the last keyboard message per
chat (`Session.KeyboardMsgID`):

| Call | Effect |
|---|---|
| `c.CleanEdit(text, kb)` | **edits** the current keyboard message in place, so one message morphs as the user navigates. Falls back to delete + send when it can't edit (none yet, a photo, too old). "Not modified" is a no-op. |
| `c.CleanSend(what, kb)` | strips the old message's keyboard, sends a new one |
| `c.CleanSendDelete(what, kb)` | deletes the old message (or strips it if deletion fails), sends a new one |

Rule of thumb: `CleanEdit` for menu navigation, `CleanSend` when the history
matters (receipts, results), and `CleanSendDelete` when leaving a flow.

### Database and accounts

| Configuration | Result |
|---|---|
| *(nothing)* | SQLite at `./bot.db`: WAL, busy timeout, foreign keys |
| `DB__DSN=file:data/bot.db?_pragma=busy_timeout(5000)` | SQLite at that path |
| `DB__DSN=postgres://user:pass@host:5432/bot?sslmode=disable` | Postgres |
| `WithDB(db)` | your existing `*bun.DB` |

The framework owns three tables and migrates them on startup:

| Table | Content |
|---|---|
| `accounts` | one row per Telegram user: `id`, `telegram_id`, `username`, `first_name`, `last_name`, `language` |
| `sessions` | one JSON document per chat |
| `tbb_schema` | the framework's migration history (separate from yours) |

**Your own tables** live in the same database. Key them by `accounts.id`
(`c.UserID()`), and migrate them with whatever tool you like:

```go
type Feedback struct {
	ID        int64 `bun:",pk,autoincrement"`
	AccountID int64 `bun:",notnull"`
	Rating    int
	Comment   string
}

_, err := app.DB().NewInsert().Model(&Feedback{AccountID: c.UserID(), Rating: 5}).Exec(ctx)
```

`c.Account()` returns the caller's row; `app.Store().AccountByID(...)` works
anywhere, for example to look up the chat to notify from a background job.

### Configuration

`LoadConfig(&cfg)` applies `default:"…"` tags, loads `.env` (real environment
variables win), reads the environment with `__` as the nesting separator, and
validates `validate:"…"` tags. Embed `tbb.Config` to inherit the
framework's keys next to yours:

```go
type Config struct {
	tbb.Config                                            // framework keys
	AdminID int64  `json:"admin_id" validate:"required"`     // ADMIN_ID
	Wallet  string `json:"wallet"   default:"UQ…"`           // WALLET
}

var cfg Config
if err := tbb.LoadConfig(&cfg); err != nil { log.Fatal(err) }
app, _ := tbb.New[State](cfg.Config)
```

| Variable | Default | |
|---|---|---|
| `TELEGRAM__TOKEN` | — | required |
| `TELEGRAM__URL` | `https://api.telegram.org` | e.g. a local `telegram-bot-api` server |
| `TELEGRAM__POLLER_TIMEOUT` | `10s` | long-polling timeout |
| `DB__DRIVER` | inferred | `sqlite` or `postgres` |
| `DB__DSN` | `./bot.db` | |
| `DEFAULT_LANGUAGE` | `en` | fallback language |

### Middleware

Framework middleware sees the full `Ctx[T]`, session included:

```go
func adminsOnly(ids ...int64) tbb.Middleware[State] {
	return func(next tbb.Handler[State]) tbb.Handler[State] {
		return func(c tbb.Ctx[State]) error {
			if !slices.Contains(ids, c.Sender().ID) {
				return c.RespondL8d("error.forbidden")
			}
			return next(c)
		}
	}
}

app.Use(adminsOnly(cfg.AdminID))
```

telebot's middleware package (`github.com/arshamalh/tbb/middleware`:
`Logger`, `AutoRespond`, `Whitelist`, `Recover`, …) still works at the bot
level via `app.Bot().Use(...)`.

### Long polling, webhooks and shutdown

Long polling is the default. For a webhook, swap the poller:

```go
app, err := tbb.New[State](cfg, tbb.WithSettings[State](func(s *tbb.Settings) {
	s.Poller = &tbb.Webhook{
		Listen:   ":8443",
		Endpoint: &tbb.WebhookEndpoint{PublicURL: "https://bot.example.com"},
	}
}))
```

`app.Start()` runs until SIGINT/SIGTERM; `app.Run(ctx)` until `ctx` is
cancelled. Either way, shutdown stops polling, **waits for in-flight
handlers** (up to `WithShutdownTimeout`), and closes the database if tbb
opened it. Sessions are written through as they change, so nothing needs
flushing.

### Files

```go
app.Handle(tbb.OnDocument, func(c tbb.Ctx[State]) error {
	doc := c.Message().Document
	// Stream straight to S3/GCS/another service, no temp file:
	return c.Bot().DownloadToWriter(&doc.File, uploader)
})
```

`Download(file, path)` and `File(file) (io.ReadCloser, error)` are there too.

### Testing your bot

Use an in-memory store and point the bot at a fake Bot API server; then feed
updates synchronously:

```go
func TestStart(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Record r.URL.Path / body here to assert on what was sent.
		fmt.Fprint(w, `{"ok":true,"result":{"message_id":1,"chat":{"id":7}}}`)
	}))
	defer api.Close()

	app, err := tbb.New[State](
		tbb.Config{Telegram: tbb.TelegramConfig{Token: "test", URL: api.URL}},
		tbb.WithStore[State](tbb.NewMemoryStore()),
		tbb.WithSettings[State](func(s *tbb.Settings) {
			s.Offline = true     // skip getMe
			s.Synchronous = true // ProcessUpdate returns after the handler
		}),
	)
	require.NoError(t, err)
	register(app)

	app.Bot().ProcessUpdate(tbb.Update{Message: &tbb.Message{
		Text:   "/start",
		Sender: &tbb.User{ID: 7},
		Chat:   &tbb.Chat{ID: 7, Type: tbb.ChatPrivate},
	}})

	s, _ := app.Sessions().Get(context.Background(), 7)
	assert.Equal(t, tbb.Scene(0), s.GetScene())
}
```

tbb's own [`app_test.go`](app_test.go) has a fuller fake server that
records every call, which is useful as a template.

### Deployment

SQLite is pure Go, so a static binary works anywhere:

```dockerfile
FROM golang:1.25 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /bot ./cmd/bot

FROM gcr.io/distroless/static
COPY --from=build /bot /bot
WORKDIR /data                # bot.db lives here; mount a volume
ENTRYPOINT ["/bot"]
```

- With SQLite, run **one replica** and put `/data` on a persistent volume.
- For several replicas, use Postgres **and** a webhook behind a load balancer.
  The per-chat lock is in-process, so make sure one chat's updates reach one
  instance at a time (or accept rare races between instances).

## What the fork changes in telebot

tbb vendors telebot v4 at upstream
[`ee8708c`](https://github.com/tucnak/telebot/commit/ee8708c) (Bot API 10.1) as
the root package. The upstream guide, with updated import paths, is in
[docs/telebot.md](docs/telebot.md). Changes compared with upstream:

| Change | Why |
|---|---|
| Callback data is split for **every** callback before dispatch; `OnCallback` gets `Unique`/`Data` too; new `Callback.RawData` and `ParseCallbackData` | upstream only split when a unique handler existed, so apps parsed `\f…` a second time; empty and multi-line payloads were rejected |
| `ParseEntitiesError` / `ErrCantParseEntities` with offset and excerpt | markup errors were untyped strings without context ([#553](https://github.com/tucnak/telebot/issues/553)) |
| Escaping helpers for all parse modes | none existed ([#553](https://github.com/tucnak/telebot/issues/553)) |
| `&SendOptions{}` without `ParseMode` keeps the bot's mode | it silently dropped it, so escapes printed as backslashes |
| `Bot.DownloadToWriter` | stream files without touching disk ([#704](https://github.com/tucnak/telebot/pull/704)) |
| `layout` package removed | replaced by tbb's i18n and `BtnKey`; drops viper/yaml deps |
| Go 1.25, CI with `-race` | generics, modern toolchain |

## Project layout

```
tbb/
├── app.go              App[T], options, routing, the per-update wrapper, Run/Start
├── ctx.go              Ctx[T]: session access, i18n, Clean* helpers, rendering
├── session.go          Session[T], Scene, Question
├── session_store.go    write-through session cache + per-chat locks
├── store.go            Store interface, Account
├── sql_store.go        SQLite/Postgres store (bun), OpenDB
├── migrate.go          framework migrations (tbb_schema)
├── memory_store.go     in-memory Store for tests
├── i18n.go             translation registry, Replace
├── language.go         built-in language picker
├── button_key.go       BtnKey
├── escape.go           escaping helpers for every parse mode
├── parse_error.go      ParseEntitiesError
├── env_config.go       LoadConfig
├── *.go                the telebot fork (bot.go, context.go, update.go, ...)
├── middleware/         telebot middleware
├── react/              reaction helpers
├── examples/basic/     a complete example bot
└── docs/telebot.md     upstream telebot guide
```

## FAQ

**Why one package instead of a wrapper around telebot?**
A wrapper means two `Context` types, two routers and constant conversion
between them. Owning the code let us fix things at the source (callback
parsing, parse errors, `SendOptions`) instead of working around them.

**Can I still use plain telebot handlers?**
Yes: `app.Bot().Handle(endpoint, func(c tbb.Context) error {...})`. They
just don't get sessions. Prefer `app.Handle`, which does.

**Do I have to use scenes?**
No. Ignore them and use `Command`, `Callback` and `DefaultText`. The session is
still there for your `State`.

**Is the session saved if my handler returns an error?**
Yes. Changes made before the error are kept. A panic, however, skips the save.

**Why does `GetL8d` output contain backslashes in my button labels?**
`GetL8d` escapes for the parse mode, and button labels aren't parsed. Use
`GetL8dBtn` for labels.

**Can I keep using my own migration tool?**
Yes. The framework only touches `accounts`, `sessions` and `tbb_schema`.

**How do I upgrade the SQLite driver or bun?**
`go get -u modernc.org/sqlite`. For bun, all `github.com/uptrace/bun/...`
modules (core, `sqlitedialect`, `pgdialect`, `pgdriver`) must share **one
version**, or `sqlitedialect` panics at startup.

## Roadmap

- [ ] Align bun modules on the latest release (currently pinned to v1.2.15)
- [ ] Session eviction (TTL / LRU) for very large bots
- [ ] Optional distributed lock for multi-instance deployments
- [ ] `go generate` for typed translation keys
- [ ] Rate-limit-aware sender (honouring `FloodError.RetryAfter`)
- [ ] Admin helpers: broadcast, roles

## Contributing

```bash
go vet ./...
go test -race ./...

# Run the store tests against Postgres too:
TBB_TEST_POSTGRES_DSN=postgres://postgres:postgres@localhost:5432/tbb_test?sslmode=disable \
  go test -race -run 'SQLStore|SessionStoreOverSQL' .
```

Commits are small and self-contained, with a message explaining *why*. The
telebot files keep upstream's formatting to ease future syncs.

## Credits and license

- The Telegram client is [**telebot**](https://github.com/tucnak/telebot) by
  Ilya Kowalewski, Demian and all its contributors.
- Framework patterns come from years of production bots by
  [Arsham Arya](https://github.com/arshamalh).

MIT, see [LICENSE](LICENSE).

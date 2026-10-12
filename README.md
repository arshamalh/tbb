# botbase

A Telegram bot framework for Go, distilled from years of production bots
(Boteland and its predecessors). botbase is a fork of
[telebot v4](https://github.com/tucnak/telebot) with these built in:

- **Sessions with scenes.** Per-chat state with a scene/question router for
  multi-step flows. Your own typed state rides along through generics.
  Sessions are cached in memory, written through to the database, and
  reloaded lazily after a restart.
- **A database by default.** SQLite (pure Go, no CGO) with zero configuration.
  Postgres with a DSN. The framework migrates its own tables, and you get the
  `*bun.DB` for yours.
- **Localization.** Translations are plain Go maps, with fallback to a default
  language, `{placeholder}` templates, a built-in language picker, and reply
  buttons matched in every language.
- **Clean keyboards.** `CleanSend`, `CleanEdit` and `CleanSendDelete` keep one
  "control panel" message instead of a trail of dead buttons.
- **Operations.** Per-chat serialization, panic recovery, `slog` logging,
  env/.env config loading and graceful shutdown.

Everything from telebot is still there, in the same package: `Bot`,
`Context`, `ReplyMarkup`, `OnDocument`… See [docs/telebot.md](docs/telebot.md)
for the upstream guide.

```bash
go get github.com/arshamalh/botbase
```

Requires Go 1.25+.

## Quick start

```go
package main

import "github.com/arshamalh/botbase"

type State struct{} // your own per-chat session data

func main() {
	var cfg botbase.Config
	if err := botbase.LoadConfig(&cfg); err != nil { // TELEGRAM__TOKEN=...
		panic(err)
	}

	app, err := botbase.New[State](cfg) // opens ./bot.db (SQLite)
	if err != nil {
		panic(err)
	}

	app.I18n().Register("en", "English", botbase.Translations{
		"hello": "Hi {name}!",
	})

	app.Command("/start", func(c botbase.Ctx[State]) error {
		return c.Send(c.GetL8d("hello", "name", c.Sender().FirstName))
	})

	app.Start() // until SIGINT/SIGTERM
}
```

[`examples/basic`](examples/basic) is a complete bot with a language picker,
menus, a two-step scene and a localized cancel button.

## Concepts

### App and handlers

`botbase.New[T](cfg, opts...)` creates the bot. `T` is your session state
type (use `struct{}` if you have none). Handlers take a `botbase.Ctx[T]`,
which is a telebot `Context` plus the framework helpers.

| Register with | Fires on |
|---|---|
| `app.Command("/start", h)` | a command |
| `app.Callback(btnKey, h)` | an inline button built from that `BtnKey` |
| `app.Reply("btn.cancel", h)` | a reply-keyboard button whose label is that translation key, in any language |
| `app.Text("exact text", h)` | an exact text message |
| `app.Scene(s).On(q, h)` | text while the chat is in scene `s` at question `q` |
| `app.Scene(s).Default(h)` | text in scene `s` at a question without a handler |
| `app.DefaultText(h)` / `app.DefaultCallback(h)` | everything not matched above |
| `app.Handle(botbase.OnDocument, h)` | any other telebot event, with sessions |
| `app.Use(mw...)` | middleware around all of the above |

For text, `Text` and `Reply` routes take precedence over scenes, so a
Cancel button works from anywhere.

Options: `WithLogger`, `WithI18n`, `WithStore`, `WithDB`,
`WithSettings` (telebot settings such as a webhook poller or another parse
mode), and `WithShutdownTimeout`.

### Buttons

```go
const TopUpAmount botbase.BtnKey = "tuAmt"

kb := &botbase.ReplyMarkup{}
kb.Inline(TopUpAmount.AsRow("$5", "ton", "500"))  // data: "ton|500"

app.Callback(TopUpAmount, func(c botbase.Ctx[State]) error {
	kind, cents := c.Args()[0], c.Args()[1]
	...
})
```

### Sessions and scenes

```go
type State struct {
	TopUpKind string `json:"top_up_kind,omitempty"`
}

const SceneTopUp botbase.Scene = 1
const QAmount botbase.Question = 1

app.Callback(TopUp, func(c botbase.Ctx[State]) error {
	c.State().TopUpKind = "ton"
	c.Enter(SceneTopUp, QAmount)
	return c.CleanEdit(c.GetL8d("topup.ask_amount"))
})

app.Scene(SceneTopUp).On(QAmount, func(c botbase.Ctx[State]) error {
	amount := c.Text()
	c.ResetScene()
	...
})
```

The session (`c.Session()`) holds the chat ID, the account ID, the scene and
question, the language, the last keyboard message, and your `State`. The
framework saves it after each handler that changed it. Updates from the same
chat are handled one at a time, so a scene can't be raced by a double tap.

To change a session outside a handler (from a scheduler or a webhook):

```go
unlock := app.Sessions().Lock(chatID)
s, _ := app.Sessions().Get(ctx, chatID)
s.State.X = ...
_ = app.Sessions().Save(ctx, s)
unlock()
```

### Localization

```go
app.I18n().Register("en", "English 🇬🇧", en) // en is a botbase.Translations map
app.I18n().Register("fa", "پارسی 🇮🇷", fa)

c.GetL8d("welcome", "name", firstName) // translated and escaped for the parse mode
c.GetL8dBtn("btn.cancel")              // raw text, for buttons
c.SendL8d("done")
c.RespondL8d("error.no_handler")       // callback toast

app.LanguageKeyboard()     // inline keyboard of registered languages
app.OnLanguageSelected(h)  // sets and stores the language, then runs h
```

Missing keys fall back to the default language (`Config.DefaultLanguage`,
"en"), then render as `[missing: key]`.

With MarkdownV2 (the default parse mode), `GetL8d` keeps the template's own
`*bold*`, `_italic_` and `` `code` `` and escapes everything else. Placeholder
values are fully escaped, so user input is safe. `EscapeMarkdownV2` and
`EscapeMarkdownV2Basics` are available for hand-built messages.

### Clean keyboards

- `c.CleanEdit(text, kb)`: edit the current keyboard message in place. If that
  isn't possible it falls back to delete and send. A "not modified" result is a
  no-op.
- `c.CleanSend(text, kb)`: strip the old keyboard and send a new message.
- `c.CleanSendDelete(text, kb)`: delete the old message and send a new one.

### Database

| Config | Result |
|---|---|
| nothing | SQLite at `./bot.db` (WAL, busy timeout) |
| `DB__DSN=file:data/bot.db?_pragma=busy_timeout(5000)` | SQLite at that path |
| `DB__DSN=postgres://user:pass@host:5432/bot?sslmode=disable` | Postgres |

The framework owns three tables: `accounts` (one row per Telegram user),
`sessions` (JSON per chat) and `botbase_schema` (its migration history).
`app.DB()` returns the `*bun.DB` for your own tables. Keep migrating those with
your own tool. `c.Account()` / `c.UserID()` give you the account row to key
them by.

For tests, use `botbase.WithStore[State](botbase.NewMemoryStore())`.

### Configuration

`LoadConfig(&cfg)` applies `default:` tags, loads `.env`, reads environment
variables (nesting with `__`) and validates `validate:` tags. Embed
`botbase.Config` to inherit the framework keys:

```go
type Config struct {
	botbase.Config                              // TELEGRAM__TOKEN, DB__DSN, DEFAULT_LANGUAGE
	TONWallet string `json:"ton_wallet" validate:"required"` // TON_WALLET
}
```

| Variable | Default |
|---|---|
| `TELEGRAM__TOKEN` | required |
| `TELEGRAM__URL` | Telegram's API |
| `TELEGRAM__POLLER_TIMEOUT` | `10s` |
| `DB__DRIVER` | inferred from the DSN |
| `DB__DSN` | `./bot.db` |
| `DEFAULT_LANGUAGE` | `en` |

## Coming from Boteland

| Boteland | botbase |
|---|---|
| `handlers.SharedContext` | `botbase.Ctx[State]` |
| `entities.UserData` (with top-up fields) | `botbase.Session[State]`, where the top-up fields move into `State` |
| `ctx.SetTopUpKind(k)` | `ctx.State().TopUpKind = k` |
| `session.TelegramUD` + `GetUsersSessionData` | `app.Sessions()` (lazy load, write-through) |
| `NewCallbacksHandler` / `cbsHandler.Register(btn, h)` | `app.Callback(btn, h)` |
| `NewTextHandler` / `NewSceneTextHandler` | `app.Scene(s).On(q, h)` / `.Default(h)` |
| `RegisterReplyKeyboardHandler(msgs.Cancel, h)` | `app.Reply("btn.cancel", h)` (localized) |
| `btns.BtnKey` | `botbase.BtnKey` |
| `msgs.GetTranslation(lang, key)` | `app.I18n().Get(lang, key)` |
| `msgs.FmtBasics` | `botbase.EscapeMarkdownV2Basics` (or just `ctx.GetL8d`) |
| `msgs.FmtWelcome(lang, name)` style helpers | `ctx.GetL8d("welcome", "name", name)` |
| language keyboard + `SetLanguage` handler | `app.LanguageKeyboard()` + `app.OnLanguageSelected(h)` |
| `users` / `languages` / `user_sessions` tables | `accounts` / translations in code / `sessions` |
| `config.C` from `init()` | `botbase.LoadConfig(&cfg)` |
| `zap` via `log.Gl` | `log/slog` (`WithLogger`, `ctx.Logger()`) |

## Credits

The Telegram client is [telebot](https://github.com/tucnak/telebot) by its
authors and contributors, under the MIT license (see [LICENSE](LICENSE)).

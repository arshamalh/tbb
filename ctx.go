package botbase

import (
	"context"
	"errors"
	"html"
	"log/slog"
	"strconv"
	"strings"
)

// Ctx is the context handlers receive. It is a telebot Context (so Send,
// Edit, Respond, Callback, Message… all work as usual) extended with the
// chat's session, localization and the "clean" keyboard helpers.
type Ctx[T any] interface {
	Context

	// Session returns the chat's session. Changes made during the handler
	// are persisted automatically when it returns.
	Session() *Session[T]
	// State returns the app-specific part of the session.
	State() *T

	// ID returns the chat ID.
	ID() int64
	// UserID returns the account ID (accounts.id) of the user, or 0 for
	// updates that have no linked account (e.g. group chats).
	UserID() int64

	GetScene() Scene
	SetScene(scene Scene)
	GetCurrentQuestion() Question
	SetNextQuestion(q Question)
	IncQuestion()
	Enter(scene Scene, q Question)
	ResetScene()

	// GetLanguage returns the chat's language ("" until one is chosen;
	// translations then use the fallback language).
	GetLanguage() string
	// SetLanguage changes the chat's language and stores it on the
	// account as well.
	SetLanguage(lang string) error

	// Account returns the stored account of the user who sent the update.
	Account() (*Account, error)

	// GetL8d returns the translation of key in the chat's language, with
	// {placeholders} filled from kv pairs, escaped for the bot's parse
	// mode: the template's own markup is kept and the values are escaped
	// fully, so user input can be passed safely.
	GetL8d(key string, kv ...string) string
	// GetL8dRaw is GetL8d without any escaping. Use it for button labels,
	// callback answers, or text sent without a parse mode.
	GetL8dRaw(key string, kv ...string) string
	// GetL8dBtn is an alias of GetL8dRaw, for button labels.
	GetL8dBtn(key string, kv ...string) string
	// SendL8d sends the translation of key. opts are passed to Send.
	SendL8d(key string, opts ...any) error
	// RespondL8d answers a callback query with the translation of key
	// shown as a toast.
	RespondL8d(key string, kv ...string) error

	// CleanSend strips the inline keyboard from the previous keyboard
	// message, sends a new message and remembers it as the current one.
	CleanSend(what any, opts ...any) error
	// CleanSendDelete is CleanSend, except the previous keyboard message
	// is deleted entirely (falling back to stripping its keyboard).
	CleanSendDelete(what any, opts ...any) error
	// CleanEdit edits the previous keyboard message in place, so a single
	// "control panel" message morphs as the user navigates menus. It falls
	// back to CleanSendDelete when there is nothing to edit or the edit is
	// impossible (different message type, too old, deleted).
	CleanEdit(what any, opts ...any) error

	// I18n returns the app's translation registry.
	I18n() *I18n
	// Logger returns the app's logger, annotated with the chat ID.
	Logger() *slog.Logger
}

// sessionState lets botCtx embed the session (promoting its methods) under a
// field name that doesn't collide with the Session() method.
type sessionState[T any] = Session[T]

type botCtx[T any] struct {
	Context
	*sessionState[T]

	app     *App[T]
	account *Account
}

var _ Ctx[struct{}] = (*botCtx[struct{}])(nil)

func (c *botCtx[T]) Session() *Session[T] { return c.sessionState }
func (c *botCtx[T]) State() *T            { return &c.sessionState.State }
func (c *botCtx[T]) I18n() *I18n          { return c.app.i18n }

func (c *botCtx[T]) Logger() *slog.Logger {
	return c.app.logger.With("chat_id", c.ChatID)
}

func (c *botCtx[T]) SetLanguage(lang string) error {
	c.sessionState.Language = lang
	if c.UID == 0 {
		return nil
	}
	if err := c.app.store.SetAccountLanguage(context.Background(), c.UID, lang); err != nil {
		return err
	}
	if c.account != nil {
		c.account.Language = lang
	}
	return nil
}

func (c *botCtx[T]) Account() (*Account, error) {
	if c.account != nil {
		return c.account, nil
	}
	ctx := context.Background()
	if c.UID != 0 {
		a, err := c.app.store.AccountByID(ctx, c.UID)
		if err == nil {
			c.account = a
		}
		return a, err
	}
	sender := c.Sender()
	if sender == nil {
		return nil, ErrRecordNotFound
	}
	a := accountFromUser(sender)
	if err := c.app.store.UpsertAccount(ctx, a); err != nil {
		return nil, err
	}
	c.account = a
	return a, nil
}

func accountFromUser(u *User) *Account {
	return &Account{
		TelegramID: u.ID,
		Username:   u.Username,
		FirstName:  u.FirstName,
		LastName:   u.LastName,
	}
}

func (c *botCtx[T]) GetL8d(key string, kv ...string) string {
	return c.app.render(c.sessionState.Language, key, kv...)
}

func (c *botCtx[T]) GetL8dRaw(key string, kv ...string) string {
	return Replace(c.app.i18n.Get(c.sessionState.Language, key), kv...)
}

func (c *botCtx[T]) GetL8dBtn(key string, kv ...string) string {
	return c.GetL8dRaw(key, kv...)
}

func (c *botCtx[T]) SendL8d(key string, opts ...any) error {
	return c.Send(c.GetL8d(key), opts...)
}

func (c *botCtx[T]) RespondL8d(key string, kv ...string) error {
	return c.Respond(&CallbackResponse{Text: c.GetL8dRaw(key, kv...)})
}

func (c *botCtx[T]) prevKeyboardMsg() StoredMessage {
	return StoredMessage{MessageID: strconv.Itoa(c.KeyboardMsgID), ChatID: c.ChatID}
}

func (c *botCtx[T]) sendAndRemember(what any, opts ...any) error {
	msg, err := c.Bot().Send(ChatID(c.ChatID), what, opts...)
	if err != nil {
		return err
	}
	c.KeyboardMsgID = msg.ID
	return nil
}

func (c *botCtx[T]) CleanSend(what any, opts ...any) error {
	if c.KeyboardMsgID != 0 {
		// Best effort: the old message may be gone or too old to edit.
		_, _ = c.Bot().EditReplyMarkup(c.prevKeyboardMsg(), nil)
	}
	return c.sendAndRemember(what, opts...)
}

func (c *botCtx[T]) CleanSendDelete(what any, opts ...any) error {
	if c.KeyboardMsgID != 0 {
		prev := c.prevKeyboardMsg()
		// Deletion fails for messages older than 48h or already deleted;
		// at least make sure the old buttons can't be pressed.
		if err := c.Bot().Delete(prev); err != nil {
			_, _ = c.Bot().EditReplyMarkup(prev, nil)
		}
	}
	return c.sendAndRemember(what, opts...)
}

func (c *botCtx[T]) CleanEdit(what any, opts ...any) error {
	if c.KeyboardMsgID == 0 {
		return c.CleanSendDelete(what, opts...)
	}
	if _, err := c.Bot().Edit(c.prevKeyboardMsg(), what, opts...); err != nil {
		if errors.Is(err, ErrMessageNotModified) || errors.Is(err, ErrSameMessageContent) {
			// Already showing exactly this content (e.g. the same button
			// tapped twice): nothing to do.
			return nil
		}
		return c.CleanSendDelete(what, opts...)
	}
	return nil
}

// render looks up and fills a template, escaping for the bot's parse mode.
func (a *App[T]) render(lang, key string, kv ...string) string {
	template := a.i18n.Get(lang, key)
	switch a.bot.parseMode {
	case ModeMarkdownV2:
		// Escape the template's punctuation (braces included), then put
		// fully escaped values where the escaped placeholders are.
		out := EscapeMarkdownV2Basics(template)
		for i := 0; i+1 < len(kv); i += 2 {
			out = strings.ReplaceAll(out, `\{`+kv[i]+`\}`, EscapeMarkdownV2(kv[i+1]))
		}
		return out
	case ModeHTML:
		escaped := make([]string, len(kv))
		for i := range kv {
			escaped[i] = kv[i]
			if i%2 == 1 {
				escaped[i] = html.EscapeString(kv[i])
			}
		}
		return Replace(template, escaped...)
	default:
		return Replace(template, kv...)
	}
}

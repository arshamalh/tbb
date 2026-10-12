package botbase

import (
	"context"
	"errors"
	"time"
)

// ErrRecordNotFound is returned by a Store when a record does not exist.
// (ErrNotFound is taken by the Telegram API error of the same name.)
var ErrRecordNotFound = errors.New("botbase: not found")

// Account is a Telegram user the bot has seen, stored in the "accounts"
// table. The table is owned by the framework; apps that need more columns
// keep them in their own tables keyed by accounts.id.
type Account struct {
	ID         int64     `bun:",pk,autoincrement"`
	TelegramID int64     `bun:",unique,notnull"`
	Username   string    `bun:",nullzero"`
	FirstName  string    `bun:",nullzero"`
	LastName   string    `bun:",nullzero"`
	Language   string    `bun:",nullzero"`
	CreatedAt  time.Time `bun:",notnull,default:current_timestamp"`
	UpdatedAt  time.Time `bun:",notnull,default:current_timestamp"`
}

// Store persists the framework's own data: accounts and chat sessions.
// The default implementation (NewSQLStore) works on SQLite and Postgres;
// a custom one can be passed with WithStore.
type Store interface {
	// LoadSession returns the JSON payload saved for a chat, or ErrRecordNotFound.
	LoadSession(ctx context.Context, chatID int64) ([]byte, error)
	// SaveSession upserts the JSON payload of a chat's session.
	SaveSession(ctx context.Context, chatID int64, payload []byte) error

	// UpsertAccount inserts the account or, when one with the same
	// TelegramID exists, refreshes its profile fields. a.ID is set on return.
	UpsertAccount(ctx context.Context, a *Account) error
	AccountByID(ctx context.Context, id int64) (*Account, error)
	AccountByTelegramID(ctx context.Context, telegramID int64) (*Account, error)
	SetAccountLanguage(ctx context.Context, accountID int64, lang string) error
}

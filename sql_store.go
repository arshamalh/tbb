package tbb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/pgdriver"
	_ "modernc.org/sqlite" // pure-Go SQLite driver, registered as "sqlite"
)

// Supported database drivers.
const (
	DriverSQLite   = "sqlite"
	DriverPostgres = "postgres"
)

// DefaultSQLiteDSN is used when no database is configured: a bot.db file in
// the working directory, in WAL mode with a busy timeout so concurrent
// handlers wait for the write lock instead of failing.
const DefaultSQLiteDSN = "file:bot.db?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"

// DBConfig selects the database. Everything is optional: the zero value opens
// DefaultSQLiteDSN.
type DBConfig struct {
	// Driver is "sqlite" or "postgres". When empty it is inferred from the
	// DSN: postgres:// and postgresql:// URLs select Postgres, anything else
	// SQLite.
	Driver string `json:"driver"`

	// DSN is the connection string, e.g.
	//   file:data/bot.db?_pragma=busy_timeout(5000)
	//   postgres://user:pass@localhost:5432/bot?sslmode=disable
	DSN string `json:"dsn"`
}

func (c DBConfig) driver() string {
	if c.Driver != "" {
		return strings.ToLower(c.Driver)
	}
	if strings.HasPrefix(c.DSN, "postgres://") || strings.HasPrefix(c.DSN, "postgresql://") {
		return DriverPostgres
	}
	return DriverSQLite
}

// OpenDB opens the configured database and verifies the connection.
// The returned *bun.DB can be shared with the app for its own tables.
func OpenDB(cfg DBConfig) (*bun.DB, error) {
	switch cfg.driver() {
	case DriverSQLite:
		dsn := cfg.DSN
		if dsn == "" {
			dsn = DefaultSQLiteDSN
		}
		sqldb, err := sql.Open("sqlite", dsn)
		if err != nil {
			return nil, fmt.Errorf("open sqlite: %w", err)
		}
		// SQLite allows a single writer; one connection avoids SQLITE_BUSY
		// errors and makes ":memory:" databases behave as one database.
		sqldb.SetMaxOpenConns(1)
		db := bun.NewDB(sqldb, sqlitedialect.New())
		if err := db.Ping(); err != nil {
			db.Close()
			return nil, fmt.Errorf("connect to sqlite: %w", err)
		}
		return db, nil

	case DriverPostgres:
		if cfg.DSN == "" {
			return nil, errors.New("postgres driver needs a DSN")
		}
		sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(cfg.DSN)))
		db := bun.NewDB(sqldb, pgdialect.New())
		if err := db.Ping(); err != nil {
			db.Close()
			return nil, fmt.Errorf("connect to postgres: %w", err)
		}
		return db, nil

	default:
		return nil, fmt.Errorf("unsupported database driver %q (use %q or %q)", cfg.Driver, DriverSQLite, DriverPostgres)
	}
}

// sessionRecord is the row backing a Session.
type sessionRecord struct {
	bun.BaseModel `bun:"table:sessions"`

	ChatID    int64     `bun:",pk"`
	Payload   string    `bun:",notnull"`
	CreatedAt time.Time `bun:",notnull,default:current_timestamp"`
	UpdatedAt time.Time `bun:",notnull,default:current_timestamp"`
}

// SQLStore is the default Store, backed by bun on SQLite or Postgres.
type SQLStore struct {
	db *bun.DB
}

// NewSQLStore wraps an open database. Call Migrate once before using it.
func NewSQLStore(db *bun.DB) *SQLStore {
	return &SQLStore{db: db}
}

// DB returns the underlying database handle.
func (s *SQLStore) DB() *bun.DB { return s.db }

// Migrate creates or upgrades the framework's tables.
func (s *SQLStore) Migrate(ctx context.Context) error {
	return migrate(ctx, s.db, frameworkMigrations)
}

func (s *SQLStore) LoadSession(ctx context.Context, chatID int64) ([]byte, error) {
	rec := new(sessionRecord)
	err := s.db.NewSelect().Model(rec).Where("chat_id = ?", chatID).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRecordNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load session: %w", err)
	}
	return []byte(rec.Payload), nil
}

func (s *SQLStore) SaveSession(ctx context.Context, chatID int64, payload []byte) error {
	now := time.Now().UTC()
	rec := &sessionRecord{ChatID: chatID, Payload: string(payload), CreatedAt: now, UpdatedAt: now}
	_, err := s.db.NewInsert().Model(rec).
		On("CONFLICT (chat_id) DO UPDATE").
		Set("payload = EXCLUDED.payload").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("save session: %w", err)
	}
	return nil
}

func (s *SQLStore) UpsertAccount(ctx context.Context, a *Account) error {
	now := time.Now().UTC()
	a.CreatedAt, a.UpdatedAt = now, now
	_, err := s.db.NewInsert().Model(a).
		On("CONFLICT (telegram_id) DO UPDATE").
		Set("username = EXCLUDED.username").
		Set("first_name = EXCLUDED.first_name").
		Set("last_name = EXCLUDED.last_name").
		Set("updated_at = EXCLUDED.updated_at").
		Returning("*").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("upsert account: %w", err)
	}
	return nil
}

func (s *SQLStore) AccountByID(ctx context.Context, id int64) (*Account, error) {
	return s.account(ctx, "id = ?", id)
}

func (s *SQLStore) AccountByTelegramID(ctx context.Context, telegramID int64) (*Account, error) {
	return s.account(ctx, "telegram_id = ?", telegramID)
}

func (s *SQLStore) account(ctx context.Context, where string, arg any) (*Account, error) {
	a := new(Account)
	err := s.db.NewSelect().Model(a).Where(where, arg).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRecordNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get account: %w", err)
	}
	return a, nil
}

func (s *SQLStore) SetAccountLanguage(ctx context.Context, accountID int64, lang string) error {
	res, err := s.db.NewUpdate().Model((*Account)(nil)).
		Set("language = ?", lang).
		Set("updated_at = ?", time.Now().UTC()).
		Where("id = ?", accountID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("set account language: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrRecordNotFound
	}
	return nil
}

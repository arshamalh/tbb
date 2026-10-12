package botbase

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestSQLStore opens a migrated store. Set BOTBASE_TEST_POSTGRES_DSN to run
// the same tests against Postgres.
func newTestSQLStore(t *testing.T) *SQLStore {
	t.Helper()
	cfg := DBConfig{DSN: "file:" + filepath.Join(t.TempDir(), "test.db") + "?_pragma=busy_timeout(5000)"}
	if dsn := os.Getenv("BOTBASE_TEST_POSTGRES_DSN"); dsn != "" {
		cfg = DBConfig{DSN: dsn}
	}
	db, err := OpenDB(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	s := NewSQLStore(db)
	require.NoError(t, s.Migrate(context.Background()))
	return s
}

func TestDBConfigDriver(t *testing.T) {
	assert.Equal(t, DriverSQLite, DBConfig{}.driver())
	assert.Equal(t, DriverSQLite, DBConfig{DSN: "file:x.db"}.driver())
	assert.Equal(t, DriverPostgres, DBConfig{DSN: "postgres://u:p@h/db"}.driver())
	assert.Equal(t, DriverPostgres, DBConfig{DSN: "postgresql://u:p@h/db"}.driver())
	assert.Equal(t, DriverPostgres, DBConfig{Driver: "Postgres"}.driver())

	_, err := OpenDB(DBConfig{Driver: "mysql"})
	assert.ErrorContains(t, err, "unsupported")
}

func TestSQLStoreMigrateIsIdempotent(t *testing.T) {
	s := newTestSQLStore(t)
	require.NoError(t, s.Migrate(context.Background()))

	n, err := s.DB().NewSelect().Model((*schemaVersion)(nil)).Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, len(frameworkMigrations), n)
}

func TestSQLStoreSessions(t *testing.T) {
	ctx := context.Background()
	s := newTestSQLStore(t)

	_, err := s.LoadSession(ctx, 1)
	assert.ErrorIs(t, err, ErrRecordNotFound)

	require.NoError(t, s.SaveSession(ctx, 1, []byte(`{"scene":1}`)))
	require.NoError(t, s.SaveSession(ctx, 1, []byte(`{"scene":2}`)))

	got, err := s.LoadSession(ctx, 1)
	require.NoError(t, err)
	assert.JSONEq(t, `{"scene":2}`, string(got))
}

func TestSQLStoreAccounts(t *testing.T) {
	ctx := context.Background()
	s := newTestSQLStore(t)

	a := &Account{TelegramID: 100, Username: "arsham", FirstName: "Arsham", Language: "fa"}
	require.NoError(t, s.UpsertAccount(ctx, a))
	require.NotZero(t, a.ID)

	// A second upsert refreshes the profile and keeps the ID and language.
	b := &Account{TelegramID: 100, Username: "arsham2", FirstName: "Arsham"}
	require.NoError(t, s.UpsertAccount(ctx, b))
	assert.Equal(t, a.ID, b.ID)
	assert.Equal(t, "arsham2", b.Username)
	assert.Equal(t, "fa", b.Language)

	require.NoError(t, s.SetAccountLanguage(ctx, a.ID, "de"))
	got, err := s.AccountByTelegramID(ctx, 100)
	require.NoError(t, err)
	assert.Equal(t, "de", got.Language)

	got, err = s.AccountByID(ctx, a.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(100), got.TelegramID)

	_, err = s.AccountByID(ctx, 999)
	assert.ErrorIs(t, err, ErrRecordNotFound)
	assert.ErrorIs(t, s.SetAccountLanguage(ctx, 999, "en"), ErrRecordNotFound)
}

func TestSessionStoreOverSQL(t *testing.T) {
	ctx := context.Background()
	backend := newTestSQLStore(t)

	ss := NewSessionStore[testState](backend)
	sess, err := ss.Get(ctx, 5)
	require.NoError(t, err)
	sess.Enter(1, 2)
	sess.State.Step = "amount"
	require.NoError(t, ss.Save(ctx, sess))

	reloaded, err := NewSessionStore[testState](backend).Get(ctx, 5)
	require.NoError(t, err)
	assert.Equal(t, "amount", reloaded.State.Step)
	assert.Equal(t, Question(2), reloaded.GetCurrentQuestion())
}

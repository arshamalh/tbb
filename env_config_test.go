package tbb

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testAppConfig struct {
	Config
	AdminID  int64  `json:"admin_id" validate:"required"`
	Greeting string `json:"greeting" default:"hello"`
}

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "test.env")
	require.NoError(t, os.WriteFile(envFile, []byte("ADMIN_ID=42\nTELEGRAM__TOKEN=from-file\n"), 0o600))

	t.Setenv("TELEGRAM__TOKEN", "from-env") // real env wins over the file
	t.Setenv("TELEGRAM__POLLER_TIMEOUT", "3s")
	t.Setenv("DB__DSN", "postgres://u:p@localhost/bot")
	t.Cleanup(func() { os.Unsetenv("ADMIN_ID") })

	var cfg testAppConfig
	require.NoError(t, LoadConfig(&cfg, envFile))

	assert.Equal(t, "from-env", cfg.Telegram.Token)
	assert.Equal(t, 3*time.Second, cfg.Telegram.PollerTimeout)
	assert.Equal(t, "postgres://u:p@localhost/bot", cfg.DB.DSN)
	assert.Equal(t, int64(42), cfg.AdminID)
	assert.Equal(t, "hello", cfg.Greeting)
	assert.Equal(t, "en", cfg.DefaultLanguage)
}

func TestLoadConfigValidates(t *testing.T) {
	t.Setenv("TELEGRAM__TOKEN", "")
	var cfg Config
	err := LoadConfig(&cfg, filepath.Join(t.TempDir(), "missing.env"))
	assert.ErrorContains(t, err, "Token")
}

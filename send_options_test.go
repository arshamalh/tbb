package botbase

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendOptionsInheritBotParseMode(t *testing.T) {
	b, err := NewBot(Settings{Offline: true, ParseMode: ModeMarkdownV2})
	require.NoError(t, err)

	opts := b.extractOptions([]interface{}{&SendOptions{DisableNotification: true}})
	assert.Equal(t, ModeMarkdownV2, opts.ParseMode, "SendOptions without a mode keep the bot's")

	opts = b.extractOptions([]interface{}{&SendOptions{ParseMode: ModeHTML}})
	assert.Equal(t, ModeHTML, opts.ParseMode, "an explicit mode wins")

	opts = b.extractOptions([]interface{}{&SendOptions{DisableNotification: true}, ModeDefault})
	assert.Equal(t, ModeDefault, opts.ParseMode, "a trailing ModeDefault sends plain text")
	assert.True(t, opts.DisableNotification)

	opts = b.extractOptions([]interface{}{ModeDefault})
	assert.Equal(t, ModeDefault, opts.ParseMode)

	plain, err := NewBot(Settings{Offline: true})
	require.NoError(t, err)
	assert.Equal(t, ModeDefault, plain.extractOptions([]interface{}{&SendOptions{}}).ParseMode)
}

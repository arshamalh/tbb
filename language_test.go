package botbase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLanguagePicker(t *testing.T) {
	store := NewMemoryStore()
	app, api := newTestApp(t, store)

	kb := app.LanguageKeyboard()
	require.Len(t, kb.InlineKeyboard, 2)
	assert.Equal(t, "English", kb.InlineKeyboard[0][0].Text)
	assert.Equal(t, "فارسی", kb.InlineKeyboard[1][0].Text)

	app.OnLanguageSelected(func(c Ctx[flowState]) error {
		return c.SendL8d("welcome")
	})
	app.Bot().ProcessUpdate(callbackUpdate(LanguageBtn, "fa"))

	assert.Equal(t, `سلام \{name\}\!`, api.last("sendMessage")["text"], "SendL8d sends the template as is")
	acc, err := store.AccountByTelegramID(context.Background(), 7)
	require.NoError(t, err)
	assert.Equal(t, "fa", acc.Language)

	// A cold cache (restart) restores the language from the account.
	restarted, api2 := newTestApp(t, store)
	restarted.Command("/start", func(c Ctx[flowState]) error {
		return c.Send(c.GetL8d("welcome", "name", "A"))
	})
	restarted.Bot().ProcessUpdate(textUpdate("/start"))
	assert.Equal(t, `سلام A\!`, api2.last("sendMessage")["text"])

	// Unknown codes are ignored.
	api.reset()
	app.Bot().ProcessUpdate(callbackUpdate(LanguageBtn, "xx"))
	assert.Equal(t, []string{"answerCallbackQuery"}, api.methods())
}

func TestLanguagePickerLegacyKeys(t *testing.T) {
	app, _ := newTestApp(t, nil)

	var picked []string
	app.OnLanguageSelected(func(c Ctx[flowState]) error {
		picked = append(picked, c.GetLanguage())
		return nil
	}, "setLang")

	app.Bot().ProcessUpdate(callbackUpdate("setLang", "fa"))   // a keyboard sent before the migration
	app.Bot().ProcessUpdate(callbackUpdate(LanguageBtn, "en")) // the built-in picker
	assert.Equal(t, []string{"fa", "en"}, picked)
}

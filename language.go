package tbb

// LanguageBtn is the built-in button of the language picker. Its data is the
// language code.
const LanguageBtn BtnKey = "tbbLang"

// LanguageKeyboard returns an inline keyboard with one button per registered
// language, in registration order, labelled with the language's name.
func (a *App[T]) LanguageKeyboard() *ReplyMarkup {
	langs := a.i18n.Languages()
	rows := make([]Row, 0, len(langs))
	for _, l := range langs {
		rows = append(rows, LanguageBtn.AsRow(l.Name, l.Code))
	}
	kb := &ReplyMarkup{}
	kb.Inline(rows...)
	return kb
}

// OnLanguageSelected handles presses on LanguageKeyboard buttons: the chat's
// language is set (and stored on the account), the button's spinner is
// dismissed, then next runs, typically to show the main menu in the new
// language. next may be nil.
//
// legacy lists older button keys whose payload is also a language code, so
// language keyboards already sitting in chats keep working after a bot
// switches to the built-in picker (e.g. Boteland's "setLang").
func (a *App[T]) OnLanguageSelected(next Handler[T], legacy ...BtnKey) {
	h := func(c Ctx[T]) error {
		code := c.Callback().Data
		if !a.i18n.Has(code) {
			c.Logger().Warn("language picker: unknown language", "code", code)
			return c.Respond()
		}
		if err := c.SetLanguage(code); err != nil {
			c.Logger().Error("store account language", "err", err)
		}
		if err := c.Respond(); err != nil {
			c.Logger().Debug("respond to language button", "err", err)
		}
		if next == nil {
			return nil
		}
		return next(c)
	}
	a.Callback(LanguageBtn, h)
	for _, key := range legacy {
		a.Callback(key, h)
	}
}

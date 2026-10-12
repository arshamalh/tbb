package main

import (
	"fmt"
	"strings"

	"github.com/arshamalh/botbase"
)

type Ctx = botbase.Ctx[State]

// Inline buttons.
const (
	btnFeedback botbase.BtnKey = "feedback"
	btnProfile  botbase.BtnKey = "profile"
	btnLanguage botbase.BtnKey = "language"
)

// Scenes and their questions.
const (
	sceneFeedback botbase.Scene = iota + 1
)

const (
	qRating botbase.Question = iota + 1
	qComment
)

type handlers struct {
	app *botbase.App[State]
}

func (h *handlers) register() {
	h.app.Command("/start", h.start)

	h.app.Callback(btnFeedback, h.feedbackStart)
	h.app.Callback(btnProfile, h.profile)
	h.app.Callback(btnLanguage, h.languageMenu)
	h.app.OnLanguageSelected(h.mainMenu)

	h.app.Scene(sceneFeedback).
		On(qRating, h.feedbackRating).
		On(qComment, h.feedbackComment)

	// The cancel button is a reply-keyboard button: its label differs per
	// language, and it works whatever scene the chat is in.
	h.app.Reply(btnCancelText, h.cancel)

	// Anything else brings the menu back.
	h.app.DefaultText(h.mainMenu)
}

func mainMenuKeyboard(c Ctx) *botbase.ReplyMarkup {
	kb := &botbase.ReplyMarkup{}
	kb.Inline(
		btnFeedback.AsRow(c.GetL8dBtn(btnFeedbackText)),
		botbase.Row{
			btnProfile.Build(c.GetL8dBtn(btnProfileText)),
			btnLanguage.Build(c.GetL8dBtn(btnLanguageText)),
		},
	)
	return kb
}

func cancelKeyboard(c Ctx) *botbase.ReplyMarkup {
	kb := &botbase.ReplyMarkup{ResizeKeyboard: true}
	kb.Reply(botbase.Row{kb.Text(c.GetL8dBtn(btnCancelText))})
	return kb
}

func (h *handlers) start(c Ctx) error {
	c.ResetScene()
	if c.GetLanguage() == "" {
		return c.CleanSend(c.GetL8d(keyChooseLanguage), h.app.LanguageKeyboard())
	}
	return h.mainMenu(c)
}

func (h *handlers) mainMenu(c Ctx) error {
	text := c.GetL8d(keyWelcome, "name", c.Sender().FirstName)
	return c.CleanEdit(text, mainMenuKeyboard(c))
}

func (h *handlers) languageMenu(c Ctx) error {
	_ = c.Respond()
	return c.CleanEdit(c.GetL8d(keyChooseLanguage), h.app.LanguageKeyboard())
}

func (h *handlers) profile(c Ctx) error {
	_ = c.Respond()
	acc, err := c.Account()
	if err != nil {
		return err
	}
	text := c.GetL8d(keyProfile,
		"id", fmt.Sprint(acc.ID),
		"username", "@"+acc.Username,
		"lang", c.GetLanguage(),
	)
	return c.CleanEdit(text, mainMenuKeyboard(c))
}

func (h *handlers) feedbackStart(c Ctx) error {
	_ = c.Respond()
	c.Enter(sceneFeedback, qRating)
	// Remove the menu message and show the reply keyboard with Cancel.
	if err := c.CleanSendDelete(c.GetL8d(keyAskRating), cancelKeyboard(c)); err != nil {
		return err
	}
	// The prompt has a reply keyboard, not an inline one: forget it so the
	// next CleanEdit sends a fresh menu instead of editing it.
	c.Session().SetPreviousMessageWithKeyboard(0)
	return nil
}

func (h *handlers) feedbackRating(c Ctx) error {
	rating := strings.TrimSpace(c.Text())
	if len(rating) != 1 || rating < "1" || rating > "5" {
		return c.SendL8d(keyBadRating)
	}
	c.State().Rating = rating
	c.IncQuestion()
	return c.Send(c.GetL8d(keyAskComment, "rating", rating))
}

func (h *handlers) feedbackComment(c Ctx) error {
	rating := c.State().Rating
	c.State().Rating = ""
	c.ResetScene()

	// A real bot would store the feedback in its own table via h.app.DB().
	text := c.GetL8d(keyThanks, "rating", rating, "comment", c.Text())
	if err := c.Send(text, &botbase.ReplyMarkup{RemoveKeyboard: true}); err != nil {
		return err
	}
	return h.mainMenu(c)
}

func (h *handlers) cancel(c Ctx) error {
	c.ResetScene()
	c.State().Rating = ""
	if err := c.SendL8d(keyCancelled, &botbase.ReplyMarkup{RemoveKeyboard: true}); err != nil {
		return err
	}
	return h.mainMenu(c)
}

package main

import "github.com/arshamalh/botbase"

// Translation keys. Keeping them as constants gives compile-time checks.
const (
	keyWelcome        = "welcome"
	keyChooseLanguage = "choose_language"
	keyProfile        = "profile"
	keyAskRating      = "feedback.ask_rating"
	keyBadRating      = "feedback.bad_rating"
	keyAskComment     = "feedback.ask_comment"
	keyThanks         = "feedback.thanks"
	keyCancelled      = "cancelled"

	btnFeedbackText = "btn.feedback"
	btnProfileText  = "btn.profile"
	btnLanguageText = "btn.language"
	btnCancelText   = "btn.cancel"
)

var en = botbase.Translations{
	botbase.LangNameKey: "English 🇬🇧",

	keyWelcome:        "Hi {name}! 👋\nThis is a *botbase* example. What would you like to do?",
	keyChooseLanguage: "Please choose your language:",
	keyProfile:        "*Your profile*\nID: {id}\nUsername: {username}\nLanguage: {lang}",
	keyAskRating:      "How would you rate botbase? Send a number from 1 to 5.",
	keyBadRating:      "Please send a number from 1 to 5.",
	keyAskComment:     "Thanks! Any comment? (rating: {rating})",
	keyThanks:         "✅ Feedback saved: {rating}/5, \"{comment}\".",
	keyCancelled:      "Cancelled.",

	btnFeedbackText: "📝 Feedback",
	btnProfileText:  "👤 Profile",
	btnLanguageText: "🌐 Language",
	btnCancelText:   "Cancel ❌",
}

var fa = botbase.Translations{
	botbase.LangNameKey: "پارسی 🇮🇷",

	keyWelcome:        "سلام {name}! 👋\nاین یک نمونه *botbase* است. چه کاری انجام بدهیم؟",
	keyChooseLanguage: "لطفا زبان خود را انتخاب کنید:",
	keyProfile:        "*پروفایل شما*\nشناسه: {id}\nنام کاربری: {username}\nزبان: {lang}",
	keyAskRating:      "به botbase از ۱ تا ۵ چه امتیازی می‌دهید؟",
	keyBadRating:      "لطفا عددی بین ۱ تا ۵ بفرستید.",
	keyAskComment:     "ممنون! نظری دارید؟ (امتیاز: {rating})",
	keyThanks:         "✅ بازخورد ثبت شد: {rating}/5، «{comment}».",
	keyCancelled:      "لغو شد.",

	btnFeedbackText: "📝 بازخورد",
	btnProfileText:  "👤 پروفایل",
	btnLanguageText: "🌐 زبان",
	btnCancelText:   "انصراف ❌",
}

func registerLanguages(i *botbase.I18n) {
	// The first registered language is listed first in the picker.
	i.Register("en", "", en)
	i.Register("fa", "", fa)
}

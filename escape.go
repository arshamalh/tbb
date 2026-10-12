package tbb

import "strings"

// markdownV2Basics escapes MarkdownV2's reserved characters except the ones
// used for inline formatting (* _ ` and the backslash itself), so translation
// templates can keep *bold*, _italic_ and `code` while everything else (dots,
// dashes, parentheses, exclamation marks…) is sent literally.
var markdownV2Basics = strings.NewReplacer(
	"[", `\[`,
	"]", `\]`,
	"(", `\(`,
	")", `\)`,
	"~", `\~`,
	">", `\>`,
	"<", `\<`,
	"#", `\#`,
	"+", `\+`,
	"-", `\-`,
	"=", `\=`,
	"|", `\|`,
	"{", `\{`,
	"}", `\}`,
	".", `\.`,
	"!", `\!`,
)

// markdownV2All escapes every character MarkdownV2 reserves, including the
// formatting ones and the backslash.
var markdownV2All = strings.NewReplacer(
	`\`, `\\`,
	"_", `\_`,
	"*", `\*`,
	"`", "\\`",
	"[", `\[`,
	"]", `\]`,
	"(", `\(`,
	")", `\)`,
	"~", `\~`,
	">", `\>`,
	"<", `\<`,
	"#", `\#`,
	"+", `\+`,
	"-", `\-`,
	"=", `\=`,
	"|", `\|`,
	"{", `\{`,
	"}", `\}`,
	".", `\.`,
	"!", `\!`,
)

// EscapeMarkdownV2Basics prepares an authored template for ModeMarkdownV2:
// punctuation is escaped but *bold*, _italic_ and `code` markers survive.
// This is what localized sends (SendL8d, GetL8d) apply.
//
// Do not use it for user-supplied text (names, usernames, free input); a
// stray "_" or "*" there would break parsing. Use EscapeMarkdownV2 instead.
func EscapeMarkdownV2Basics(s string) string {
	return markdownV2Basics.Replace(s)
}

// EscapeMarkdownV2 escapes every MarkdownV2 reserved character, making any
// string safe to embed as plain text. Use it for user-supplied values before
// inserting them into a template.
func EscapeMarkdownV2(s string) string {
	return markdownV2All.Replace(s)
}

// markdownV2Code escapes the only characters that are special inside a
// MarkdownV2 `code` span or ```pre``` block.
var markdownV2Code = strings.NewReplacer(`\`, `\\`, "`", "\\`")

// markdownV2URL escapes the characters that are special inside the (...)
// part of a MarkdownV2 inline link or custom emoji.
var markdownV2URL = strings.NewReplacer(`\`, `\\`, ")", `\)`)

// markdownLegacy escapes the entity characters of the legacy Markdown mode.
var markdownLegacy = strings.NewReplacer("_", `\_`, "*", `\*`, "`", "\\`", "[", `\[`)

// htmlText escapes the characters Telegram's HTML mode reserves. Quotes are
// included so the result is also safe inside an attribute (href="...").
var htmlText = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")

// EscapeMarkdownV2Code escapes text placed inside a MarkdownV2 code span or
// pre block, where only '`' and '\' are special. Other characters must NOT be
// escaped there, or the backslashes show up in the code.
func EscapeMarkdownV2Code(s string) string {
	return markdownV2Code.Replace(s)
}

// EscapeMarkdownV2URL escapes a URL placed in a MarkdownV2 inline link:
// [text](url). Only ')' and '\' are special inside the parentheses.
func EscapeMarkdownV2URL(s string) string {
	return markdownV2URL.Replace(s)
}

// EscapeMarkdown escapes text for the legacy ModeMarkdown, where '_', '*',
// '`' and '[' start entities. Telegram recommends MarkdownV2 or HTML instead.
func EscapeMarkdown(s string) string {
	return markdownLegacy.Replace(s)
}

// EscapeHTML escapes text for ModeHTML: '&', '<', '>' and '"'.
func EscapeHTML(s string) string {
	return htmlText.Replace(s)
}

// Escape escapes plain text so it renders literally under mode. For
// ModeDefault (no parse mode) the text is returned unchanged.
func Escape(mode ParseMode, s string) string {
	switch mode {
	case ModeMarkdownV2:
		return EscapeMarkdownV2(s)
	case ModeMarkdown:
		return EscapeMarkdown(s)
	case ModeHTML:
		return EscapeHTML(s)
	default:
		return s
	}
}

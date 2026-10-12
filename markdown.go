package botbase

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

package tbb

import "strings"

// BtnKey is the stable identifier ("unique") of an inline button. Declare one
// constant per button and use it both to build keyboards and to register the
// callback handler, so the two can never drift apart:
//
//	const Balance tbb.BtnKey = "balance"
//
//	kb.Inline(Balance.AsRow(ctx.GetL8dBtn("btn.balance")))
//	app.Callback(Balance, h.Balance)
//
// The key must match [-\w]+ (letters, digits, '_' and '-') and, together with
// the data, fit in Telegram's 64-byte callback_data limit.
type BtnKey string

// String returns the bare key.
func (k BtnKey) String() string { return string(k) }

// Endpoint returns the "\f"-prefixed form used as a Bot.Handle endpoint.
func (k BtnKey) Endpoint() string { return "\f" + string(k) }

// Build returns an inline button with the given text. data values are joined
// with "|" and can be read back in the handler with ctx.Args().
func (k BtnKey) Build(text string, data ...string) Btn {
	return Btn{
		Unique: string(k),
		Text:   text,
		Data:   strings.Join(data, "|"),
	}
}

// AsRow returns a single-button row.
func (k BtnKey) AsRow(text string, data ...string) Row {
	return Row{k.Build(text, data...)}
}

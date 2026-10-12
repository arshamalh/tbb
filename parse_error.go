package tbb

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ErrCantParseEntities matches, with errors.Is, every error Telegram returns
// when a message's markup is invalid for its parse mode ("Bad Request: can't
// parse entities: ..."). Use errors.As with *ParseEntitiesError for details.
var ErrCantParseEntities = NewError(400, "Bad Request: can't parse entities")

// ParseEntitiesError is returned when Telegram rejects a message's markup,
// typically an unescaped MarkdownV2 character. Telegram only reports a byte
// offset, so the error also carries the text around it: in
//
//	can't parse entities: Can't find end of the entity starting at byte offset 9
//	near "Hi john▶_doe!"
//
// the '▶' marks the offending byte. Escape user-supplied values with Escape,
// EscapeMarkdownV2 or EscapeHTML to avoid it.
type ParseEntitiesError struct {
	// Description is Telegram's description, e.g. "Bad Request: can't parse
	// entities: Character '.' is reserved and must be escaped with the
	// preceding '\'".
	Description string
	// Offset is the reported byte offset into Text, or -1 if none was given.
	Offset int
	// Text is the text or caption that was sent, when it is known.
	Text string
}

var entityOffsetRe = regexp.MustCompile(`byte offset (\d+)`)

func newParseEntitiesError(description string) *ParseEntitiesError {
	e := &ParseEntitiesError{Description: description, Offset: -1}
	if m := entityOffsetRe.FindStringSubmatch(description); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil {
			e.Offset = n
		}
	}
	return e
}

func isParseEntitiesDescription(description string) bool {
	return strings.Contains(description, "can't parse entities")
}

// Error implements error.
func (e *ParseEntitiesError) Error() string {
	msg := strings.TrimPrefix(e.Description, "Bad Request: ")
	if near := e.Excerpt(); near != "" {
		return fmt.Sprintf("telegram: %s near %q (400)", msg, near)
	}
	return fmt.Sprintf("telegram: %s (400)", msg)
}

// Is makes errors.Is(err, ErrCantParseEntities) true.
func (e *ParseEntitiesError) Is(target error) bool {
	return target == ErrCantParseEntities
}

// Excerpt returns up to 20 bytes of Text on each side of Offset, with '▶'
// inserted before the offending byte. It is empty when Text or Offset is
// unknown.
func (e *ParseEntitiesError) Excerpt() string {
	if e.Text == "" || e.Offset < 0 || e.Offset > len(e.Text) {
		return ""
	}
	const radius = 20
	start, end := e.Offset-radius, e.Offset+radius
	if start < 0 {
		start = 0
	}
	if end > len(e.Text) {
		end = len(e.Text)
	}
	// Don't cut a multi-byte character in half.
	for start > 0 && !utf8.RuneStart(e.Text[start]) {
		start--
	}
	for end < len(e.Text) && !utf8.RuneStart(e.Text[end]) {
		end++
	}
	at := e.Offset
	for at > start && at < len(e.Text) && !utf8.RuneStart(e.Text[at]) {
		at--
	}

	var b strings.Builder
	if start > 0 {
		b.WriteString("…")
	}
	b.WriteString(e.Text[start:at])
	b.WriteString("▶")
	b.WriteString(e.Text[at:end])
	if end < len(e.Text) {
		b.WriteString("…")
	}
	return b.String()
}

// attachSentText adds the sent text or caption to a ParseEntitiesError, so
// its message can point at the offending characters.
func attachSentText(err error, payload interface{}) error {
	pe, ok := err.(*ParseEntitiesError)
	if !ok || pe.Text != "" {
		return err
	}
	params, ok := payload.(map[string]string)
	if !ok {
		return err
	}
	for _, key := range []string{"text", "caption", "explanation", "question"} {
		if v := params[key]; v != "" {
			pe.Text = v
			break
		}
	}
	return err
}

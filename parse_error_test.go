package tbb

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newEntityErrorBot(t *testing.T, description string) *Bot {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"ok":false,"error_code":400,"description":%q}`, description)
	}))
	t.Cleanup(srv.Close)
	b, err := NewBot(Settings{URL: srv.URL, Token: "t", Offline: true, ParseMode: ModeMarkdownV2})
	require.NoError(t, err)
	return b
}

func TestParseEntitiesErrorFromSend(t *testing.T) {
	b := newEntityErrorBot(t, "Bad Request: can't parse entities: Can't find end of the entity starting at byte offset 7")

	_, err := b.Send(ChatID(1), "Hi john_doe!")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCantParseEntities)

	var pe *ParseEntitiesError
	require.ErrorAs(t, err, &pe)
	assert.Equal(t, 7, pe.Offset)
	assert.Equal(t, "Hi john_doe!", pe.Text)
	assert.Equal(t, "Hi john▶_doe!", pe.Excerpt())
	assert.Equal(t,
		`telegram: can't parse entities: Can't find end of the entity starting at byte offset 7 near "Hi john▶_doe!" (400)`,
		err.Error())
}

func TestParseEntitiesErrorFromMultipartCaption(t *testing.T) {
	b := newEntityErrorBot(t, "Bad Request: can't parse entities: Character '.' is reserved and must be escaped with the preceding '\\'")

	photo := &Photo{File: FromReader(strings.NewReader("png")), Caption: "Total: 1.50"}
	_, err := b.Send(ChatID(1), photo)

	var pe *ParseEntitiesError
	require.ErrorAs(t, err, &pe)
	assert.Equal(t, -1, pe.Offset, "this description has no offset")
	assert.Equal(t, "Total: 1.50", pe.Text)
	assert.Equal(t, "", pe.Excerpt())
	assert.Equal(t, `telegram: can't parse entities: Character '.' is reserved and must be escaped with the preceding '\' (400)`, err.Error())
}

func TestParseEntitiesErrorExcerpt(t *testing.T) {
	long := strings.Repeat("a", 30) + "_" + strings.Repeat("b", 30)
	pe := &ParseEntitiesError{Offset: 30, Text: long}
	assert.Equal(t, "…"+strings.Repeat("a", 20)+"▶_"+strings.Repeat("b", 19)+"…", pe.Excerpt())

	// Multi-byte text: the window never splits a rune.
	pe = &ParseEntitiesError{Offset: len("سلام "), Text: "سلام _دنیا"}
	assert.Equal(t, "سلام ▶_دنیا", pe.Excerpt())

	pe = &ParseEntitiesError{Offset: 99, Text: "short"}
	assert.Equal(t, "", pe.Excerpt(), "an out-of-range offset gives no excerpt")
}

func TestOtherErrorsAreUnchanged(t *testing.T) {
	b := newEntityErrorBot(t, "Bad Request: message is not modified")
	_, err := b.Send(ChatID(1), "x")
	assert.True(t, errors.Is(err, ErrMessageNotModified))
	assert.False(t, errors.Is(err, ErrCantParseEntities))
}

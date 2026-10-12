package botbase

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newDownloadTestBot(t *testing.T, fileBody string, fileStatus int) *Bot {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/getFile"):
			fmt.Fprint(w, `{"ok":true,"result":{"file_id":"f1","file_path":"docs/file_1.zip"}}`)
		case r.URL.Path == "/file/bottoken/docs/file_1.zip":
			w.WriteHeader(fileStatus)
			fmt.Fprint(w, fileBody)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	b, err := NewBot(Settings{URL: srv.URL, Token: "token", Offline: true})
	require.NoError(t, err)
	return b
}

func TestDownloadToWriter(t *testing.T) {
	b := newDownloadTestBot(t, "zip-bytes", http.StatusOK)

	var buf bytes.Buffer
	file := &File{FileID: "f1"}
	require.NoError(t, b.DownloadToWriter(file, &buf))
	assert.Equal(t, "zip-bytes", buf.String())
	assert.Equal(t, "docs/file_1.zip", file.FilePath)
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestDownloadToWriterErrors(t *testing.T) {
	b := newDownloadTestBot(t, "gone", http.StatusNotFound)
	err := b.DownloadToWriter(&File{FileID: "f1"}, &bytes.Buffer{})
	assert.ErrorContains(t, err, "expected status 200")

	b = newDownloadTestBot(t, "zip-bytes", http.StatusOK)
	err = b.DownloadToWriter(&File{FileID: "f1"}, failingWriter{})
	assert.ErrorContains(t, err, "disk full")
}

// Handlers reach the bot through Context.Bot(), which returns the API
// interface, so DownloadToWriter must be part of it.
var _ API = (*Bot)(nil)

func TestDownloadToWriterViaContext(t *testing.T) {
	b := newDownloadTestBot(t, "zip-bytes", http.StatusOK)
	c := b.NewContext(Update{})

	var buf bytes.Buffer
	require.NoError(t, c.Bot().DownloadToWriter(&File{FileID: "f1"}, &buf))
	assert.Equal(t, "zip-bytes", buf.String())
}

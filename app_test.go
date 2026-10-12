package botbase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeAPI is a minimal Bot API server that records calls.
type fakeAPI struct {
	t   *testing.T
	srv *httptest.Server

	mu        sync.Mutex
	calls     []apiCall
	nextMsgID int
	// failNext makes the next call to a method fail with the description.
	failNext map[string]string
}

type apiCall struct {
	Method string
	Params map[string]any
}

func newFakeAPI(t *testing.T) *fakeAPI {
	f := &fakeAPI{t: t, nextMsgID: 100, failNext: map[string]string{}}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeAPI) serve(w http.ResponseWriter, r *http.Request) {
	method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
	body, _ := io.ReadAll(r.Body)
	params := map[string]any{}
	_ = json.Unmarshal(body, &params)

	f.mu.Lock()
	if method != "getUpdates" {
		f.calls = append(f.calls, apiCall{method, params})
	}
	desc, fail := f.failNext[method]
	delete(f.failNext, method)
	f.nextMsgID++
	msgID := f.nextMsgID
	f.mu.Unlock()

	if fail {
		fmt.Fprintf(w, `{"ok":false,"error_code":400,"description":%q}`, desc)
		return
	}

	var result string
	switch method {
	case "getMe":
		result = `{"id":1,"is_bot":true,"first_name":"Test","username":"test_bot"}`
	case "getUpdates":
		time.Sleep(20 * time.Millisecond)
		result = `[]`
	case "sendMessage", "editMessageText", "editMessageReplyMarkup":
		chat, _ := params["chat_id"].(string)
		result = fmt.Sprintf(`{"message_id":%d,"chat":{"id":%s,"type":"private"},"text":"x"}`, msgID, chat)
	default:
		result = `true`
	}
	fmt.Fprintf(w, `{"ok":true,"result":%s}`, result)
}

func (f *fakeAPI) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = nil
}

func (f *fakeAPI) methods() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.calls))
	for _, c := range f.calls {
		out = append(out, c.Method)
	}
	return out
}

func (f *fakeAPI) last(method string) map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.calls) - 1; i >= 0; i-- {
		if f.calls[i].Method == method {
			return f.calls[i].Params
		}
	}
	f.t.Fatalf("no %s call recorded", method)
	return nil
}

func (f *fakeAPI) fail(method, description string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failNext[method] = description
}

type flowState struct {
	Amount string `json:"amount,omitempty"`
}

const (
	sceneTopUp Scene = iota + 1
)

const (
	qAmount Question = iota + 1
	qConfirm
)

const btnTopUp BtnKey = "topup"

func newTestApp(t *testing.T, store Store, opts ...AppOption[flowState]) (*App[flowState], *fakeAPI) {
	t.Helper()
	api := newFakeAPI(t)
	if store == nil {
		store = NewMemoryStore()
	}
	base := []AppOption[flowState]{
		WithStore[flowState](store),
		WithLogger[flowState](slog.New(slog.NewTextHandler(io.Discard, nil))),
		WithSettings[flowState](func(s *Settings) {
			s.URL = api.srv.URL
			s.Synchronous = true
		}),
	}
	app, err := New[flowState](Config{Telegram: TelegramConfig{Token: "test-token"}}, append(base, opts...)...)
	require.NoError(t, err)

	app.I18n().Register("en", "English", Translations{
		"welcome":    "Hi {name}! Balance: $1.50 (approx.)",
		"btn.cancel": "Cancel",
		"ask.amount": "How much?",
		"done":       "Done.",
	})
	app.I18n().Register("fa", "فارسی", Translations{
		"welcome":    "سلام {name}!",
		"btn.cancel": "انصراف",
	})
	return app, api
}

var testSender = &User{ID: 7, FirstName: "Ars_ham", Username: "arsham"}

func textUpdate(text string) Update {
	return Update{Message: &Message{
		ID:     1,
		Text:   text,
		Sender: testSender,
		Chat:   &Chat{ID: testSender.ID, Type: ChatPrivate},
	}}
}

func callbackUpdate(key BtnKey, data string) Update {
	payload := "\f" + key.String()
	if data != "" {
		payload += "|" + data
	}
	return Update{Callback: &Callback{
		ID:     "cb1",
		Sender: testSender,
		Data:   payload,
		Message: &Message{
			ID:   50,
			Chat: &Chat{ID: testSender.ID, Type: ChatPrivate},
		},
	}}
}

func TestCommandLinksAccountAndRendersMarkdown(t *testing.T) {
	store := NewMemoryStore()
	app, api := newTestApp(t, store)

	app.Command("/start", func(c Ctx[flowState]) error {
		return c.Send(c.GetL8d("welcome", "name", c.Sender().FirstName))
	})
	app.Bot().ProcessUpdate(textUpdate("/start"))

	sent := api.last("sendMessage")
	assert.Equal(t, `Hi Ars\_ham\! Balance: $1\.50 \(approx\.\)`, sent["text"])
	assert.Equal(t, "MarkdownV2", sent["parse_mode"])

	acc, err := store.AccountByTelegramID(context.Background(), 7)
	require.NoError(t, err)
	assert.Equal(t, "arsham", acc.Username)

	sess, err := app.Sessions().Get(context.Background(), 7)
	require.NoError(t, err)
	assert.Equal(t, acc.ID, sess.UserID())

	payload, err := store.LoadSession(context.Background(), 7)
	require.NoError(t, err, "linking the account changed the session, so it was saved")
	assert.Contains(t, string(payload), `"uid":1`)
}

func TestSceneRouting(t *testing.T) {
	app, api := newTestApp(t, nil)

	var got []string
	app.Callback(btnTopUp, func(c Ctx[flowState]) error {
		got = append(got, "callback:"+c.Callback().Data)
		c.Enter(sceneTopUp, qAmount)
		return c.Respond()
	})
	app.Scene(sceneTopUp).
		On(qAmount, func(c Ctx[flowState]) error {
			c.State().Amount = c.Text()
			c.IncQuestion()
			got = append(got, "amount:"+c.Text())
			return nil
		}).
		On(qConfirm, func(c Ctx[flowState]) error {
			got = append(got, "confirm:"+c.State().Amount)
			c.ResetScene()
			return nil
		})
	app.Reply("btn.cancel", func(c Ctx[flowState]) error {
		got = append(got, "cancel")
		c.ResetScene()
		return nil
	})
	app.DefaultText(func(c Ctx[flowState]) error {
		got = append(got, "default:"+c.Text())
		return nil
	})

	bot := app.Bot()
	bot.ProcessUpdate(textUpdate("hello"))             // no scene → default
	bot.ProcessUpdate(callbackUpdate(btnTopUp, "ton")) // enters the scene
	bot.ProcessUpdate(textUpdate("25"))                // qAmount
	bot.ProcessUpdate(textUpdate("yes"))               // qConfirm, leaves scene
	bot.ProcessUpdate(textUpdate("again"))             // default
	bot.ProcessUpdate(callbackUpdate(btnTopUp, ""))    // enters again
	bot.ProcessUpdate(textUpdate("انصراف"))            // reply button in Farsi wins over the scene
	bot.ProcessUpdate(textUpdate("after cancel"))      // default

	assert.Equal(t, []string{
		"default:hello",
		"callback:ton",
		"amount:25",
		"confirm:25",
		"default:again",
		"callback:",
		"cancel",
		"default:after cancel",
	}, got)
	assert.Contains(t, api.methods(), "answerCallbackQuery")
}

func TestSceneDefaultAndUnknownCallback(t *testing.T) {
	app, api := newTestApp(t, nil)

	var got []string
	app.Callback(btnTopUp, func(c Ctx[flowState]) error {
		c.Enter(sceneTopUp, 99)
		return nil
	})
	app.Scene(sceneTopUp).Default(func(c Ctx[flowState]) error {
		got = append(got, "scene-default")
		return nil
	})

	app.Bot().ProcessUpdate(callbackUpdate(btnTopUp, ""))
	app.Bot().ProcessUpdate(textUpdate("x"))
	assert.Equal(t, []string{"scene-default"}, got)

	api.reset()
	app.Bot().ProcessUpdate(callbackUpdate("unknown", ""))
	assert.Equal(t, []string{"answerCallbackQuery"}, api.methods(), "unknown buttons stop spinning")
}

func TestSessionSurvivesRestart(t *testing.T) {
	store := NewMemoryStore()
	app, _ := newTestApp(t, store)
	app.Callback(btnTopUp, func(c Ctx[flowState]) error {
		c.Enter(sceneTopUp, qAmount)
		c.State().Amount = "10"
		return nil
	})
	app.Bot().ProcessUpdate(callbackUpdate(btnTopUp, ""))

	restarted, _ := newTestApp(t, store)
	var seen string
	restarted.Scene(sceneTopUp).On(qAmount, func(c Ctx[flowState]) error {
		seen = c.State().Amount
		return nil
	})
	restarted.Bot().ProcessUpdate(textUpdate("20"))
	assert.Equal(t, "10", seen)
}

func TestCleanHelpers(t *testing.T) {
	app, api := newTestApp(t, nil)

	var step func(c Ctx[flowState]) error
	app.Command("/go", func(c Ctx[flowState]) error { return step(c) })
	run := func(fn func(c Ctx[flowState]) error) {
		api.reset()
		step = fn
		app.Bot().ProcessUpdate(textUpdate("/go"))
	}

	// First CleanEdit has nothing to edit: it sends.
	run(func(c Ctx[flowState]) error { return c.CleanEdit("menu") })
	assert.Equal(t, []string{"sendMessage"}, api.methods())
	sess, _ := app.Sessions().Get(context.Background(), 7)
	first := sess.GetPreviousMessageWithKeyboard()
	require.NotZero(t, first)

	// CleanEdit edits that message in place.
	run(func(c Ctx[flowState]) error { return c.CleanEdit("balance") })
	assert.Equal(t, []string{"editMessageText"}, api.methods())
	assert.Equal(t, fmt.Sprint(first), fmt.Sprint(api.last("editMessageText")["message_id"]))
	assert.Equal(t, first, sess.GetPreviousMessageWithKeyboard())

	// "Not modified" is not an error and doesn't send anything.
	api.fail("editMessageText", "Bad Request: message is not modified")
	run(func(c Ctx[flowState]) error { return c.CleanEdit("balance") })
	assert.Equal(t, []string{"editMessageText"}, api.methods())

	// Other edit failures fall back to delete + send.
	api.fail("editMessageText", "Bad Request: there is no text in the message to edit")
	run(func(c Ctx[flowState]) error { return c.CleanEdit("menu") })
	assert.Equal(t, []string{"editMessageText", "deleteMessage", "sendMessage"}, api.methods())
	second := sess.GetPreviousMessageWithKeyboard()
	assert.NotEqual(t, first, second)

	// CleanSend strips the old keyboard and sends.
	run(func(c Ctx[flowState]) error { return c.CleanSend("new") })
	assert.Equal(t, []string{"editMessageReplyMarkup", "sendMessage"}, api.methods())
	assert.Equal(t, fmt.Sprint(second), fmt.Sprint(api.last("editMessageReplyMarkup")["message_id"]))

	// CleanSendDelete falls back to stripping when deletion fails.
	api.fail("deleteMessage", "Bad Request: message can't be deleted")
	run(func(c Ctx[flowState]) error { return c.CleanSendDelete("again") })
	assert.Equal(t, []string{"deleteMessage", "editMessageReplyMarkup", "sendMessage"}, api.methods())
}

func TestRenderParseModes(t *testing.T) {
	app, _ := newTestApp(t, nil, WithSettings[flowState](func(s *Settings) { s.ParseMode = ModeHTML }))
	app.I18n().Register("en", "", Translations{"hi": "<b>Hi</b> {name}"})
	assert.Equal(t, "<b>Hi</b> &lt;script&gt;", app.render("en", "hi", "name", "<script>"))

	plain, _ := newTestApp(t, nil, WithSettings[flowState](func(s *Settings) { s.ParseMode = ModeDefault }))
	plain.I18n().Register("en", "", Translations{"hi": "Hi {name}."})
	assert.Equal(t, "Hi a_b.", plain.render("en", "hi", "name", "a_b"))
}

func TestMiddlewareOrderAndPanicRecovery(t *testing.T) {
	var errs []error
	app, _ := newTestApp(t, nil, WithSettings[flowState](func(s *Settings) {
		s.OnError = func(err error, _ Context) { errs = append(errs, err) }
	}))

	var order []string
	mw := func(name string) Middleware[flowState] {
		return func(next Handler[flowState]) Handler[flowState] {
			return func(c Ctx[flowState]) error {
				order = append(order, name)
				return next(c)
			}
		}
	}
	app.Use(mw("a"), mw("b"))
	app.Command("/ok", func(c Ctx[flowState]) error { order = append(order, "h"); return nil })
	app.Command("/panic", func(c Ctx[flowState]) error { panic("boom") })
	app.Command("/fail", func(c Ctx[flowState]) error { return errors.New("nope") })

	app.Bot().ProcessUpdate(textUpdate("/ok"))
	assert.Equal(t, []string{"a", "b", "h"}, order)

	app.Bot().ProcessUpdate(textUpdate("/panic"))
	app.Bot().ProcessUpdate(textUpdate("/fail"))
	require.Len(t, errs, 2)
	assert.ErrorContains(t, errs[0], "handler panic: boom")
	assert.EqualError(t, errs[1], "nope")

	// The chat lock was released despite the panic.
	done := make(chan struct{})
	go func() { app.Bot().ProcessUpdate(textUpdate("/ok")); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("chat lock leaked after panic")
	}
}

func TestHandleRawEndpointAndRun(t *testing.T) {
	app, _ := newTestApp(t, nil)

	var doc string
	app.Handle(OnDocument, func(c Ctx[flowState]) error {
		doc = c.Message().Document.FileName
		return nil
	})
	upd := textUpdate("")
	upd.Message.Document = &Document{FileName: "app.zip"}
	app.Bot().ProcessUpdate(upd)
	assert.Equal(t, "app.zip", doc)

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- app.Run(ctx) }()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case err := <-result:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop")
	}
}

func TestNewWithDefaultSQLiteStore(t *testing.T) {
	api := newFakeAPI(t)
	dsn := "file:" + t.TempDir() + "/bot.db"
	app, err := New[flowState](Config{
		Telegram: TelegramConfig{Token: "t", URL: api.srv.URL},
		DB:       DBConfig{DSN: dsn},
	}, WithSettings[flowState](func(s *Settings) { s.Synchronous = true }))
	require.NoError(t, err)
	require.NotNil(t, app.DB())
	defer app.Close()

	app.Command("/start", func(c Ctx[flowState]) error {
		c.State().Amount = "5"
		return nil
	})
	app.Bot().ProcessUpdate(textUpdate("/start"))

	payload, err := app.Store().LoadSession(context.Background(), 7)
	require.NoError(t, err)
	assert.Contains(t, string(payload), `"amount":"5"`)
}

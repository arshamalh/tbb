package tbb

// Scene groups the steps of a multi-message conversation flow (a "wizard"),
// such as "top up" or "deploy". Zero means "no scene": the user is at the
// main menu and free text goes to the default text handler.
type Scene int

// Question is a step inside a Scene. Zero is the scene's entry point.
type Question int

// Next returns the following question.
func (q Question) Next() Question { return q + 1 }

// Session is the per-chat conversation state. The framework owns the core
// fields; the app adds its own state through the type parameter T (any
// JSON-serializable struct), reachable as State.
//
// Sessions are cached in memory and written through to the database whenever
// a handler changes them, so they survive restarts.
type Session[T any] struct {
	ChatID int64 `json:"chat_id"`

	// UID is the database ID of the account (accounts.id) behind this chat.
	// It is filled automatically the first time the chat talks to the bot.
	UID int64 `json:"uid,omitempty"`

	Scene    Scene    `json:"scene,omitempty"`
	Question Question `json:"question,omitempty"`
	Language string   `json:"language,omitempty"`

	// KeyboardMsgID is the last message sent with an inline keyboard; the
	// Clean* helpers use it to strip, edit or delete that message.
	KeyboardMsgID int `json:"keyboard_msg_id,omitempty"`

	// State holds the app-specific session data.
	State T `json:"state"`
}

func newSession[T any](chatID int64) *Session[T] {
	return &Session[T]{ChatID: chatID}
}

// ID returns the Telegram chat ID the session belongs to.
func (s *Session[T]) ID() int64 { return s.ChatID }

// UserID returns the database ID (accounts.id) of the user behind the chat.
func (s *Session[T]) UserID() int64 { return s.UID }

// SetUserID sets the database ID (accounts.id) of the user behind the chat.
func (s *Session[T]) SetUserID(id int64) { s.UID = id }

// GetScene returns the current scene.
func (s *Session[T]) GetScene() Scene { return s.Scene }

// SetScene moves the chat into a scene and resets the question to the
// scene's entry point.
func (s *Session[T]) SetScene(scene Scene) {
	s.Scene = scene
	s.Question = 0
}

// GetCurrentQuestion returns the current question of the scene.
func (s *Session[T]) GetCurrentQuestion() Question { return s.Question }

// SetNextQuestion sets the question the next text message answers.
func (s *Session[T]) SetNextQuestion(q Question) { s.Question = q }

// IncQuestion advances to the next question.
func (s *Session[T]) IncQuestion() { s.Question++ }

// Enter is a shortcut for SetScene followed by SetNextQuestion.
func (s *Session[T]) Enter(scene Scene, q Question) {
	s.Scene = scene
	s.Question = q
}

// ResetScene leaves any scene, returning the chat to the main menu.
func (s *Session[T]) ResetScene() {
	s.Scene = 0
	s.Question = 0
}

// GetLanguage returns the chat's language code ("" when not chosen yet).
func (s *Session[T]) GetLanguage() string { return s.Language }

// GetPreviousMessageWithKeyboard returns the ID of the last message sent with
// an inline keyboard through the Clean* helpers.
func (s *Session[T]) GetPreviousMessageWithKeyboard() int { return s.KeyboardMsgID }

// SetPreviousMessageWithKeyboard records the last message that carries an
// inline keyboard.
func (s *Session[T]) SetPreviousMessageWithKeyboard(id int) { s.KeyboardMsgID = id }

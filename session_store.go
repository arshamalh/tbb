package botbase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

// SessionStore is a write-through cache of sessions: reads are served from
// memory, every change is persisted to the Store, and a chat missing from the
// cache (e.g. after a restart) is loaded from the Store on first access.
//
// It also hands out a per-chat lock so updates from the same chat are handled
// one at a time, which keeps scene transitions consistent even though the bot
// processes updates concurrently.
type SessionStore[T any] struct {
	store Store

	mu       sync.RWMutex
	sessions map[int64]*Session[T]

	locksMu sync.Mutex
	locks   map[int64]*chatLock
}

type chatLock struct {
	mu   sync.Mutex
	refs int
}

// NewSessionStore creates a SessionStore backed by store.
func NewSessionStore[T any](store Store) *SessionStore[T] {
	return &SessionStore[T]{
		store:    store,
		sessions: make(map[int64]*Session[T]),
		locks:    make(map[int64]*chatLock),
	}
}

// Lock serializes work for one chat. Call the returned function to release.
func (ss *SessionStore[T]) Lock(chatID int64) (unlock func()) {
	ss.locksMu.Lock()
	l, ok := ss.locks[chatID]
	if !ok {
		l = &chatLock{}
		ss.locks[chatID] = l
	}
	l.refs++
	ss.locksMu.Unlock()

	l.mu.Lock()
	return func() {
		l.mu.Unlock()
		ss.locksMu.Lock()
		l.refs--
		if l.refs == 0 {
			delete(ss.locks, chatID)
		}
		ss.locksMu.Unlock()
	}
}

// Get returns the session of a chat, loading it from the Store or creating a
// fresh one when needed. The returned pointer is shared: hold the chat's Lock
// while mutating it.
func (ss *SessionStore[T]) Get(ctx context.Context, chatID int64) (*Session[T], error) {
	ss.mu.RLock()
	s, ok := ss.sessions[chatID]
	ss.mu.RUnlock()
	if ok {
		return s, nil
	}

	s = newSession[T](chatID)
	payload, err := ss.store.LoadSession(ctx, chatID)
	switch {
	case errors.Is(err, ErrRecordNotFound):
		// First contact: start with an empty session.
	case err != nil:
		return nil, fmt.Errorf("load session %d: %w", chatID, err)
	default:
		if err := json.Unmarshal(payload, s); err != nil {
			return nil, fmt.Errorf("decode session %d: %w", chatID, err)
		}
		s.ChatID = chatID
	}

	ss.mu.Lock()
	defer ss.mu.Unlock()
	if existing, ok := ss.sessions[chatID]; ok {
		// Another goroutine loaded it first; keep a single shared instance.
		return existing, nil
	}
	ss.sessions[chatID] = s
	return s, nil
}

// Save persists a session unconditionally.
func (ss *SessionStore[T]) Save(ctx context.Context, s *Session[T]) error {
	payload, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("encode session %d: %w", s.ChatID, err)
	}
	return ss.store.SaveSession(ctx, s.ChatID, payload)
}

// Snapshot returns the JSON encoding of a session, used to detect changes.
func (ss *SessionStore[T]) Snapshot(s *Session[T]) []byte {
	b, _ := json.Marshal(s)
	return b
}

// Forget drops a chat from the in-memory cache; the next Get reloads it from
// the Store.
func (ss *SessionStore[T]) Forget(chatID int64) {
	ss.mu.Lock()
	delete(ss.sessions, chatID)
	ss.mu.Unlock()
}

// Len returns the number of cached sessions.
func (ss *SessionStore[T]) Len() int {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return len(ss.sessions)
}

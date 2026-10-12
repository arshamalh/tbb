package botbase

import (
	"context"
	"sync"
	"time"
)

// MemoryStore is a Store that keeps everything in memory. Nothing survives a
// restart, so it is meant for tests and throwaway bots.
type MemoryStore struct {
	mu       sync.Mutex
	sessions map[int64][]byte
	accounts map[int64]*Account // by ID
	nextID   int64
}

// NewMemoryStore returns an empty MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		sessions: make(map[int64][]byte),
		accounts: make(map[int64]*Account),
	}
}

func (m *MemoryStore) LoadSession(_ context.Context, chatID int64) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.sessions[chatID]
	if !ok {
		return nil, ErrRecordNotFound
	}
	return append([]byte(nil), p...), nil
}

func (m *MemoryStore) SaveSession(_ context.Context, chatID int64, payload []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[chatID] = append([]byte(nil), payload...)
	return nil
}

func (m *MemoryStore) UpsertAccount(_ context.Context, a *Account) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for _, existing := range m.accounts {
		if existing.TelegramID == a.TelegramID {
			existing.Username, existing.FirstName, existing.LastName = a.Username, a.FirstName, a.LastName
			existing.UpdatedAt = now
			*a = *existing
			return nil
		}
	}
	m.nextID++
	a.ID = m.nextID
	a.CreatedAt, a.UpdatedAt = now, now
	cp := *a
	m.accounts[a.ID] = &cp
	return nil
}

func (m *MemoryStore) AccountByID(_ context.Context, id int64) (*Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.accounts[id]
	if !ok {
		return nil, ErrRecordNotFound
	}
	cp := *u
	return &cp, nil
}

func (m *MemoryStore) AccountByTelegramID(_ context.Context, telegramID int64) (*Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.accounts {
		if u.TelegramID == telegramID {
			cp := *u
			return &cp, nil
		}
	}
	return nil, ErrRecordNotFound
}

func (m *MemoryStore) SetAccountLanguage(_ context.Context, accountID int64, lang string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.accounts[accountID]
	if !ok {
		return ErrRecordNotFound
	}
	u.Language = lang
	u.UpdatedAt = time.Now()
	return nil
}

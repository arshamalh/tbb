package tbb

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testState struct {
	Step  string `json:"step,omitempty"`
	Count int    `json:"count,omitempty"`
}

func TestSessionStoreWriteThroughAndReload(t *testing.T) {
	ctx := context.Background()
	backend := NewMemoryStore()

	ss := NewSessionStore[testState](backend)
	s, err := ss.Get(ctx, 42)
	require.NoError(t, err)
	assert.Equal(t, int64(42), s.ID())

	s.Enter(3, 2)
	s.Language = "fa"
	s.State.Count = 7
	require.NoError(t, ss.Save(ctx, s))

	again, err := ss.Get(ctx, 42)
	require.NoError(t, err)
	assert.Same(t, s, again, "cached sessions are shared")

	// A fresh store (simulating a restart) reloads from the backend.
	restarted := NewSessionStore[testState](backend)
	loaded, err := restarted.Get(ctx, 42)
	require.NoError(t, err)
	assert.Equal(t, Scene(3), loaded.GetScene())
	assert.Equal(t, Question(2), loaded.GetCurrentQuestion())
	assert.Equal(t, "fa", loaded.GetLanguage())
	assert.Equal(t, 7, loaded.State.Count)
}

func TestSessionStoreSnapshotDetectsStateChanges(t *testing.T) {
	ss := NewSessionStore[testState](NewMemoryStore())
	s, _ := ss.Get(context.Background(), 1)

	before := ss.Snapshot(s)
	assert.Equal(t, before, ss.Snapshot(s))
	s.State.Step = "x"
	assert.NotEqual(t, before, ss.Snapshot(s))
}

func TestSessionHelpers(t *testing.T) {
	s := newSession[testState](1)
	s.Enter(2, 5)
	s.SetScene(4)
	assert.Equal(t, Question(0), s.GetCurrentQuestion(), "SetScene starts at the entry question")
	s.IncQuestion()
	assert.Equal(t, Question(1), s.GetCurrentQuestion())
	assert.Equal(t, Question(2), s.GetCurrentQuestion().Next())
	s.ResetScene()
	assert.Equal(t, Scene(0), s.GetScene())
}

func TestSessionStoreLockSerializesPerChat(t *testing.T) {
	ss := NewSessionStore[testState](NewMemoryStore())

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		active  int
		maxSeen int
	)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unlock := ss.Lock(7)
			defer unlock()
			mu.Lock()
			active++
			if active > maxSeen {
				maxSeen = active
			}
			mu.Unlock()
			time.Sleep(time.Millisecond)
			mu.Lock()
			active--
			mu.Unlock()
		}()
	}
	wg.Wait()
	assert.Equal(t, 1, maxSeen)
	assert.Empty(t, ss.locks, "locks are released once nobody holds them")
}

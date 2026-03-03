package store

import (
	"sync"
)

// Session holds the current state for a task session.
type Session struct {
	ID        string `json:"id"`
	CSS       string `json:"css"`
	Code      string `json:"code"`
	Language  string `json:"language"`
	Spec      string `json:"spec,omitempty"`
	Narration string `json:"narration,omitempty"`
}

// Store is an in-memory session store (MVP; no DB required).
type Store struct {
	mu   sync.RWMutex
	sess map[string]*Session
}

// New returns a new in-memory store.
func New() *Store {
	return &Store{sess: make(map[string]*Session)}
}

// Put saves or updates a session.
func (s *Store) Put(sess *Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sess[sess.ID] = sess
}

// Get returns a session by ID, or nil.
func (s *Store) Get(id string) *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sess[id]
	if !ok {
		return nil
	}
	// Return a copy so callers don't mutate shared state
	return &Session{
		ID:        sess.ID,
		CSS:       sess.CSS,
		Code:      sess.Code,
		Language:  sess.Language,
		Spec:      sess.Spec,
		Narration: sess.Narration,
	}
}

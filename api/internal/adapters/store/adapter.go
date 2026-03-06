package store

import (
	core "code-commenter/api/internal/store"
	"code-commenter/api/internal/ports"
)

// Adapter maps in-memory store to session repository port.
type Adapter struct {
	Store *core.Store
}

func (a *Adapter) Put(sess ports.SessionData) {
	a.Store.Put(&core.Session{
		ID:        sess.ID,
		CSS:       sess.CSS,
		Code:      sess.Code,
		Language:  sess.Language,
		Spec:      sess.Spec,
		Narration: sess.Narration,
	})
}

func (a *Adapter) Get(id string) *ports.SessionData {
	sess := a.Store.Get(id)
	if sess == nil {
		return nil
	}
	return &ports.SessionData{
		ID:        sess.ID,
		CSS:       sess.CSS,
		Code:      sess.Code,
		Language:  sess.Language,
		Spec:      sess.Spec,
		Narration: sess.Narration,
	}
}

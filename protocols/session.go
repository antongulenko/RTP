package protocols

import (
	"fmt"
	"sync"

	"github.com/antongulenko/RTP/helpers"
)

type Sessions map[interface{}]*SessionBase

type SessionBase struct {
	Wg         *sync.WaitGroup
	Stopped    *helpers.OneshotCondition
	CleanupErr error
	Session    Session
}

type Session interface {
	Start()
	Observees() []helpers.Observee
	Cleanup()
}

func (sessions Sessions) NewSession(key interface{}, session Session) *SessionBase {
	base := &SessionBase{
		Wg:      new(sync.WaitGroup),
		Stopped: helpers.NewOneshotCondition(),
		Session: session,
	}
	sessions[key] = base
	base.observe()
	session.Start()
	return base
}

func (sessions Sessions) Get(key interface{}) Session {
	if base, ok := sessions[key]; ok {
		return base.Session
	} else {
		return nil
	}
}

func (sessions Sessions) ReKeySession(oldKey, newKey interface{}) (*SessionBase, error) {
	if session, ok := sessions[oldKey]; ok {
		if _, ok := sessions[newKey]; ok {
			return nil, fmt.Errorf("Session already exists for %v", newKey)
		} else {
			sessions[newKey] = session
			delete(sessions, oldKey)
			return session, nil
		}
	} else {
		return nil, fmt.Errorf("No session found for %v", oldKey)
	}
}

func (sessions Sessions) StopSessions() {
	for _, session := range sessions {
		session.Stop()
	}
}

func (sessions Sessions) StopSession(key interface{}) error {
	if session, ok := sessions[key]; !ok {
		return fmt.Errorf("No session found for %v", key)
	} else {
		var err error
		if session.Stopped.Enabled() {
			var errStr string
			if session.CleanupErr == nil {
				errStr = "(no error)"
			} else {
				errStr = session.CleanupErr.Error()
			}
			err = fmt.Errorf("Session stopped prematurely: %v", errStr)
		} else {
			session.Stop()
			err = session.CleanupErr
		}
		delete(sessions, key)
		return err
	}
	return nil
}

func (base *SessionBase) observe() {
	if len(base.Session.Observees()) < 1 {
		return
	}
	go func() {
		helpers.WaitForAnyObservee(base.Wg, base.Session.Observees())
		base.Stop()
	}()
}

func (base *SessionBase) Stop() {
	base.Stopped.Enable(func() {
		for _, observee := range base.Session.Observees() {
			observee.Stop()
		}
		base.Wg.Wait()
		base.Session.Cleanup()
	})
}

package protocols

import (
	"fmt"
	"sync"

	"github.com/antongulenko/golib"
)

type Sessions map[interface{}]*SessionBase

type SessionBase struct {
	Wg         *sync.WaitGroup
	Stopped    *golib.OneshotCondition
	CleanupErr error
	Session    Session
}

type Session interface {
	Start(base *SessionBase)
	Observees() []golib.Observee
	Cleanup()
}

func (sessions Sessions) StartSession(key interface{}, session Session) {
	base := &SessionBase{
		Wg:      new(sync.WaitGroup),
		Stopped: golib.NewOneshotCondition(),
		Session: session,
	}
	sessions[key] = base
	base.observe()
	session.Start(base)
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
		if newKey == oldKey {
			return session, nil
		}
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

func (sessions Sessions) DeleteSessions() error {
	errors := make(golib.MultiError, 0, len(sessions))
	for key, session := range sessions {
		if err := session.StopAndFormatError(); err != nil {
			errors = append(errors, err)
		}
		delete(sessions, key)
	}
	return errors.NilOrError()
}

func (sessions Sessions) DeleteSession(key interface{}) error {
	if session, ok := sessions[key]; !ok {
		return fmt.Errorf("No session found for %v", key)
	} else {
		err := session.StopAndFormatError()
		delete(sessions, key)
		return err
	}
	return nil
}

func (sessions Sessions) StopSession(key interface{}) error {
	if session, ok := sessions[key]; !ok {
		return fmt.Errorf("No session found for %v", key)
	} else {
		session.Stop()
		return session.CleanupErr
	}
}

func (base *SessionBase) StopAndFormatError() error {
	if base.Stopped.Enabled() {
		var errStr string
		if base.CleanupErr == nil {
			errStr = "(no error)"
		} else {
			errStr = base.CleanupErr.Error()
		}
		return fmt.Errorf("Session stopped prematurely: %v", errStr)
	} else {
		base.Stop()
		return base.CleanupErr
	}
}

func (base *SessionBase) observe() {
	if len(base.Session.Observees()) < 1 {
		return
	}
	go func() {
		golib.WaitForAnyObservee(base.Wg, base.Session.Observees())
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

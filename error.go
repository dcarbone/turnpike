package turnpike

import "fmt"

type Error interface {
	error
	Terminal() bool
}

type errPeerClosed struct{}

func (errPeerClosed) Error() string  { return "peer is closed" }
func (errPeerClosed) Terminal() bool { return true }

type errMessageContextFinished struct{ err error }

func (e errMessageContextFinished) Error() string { return e.err.Error() }
func (errMessageContextFinished) Terminal() bool  { return false }

type errMessageIsNil struct{}

func (errMessageIsNil) Error() string  { return "message cannot be nil" }
func (errMessageIsNil) Terminal() bool { return false }

type errMessageSerialize struct{ err error }

func (e errMessageSerialize) Error() string { return fmt.Sprintf("serialize error: %s", e.err.Error()) }
func (errMessageSerialize) Terminal() bool  { return false }

type errMessageDeserialize struct{ err error }

func (e errMessageDeserialize) Error() string {
	return fmt.Sprintf("deserialize error: %s", e.err.Error())
}
func (errMessageDeserialize) Terminal() bool { return false }

type errSocketWrite struct{ err error }

func (e errSocketWrite) Error() string { return fmt.Sprintf("socket write error: %s", e.err.Error()) }
func (errSocketWrite) Terminal() bool  { return true }

type errSocketRead struct{ err error }

func (e errSocketRead) Error() string { return fmt.Sprintf("socket read error: %s", e.err.Error()) }
func (errSocketRead) Terminal() bool  { return true }

// TODO: use later.
//type MultiError struct {
//	mu       sync.Mutex
//	errors   []error
//	terminal bool
//}
//
//func newMultiError(err error, t bool) MultiError {
//	e := MultiError{
//		errors:   make([]error, 0),
//		terminal: t,
//	}
//	e.Add(err)
//	return e
//}
//
//// Add will append the incoming error to the internal list if not nil.  If nil, this is a no-op
//func (e *MultiError) Add(err error) {
//	e.mu.Lock()
//	if err != nil {
//		e.errors = append(e.errors, err)
//	}
//	e.mu.Unlock()
//}
//
//func (e MultiError) Len() int {
//	e.mu.Lock()
//	l := len(e.errors)
//	e.mu.Unlock()
//	return l
//}
//
//func (e MultiError) Error() string {
//	e.mu.Lock()
//	var s string
//	l := len(e.errors)
//	for i := l - 1; i >= 0; i-- {
//		s = fmt.Sprintf("\t%d: %s\n%s", i, e.errors[i], s)
//	}
//	s = fmt.Sprintf("%d error(s):\n%s", l, s)
//	e.mu.Unlock()
//	return s
//}
//
//func (e MultiError) String() string {
//	return e.Error()
//}
//
//func (e MultiError) Err() error {
//	if e.Len() > 0 {
//		return errors.New(e.Error())
//	}
//	return nil
//}
//
//func (e MultiError) Terminal() bool {
//	e.mu.Lock()
//	t := e.terminal
//	e.mu.Unlock()
//	return t
//}

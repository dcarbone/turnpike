package turnpike

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var defaultWelcomeDetails = map[string]interface{}{
	"roles": map[string]struct{}{
		"broker": {},
		"dealer": {},
	},
}

type RealmExistsError string

func (e RealmExistsError) Error() string {
	return "realm exists: " + string(e)
}

type NoSuchRealmError string

func (e NoSuchRealmError) Error() string {
	return "no such realm: " + string(e)
}

// Peer was unable to authenticate
type AuthenticationError string

func (e AuthenticationError) Error() string {
	return "authentication error: " + string(e)
}

// A Router handles new Peers and routes requests to the requested Realm.
type Router interface {
	Accept(Peer) error
	Close() error
	RegisterRealm(*Realm) error
	GetLocalPeer(URI, map[string]interface{}) (Peer, error)
	AddSessionOpenCallback(func(*Session, string))
	AddSessionCloseCallback(func(*Session, string))
}

// DefaultRouter is the default WAMP router implementation.
type defaultRouter struct {
	ctx context.Context

	realms     map[URI]*Realm
	realmsLock sync.RWMutex

	closed     bool
	closedLock sync.RWMutex

	sessionOpenCallbacks     []func(*Session, string)
	sessionOpenCallbacksLock sync.RWMutex

	sessionCloseCallbacks     []func(*Session, string)
	sessionCloseCallbacksLock sync.RWMutex
}

// NewDefaultRouter creates a very basic WAMP router.
func NewDefaultRouter() Router {
	return &defaultRouter{
		ctx: context.Background(),

		realms: make(map[URI]*Realm),

		sessionOpenCallbacks:  []func(*Session, string){},
		sessionCloseCallbacks: []func(*Session, string){},
	}
}

func (r *defaultRouter) AddSessionOpenCallback(fn func(*Session, string)) {
	r.closedLock.RLock()
	defer r.closedLock.RUnlock()
	if r.closed {
		log.Println("Router is closed, will not add session open callback")
		return
	}

	r.sessionOpenCallbacksLock.Lock()
	defer r.sessionOpenCallbacksLock.Unlock()

	r.sessionOpenCallbacks = append(r.sessionOpenCallbacks, fn)
}

func (r *defaultRouter) AddSessionCloseCallback(fn func(*Session, string)) {
	r.closedLock.RLock()
	defer r.closedLock.RUnlock()
	if r.closed {
		log.Println("Router is closed, will not add session close callback")
		return
	}

	r.sessionCloseCallbacksLock.Lock()
	defer r.sessionCloseCallbacksLock.Unlock()

	r.sessionCloseCallbacks = append(r.sessionCloseCallbacks, fn)
}

func (r *defaultRouter) Close() error {
	r.closedLock.Lock()
	if r.closed {
		r.closedLock.Unlock()
		return fmt.Errorf("Router is already closed")
	}
	r.closed = true
	r.closedLock.Unlock()

	defer func() { log.Printf("Router closed") }()

	r.realmsLock.Lock()
	defer r.realmsLock.Unlock()

	rLen := len(r.realms)
	if 0 == rLen {
		return nil
	}

	wg := &sync.WaitGroup{}
	wg.Add(rLen)

	for _, realm := range r.realms {
		go func(realm *Realm) {
			// populated if realm closes on time
			realmClosed := make(chan struct{})

			// give realm 5 seconds to several all connections (each connection should close asynchronously...)
			ctx, cancel := context.WithTimeout(realm.ctx, 5*time.Second)

			// attempt to close realm politely...
			go func(r *Realm, c chan struct{}) {
				r.Close()
				c <- struct{}{}
			}(realm, realmClosed)

			// wait for something to happen....
			select {
			case <-ctx.Done():
				logErr(fmt.Errorf("Unable to close realm \"%s\": %s", realm.URI, ctx.Err()))
			case <-realmClosed:
			}

			// decrement wait group
			wg.Done()

			// cancel timer
			cancel()
		}(realm)
	}

	wg.Wait()

	r.realms = make(map[URI]*Realm)

	return nil
}

func (r *defaultRouter) RegisterRealm(realm *Realm) error {
	if "" == realm.URI {
		return errors.New("Unable to register realm: \"URI\" cannot be empty")
	}

	r.realmsLock.Lock()
	defer r.realmsLock.Unlock()

	if _, ok := r.realms[realm.URI]; ok {
		return RealmExistsError(realm.URI)
	}

	r.realms[realm.URI] = realm

	realm.init()

	log.Println("registered realm:", realm.URI)

	return nil
}

func (r *defaultRouter) Accept(peer Peer) error {
	if r.closed {
		logErr(peer.Send(&Abort{Reason: ErrSystemShutdown}))
		return fmt.Errorf("Router is closing, no new connections are allowed")
	}

	msg, err := GetMessageTimeout(peer, 5*time.Second)
	if err != nil {
		return err
	}
	log.Printf("%s: %+v", msg.MessageType(), msg)

	hello, ok := msg.(*HelloMessage)
	if !ok {
		logErr(peer.Send(&Abort{Reason: URI("wamp.error.protocol_violation")}))
		return fmt.Errorf("protocol violation: expected HELLO, received %s", msg.MessageType())
	}

	r.realmsLock.RLock()
	defer r.realmsLock.RUnlock()

	realm, ok := r.realms[hello.Realm]
	if !ok {
		logErr(peer.Send(&Abort{Reason: ErrNoSuchRealm}))
		return NoSuchRealmError(hello.Realm)
	}

	welcome, err := realm.handleAuth(peer, hello.Details)
	if err != nil {
		abort := &Abort{
			Reason:  ErrAuthorizationFailed, // TODO: should this be AuthenticationFailed?
			Details: map[string]interface{}{"error": err.Error()},
		}
		logErr(peer.Send(abort))
		return AuthenticationError(err.Error())
	}

	welcome.ID = NewID()

	if welcome.Details == nil {
		welcome.Details = make(map[string]interface{})
	}
	// add default details to welcome message
	for k, v := range defaultWelcomeDetails {
		if _, ok := welcome.Details[k]; !ok {
			welcome.Details[k] = v
		}
	}
	if err := peer.Send(welcome); err != nil {
		return err
	}
	log.Println("Established session:", welcome.ID)

	// session details
	welcome.Details["session"] = welcome.ID
	welcome.Details["realm"] = hello.Realm
	sess := &Session{
		Peer:    peer,
		ID:      welcome.ID,
		Details: welcome.Details,
	}
	r.sessionOpenCallbacksLock.RLock()
	for _, callback := range r.sessionOpenCallbacks {
		go callback(sess, string(hello.Realm))
	}
	r.sessionOpenCallbacksLock.RUnlock()

	go func() {
		realm.handleSession(sess)

		r.sessionCloseCallbacksLock.RLock()
		for _, callback := range r.sessionCloseCallbacks {
			go callback(sess, string(hello.Realm))
		}
		r.sessionCloseCallbacksLock.RUnlock()
	}()
	return nil
}

// GetLocalPeer returns an internal peer connected to the specified realm.
func (r *defaultRouter) GetLocalPeer(realmURI URI, details map[string]interface{}) (Peer, error) {
	r.realmsLock.RLock()
	defer r.realmsLock.RUnlock()

	realm, ok := r.realms[realmURI]
	if !ok {
		return nil, NoSuchRealmError(realmURI)
	}
	// TODO: session open/close callbacks?
	return realm.getPeer(details)
}

func (r *defaultRouter) getTestPeer() Peer {
	peerA, peerB := localPipe()
	go r.Accept(peerA)
	return peerB
}

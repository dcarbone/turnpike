package turnpike

import (
	"context"
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
	RegisterRealm(URI, *Realm) error
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

	sessionOpenCallbacks  []func(*Session, string)
	sessionCloseCallbacks []func(*Session, string)
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

	r.sessionOpenCallbacks = append(r.sessionOpenCallbacks, fn)
}

func (r *defaultRouter) AddSessionCloseCallback(fn func(*Session, string)) {
	r.closedLock.RLock()
	defer r.closedLock.RUnlock()
	if r.closed {
		log.Println("Router is closed, will not add session close callback")
		return
	}

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

	rLen := len(r.realms)
	if 0 == rLen {
		return nil
	}

	wg := &sync.WaitGroup{}
	wg.Add(rLen)

	for _, realm := range r.realms {
		go func(r *Realm) {
			// populated if realm closes on time
			realmClosed := make(chan struct{})

			// give realm 5 seconds to several all connections (each connection should close asynchronously...)
			ctx, cancel := context.WithTimeout(r.ctx, 5*time.Second)

			// attempt to close realm politely...
			go func(r *Realm, c chan struct{}) {
				r.Close()
				c <- struct{}{}
			}(r, realmClosed)

			// wait for something to happen....
			select {
			case <-ctx.Done():
				logErr(fmt.Errorf("Unable to close realm \"%s\": %s", r.URI, ctx.Err()))
			case <-realmClosed:
			}

			// decrement wait group
			wg.Done()

			// cancel timer
			cancel()
		}(realm)
	}

	wg.Wait()

	return nil
}

func (r *defaultRouter) RegisterRealm(uri URI, realm *Realm) error {
	if _, ok := r.realms[uri]; ok {
		return RealmExistsError(uri)
	}
	realm.init()
	r.realms[uri] = realm
	log.Println("registered realm:", uri)
	return nil
}

func (r *defaultRouter) Accept(peer Peer) error {
	if r.closed {
		logErr(peer.Send(&Abort{Reason: ErrSystemShutdown}))
		logErr(peer.Close())
		return fmt.Errorf("Router is closing, no new connections are allowed")
	}

	msg, err := GetMessageTimeout(peer, 5*time.Second)
	if err != nil {
		return err
	}
	log.Printf("%s: %+v", msg.MessageType(), msg)

	hello, ok := msg.(*Hello)
	if !ok {
		logErr(peer.Send(&Abort{Reason: URI("wamp.error.protocol_violation")}))
		logErr(peer.Close())
		return fmt.Errorf("protocol violation: expected HELLO, received %s", msg.MessageType())
	}

	realm, ok := r.realms[hello.Realm]
	if !ok {
		logErr(peer.Send(&Abort{Reason: ErrNoSuchRealm}))
		logErr(peer.Close())
		return NoSuchRealmError(hello.Realm)
	}

	welcome, err := realm.handleAuth(peer, hello.Details)
	if err != nil {
		abort := &Abort{
			Reason:  ErrAuthorizationFailed, // TODO: should this be AuthenticationFailed?
			Details: map[string]interface{}{"error": err.Error()},
		}
		logErr(peer.Send(abort))
		logErr(peer.Close())
		return AuthenticationError(err.Error())
	}

	welcome.Id = NewID()

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
	log.Println("Established session:", welcome.Id)

	// session details
	welcome.Details["session"] = welcome.Id
	welcome.Details["realm"] = hello.Realm
	sess := &Session{
		Peer:    peer,
		Id:      welcome.Id,
		Details: welcome.Details,
	}
	for _, callback := range r.sessionOpenCallbacks {
		go callback(sess, string(hello.Realm))
	}
	go func() {
		realm.handleSession(sess)
		for _, callback := range r.sessionCloseCallbacks {
			go callback(sess, string(hello.Realm))
		}
	}()
	return nil
}

// GetLocalPeer returns an internal peer connected to the specified realm.
func (r *defaultRouter) GetLocalPeer(realmURI URI, details map[string]interface{}) (Peer, error) {
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

package turnpike

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	defaultAuthTimeout = 2 * time.Minute
)

// A Realm is a WAMP routing and administrative domain.
//
// Clients that have connected to a WAMP router are joined to a realm and all
// message delivery is handled by the realm.
type Realm struct {
	Broker
	Dealer
	Authorizer
	Interceptor

	ctx context.Context

	URI              URI
	CRAuthenticators map[string]CRAuthenticator
	Authenticators   map[string]Authenticator
	AuthTimeout      time.Duration

	sessions     map[ID]*Session
	sessionsLock sync.RWMutex

	closed     bool
	closedLock sync.RWMutex

	localClient
}

type localClient struct {
	*Client
}

func (r *Realm) init() {
	r.sessions = make(map[ID]*Session)

	r.ctx = context.Background()

	p, _ := r.getPeer(nil)

	r.localClient.Client = NewClient(p)

	if r.Broker == nil {
		r.Broker = NewDefaultBroker()
	}
	if r.Dealer == nil {
		r.Dealer = NewDefaultDealer()
	}
	if r.Authorizer == nil {
		r.Authorizer = NewDefaultAuthorizer()
	}
	if r.Interceptor == nil {
		r.Interceptor = NewDefaultInterceptor()
	}
	if r.AuthTimeout == 0 {
		r.AuthTimeout = defaultAuthTimeout
	}

	go r.localClient.Receive()
}

func (r *Realm) Closed() bool {
	r.closedLock.RLock()
	defer r.closedLock.RUnlock()
	return r.closed
}

// Close disconnects all clients after sending a goodbye message
func (r *Realm) Close() {
	r.closedLock.Lock()

	if r.closed {
		log.Printf("Realm \"%s\" is already closing", string(r.URI))
		r.closedLock.Unlock()
		return
	}

	r.closedLock.Unlock()

	// log when done
	defer func() { log.Printf("Realm \"%s\" is now closed.", string(r.URI)) }()

	sLen := len(r.sessions)

	// if there are no active sessions, move on.
	if 0 == sLen {
		return
	}

	wg := &sync.WaitGroup{}
	wg.Add(sLen)

	// attempt to politely close connections.
	for _, session := range r.sessions {
		go func(s *Session) {
			if _, ok := s.Peer.(*localPeer); !ok {
				// will contain the message from session close
				closeChan := make(chan error)

				// we'll give them 2 seconds to ack the disconnect...
				ctx, cancel := context.WithTimeout(r.ctx, 2*time.Second)

				// attempt to close client connection gracefully...
				go func(s *Session, c chan error) { c <- s.Close() }(s, closeChan)

				// wait around for something to happen...
				select {
				case <-ctx.Done():
					logErr(fmt.Errorf("Unable to close session \"%d\": %s", s.Id, ctx.Err()))
				case err := <-closeChan:
					logErr(err)
				}

				// do you even cancel, bro?
				cancel()
			}

			// decrement wait group
			wg.Done()
		}(session)
	}

	wg.Wait()
}

func (l *localClient) onJoin(details map[string]interface{}) {
	l.Publish("wamp.session.on_join", nil, []interface{}{details}, nil)
}

func (l *localClient) onLeave(session ID) {
	l.Publish("wamp.session.on_leave", nil, []interface{}{session}, nil)
}

func (r *Realm) handleSession(sess *Session) {
	r.closedLock.RLock()
	if r.closed {
		log.Printf("Will not handle session \"%d\" as realm \"%s\" is already closed", sess.Id, string(r.URI))
		r.closedLock.RUnlock()
		return
	}

	r.closedLock.RUnlock()

	r.sessionsLock.Lock()
	r.sessions[sess.Id] = sess
	r.sessionsLock.Unlock()

	r.onJoin(sess.Details)

	for {
		msg, ok := <-sess.Receive()
		if !ok {
			log.Println("lost session:", sess)
			break
		}

		log.Printf("[%s] %s: %+v", sess, msg.MessageType(), msg)
		if isAuthz, err := r.Authorizer.Authorize(sess, msg); !isAuthz {
			errMsg := &Error{Type: msg.MessageType()}
			switch msg := msg.(type) {
			case *Publish:
				errMsg.Request = msg.Request
			case *Subscribe:
				errMsg.Request = msg.Request
			case *Unsubscribe:
				errMsg.Request = msg.Request
			case *Register:
				errMsg.Request = msg.Request
			case *Unregister:
				errMsg.Request = msg.Request
			case *Call:
				errMsg.Request = msg.Request
			case *Yield:
				errMsg.Request = msg.Request
			}
			if err != nil {
				errMsg.Error = ErrAuthorizationFailed
				log.Printf("[%s] authorization failed: %v", sess, err)
			} else {
				errMsg.Error = ErrNotAuthorized
				log.Printf("[%s] %s UNAUTHORIZED", sess, msg.MessageType())
			}
			logErr(sess.Send(errMsg))
			continue
		}

		r.Interceptor.Intercept(sess, &msg)

		switch msg := msg.(type) {
		case *Goodbye:
			logErr(sess.Send(&Goodbye{Reason: ErrGoodbyeAndOut, Details: make(map[string]interface{})}))
			log.Printf("[%s] leaving: %v", sess, msg.Reason)
			return

		// Broker messages
		case *Publish:
			r.Broker.Publish(sess, msg)
		case *Subscribe:
			r.Broker.Subscribe(sess, msg)
		case *Unsubscribe:
			r.Broker.Unsubscribe(sess, msg)

		// Dealer messages
		case *Register:
			r.Dealer.Register(sess, msg)
		case *Unregister:
			r.Dealer.Unregister(sess, msg)
		case *Call:
			r.Dealer.Call(sess, msg)
		case *Yield:
			r.Dealer.Yield(sess, msg)

		// Error messages
		case *Error:
			if msg.Type == MessageTypeInvocation {
				// the only type of ERROR message the router should receive
				r.Dealer.Error(sess, msg)
			} else {
				log.Printf("invalid ERROR message received: %v", msg)
			}

		default:
			log.Println("Unhandled message:", msg.MessageType())
		}
	}

	r.sessionsLock.Lock()
	delete(r.sessions, sess.Id)
	r.sessionsLock.Unlock()

	r.Dealer.RemoveSession(sess)
	r.Broker.RemoveSession(sess)
	r.onLeave(sess.Id)
}

func (r *Realm) handleAuth(client Peer, details map[string]interface{}) (*Welcome, error) {
	msg, err := r.authenticate(details)
	if err != nil {
		return nil, err
	}
	// we should never get anything besides WELCOME and CHALLENGE
	if msg.MessageType() == MessageTypeWelcome {
		return msg.(*Welcome), nil
	}
	// Challenge response
	challenge := msg.(*Challenge)
	if err := client.Send(challenge); err != nil {
		return nil, err
	}

	msg, err = GetMessageTimeout(client, r.AuthTimeout)
	if err != nil {
		return nil, err
	}
	log.Printf("%s: %+v", msg.MessageType(), msg)
	if authenticate, ok := msg.(*Authenticate); !ok {
		return nil, fmt.Errorf("unexpected %s message received", msg.MessageType())
	} else {
		return r.checkResponse(challenge, authenticate)
	}
}

// Authenticate either authenticates a client or returns a challenge message if
// challenge/response authentication is to be used.
func (r *Realm) authenticate(details map[string]interface{}) (Message, error) {
	log.Println("details:", details)
	if len(r.Authenticators) == 0 && len(r.CRAuthenticators) == 0 {
		return &Welcome{}, nil
	}
	// TODO: this might not always be a []interface{}. Using the JSON unmarshaller it will be,
	// but we may have serializations that preserve more of the original type.
	// For now, the tests just explicitly send a []interface{}
	_authmethods, ok := details["authmethods"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("No authentication supplied")
	}
	authmethods := []string{}
	for _, method := range _authmethods {
		if m, ok := method.(string); ok {
			authmethods = append(authmethods, m)
		} else {
			log.Printf("invalid authmethod value: %v", method)
		}
	}
	for _, method := range authmethods {
		if auth, ok := r.CRAuthenticators[method]; ok {
			if challenge, err := auth.Challenge(details); err != nil {
				return nil, err
			} else {
				return &Challenge{AuthMethod: method, Extra: challenge}, nil
			}
		}
		if auth, ok := r.Authenticators[method]; ok {
			if authDetails, err := auth.Authenticate(details); err != nil {
				return nil, err
			} else {
				return &Welcome{Details: addAuthMethod(authDetails, method)}, nil
			}
		}
	}
	// TODO: check default auth (special '*' auth?)
	return nil, fmt.Errorf("could not authenticate with any method")
}

// checkResponse determines whether the response to the challenge is sufficient to gain access to the Realm.
func (r *Realm) checkResponse(chal *Challenge, auth *Authenticate) (*Welcome, error) {
	authenticator, ok := r.CRAuthenticators[chal.AuthMethod]
	if !ok {
		return nil, fmt.Errorf("authentication method has been removed")
	}
	if details, err := authenticator.Authenticate(chal.Extra, auth.Signature); err != nil {
		return nil, err
	} else {
		return &Welcome{Details: addAuthMethod(details, chal.AuthMethod)}, nil
	}
}

func (r *Realm) getPeer(details map[string]interface{}) (Peer, error) {
	peerA, peerB := localPipe()
	if details == nil {
		details = make(map[string]interface{})
	}
	sess := Session{Peer: peerA, Id: NewID(), Details: details}
	go r.handleSession(&sess)
	log.Println("Established internal session:", sess)
	return peerB, nil
}

func addAuthMethod(details map[string]interface{}, method string) map[string]interface{} {
	if details == nil {
		details = make(map[string]interface{})
	}
	details["authmethod"] = method
	return details
}

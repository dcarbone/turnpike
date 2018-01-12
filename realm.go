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
	sessionsLock sync.Mutex

	closed     bool
	closedLock sync.Mutex

	localClient
}

type localClient struct {
	*Client
}

func (r *Realm) init() {
	var err error

	r.sessions = make(map[ID]*Session)

	r.ctx = context.Background()

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

	peerA, peerB := localPipe()
	sess := Session{Peer: peerA, ID: NewID()}
	r.localClient.Client, err = NewClient(peerB)
	if nil != err {
		panic(fmt.Sprintf("Unable to initialize Realm: %s", err))
	}

	go r.localClient.Receive()
	go r.handleSession(&sess)
}

func (r *Realm) Closed() bool {
	r.closedLock.Lock()
	defer r.closedLock.Lock()
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
	r.closed = true
	r.closedLock.Unlock()

	r.sessionsLock.Lock()

	// log when done
	defer func(uri URI) {
		r.sessionsLock.Lock()
		log.Printf("Realm \"%s\" is now closed.", uri)
	}(r.URI)

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
					logErr(fmt.Errorf("unable to close session \"%d\": %s", s.ID, ctx.Err()))
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
	r.closedLock.Lock()
	if r.closed {
		log.Printf("Will not handle session \"%d\" as realm \"%s\" is already closed", sess.ID, string(r.URI))
		r.closedLock.Lock()
		return
	}

	r.closedLock.Unlock()

	r.sessionsLock.Lock()
	r.sessions[sess.ID] = sess
	r.sessionsLock.Unlock()

	r.onJoin(sess.Details)

	var msg Message
	var err error
	var terr Error
	var ok bool
	var ctx context.Context
	var cancel context.CancelFunc
	var isAuthz bool
	var authErr error

sessionLoop:
	for {
		msg, err = sess.Receive()
		log.Printf("[session-%s] %s: %+v", sess, msg.MessageType(), msg)

		if err != nil {
			if terr, ok = err.(Error); ok && terr.Terminal() {
				log.Printf("Saw terminal error: %s", err)
				break sessionLoop
			}
		}

		// attempt authorization...
		isAuthz, authErr = r.Authorizer.Authorize(sess, msg)
		if isAuthz {
			// if authorized

			// call interceptor
			r.Interceptor.Intercept(sess, &msg)

			// TODO: do this better.
			ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)

			// handle message
			switch msg := msg.(type) {
			case *MessageGoodbye:
				log.Printf("[%s] leaving: %v", sess, msg.Reason)
				logErr(sess.Send(ctx, &MessageGoodbye{Reason: ErrGoodbyeAndOut, Details: make(map[string]interface{})}))
				cancel()
				return

				// Broker messages
			case *MessagePublish:
				r.Broker.Publish(ctx, sess, msg)
			case *MessageSubscribe:
				r.Broker.Subscribe(ctx, sess, msg)
			case *MessageUnsubscribe:
				r.Broker.Unsubscribe(ctx, sess, msg)

				// Dealer messages
			case *MessageRegister:
				r.Dealer.Register(ctx, sess, msg)
			case *MessageUnregister:
				r.Dealer.Unregister(ctx, sess, msg)
			case *MessageCall:
				r.Dealer.Call(ctx, sess, msg)
			case *MessageYield:
				r.Dealer.Yield(ctx, sess, msg)

				// MessageError messages
			case *MessageError:
				if msg.Type == MessageTypeInvocation {
					// the only type of ERROR message the router should receive
					r.Dealer.Error(ctx, sess, msg)
				} else {
					log.Printf("invalid ERROR message received: %v", msg)
				}

			default:
				log.Println("Unhandled message:", msg.MessageType())
			}

			cancel()
		} else {
			// if unauthorized...

			// create error message...
			errMsg := &MessageError{Type: msg.MessageType()}
			switch msg := msg.(type) {
			case *MessagePublish:
				errMsg.Request = msg.Request
			case *MessageSubscribe:
				errMsg.Request = msg.Request
			case *MessageUnsubscribe:
				errMsg.Request = msg.Request
			case *MessageRegister:
				errMsg.Request = msg.Request
			case *MessageUnregister:
				errMsg.Request = msg.Request
			case *MessageCall:
				errMsg.Request = msg.Request
			case *MessageYield:
				errMsg.Request = msg.Request
			}

			if authErr != nil {
				errMsg.Error = ErrAuthorizationFailed
				log.Printf("[%s] authorization failed: %v", sess, authErr)
			} else {
				errMsg.Error = ErrNotAuthorized
				log.Printf("[%s] %s UNAUTHORIZED", sess, msg.MessageType())
			}

			// send error
			ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
			logErr(sess.Send(ctx, errMsg))
			cancel()
		}
	}

	r.sessionsLock.Lock()
	delete(r.sessions, sess.ID)
	r.sessionsLock.Unlock()

	r.Dealer.RemoveSession(sess)
	r.Broker.RemoveSession(sess)
	r.onLeave(sess.ID)
}

func (r *Realm) handleAuth(client Peer, details map[string]interface{}) (*MessageWelcome, error) {
	msg, err := r.authenticate(details)
	if err != nil {
		return nil, err
	}
	// we should never get anything besides WELCOME and CHALLENGE
	if msg.MessageType() == MessageTypeWelcome {
		return msg.(*MessageWelcome), nil
	}
	// MessageChallenge response
	ctx, cancel := context.WithTimeout(context.Background(), r.AuthTimeout)
	challenge := msg.(*MessageChallenge)
	err = client.Send(ctx, challenge)
	cancel()
	if err != nil {
		return nil, err
	}

	ctx, cancel = context.WithTimeout(context.Background(), r.AuthTimeout)
	msg, err = client.ReceiveUntil(ctx)
	cancel()
	if err != nil {
		return nil, err
	}
	log.Printf("%s: %+v", msg.MessageType(), msg)
	if authenticate, ok := msg.(*MessageAuthenticate); !ok {
		return nil, fmt.Errorf("unexpected %s message received", msg.MessageType())
	} else {
		return r.checkResponse(challenge, authenticate)
	}
}

// MessageAuthenticate either authenticates a client or returns a challenge message if
// challenge/response authentication is to be used.
func (r *Realm) authenticate(details map[string]interface{}) (Message, error) {
	log.Println("[realm] authenticate() details:", details)
	if len(r.Authenticators) == 0 && len(r.CRAuthenticators) == 0 {
		return &MessageWelcome{}, nil
	}
	// TODO: this might not always be a []interface{}. Using the JSON unmarshaller it will be,
	// but we may have serializations that preserve more of the original type.
	// For now, the tests just explicitly send a []interface{}
	incomingAuthMethods, ok := details["authmethods"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("no authentication supplied")
	}
	authMethods := []string{}
	for _, method := range incomingAuthMethods {
		if m, ok := method.(string); ok {
			authMethods = append(authMethods, m)
		} else {
			log.Printf("invalid authmethod value: %v", method)
		}
	}
	for _, method := range authMethods {
		if auth, ok := r.CRAuthenticators[method]; ok {
			if challenge, err := auth.Challenge(details); err != nil {
				return nil, err
			} else {
				return &MessageChallenge{AuthMethod: method, Extra: challenge}, nil
			}
		}
		if auth, ok := r.Authenticators[method]; ok {
			if authDetails, err := auth.Authenticate(details); err != nil {
				return nil, err
			} else {
				return &MessageWelcome{Details: addAuthMethod(authDetails, method)}, nil
			}
		}
	}
	// TODO: check default auth (special '*' auth?)
	return nil, fmt.Errorf("could not authenticate with any method")
}

// checkResponse determines whether the response to the challenge is sufficient to gain access to the Realm.
func (r *Realm) checkResponse(chal *MessageChallenge, auth *MessageAuthenticate) (*MessageWelcome, error) {
	authenticator, ok := r.CRAuthenticators[chal.AuthMethod]
	if !ok {
		return nil, fmt.Errorf("authentication method has been removed")
	}
	if details, err := authenticator.Authenticate(chal.Extra, auth.Signature); err != nil {
		return nil, err
	} else {
		return &MessageWelcome{Details: addAuthMethod(details, chal.AuthMethod)}, nil
	}
}

func (r *Realm) getPeer(details map[string]interface{}) (Peer, error) {
	peerA, peerB := localPipe()

	if details == nil {
		details = make(map[string]interface{})
	}

	sess := Session{Peer: peerA, ID: NewID(), Details: details}

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

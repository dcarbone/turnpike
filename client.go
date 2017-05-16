package turnpike

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"sync"
	"time"
)

// TODO: completely re-do event system in here....

var (
	msgAbortUnexpectedMessageType = &Abort{
		Details: map[string]interface{}{},
		Reason:  "turnpike.error.unexpected_message_type",
	}
	msgAbortNoAuthHandler = &Abort{
		Details: map[string]interface{}{},
		Reason:  "turnpike.error.no_handler_for_authmethod",
	}
	msgAbortAuthFailure = &Abort{
		Details: map[string]interface{}{},
		Reason:  "turnpike.error.authentication_failure",
	}
	msgGoodbyeClient = &Goodbye{
		Details: map[string]interface{}{},
		Reason:  ErrCloseRealm,
	}
)

type (
	// AuthFunc takes the HELLO details and CHALLENGE details and returns the
	// signature string and a details map
	AuthFunc func(map[string]interface{}, map[string]interface{}) (string, map[string]interface{}, error)

	// EventHandler handles a publish event.
	EventHandler func(args []interface{}, kwargs map[string]interface{})

	// MethodHandler is an RPC endpoint.
	MethodHandler func(args []interface{}, kwargs map[string]interface{}, details map[string]interface{}) (result *CallResult)

	// BasicMethodHandler is an RPC endpoint that doesn't expect the `Details` map
	BasicMethodHandler func(args []interface{}, kwargs map[string]interface{}) (result *CallResult)

	// A Client routes messages to/from a WAMP router.
	Client struct {
		Peer

		// ReceiveTimeout is the amount of time that the client will block waiting for a response from the router.
		ReceiveTimeout time.Duration
		// Auth is a map of WAMP authmethods to functions that will handle each auth type
		Auth map[string]AuthFunc

		// ReceiveDone is notified when the client's connection to the router is lost.
		ReceiveDone chan bool

		ctx context.Context

		listeners     map[ID]chan Message
		listenersLock sync.RWMutex

		events     map[ID]*eventDescription
		eventsLock sync.RWMutex

		procedures     map[ID]*procedureDescription
		proceduresLock sync.RWMutex
	}

	procedureDescription struct {
		name    string
		handler MethodHandler
	}

	eventDescription struct {
		topic   string
		handler EventHandler
	}
)

// NewWebSocketClient creates a new websocket client connected to the specified
// `url` and using the specified `serialization`.
func NewWebSocketClient(serialization SerializationFormat, url string, tlscfg *tls.Config, dial DialFunc) (*Client, error) {
	p, err := NewWebSocketPeer(serialization, url, tlscfg, dial)
	if nil != err {
		return nil, err
	}

	return NewClient(p), nil
}

// NewClient takes a connected Peer and returns a new Client
func NewClient(p Peer) *Client {
	c := &Client{
		Peer:           p,
		ReceiveTimeout: 10 * time.Second,

		ctx: context.Background(),

		listeners:  make(map[ID]chan Message),
		events:     make(map[ID]*eventDescription),
		procedures: make(map[ID]*procedureDescription),
	}

	go c.run()

	return c
}

func (c *Client) run() {

}

// LeaveRealm leaves the current realm without closing the connection to the server.
func (c *Client) LeaveRealm() error {
	if err := c.Send(msgGoodbyeClient); err != nil {
		return fmt.Errorf("error leaving realm: %v", err)
	}
	return nil
}

// Close closes the connection to the server.
func (c *Client) Close() error {
	if c.Peer.Closed() {
		return errors.New("Client already closed")
	}

	// always attempt to close peer
	defer func(p Peer) {
		if err := p.Close(); err != nil {
			log.Printf("error closing client connection: %v", err)
		}
	}(c.Peer)

	// attempt to leave realm
	if err := c.LeaveRealm(); err != nil {
		return err
	}

	return nil
}

// JoinRealm joins a WAMP realm, but does not handle challenge/response authentication.
func (c *Client) JoinRealm(realm string, details map[string]interface{}) (map[string]interface{}, error) {
	if c.Peer.Closed() {
		return nil, errors.New("Client is closed")
	}

	if details == nil {
		details = map[string]interface{}{}
	}

	details["roles"] = clientRoles()

	if c.Auth != nil && len(c.Auth) > 0 {
		return c.joinRealmCRA(realm, details)
	}

	if err := c.Send(&Hello{Realm: URI(realm), Details: details}); err != nil {
		logErr(c.Close())
		return nil, err
	}

	msg, err := GetMessageTimeout(c.Peer, c.ReceiveTimeout)
	if err != nil {
		logErr(c.Close())
		return nil, err
	}

	welcome, ok := msg.(*Welcome)
	if !ok {
		c.Send(msgAbortUnexpectedMessageType)
		logErr(c.Close())
		return nil, fmt.Errorf(formatUnexpectedMessage(msg, MessageTypeWelcome))
	}

	go c.Receive()

	return welcome.Details, nil
}

// joinRealmCRA joins a WAMP realm and handles challenge/response authentication.
func (c *Client) joinRealmCRA(realm string, details map[string]interface{}) (map[string]interface{}, error) {
	authmethods := []interface{}{}
	for m := range c.Auth {
		authmethods = append(authmethods, m)
	}
	details["authmethods"] = authmethods

	err := c.Send(&Hello{Realm: URI(realm), Details: details})
	if err != nil {
		logErr(c.Close())
		return nil, err
	}

	msg, err := GetMessageTimeout(c.Peer, c.ReceiveTimeout)
	if err != nil {
		logErr(c.Close())
		return nil, err
	}

	challenge, ok := msg.(*Challenge)
	if !ok {
		c.Send(msgAbortUnexpectedMessageType)
		logErr(c.Close())
		return nil, fmt.Errorf(formatUnexpectedMessage(msg, MessageTypeChallenge))
	}

	authFunc, ok := c.Auth[challenge.AuthMethod]
	if !ok {
		c.Send(msgAbortNoAuthHandler)
		logErr(c.Close())
		return nil, fmt.Errorf("no auth handler for method: %s", challenge.AuthMethod)
	}

	signature, authDetails, err := authFunc(details, challenge.Extra)
	if err != nil {
		c.Send(msgAbortAuthFailure)
		logErr(c.Close())
		return nil, err
	}

	err = c.Send(&Authenticate{Signature: signature, Extra: authDetails})
	if err != nil {
		logErr(c.Close())
		return nil, err
	}

	msg, err = GetMessageTimeout(c.Peer, c.ReceiveTimeout)
	if err != nil {
		logErr(c.Close())
		return nil, err
	}

	welcome, ok := msg.(*Welcome)
	if !ok {
		c.Send(msgAbortUnexpectedMessageType)
		logErr(c.Close())
		return nil, fmt.Errorf(formatUnexpectedMessage(msg, MessageTypeWelcome))
	}

	go c.Receive()

	return welcome.Details, nil
}

// Receive handles messages from the server until this client disconnects.
//
// This function blocks and is most commonly run in a goroutine.
func (c *Client) Receive() {
	for msg := range c.Peer.Receive() {

		switch msg := msg.(type) {

		case *Event:
			c.handleEvent(msg)

		case *Invocation:
			c.handleInvocation(msg)

		case *Registered:
			c.notifyListener(msg, msg.Request)
		case *Subscribed:
			c.notifyListener(msg, msg.Request)
		case *Unsubscribed:
			c.notifyListener(msg, msg.Request)
		case *Unregistered:
			c.notifyListener(msg, msg.Request)

		case *Result:
			c.notifyListener(msg, msg.Request)
		case *Error:
			c.notifyListener(msg, msg.Request)

		case *Goodbye:
			log.Println("client received Goodbye message")
			break

		default:
			log.Println("unhandled message:", msg.MessageType(), msg)
		}
	}

	log.Println("client closed")

	if c.ReceiveDone != nil {
		c.ReceiveDone <- true
	}
}

// Subscribe registers the EventHandler to be called for every message in the provided topic.
func (c *Client) Subscribe(topic string, options map[string]interface{}, fn EventHandler) error {
	if options == nil {
		options = make(map[string]interface{})
	}

	id := NewID()
	c.registerListener(id)

	sub := &Subscribe{
		Request: id,
		Options: options,
		Topic:   URI(topic),
	}

	err := c.Send(sub)

	if err != nil {
		return err
	}

	// wait to receive SUBSCRIBED message
	msg, err := c.waitOnListener(id)
	c.deleteListener(id)

	if err != nil {
		return err
	} else if e, ok := msg.(*Error); ok {
		return fmt.Errorf("error subscribing to topic '%v': %v", topic, e.Error)
	} else if subscribed, ok := msg.(*Subscribed); !ok {
		return fmt.Errorf(formatUnexpectedMessage(msg, MessageTypeSubscribed))
	} else {
		// register the event handler with this subscription
		c.registerEventHandler(subscribed.Subscription, &eventDescription{topic, fn})
	}
	return nil
}

// Unsubscribe removes the registered EventHandler from the topic.
func (c *Client) Unsubscribe(topic string) error {
	c.eventsLock.RLock()
	defer c.eventsLock.RUnlock()

	var subscriptionID ID

	for id, desc := range c.events {
		if desc.topic == topic {
			subscriptionID = id
			break
		}
	}

	if 0 == subscriptionID {
		return fmt.Errorf("Event %s is not registered with this client.", topic)
	}

	id := NewID()
	c.registerListener(id)

	sub := &Unsubscribe{
		Request:      id,
		Subscription: subscriptionID,
	}

	err := c.Send(sub)
	if err != nil {
		return err
	}

	// wait to receive UNSUBSCRIBED message
	msg, err := c.waitOnListener(id)
	c.deleteListener(id)

	if err != nil {
		return err
	} else if e, ok := msg.(*Error); ok {
		return fmt.Errorf("error unsubscribing to topic '%v': %v", topic, e.Error)
	} else if _, ok := msg.(*Unsubscribed); !ok {
		return fmt.Errorf(formatUnexpectedMessage(msg, MessageTypeUnsubscribed))
	}

	c.deleteEventHandler(subscriptionID)

	return nil
}

// Publish publishes an EVENT to all subscribed peers.
func (c *Client) Publish(topic string, options map[string]interface{}, args []interface{}, kwargs map[string]interface{}) error {
	if options == nil {
		options = make(map[string]interface{})
	}
	return c.Send(&Publish{
		Request:     NewID(),
		Options:     options,
		Topic:       URI(topic),
		Arguments:   args,
		ArgumentsKw: kwargs,
	})
}

// Register registers a MethodHandler procedure with the router.
func (c *Client) Register(procedure string, fn MethodHandler, options map[string]interface{}) error {
	id := NewID()
	c.registerListener(id)

	register := &Register{
		Request:   id,
		Options:   options,
		Procedure: URI(procedure),
	}

	err := c.Send(register)
	if err != nil {
		return err
	}

	// wait to receive REGISTERED message
	msg, err := c.waitOnListener(id)
	c.deleteListener(id)

	if err != nil {
		return err
	} else if e, ok := msg.(*Error); ok {
		return fmt.Errorf("error registering procedure '%v': %v", procedure, e.Error)
	} else if registered, ok := msg.(*Registered); !ok {
		return fmt.Errorf(formatUnexpectedMessage(msg, MessageTypeRegistered))
	} else {
		// register the event handler with this registration
		c.registerProcedure(registered.Registration, &procedureDescription{procedure, fn})
	}
	return nil
}

// BasicRegister registers a BasicMethodHandler procedure with the router
func (c *Client) BasicRegister(procedure string, fn BasicMethodHandler) error {
	wrap := func(args []interface{}, kwargs map[string]interface{},
		details map[string]interface{}) (result *CallResult) {
		return fn(args, kwargs)
	}
	return c.Register(procedure, wrap, make(map[string]interface{}))
}

// Unregister removes a procedure with the router
func (c *Client) Unregister(procedure string) error {
	c.proceduresLock.RLock()

	var procedureID ID
	var found bool

	for id, p := range c.procedures {
		if p.name == procedure {
			procedureID = id
			found = true
			break
		}
	}

	c.proceduresLock.RUnlock()

	if !found {
		return fmt.Errorf("Procedure %s is not registered with this client.", procedure)
	}

	id := NewID()
	c.registerListener(id)
	unregister := &Unregister{
		Request:      id,
		Registration: procedureID,
	}

	if err := c.Send(unregister); err != nil {
		return err
	}

	// wait to receive UNREGISTERED message
	msg, err := c.waitOnListener(id)
	c.deleteListener(id)

	if err != nil {
		return err
	} else if e, ok := msg.(*Error); ok {
		return fmt.Errorf("error unregister to procedure '%v': %v", procedure, e.Error)
	} else if _, ok := msg.(*Unregistered); !ok {
		return fmt.Errorf(formatUnexpectedMessage(msg, MessageTypeUnregistered))
	}

	c.deleteProcedure(procedureID)

	return nil
}

// Call calls a procedure given a URI.
func (c *Client) Call(procedure string, options map[string]interface{}, args []interface{}, kwargs map[string]interface{}) (*Result, error) {
	id := NewID()
	c.registerListener(id)

	call := &Call{
		Request:     id,
		Procedure:   URI(procedure),
		Options:     options,
		Arguments:   args,
		ArgumentsKw: kwargs,
	}
	err := c.Send(call)
	if err != nil {
		return nil, err
	}

	// wait to receive RESULT message
	var msg Message
	msg, err = c.waitOnListener(id)
	c.deleteListener(id)

	if err != nil {
		return nil, err
	} else if e, ok := msg.(*Error); ok {
		return nil, RPCError{e, procedure}
	} else if result, ok := msg.(*Result); !ok {
		return nil, fmt.Errorf(formatUnexpectedMessage(msg, MessageTypeResult))
	} else {
		return result, nil
	}
}

func (c *Client) registerListener(id ID) {
	c.listenersLock.Lock()
	defer c.listenersLock.Unlock()

	log.Println("register listener:", id)

	wait := make(chan Message, 1)
	c.listeners[id] = wait
}

func (c *Client) deleteListener(id ID) {
	c.listenersLock.Lock()
	defer c.listenersLock.Unlock()
	delete(c.listeners, id)
}

func (c *Client) waitOnListener(id ID) (msg Message, err error) {
	c.listenersLock.RLock()
	defer c.listenersLock.RUnlock()

	log.Println("wait on listener:", id)

	var wait chan Message

	wait, ok := c.listeners[id]
	if !ok {
		return nil, fmt.Errorf("unknown listener ID: %v", id)
	}

	select {
	case msg = <-wait:
	case <-time.After(c.ReceiveTimeout):
		err = fmt.Errorf("timeout while waiting for message")
	}

	return
}

func (c *Client) registerEventHandler(id ID, desc *eventDescription) {
	c.eventsLock.Lock()
	defer c.eventsLock.Unlock()
	c.events[id] = desc
}

func (c *Client) deleteEventHandler(id ID) {
	c.eventsLock.Lock()
	defer c.eventsLock.Unlock()
	delete(c.events, id)
}

func (c *Client) handleEvent(msg *Event) {
	c.eventsLock.RLock()
	defer c.eventsLock.RUnlock()

	if event, ok := c.events[msg.Subscription]; ok {
		go event.handler(msg.Arguments, msg.ArgumentsKw)
	} else {
		log.Println("no handler registered for subscription:", msg.Subscription)
	}
}

func (c *Client) notifyListener(msg Message, requestID ID) {
	c.listenersLock.RLock()
	defer c.listenersLock.RUnlock()

	l, ok := c.listeners[requestID]
	if ok {
		l <- msg
	} else {
		log.Println("no listener for message", msg.MessageType(), requestID)
	}
}

func (c *Client) registerProcedure(id ID, desc *procedureDescription) {
	c.proceduresLock.Lock()
	defer c.proceduresLock.Unlock()
	c.procedures[id] = desc
}

func (c *Client) deleteProcedure(id ID) {
	c.proceduresLock.Lock()
	defer c.proceduresLock.Unlock()
	delete(c.procedures, id)
}

func (c *Client) handleInvocation(msg *Invocation) {
	c.proceduresLock.RLock()
	defer c.proceduresLock.RUnlock()

	if proc, ok := c.procedures[msg.Registration]; ok {
		go func() {
			result := proc.handler(msg.Arguments, msg.ArgumentsKw, msg.Details)

			var toSend Message
			toSend = &Yield{
				Request:     msg.Request,
				Options:     make(map[string]interface{}),
				Arguments:   result.Args,
				ArgumentsKw: result.Kwargs,
			}

			if result.Err != "" {
				toSend = &Error{
					Type:        MessageTypeInvocation,
					Request:     msg.Request,
					Details:     make(map[string]interface{}),
					Arguments:   result.Args,
					ArgumentsKw: result.Kwargs,
					Error:       result.Err,
				}
			}

			if err := c.Send(toSend); err != nil {
				log.Println("error sending message:", err)
			}
		}()
	} else {
		log.Println("no handler registered for registration:", msg.Registration)
		if err := c.Send(&Error{
			Type:    MessageTypeInvocation,
			Request: msg.Request,
			Details: make(map[string]interface{}),
			Error:   URI(fmt.Sprintf("no handler for registration: %v", msg.Registration)),
		}); err != nil {
			log.Println("error sending message:", err)
		}
	}
}

type RPCError struct {
	ErrorMessage *Error
	Procedure    string
}

func (rpc RPCError) Error() string {
	return fmt.Sprintf("error calling procedure '%v': %v: %v: %v", rpc.Procedure, rpc.ErrorMessage.Error, rpc.ErrorMessage.Arguments, rpc.ErrorMessage.ArgumentsKw)
}

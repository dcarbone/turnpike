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
	AuthFunc func(helloDetails, challengeDetails map[string]interface{}) (string, map[string]interface{}, error)

	// PublishEventHandler handles a publish event.
	PublishEventHandler func(*Event)

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

		subscriptions     map[ID]*subscription
		subscriptionsLock sync.RWMutex

		listeners     map[ID]chan Message
		listenersLock sync.RWMutex

		procedures     map[ID]*procedureDescription
		proceduresLock sync.RWMutex
	}

	procedureDescription struct {
		name    string
		handler MethodHandler
	}

	subscription struct {
		topic   string
		handler PublishEventHandler
	}
)

// NewWebSocketClient creates a new websocket client connected to the specified
// `url` and using the specified `serialization`.
func NewWebSocketClient(serialization SerializationFormat, url string, tlscfg *tls.Config, dial DialFunc) (*Client, error) {
	p, err := NewWebSocketPeer(serialization, url, tlscfg, dial)
	if nil != err {
		return nil, err
	}

	return NewClient(p)
}

// NewClient takes a connected Peer and returns a new Client
func NewClient(p Peer) (*Client, error) {
	if nil == p {
		return nil, errors.New("unable to construct client: Peer cannot be nil")
	}

	c := &Client{
		Peer:           p,
		ReceiveTimeout: 10 * time.Second,

		incoming: make(chan Message),

		subscriptions: make(map[ID]*subscription),

		listeners:  make(map[ID]chan Message),
		procedures: make(map[ID]*procedureDescription),
	}

	return c, nil
}

// LeaveRealm leaves the current realm without closing the connection to the server.
func (c *Client) LeaveRealm(ctx context.Context) error {

	if err := c.Send(ctx, msgGoodbyeClient); err != nil {
		return fmt.Errorf("error leaving realm: %v", err)
	}
	return nil
}

// Close closes the connection to the server.
func (c *Client) Close(ctx context.Context) error {
	if c.Peer.Closed() {
		return errors.New("client already closed")
	}

	// attempt to leave realm
	if err := c.LeaveRealm(ctx); err != nil {
		return err
	}

	return nil
}

// JoinRealm joins a WAMP realm, but does not handle challenge/response authentication.
func (c *Client) JoinRealm(ctx context.Context, realm string, details map[string]interface{}) (map[string]interface{}, error) {
	if c.Peer.Closed() {
		return nil, errors.New("client is closed")
	}

	if details == nil {
		details = map[string]interface{}{}
	}

	details["roles"] = clientRoles()

	if c.Auth != nil && len(c.Auth) > 0 {
		return c.joinRealmCRA(ctx, realm, details)
	}

	if err := c.Send(ctx, &HelloMessage{Realm: URI(realm), Details: details}); err != nil {
		logErr(c.Close(ctx))
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
func (c *Client) joinRealmCRA(ctx context.Context, realm string, details map[string]interface{}) (map[string]interface{}, error) {
	authmethods := []interface{}{}
	for m := range c.Auth {
		authmethods = append(authmethods, m)
	}
	details["authmethods"] = authmethods

	err := c.Send(&HelloMessage{Realm: URI(realm), Details: details})
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
	if c.Peer.Closed() {
		log.Printf("Client cannot receive, it's peer is closed")
		return
	}

	var msg Message
	var err error

MessageLoop:
	for {
		msg, err = c.Peer.Receive()
	}
	for msg := range c.Peer.Receive() {

		switch msg := msg.(type) {

		case *Event:
			c.subscriptionsLock.RLock()
			if s, ok := c.subscriptions[msg.Subscription]; ok {
				go s.handler(msg)
			} else {
				log.Printf("un-handled event: %v", msg)
			}
			c.subscriptionsLock.RUnlock()

		case *Invocation:
			c.handleInvocation(msg)

		case *Subscribed:
			c.notifyListener(msg, msg.Request)
		case *Unsubscribed:
			c.notifyListener(msg, msg.Request)

		case *Registered:
			c.notifyListener(msg, msg.Request)
		case *Unregistered:
			c.notifyListener(msg, msg.Request)

		case *Result:
			c.notifyListener(msg, msg.Request)
		case *Error:
			c.notifyListener(msg, msg.Request)

		case *Goodbye:
			log.Println("client received Goodbye message")
			break MessageLoop

		default:
			log.Println("unhandled message:", msg.MessageType(), msg)
		}
	}

	log.Println("client closed")

	if c.ReceiveDone != nil {
		c.ReceiveDone <- true
	}
}

// Subscribe registers the PublishEventHandler to be called for every message in the provided topic.
func (c *Client) Subscribe(topic string, options map[string]interface{}, fn PublishEventHandler) error {
	if c.Peer.Closed() {
		return errors.New("client is closed")
	}

	if nil == fn {
		return fmt.Errorf("unable to subscribe to topic \"%s\": no PublishEventHandler defined", topic)
	}

	if options == nil {
		options = make(map[string]interface{})
	}

	id := NewID()

	c.registerListener(id)
	defer c.deleteListener(id)

	sub := &Subscribe{
		Request: id,
		Options: options,
		Topic:   URI(topic),
	}

	if err := c.Send(sub); err != nil {
		return err
	}

	// wait to receive SUBSCRIBED message
	msg, err := c.waitOnListener(id)
	if err != nil {
		return err
	}

	// test for error
	e, ok := msg.(*Error)
	if ok {
		return fmt.Errorf("error subscribing to topic '%v': %v", topic, e.Error)
	}

	// ensure we got a subscribed message
	subscribed, ok := msg.(*Subscribed)
	if !ok {
		return fmt.Errorf(formatUnexpectedMessage(msg, MessageTypeSubscribed))
	}

	// add subscription to map
	c.subscriptionsLock.Lock()
	defer c.subscriptionsLock.Unlock()

	c.subscriptions[subscribed.Subscription] = &subscription{topic, fn}

	return nil
}

// Unsubscribe removes the registered PublishEventHandler from the topic.
func (c *Client) Unsubscribe(topic string) error {
	if c.Peer.Closed() {
		return errors.New("client is closed")
	}

	var subscriptionID ID

	// attempt to locate subscription ID...
	c.subscriptionsLock.RLock()
	for id, sub := range c.subscriptions {
		if sub.topic == topic {
			subscriptionID = id
			break
		}
	}
	c.subscriptionsLock.RUnlock()

	if 0 == subscriptionID {
		return fmt.Errorf("event %s is not registered with this client.", topic)
	}

	id := NewID()

	c.registerListener(id)

	// queue up on-return stuff...
	defer func(c *Client) {
		// remove listener...
		c.deleteListener(id)

		// remove subscription from map
		c.subscriptionsLock.Lock()
		delete(c.subscriptions, subscriptionID)
		c.subscriptionsLock.Unlock()
	}(c)

	sub := &Unsubscribe{
		Request:      id,
		Subscription: subscriptionID,
	}

	if err := c.Send(sub); err != nil {
		return err
	}

	// wait to receive UNSUBSCRIBED message
	msg, err := c.waitOnListener(id)
	if err != nil {
		return err
	}

	// test for error
	e, ok := msg.(*Error)
	if ok {
		return fmt.Errorf("error unsubscribing to topic '%v': %v", topic, e.Error)
	}

	// test for unknown message
	_, ok = msg.(*Unsubscribed)
	if !ok {
		return fmt.Errorf(formatUnexpectedMessage(msg, MessageTypeUnsubscribed))
	}

	return nil
}

// Publish publishes an EVENT to all subscribed peers.
func (c *Client) Publish(topic string, options map[string]interface{}, args []interface{}, kwargs map[string]interface{}) error {
	if c.Peer.Closed() {
		return errors.New("client is closed")
	}

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
	if c.Peer.Closed() {
		return errors.New("client is closed")
	}

	id := NewID()

	c.registerListener(id)
	defer c.deleteListener(id)

	register := &Register{
		Request:   id,
		Options:   options,
		Procedure: URI(procedure),
	}

	if err := c.Send(register); err != nil {
		return err
	}

	// wait to receive REGISTERED message
	msg, err := c.waitOnListener(id)
	if nil != err {
		return err
	}

	e, ok := msg.(*Error)
	if ok {
		return fmt.Errorf("error registering procedure '%v': %v", procedure, e.Error)
	}

	registered, ok := msg.(*Registered)
	if !ok {
		return fmt.Errorf(formatUnexpectedMessage(msg, MessageTypeRegistered))
	}

	// register the event handler with this registration
	c.registerProcedure(registered.Registration, &procedureDescription{procedure, fn})

	return nil
}

// BasicRegister registers a BasicMethodHandler procedure with the router
func (c *Client) BasicRegister(procedure string, fn BasicMethodHandler) error {
	if c.Peer.Closed() {
		return errors.New("client is closed")
	}

	wrap := func(args []interface{}, kwargs map[string]interface{},
		details map[string]interface{}) (result *CallResult) {
		return fn(args, kwargs)
	}
	return c.Register(procedure, wrap, make(map[string]interface{}))
}

// Unregister removes a procedure with the router
func (c *Client) Unregister(procedure string) error {
	if c.Peer.Closed() {
		return errors.New("client is closed")
	}

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
		return fmt.Errorf("procedure %s is not registered with this client.", procedure)
	}

	// queue up procedure for deletion
	defer c.deleteProcedure(procedureID)

	id := NewID()

	// register new listener
	c.registerListener(id)
	defer c.deleteListener(id)

	unregister := &Unregister{
		Request:      id,
		Registration: procedureID,
	}

	if err := c.Send(unregister); err != nil {
		return err
	}

	// wait to receive UNREGISTERED message
	msg, err := c.waitOnListener(id)

	if err != nil {
		return err
	} else if e, ok := msg.(*Error); ok {
		return fmt.Errorf("error unregister to procedure '%v': %v", procedure, e.Error)
	} else if _, ok := msg.(*Unregistered); !ok {
		return fmt.Errorf(formatUnexpectedMessage(msg, MessageTypeUnregistered))
	}

	return nil
}

// Call calls a procedure given a URI.
func (c *Client) Call(procedure string, options map[string]interface{}, args []interface{}, kwargs map[string]interface{}) (*Result, error) {
	if c.Peer.Closed() {
		return nil, errors.New("client is closed")
	}

	id := NewID()

	c.registerListener(id)
	defer c.deleteListener(id)

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

func (c *Client) waitOnListener(id ID) (Message, error) {
	c.listenersLock.RLock()
	defer c.listenersLock.RUnlock()

	log.Println("wait on listener:", id)

	wait, ok := c.listeners[id]
	if !ok {
		return nil, fmt.Errorf("unknown listener ID: %v", id)
	}

	ctx, cancel := context.WithTimeout(c.ctx, c.ReceiveTimeout)
	defer cancel()

	select {
	case msg := <-wait:
		return msg, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("timeout while waiting for message")
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

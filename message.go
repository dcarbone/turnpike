package turnpike

import (
	"context"
	"fmt"
	"reflect"
)

// Message is a generic container for a WAMP message.
type Message interface {
	MessageType() MessageType
	Context() context.Context
}

type MessageType int

func (mt MessageType) New(ctx context.Context) Message {
	switch mt {
	case HELLO:
		return &Hello{ctx: ctx}
	case WELCOME:
		return &Welcome{ctx: ctx}
	case ABORT:
		return &Abort{ctx: ctx}
	case CHALLENGE:
		return &Challenge{ctx: ctx}
	case AUTHENTICATE:
		return &Authenticate{ctx: ctx}
	case GOODBYE:
		return &Goodbye{ctx: ctx}
	case ERROR:
		return &Error{ctx: ctx}

	case PUBLISH:
		return &Publish{ctx: ctx}
	case PUBLISHED:
		return &Published{ctx: ctx}

	case SUBSCRIBE:
		return &Subscribe{ctx: ctx}
	case SUBSCRIBED:
		return &Subscribed{ctx: ctx}
	case UNSUBSCRIBE:
		return &Unsubscribe{ctx: ctx}
	case UNSUBSCRIBED:
		return &Unsubscribed{ctx: ctx}
	case EVENT:
		return &Event{ctx: ctx}

	case CALL:
		return &Call{ctx: ctx}
	case CANCEL:
		return &Cancel{ctx: ctx}
	case RESULT:
		return &Result{ctx: ctx}

	case REGISTER:
		return &Register{ctx: ctx}
	case REGISTERED:
		return &Registered{ctx: ctx}
	case UNREGISTER:
		return &Unregister{ctx: ctx}
	case UNREGISTERED:
		return &Unregistered{ctx: ctx}
	case INVOCATION:
		return &Invocation{ctx: ctx}
	case INTERRUPT:
		return &Interrupt{ctx: ctx}
	case YIELD:
		return &Yield{ctx: ctx}
	default:
		// TODO: allow custom message types?
		return nil
	}
}

func (mt MessageType) String() string {
	switch mt {
	case HELLO:
		return "HELLO"
	case WELCOME:
		return "WELCOME"
	case ABORT:
		return "ABORT"
	case CHALLENGE:
		return "CHALLENGE"
	case AUTHENTICATE:
		return "AUTHENTICATE"
	case GOODBYE:
		return "GOODBYE"
	case ERROR:
		return "ERROR"

	case PUBLISH:
		return "PUBLISH"
	case PUBLISHED:
		return "PUBLISHED"

	case SUBSCRIBE:
		return "SUBSCRIBE"
	case SUBSCRIBED:
		return "SUBSCRIBED"
	case UNSUBSCRIBE:
		return "UNSUBSCRIBE"
	case UNSUBSCRIBED:
		return "UNSUBSCRIBED"
	case EVENT:
		return "EVENT"

	case CALL:
		return "CALL"
	case CANCEL:
		return "CANCEL"
	case RESULT:
		return "RESULT"

	case REGISTER:
		return "REGISTER"
	case REGISTERED:
		return "REGISTERED"
	case UNREGISTER:
		return "UNREGISTER"
	case UNREGISTERED:
		return "UNREGISTERED"
	case INVOCATION:
		return "INVOCATION"
	case INTERRUPT:
		return "INTERRUPT"
	case YIELD:
		return "YIELD"
	default:
		return "CUSTOM"
	}
}

const (
	HELLO        MessageType = 1
	WELCOME      MessageType = 2
	ABORT        MessageType = 3
	CHALLENGE    MessageType = 4
	AUTHENTICATE MessageType = 5
	GOODBYE      MessageType = 6
	ERROR        MessageType = 8

	PUBLISH   MessageType = 16 //	Tx 	Rx
	PUBLISHED MessageType = 17 //	Rx 	Tx

	SUBSCRIBE    MessageType = 32 //	Rx 	Tx
	SUBSCRIBED   MessageType = 33 //	Tx 	Rx
	UNSUBSCRIBE  MessageType = 34 //	Rx 	Tx
	UNSUBSCRIBED MessageType = 35 //	Tx 	Rx
	EVENT        MessageType = 36 //	Tx 	Rx

	CALL   MessageType = 48 //	Tx 	Rx
	CANCEL MessageType = 49 //	Tx 	Rx
	RESULT MessageType = 50 //	Rx 	Tx

	REGISTER     MessageType = 64 //	Rx 	Tx
	REGISTERED   MessageType = 65 //	Tx 	Rx
	UNREGISTER   MessageType = 66 //	Rx 	Tx
	UNREGISTERED MessageType = 67 //	Tx 	Rx
	INVOCATION   MessageType = 68 //	Tx 	Rx
	INTERRUPT    MessageType = 69 //	Tx 	Rx
	YIELD        MessageType = 70 //	Rx 	Tx

	CUSTOM_MESSAGE MessageType = 80
)

// URIs are dot-separated identifiers, where each component *should* only contain letters, numbers or underscores.
//
// See the documentation for specifics: https://github.com/wamp-proto/wamp-proto/blob/master/rfc/text/basic/bp_identifiers.md#uris-uris
type URI string

// An ID is a unique, non-negative number. Different uses may have additional restrictions.
type ID uint64

// [HELLO, Realm|uri, Details|dict]
type Hello struct {
	ctx     context.Context
	Realm   URI
	Details map[string]interface{}
}

func (msg *Hello) MessageType() MessageType {
	return HELLO
}

func (msg *Hello) Context() context.Context {
	return msg.ctx
}

// [WELCOME, Session|id, Details|dict]
type Welcome struct {
	ctx     context.Context
	Id      ID
	Details map[string]interface{}
}

func (msg *Welcome) MessageType() MessageType {
	return WELCOME
}

func (msg *Welcome) Context() context.Context {
	return msg.ctx
}

// [ABORT, Details|dict, Reason|uri]
type Abort struct {
	ctx     context.Context
	Details map[string]interface{}
	Reason  URI
}

func (msg *Abort) MessageType() MessageType {
	return ABORT
}

func (msg *Abort) Context() context.Context {
	return msg.ctx
}

// [CHALLENGE, AuthMethod|string, Extra|dict]
type Challenge struct {
	ctx        context.Context
	AuthMethod string
	Extra      map[string]interface{}
}

func (msg *Challenge) MessageType() MessageType {
	return CHALLENGE
}

func (msg *Challenge) Context() context.Context {
	return msg.ctx
}

// [AUTHENTICATE, Signature|string, Extra|dict]
type Authenticate struct {
	ctx       context.Context
	Signature string
	Extra     map[string]interface{}
}

func (msg *Authenticate) MessageType() MessageType {
	return AUTHENTICATE
}

func (msg *Authenticate) Context() context.Context {
	return msg.ctx
}

// [GOODBYE, Details|dict, Reason|uri]
type Goodbye struct {
	ctx     context.Context
	Details map[string]interface{}
	Reason  URI
}

func (msg *Goodbye) MessageType() MessageType {
	return GOODBYE
}

func (msg *Goodbye) Context() context.Context {
	return msg.ctx
}

// [ERROR, REQUEST.Type|int, REQUEST.Request|id, Details|dict, Error|uri]
// [ERROR, REQUEST.Type|int, REQUEST.Request|id, Details|dict, Error|uri, Arguments|list]
// [ERROR, REQUEST.Type|int, REQUEST.Request|id, Details|dict, Error|uri, Arguments|list, ArgumentsKw|dict]
type Error struct {
	ctx         context.Context
	Type        MessageType
	Request     ID
	Details     map[string]interface{}
	Error       URI
	Arguments   []interface{}          `wamp:"omitempty"`
	ArgumentsKw map[string]interface{} `wamp:"omitempty"`
}

func (msg *Error) MessageType() MessageType {
	return ERROR
}

func (msg *Error) Context() context.Context {
	return msg.ctx
}

// [PUBLISH, Request|id, Options|dict, Topic|uri]
// [PUBLISH, Request|id, Options|dict, Topic|uri, Arguments|list]
// [PUBLISH, Request|id, Options|dict, Topic|uri, Arguments|list, ArgumentsKw|dict]
type Publish struct {
	ctx         context.Context
	Request     ID
	Options     map[string]interface{}
	Topic       URI
	Arguments   []interface{}          `wamp:"omitempty"`
	ArgumentsKw map[string]interface{} `wamp:"omitempty"`
}

func (msg *Publish) MessageType() MessageType {
	return PUBLISH
}

func (msg *Publish) Context() context.Context {
	return msg.ctx
}

// [PUBLISHED, PUBLISH.Request|id, Publication|id]
type Published struct {
	ctx         context.Context
	Request     ID
	Publication ID
}

func (msg *Published) MessageType() MessageType {
	return PUBLISHED
}

func (msg *Published) Context() context.Context {
	return msg.ctx
}

// [SUBSCRIBE, Request|id, Options|dict, Topic|uri]
type Subscribe struct {
	ctx     context.Context
	Request ID
	Options map[string]interface{}
	Topic   URI
}

func (msg *Subscribe) MessageType() MessageType {
	return SUBSCRIBE
}

func (msg *Subscribe) Context() context.Context {
	return msg.ctx
}

// [SUBSCRIBED, SUBSCRIBE.Request|id, Subscription|id]
type Subscribed struct {
	ctx          context.Context
	Request      ID
	Subscription ID
}

func (msg *Subscribed) MessageType() MessageType {
	return SUBSCRIBED
}

func (msg *Subscribed) Context() context.Context {
	return msg.ctx
}

// [UNSUBSCRIBE, Request|id, SUBSCRIBED.Subscription|id]
type Unsubscribe struct {
	ctx          context.Context
	Request      ID
	Subscription ID
}

func (msg *Unsubscribe) MessageType() MessageType {
	return UNSUBSCRIBE
}

func (msg *Unsubscribe) Context() context.Context {
	return msg.ctx
}

// [UNSUBSCRIBED, UNSUBSCRIBE.Request|id]
type Unsubscribed struct {
	ctx     context.Context
	Request ID
}

func (msg *Unsubscribed) MessageType() MessageType {
	return UNSUBSCRIBED
}

func (msg *Unsubscribed) Context() context.Context {
	return msg.ctx
}

// [EVENT, SUBSCRIBED.Subscription|id, PUBLISHED.Publication|id, Details|dict]
// [EVENT, SUBSCRIBED.Subscription|id, PUBLISHED.Publication|id, Details|dict, PUBLISH.Arguments|list]
// [EVENT, SUBSCRIBED.Subscription|id, PUBLISHED.Publication|id, Details|dict, PUBLISH.Arguments|list,
//     PUBLISH.ArgumentsKw|dict]
type Event struct {
	ctx          context.Context
	Subscription ID
	Publication  ID
	Details      map[string]interface{}
	Arguments    []interface{}          `wamp:"omitempty"`
	ArgumentsKw  map[string]interface{} `wamp:"omitempty"`
}

func (msg *Event) MessageType() MessageType {
	return EVENT
}

func (msg *Event) Context() context.Context {
	return msg.ctx
}

// CallResult represents the result of a CALL.
type CallResult struct {
	Args   []interface{}
	Kwargs map[string]interface{}
	Err    URI
}

// [CALL, Request|id, Options|dict, Procedure|uri]
// [CALL, Request|id, Options|dict, Procedure|uri, Arguments|list]
// [CALL, Request|id, Options|dict, Procedure|uri, Arguments|list, ArgumentsKw|dict]
type Call struct {
	ctx         context.Context
	Request     ID
	Options     map[string]interface{}
	Procedure   URI
	Arguments   []interface{}          `wamp:"omitempty"`
	ArgumentsKw map[string]interface{} `wamp:"omitempty"`
}

func (msg *Call) MessageType() MessageType {
	return CALL
}

func (msg *Call) Context() context.Context {
	return msg.ctx
}

// [RESULT, CALL.Request|id, Details|dict]
// [RESULT, CALL.Request|id, Details|dict, YIELD.Arguments|list]
// [RESULT, CALL.Request|id, Details|dict, YIELD.Arguments|list, YIELD.ArgumentsKw|dict]
type Result struct {
	ctx         context.Context
	Request     ID
	Details     map[string]interface{}
	Arguments   []interface{}          `wamp:"omitempty"`
	ArgumentsKw map[string]interface{} `wamp:"omitempty"`
}

func (msg *Result) MessageType() MessageType {
	return RESULT
}

func (msg *Result) Context() context.Context {
	return msg.ctx
}

// [REGISTER, Request|id, Options|dict, Procedure|uri]
type Register struct {
	ctx       context.Context
	Request   ID
	Options   map[string]interface{}
	Procedure URI
}

func (msg *Register) MessageType() MessageType {
	return REGISTER
}

func (msg *Register) Context() context.Context {
	return msg.ctx
}

// [REGISTERED, REGISTER.Request|id, Registration|id]
type Registered struct {
	ctx          context.Context
	Request      ID
	Registration ID
}

func (msg *Registered) MessageType() MessageType {
	return REGISTERED
}

func (msg *Registered) Context() context.Context {
	return msg.ctx
}

// [UNREGISTER, Request|id, REGISTERED.Registration|id]
type Unregister struct {
	ctx          context.Context
	Request      ID
	Registration ID
}

func (msg *Unregister) MessageType() MessageType {
	return UNREGISTER
}

func (msg *Unregister) Context() context.Context {
	return msg.ctx
}

// [UNREGISTERED, UNREGISTER.Request|id]
type Unregistered struct {
	ctx     context.Context
	Request ID
}

func (msg *Unregistered) MessageType() MessageType {
	return UNREGISTERED
}

func (msg *Unregistered) Context() context.Context {
	return msg.ctx
}

// [INVOCATION, Request|id, REGISTERED.Registration|id, Details|dict]
// [INVOCATION, Request|id, REGISTERED.Registration|id, Details|dict, CALL.Arguments|list]
// [INVOCATION, Request|id, REGISTERED.Registration|id, Details|dict, CALL.Arguments|list, CALL.ArgumentsKw|dict]
type Invocation struct {
	ctx          context.Context
	Request      ID
	Registration ID
	Details      map[string]interface{}
	Arguments    []interface{}          `wamp:"omitempty"`
	ArgumentsKw  map[string]interface{} `wamp:"omitempty"`
}

func (msg *Invocation) MessageType() MessageType {
	return INVOCATION
}

func (msg *Invocation) Context() context.Context {
	return msg.ctx
}

// [YIELD, INVOCATION.Request|id, Options|dict]
// [YIELD, INVOCATION.Request|id, Options|dict, Arguments|list]
// [YIELD, INVOCATION.Request|id, Options|dict, Arguments|list, ArgumentsKw|dict]
type Yield struct {
	ctx         context.Context
	Request     ID
	Options     map[string]interface{}
	Arguments   []interface{}          `wamp:"omitempty"`
	ArgumentsKw map[string]interface{} `wamp:"omitempty"`
}

func (msg *Yield) MessageType() MessageType {
	return YIELD
}

func (msg *Yield) Context() context.Context {
	return msg.ctx
}

// [CANCEL, CALL.Request|id, Options|dict]
type Cancel struct {
	ctx     context.Context
	Request ID
	Options map[string]interface{}
}

func (msg *Cancel) MessageType() MessageType {
	return CANCEL
}

func (msg *Cancel) Context() context.Context {
	return msg.ctx
}

// [INTERRUPT, INVOCATION.Request|id, Options|dict]
type Interrupt struct {
	ctx     context.Context
	Request ID
	Options map[string]interface{}
}

func (msg *Interrupt) MessageType() MessageType {
	return INTERRUPT
}

func (msg *Interrupt) Context() context.Context {
	return msg.ctx
}

type CustomMessage map[string]interface{}

func (msg CustomMessage) MessageType() MessageType {
	return CUSTOM_MESSAGE
}

func (msg CustomMessage) Context() context.Context {
	v, ok := msg["ctx"]
	if !ok {
		panic("CustomMessage without context seen")
	}

	ctx, ok := v.(context.Context)
	if !ok {
		panic(fmt.Sprintf(
			"CustomMessage has \"ctx\" key, but it is not of type \"context.Context\".  Type seen: %s",
			reflect.TypeOf(ctx)))
	}

	return ctx
}

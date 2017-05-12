package turnpike

import (
	"context"
	"fmt"
	"reflect"
	"sync"
)

// URIs are dot-separated identifiers, where each component *should* only contain letters, numbers or underscores.
//
// See the documentation for specifics: http://wamp-proto.org/static/rfc/draft-oberstet-hybi-crossbar-wamp.html#rfc.section.5.1.1
type URI string

// An ID is a unique, non-negative number. Different uses may have additional restrictions.
type ID uint64

type MessageType int

// constants sourced from: http://wamp-proto.org/static/rfc/draft-oberstet-hybi-crossbar-wamp.html#rfc.section.6.5
const (
	MessageTypeHello        MessageType = 1
	MessageTypeWelcome      MessageType = 2
	MessageTypeAbort        MessageType = 3
	MessageTypeChallenge    MessageType = 4
	MessageTypeAuthenticate MessageType = 5
	MessageTypeGoodbye      MessageType = 6
	MessageTypeError        MessageType = 8

	MessageTypePublish   MessageType = 16
	MessageTypePublished MessageType = 17

	MessageTypeSubscribe    MessageType = 32
	MessageTypeSubscribed   MessageType = 33
	MessageTypeUnsubscribe  MessageType = 34
	MessageTypeUnsubscribed MessageType = 35
	MessageTypeEvent        MessageType = 36

	MessageTypeCall   MessageType = 48
	MessageTypeCancel MessageType = 49
	MessageTypeResult MessageType = 50

	MessageTypeRegister     MessageType = 64
	MessageTypeRegistered   MessageType = 65
	MessageTypeUnregister   MessageType = 66
	MessageTypeUnregistered MessageType = 67
	MessageTypeInvocation   MessageType = 68
	MessageTypeInterrupt    MessageType = 69
	MessageTypeYield        MessageType = 70

	MessageTypeExtensionMin MessageType = 256
	MessageTypeExtensionMax MessageType = 1023
)

// Message is a generic container for a WAMP message.
type Message interface {
	MessageType() MessageType
	Context() context.Context
}

// ExtensionMessage is a generic container for WAMP message extensions
type ExtensionMessage interface {
	MessageType() MessageType
	MessageTypeName() string
	Context() context.Context
	SetContext(context.Context)
}

var extensions = map[MessageType]ExtensionMessage{}
var extensionsLock = sync.RWMutex{}

// RegisterExtensionMessage allows you to specify a custom message type for use within your implementation.
func RegisterExtensionMessage(mt MessageType, proto ExtensionMessage) error {

	// TODO: This could probably be made MUCH simpler...

	var protoType reflect.Type
	var protoKind reflect.Kind

	// validate type value
	if mt < MessageTypeExtensionMin || mt > MessageTypeExtensionMax {
		return fmt.Errorf(
			"Extension Messages must have a type value in range [%d...%d], %d provided",
			MessageTypeExtensionMin,
			MessageTypeExtensionMax,
			mt)
	}

	// get type
	protoType = reflect.TypeOf(proto)

	// check for pointer
	if protoKind == reflect.Ptr {
		protoType = protoType.Elem()
	}

	// get kind
	protoKind = protoType.Kind()

	// only allow structs
	if protoType.Kind() != reflect.Struct {
		return fmt.Errorf("Message implementation must be struct, %s provided", protoType)
	}

	// lock map
	extensionsLock.Lock()
	defer extensionsLock.Unlock()

	// create empty message
	extensions[mt] = reflect.New(protoType).Interface().(ExtensionMessage)

	return nil
}

// NewExtensionMessage will attempt to construct a new instance of an extension message type
func NewExtensionMessage(ctx context.Context, mt MessageType) (ExtensionMessage, error) {
	extensionsLock.RLock()
	defer extensionsLock.RUnlock()

	// has extension been defined?
	proto, ok := extensions[mt]
	if !ok {
		return nil, fmt.Errorf("\"%d\" is not a registered message extension type", mt)
	}

	// create new message
	msg := reflect.New(reflect.Type(proto)).Interface().(ExtensionMessage)

	// set context
	msg.SetContext(ctx)

	return msg, nil
}

func (mt MessageType) New(ctx context.Context) Message {
	switch mt {
	case MessageTypeHello:
		return &Hello{ctx: ctx}
	case MessageTypeWelcome:
		return &Welcome{ctx: ctx}
	case MessageTypeAbort:
		return &Abort{ctx: ctx}
	case MessageTypeChallenge:
		return &Challenge{ctx: ctx}
	case MessageTypeAuthenticate:
		return &Authenticate{ctx: ctx}
	case MessageTypeGoodbye:
		return &Goodbye{ctx: ctx}
	case MessageTypeError:
		return &Error{ctx: ctx}

	case MessageTypePublish:
		return &Publish{ctx: ctx}
	case MessageTypePublished:
		return &Published{ctx: ctx}

	case MessageTypeSubscribe:
		return &Subscribe{ctx: ctx}
	case MessageTypeSubscribed:
		return &Subscribed{ctx: ctx}
	case MessageTypeUnsubscribe:
		return &Unsubscribe{ctx: ctx}
	case MessageTypeUnsubscribed:
		return &Unsubscribed{ctx: ctx}
	case MessageTypeEvent:
		return &Event{ctx: ctx}

	case MessageTypeCall:
		return &Call{ctx: ctx}
	case MessageTypeCancel:
		return &Cancel{ctx: ctx}
	case MessageTypeResult:
		return &Result{ctx: ctx}

	case MessageTypeRegister:
		return &Register{ctx: ctx}
	case MessageTypeRegistered:
		return &Registered{ctx: ctx}
	case MessageTypeUnregister:
		return &Unregister{ctx: ctx}
	case MessageTypeUnregistered:
		return &Unregistered{ctx: ctx}
	case MessageTypeInvocation:
		return &Invocation{ctx: ctx}
	case MessageTypeInterrupt:
		return &Interrupt{ctx: ctx}
	case MessageTypeYield:
		return &Yield{ctx: ctx}
	default:
		msg, err := NewExtensionMessage(ctx, mt)
		if nil != err {
			panic(err.Error())
		}
		return msg
	}
}

func (mt MessageType) String() string {
	switch mt {
	case MessageTypeHello:
		return "HELLO"
	case MessageTypeWelcome:
		return "WELCOME"
	case MessageTypeAbort:
		return "ABORT"
	case MessageTypeChallenge:
		return "CHALLENGE"
	case MessageTypeAuthenticate:
		return "AUTHENTICATE"
	case MessageTypeGoodbye:
		return "GOODBYE"
	case MessageTypeError:
		return "ERROR"

	case MessageTypePublish:
		return "PUBLISH"
	case MessageTypePublished:
		return "PUBLISHED"

	case MessageTypeSubscribe:
		return "SUBSCRIBE"
	case MessageTypeSubscribed:
		return "SUBSCRIBED"
	case MessageTypeUnsubscribe:
		return "UNSUBSCRIBE"
	case MessageTypeUnsubscribed:
		return "UNSUBSCRIBED"
	case MessageTypeEvent:
		return "EVENT"

	case MessageTypeCall:
		return "CALL"
	case MessageTypeCancel:
		return "CANCEL"
	case MessageTypeResult:
		return "RESULT"

	case MessageTypeRegister:
		return "REGISTER"
	case MessageTypeRegistered:
		return "REGISTERED"
	case MessageTypeUnregister:
		return "UNREGISTER"
	case MessageTypeUnregistered:
		return "UNREGISTERED"
	case MessageTypeInvocation:
		return "INVOCATION"
	case MessageTypeInterrupt:
		return "INTERRUPT"
	case MessageTypeYield:
		return "YIELD"
	default:
		msg, err := NewExtensionMessage(nil, mt)
		if nil != err {
			panic(err.Error())
		}
		return msg.MessageTypeName()
	}
}

// [MessageTypeHello, Realm|uri, Details|dict]
type Hello struct {
	ctx     context.Context
	Realm   URI
	Details map[string]interface{}
}

func (msg *Hello) MessageType() MessageType {
	return MessageTypeHello
}

func (msg *Hello) Context() context.Context {
	return msg.ctx
}

// [MessageTypeWelcome, Session|id, Details|dict]
type Welcome struct {
	ctx     context.Context
	Id      ID
	Details map[string]interface{}
}

func (msg *Welcome) MessageType() MessageType {
	return MessageTypeWelcome
}

func (msg *Welcome) Context() context.Context {
	return msg.ctx
}

// [MessageTypeAbort, Details|dict, Reason|uri]
type Abort struct {
	ctx     context.Context
	Details map[string]interface{}
	Reason  URI
}

func (msg *Abort) MessageType() MessageType {
	return MessageTypeAbort
}

func (msg *Abort) Context() context.Context {
	return msg.ctx
}

// [MessageTypeChallenge, AuthMethod|string, Extra|dict]
type Challenge struct {
	ctx        context.Context
	AuthMethod string
	Extra      map[string]interface{}
}

func (msg *Challenge) MessageType() MessageType {
	return MessageTypeChallenge
}

func (msg *Challenge) Context() context.Context {
	return msg.ctx
}

// [MessageTypeAuthenticate, Signature|string, Extra|dict]
type Authenticate struct {
	ctx       context.Context
	Signature string
	Extra     map[string]interface{}
}

func (msg *Authenticate) MessageType() MessageType {
	return MessageTypeAuthenticate
}

func (msg *Authenticate) Context() context.Context {
	return msg.ctx
}

// [MessageTypeGoodbye, Details|dict, Reason|uri]
type Goodbye struct {
	ctx     context.Context
	Details map[string]interface{}
	Reason  URI
}

func (msg *Goodbye) MessageType() MessageType {
	return MessageTypeGoodbye
}

func (msg *Goodbye) Context() context.Context {
	return msg.ctx
}

// [MessageTypeError, REQUEST.Type|int, REQUEST.Request|id, Details|dict, Error|uri]
// [MessageTypeError, REQUEST.Type|int, REQUEST.Request|id, Details|dict, Error|uri, Arguments|list]
// [MessageTypeError, REQUEST.Type|int, REQUEST.Request|id, Details|dict, Error|uri, Arguments|list, ArgumentsKw|dict]
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
	return MessageTypeError
}

func (msg *Error) Context() context.Context {
	return msg.ctx
}

// [MessageTypePublish, Request|id, Options|dict, Topic|uri]
// [MessageTypePublish, Request|id, Options|dict, Topic|uri, Arguments|list]
// [MessageTypePublish, Request|id, Options|dict, Topic|uri, Arguments|list, ArgumentsKw|dict]
type Publish struct {
	ctx         context.Context
	Request     ID
	Options     map[string]interface{}
	Topic       URI
	Arguments   []interface{}          `wamp:"omitempty"`
	ArgumentsKw map[string]interface{} `wamp:"omitempty"`
}

func (msg *Publish) MessageType() MessageType {
	return MessageTypePublish
}

func (msg *Publish) Context() context.Context {
	return msg.ctx
}

// [MessageTypePublished, MessageTypePublish.Request|id, Publication|id]
type Published struct {
	ctx         context.Context
	Request     ID
	Publication ID
}

func (msg *Published) MessageType() MessageType {
	return MessageTypePublished
}

func (msg *Published) Context() context.Context {
	return msg.ctx
}

// [MessageTypeSubscribe, Request|id, Options|dict, Topic|uri]
type Subscribe struct {
	ctx     context.Context
	Request ID
	Options map[string]interface{}
	Topic   URI
}

func (msg *Subscribe) MessageType() MessageType {
	return MessageTypeSubscribe
}

func (msg *Subscribe) Context() context.Context {
	return msg.ctx
}

// [MessageTypeSubscribed, MessageTypeSubscribe.Request|id, Subscription|id]
type Subscribed struct {
	ctx          context.Context
	Request      ID
	Subscription ID
}

func (msg *Subscribed) MessageType() MessageType {
	return MessageTypeSubscribed
}

func (msg *Subscribed) Context() context.Context {
	return msg.ctx
}

// [MessageTypeUnsubscribe, Request|id, MessageTypeSubscribed.Subscription|id]
type Unsubscribe struct {
	ctx          context.Context
	Request      ID
	Subscription ID
}

func (msg *Unsubscribe) MessageType() MessageType {
	return MessageTypeUnsubscribe
}

func (msg *Unsubscribe) Context() context.Context {
	return msg.ctx
}

// [MessageTypeUnsubscribed, MessageTypeUnsubscribe.Request|id]
type Unsubscribed struct {
	ctx     context.Context
	Request ID
}

func (msg *Unsubscribed) MessageType() MessageType {
	return MessageTypeUnsubscribed
}

func (msg *Unsubscribed) Context() context.Context {
	return msg.ctx
}

// [MessageTypeEvent, MessageTypeSubscribed.Subscription|id, MessageTypePublished.Publication|id, Details|dict]
// [MessageTypeEvent, MessageTypeSubscribed.Subscription|id, MessageTypePublished.Publication|id, Details|dict, MessageTypePublish.Arguments|list]
// [MessageTypeEvent, MessageTypeSubscribed.Subscription|id, MessageTypePublished.Publication|id, Details|dict, MessageTypePublish.Arguments|list,
//     MessageTypePublish.ArgumentsKw|dict]
type Event struct {
	ctx          context.Context
	Subscription ID
	Publication  ID
	Details      map[string]interface{}
	Arguments    []interface{}          `wamp:"omitempty"`
	ArgumentsKw  map[string]interface{} `wamp:"omitempty"`
}

func (msg *Event) MessageType() MessageType {
	return MessageTypeEvent
}

func (msg *Event) Context() context.Context {
	return msg.ctx
}

// CallResult represents the result of a MessageTypeCall.
type CallResult struct {
	Args   []interface{}
	Kwargs map[string]interface{}
	Err    URI
}

// [MessageTypeCall, Request|id, Options|dict, Procedure|uri]
// [MessageTypeCall, Request|id, Options|dict, Procedure|uri, Arguments|list]
// [MessageTypeCall, Request|id, Options|dict, Procedure|uri, Arguments|list, ArgumentsKw|dict]
type Call struct {
	ctx         context.Context
	Request     ID
	Options     map[string]interface{}
	Procedure   URI
	Arguments   []interface{}          `wamp:"omitempty"`
	ArgumentsKw map[string]interface{} `wamp:"omitempty"`
}

func (msg *Call) MessageType() MessageType {
	return MessageTypeCall
}

func (msg *Call) Context() context.Context {
	return msg.ctx
}

// [MessageTypeResult, MessageTypeCall.Request|id, Details|dict]
// [MessageTypeResult, MessageTypeCall.Request|id, Details|dict, MessageTypeYield.Arguments|list]
// [MessageTypeResult, MessageTypeCall.Request|id, Details|dict, MessageTypeYield.Arguments|list, MessageTypeYield.ArgumentsKw|dict]
type Result struct {
	ctx         context.Context
	Request     ID
	Details     map[string]interface{}
	Arguments   []interface{}          `wamp:"omitempty"`
	ArgumentsKw map[string]interface{} `wamp:"omitempty"`
}

func (msg *Result) MessageType() MessageType {
	return MessageTypeResult
}

func (msg *Result) Context() context.Context {
	return msg.ctx
}

// [MessageTypeRegister, Request|id, Options|dict, Procedure|uri]
type Register struct {
	ctx       context.Context
	Request   ID
	Options   map[string]interface{}
	Procedure URI
}

func (msg *Register) MessageType() MessageType {
	return MessageTypeRegister
}

func (msg *Register) Context() context.Context {
	return msg.ctx
}

// [MessageTypeRegistered, MessageTypeRegister.Request|id, Registration|id]
type Registered struct {
	ctx          context.Context
	Request      ID
	Registration ID
}

func (msg *Registered) MessageType() MessageType {
	return MessageTypeRegistered
}

func (msg *Registered) Context() context.Context {
	return msg.ctx
}

// [MessageTypeUnregister, Request|id, MessageTypeRegistered.Registration|id]
type Unregister struct {
	ctx          context.Context
	Request      ID
	Registration ID
}

func (msg *Unregister) MessageType() MessageType {
	return MessageTypeUnregister
}

func (msg *Unregister) Context() context.Context {
	return msg.ctx
}

// [MessageTypeUnregistered, MessageTypeUnregister.Request|id]
type Unregistered struct {
	ctx     context.Context
	Request ID
}

func (msg *Unregistered) MessageType() MessageType {
	return MessageTypeUnregistered
}

func (msg *Unregistered) Context() context.Context {
	return msg.ctx
}

// [MessageTypeInvocation, Request|id, MessageTypeRegistered.Registration|id, Details|dict]
// [MessageTypeInvocation, Request|id, MessageTypeRegistered.Registration|id, Details|dict, MessageTypeCall.Arguments|list]
// [MessageTypeInvocation, Request|id, MessageTypeRegistered.Registration|id, Details|dict, MessageTypeCall.Arguments|list, MessageTypeCall.ArgumentsKw|dict]
type Invocation struct {
	ctx          context.Context
	Request      ID
	Registration ID
	Details      map[string]interface{}
	Arguments    []interface{}          `wamp:"omitempty"`
	ArgumentsKw  map[string]interface{} `wamp:"omitempty"`
}

func (msg *Invocation) MessageType() MessageType {
	return MessageTypeInvocation
}

func (msg *Invocation) Context() context.Context {
	return msg.ctx
}

// [MessageTypeYield, MessageTypeInvocation.Request|id, Options|dict]
// [MessageTypeYield, MessageTypeInvocation.Request|id, Options|dict, Arguments|list]
// [MessageTypeYield, MessageTypeInvocation.Request|id, Options|dict, Arguments|list, ArgumentsKw|dict]
type Yield struct {
	ctx         context.Context
	Request     ID
	Options     map[string]interface{}
	Arguments   []interface{}          `wamp:"omitempty"`
	ArgumentsKw map[string]interface{} `wamp:"omitempty"`
}

func (msg *Yield) MessageType() MessageType {
	return MessageTypeYield
}

func (msg *Yield) Context() context.Context {
	return msg.ctx
}

// [MessageTypeCancel, MessageTypeCall.Request|id, Options|dict]
type Cancel struct {
	ctx     context.Context
	Request ID
	Options map[string]interface{}
}

func (msg *Cancel) MessageType() MessageType {
	return MessageTypeCancel
}

func (msg *Cancel) Context() context.Context {
	return msg.ctx
}

// [MessageTypeInterrupt, MessageTypeInvocation.Request|id, Options|dict]
type Interrupt struct {
	ctx     context.Context
	Request ID
	Options map[string]interface{}
}

func (msg *Interrupt) MessageType() MessageType {
	return MessageTypeInterrupt
}

func (msg *Interrupt) Context() context.Context {
	return msg.ctx
}

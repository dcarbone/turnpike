package turnpike

import (
	"fmt"
	"reflect"
	"sync"
)

type (
	// Message is a generic container for a WAMP message.
	// URIs are dot-separated identifiers, where each component *should* only contain letters, numbers or underscores.
	//
	// See the documentation for specifics: https://github.com/wamp-proto/wamp-proto/blob/master/rfc/text/basic/bp_identifiers.md#uris-uris
	URI string

	// An ID is a unique, non-negative number. Different uses may have additional restrictions.
	ID uint64

	// constants sourced from: http://wamp-proto.org/static/rfc/draft-oberstet-hybi-crossbar-wamp.html#rfc.section.6.5
	MessageType int

	// Message is a generic container for a WAMP message.
	Message interface {
		MessageType() MessageType
	}

	// ExtensionMessage is a generic container for WAMP message extensions
	ExtensionMessage interface {
		Message

		MessageTypeName() string
	}
)

var (
	extensions     = map[MessageType]ExtensionMessage{}
	extensionsLock = sync.RWMutex{}
)

const (
	MessageTypeHello        MessageType = 1
	MessageTypeWelcome      MessageType = 2
	MessageTypeAbort        MessageType = 3
	MessageTypeChallenge    MessageType = 4
	MessageTypeAuthenticate MessageType = 5
	MessageTypeGoodbye      MessageType = 6
	MessageTypeError        MessageType = 8

	MessageTypePublish   MessageType = 16 //	Tx 	Rx
	MessageTypePublished MessageType = 17 //	Rx 	Tx

	MessageTypeSubscribe    MessageType = 32 //	Rx 	Tx
	MessageTypeSubscribed   MessageType = 33 //	Tx 	Rx
	MessageTypeUnsubscribe  MessageType = 34 //	Rx 	Tx
	MessageTypeUnsubscribed MessageType = 35 //	Tx 	Rx
	MessageTypeEvent        MessageType = 36 //	Tx 	Rx

	MessageTypeCall   MessageType = 48 //	Tx 	Rx
	MessageTypeCancel MessageType = 49 //	Tx 	Rx
	MessageTypeResult MessageType = 50 //	Rx 	Tx

	MessageTypeRegister     MessageType = 64 //	Rx 	Tx
	MessageTypeRegistered   MessageType = 65 //	Tx 	Rx
	MessageTypeUnregister   MessageType = 66 //	Rx 	Tx
	MessageTypeUnregistered MessageType = 67 //	Tx 	Rx
	MessageTypeInvocation   MessageType = 68 //	Tx 	Rx
	MessageTypeInterrupt    MessageType = 69 //	Tx 	Rx
	MessageTypeYield        MessageType = 70 //	Rx 	Tx

	MessageTypeExtensionMin MessageType = 256
	MessageTypeExtensionMax MessageType = 1023
)

func (mt MessageType) New() Message {
	switch mt {
	case MessageTypeHello:
		return new(Hello)
	case MessageTypeWelcome:
		return new(Welcome)
	case MessageTypeAbort:
		return new(Abort)
	case MessageTypeChallenge:
		return new(Challenge)
	case MessageTypeAuthenticate:
		return new(Authenticate)
	case MessageTypeGoodbye:
		return new(Goodbye)
	case MessageTypeError:
		return new(Error)

	case MessageTypePublish:
		return new(Publish)
	case MessageTypePublished:
		return new(Published)

	case MessageTypeSubscribe:
		return new(Subscribe)
	case MessageTypeSubscribed:
		return new(Subscribed)
	case MessageTypeUnsubscribe:
		return new(Unsubscribe)
	case MessageTypeUnsubscribed:
		return new(Unsubscribed)
	case MessageTypeEvent:
		return new(Event)

	case MessageTypeCall:
		return new(Call)
	case MessageTypeCancel:
		return new(Cancel)
	case MessageTypeResult:
		return new(Result)

	case MessageTypeRegister:
		return new(Register)
	case MessageTypeRegistered:
		return new(Registered)
	case MessageTypeUnregister:
		return new(Unregister)
	case MessageTypeUnregistered:
		return new(Unregistered)
	case MessageTypeInvocation:
		return new(Invocation)
	case MessageTypeInterrupt:
		return new(Interrupt)
	case MessageTypeYield:
		return new(Yield)
	default:
		msg, err := NewExtensionMessage(mt)
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
		msg, err := NewExtensionMessage(mt)
		if nil != err {
			panic(err.Error())
		}
		return msg.MessageTypeName()
	}
}

// RegisterExtensionMessage allows you to specify a custom message type for use within your implementation.
func RegisterExtensionMessage(mt MessageType, proto ExtensionMessage) error {

	// TODO: This could probably be made MUCH simpler...

	var protoType reflect.Type
	var protoKind reflect.Kind

	// validate type value
	if mt < MessageTypeExtensionMin || mt > MessageTypeExtensionMax {
		return fmt.Errorf(
			"Extension Messages must have a Type value in range [%d...%d], %d provided",
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
func NewExtensionMessage(mt MessageType) (ExtensionMessage, error) {
	extensionsLock.RLock()
	defer extensionsLock.RUnlock()

	// has extension been defined?
	proto, ok := extensions[mt]
	if !ok {
		return nil, fmt.Errorf("\"%d\" is not a registered message extension type", mt)
	}

	// create new message
	msg := reflect.New(reflect.Type(proto)).Interface().(ExtensionMessage)

	return msg, nil
}

// [HELLO, Realm|uri, Details|dict]
type Hello struct {
	Realm   URI
	Details map[string]interface{}
}

func (msg *Hello) MessageType() MessageType {
	return MessageTypeHello
}

// [WELCOME, Session|id, Details|dict]
type Welcome struct {
	Id      ID
	Details map[string]interface{}
}

func (msg *Welcome) MessageType() MessageType {
	return MessageTypeWelcome
}

// [ABORT, Details|dict, Reason|uri]
type Abort struct {
	Details map[string]interface{}
	Reason  URI
}

func (msg *Abort) MessageType() MessageType {
	return MessageTypeAbort
}

// [CHALLENGE, AuthMethod|string, Extra|dict]
type Challenge struct {
	AuthMethod string
	Extra      map[string]interface{}
}

func (msg *Challenge) MessageType() MessageType {
	return MessageTypeChallenge
}

// [AUTHENTICATE, Signature|string, Extra|dict]
type Authenticate struct {
	Signature string
	Extra     map[string]interface{}
}

func (msg *Authenticate) MessageType() MessageType {
	return MessageTypeAuthenticate
}

// [GOODBYE, Details|dict, Reason|uri]
type Goodbye struct {
	Details map[string]interface{}
	Reason  URI
}

func (msg *Goodbye) MessageType() MessageType {
	return MessageTypeGoodbye
}

// [ERROR, REQUEST.Type|int, REQUEST.Request|id, Details|dict, Error|uri]
// [ERROR, REQUEST.Type|int, REQUEST.Request|id, Details|dict, Error|uri, Arguments|list]
// [ERROR, REQUEST.Type|int, REQUEST.Request|id, Details|dict, Error|uri, Arguments|list, ArgumentsKw|dict]
type Error struct {
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

// [PUBLISH, Request|id, Options|dict, Topic|uri]
// [PUBLISH, Request|id, Options|dict, Topic|uri, Arguments|list]
// [PUBLISH, Request|id, Options|dict, Topic|uri, Arguments|list, ArgumentsKw|dict]
type Publish struct {
	Request     ID
	Options     map[string]interface{}
	Topic       URI
	Arguments   []interface{}          `wamp:"omitempty"`
	ArgumentsKw map[string]interface{} `wamp:"omitempty"`
}

func (msg *Publish) MessageType() MessageType {
	return MessageTypePublish
}

// [PUBLISHED, PUBLISH.Request|id, Publication|id]
type Published struct {
	Request     ID
	Publication ID
}

func (msg *Published) MessageType() MessageType {
	return MessageTypePublished
}

// [SUBSCRIBE, Request|id, Options|dict, Topic|uri]
type Subscribe struct {
	Request ID
	Options map[string]interface{}
	Topic   URI
}

func (msg *Subscribe) MessageType() MessageType {
	return MessageTypeSubscribe
}

// [SUBSCRIBED, SUBSCRIBE.Request|id, Subscription|id]
type Subscribed struct {
	Request      ID
	Subscription ID
}

func (msg *Subscribed) MessageType() MessageType {
	return MessageTypeSubscribed
}

// [UNSUBSCRIBE, Request|id, SUBSCRIBED.Subscription|id]
type Unsubscribe struct {
	Request      ID
	Subscription ID
}

func (msg *Unsubscribe) MessageType() MessageType {
	return MessageTypeUnsubscribe
}

// [UNSUBSCRIBED, UNSUBSCRIBE.Request|id]
type Unsubscribed struct {
	Request ID
}

func (msg *Unsubscribed) MessageType() MessageType {
	return MessageTypeUnsubscribed
}

// [EVENT, SUBSCRIBED.Subscription|id, PUBLISHED.Publication|id, Details|dict]
// [EVENT, SUBSCRIBED.Subscription|id, PUBLISHED.Publication|id, Details|dict, PUBLISH.Arguments|list]
// [EVENT, SUBSCRIBED.Subscription|id, PUBLISHED.Publication|id, Details|dict, PUBLISH.Arguments|list,
//     PUBLISH.ArgumentsKw|dict]
type Event struct {
	Subscription ID
	Publication  ID
	Details      map[string]interface{}
	Arguments    []interface{}          `wamp:"omitempty"`
	ArgumentsKw  map[string]interface{} `wamp:"omitempty"`
}

func (msg *Event) MessageType() MessageType {
	return MessageTypeEvent
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
	Request     ID
	Options     map[string]interface{}
	Procedure   URI
	Arguments   []interface{}          `wamp:"omitempty"`
	ArgumentsKw map[string]interface{} `wamp:"omitempty"`
}

func (msg *Call) MessageType() MessageType {
	return MessageTypeCall
}

// [RESULT, CALL.Request|id, Details|dict]
// [RESULT, CALL.Request|id, Details|dict, YIELD.Arguments|list]
// [RESULT, CALL.Request|id, Details|dict, YIELD.Arguments|list, YIELD.ArgumentsKw|dict]
type Result struct {
	Request     ID
	Details     map[string]interface{}
	Arguments   []interface{}          `wamp:"omitempty"`
	ArgumentsKw map[string]interface{} `wamp:"omitempty"`
}

func (msg *Result) MessageType() MessageType {
	return MessageTypeResult
}

// [REGISTER, Request|id, Options|dict, Procedure|uri]
type Register struct {
	Request   ID
	Options   map[string]interface{}
	Procedure URI
}

func (msg *Register) MessageType() MessageType {
	return MessageTypeRegister
}

// [REGISTERED, REGISTER.Request|id, Registration|id]
type Registered struct {
	Request      ID
	Registration ID
}

func (msg *Registered) MessageType() MessageType {
	return MessageTypeRegistered
}

// [UNREGISTER, Request|id, REGISTERED.Registration|id]
type Unregister struct {
	Request      ID
	Registration ID
}

func (msg *Unregister) MessageType() MessageType {
	return MessageTypeUnregister
}

// [UNREGISTERED, UNREGISTER.Request|id]
type Unregistered struct {
	Request ID
}

func (msg *Unregistered) MessageType() MessageType {
	return MessageTypeUnregistered
}

// [INVOCATION, Request|id, REGISTERED.Registration|id, Details|dict]
// [INVOCATION, Request|id, REGISTERED.Registration|id, Details|dict, CALL.Arguments|list]
// [INVOCATION, Request|id, REGISTERED.Registration|id, Details|dict, CALL.Arguments|list, CALL.ArgumentsKw|dict]
type Invocation struct {
	Request      ID
	Registration ID
	Details      map[string]interface{}
	Arguments    []interface{}          `wamp:"omitempty"`
	ArgumentsKw  map[string]interface{} `wamp:"omitempty"`
}

func (msg *Invocation) MessageType() MessageType {
	return MessageTypeInvocation
}

// [YIELD, INVOCATION.Request|id, Options|dict]
// [YIELD, INVOCATION.Request|id, Options|dict, Arguments|list]
// [YIELD, INVOCATION.Request|id, Options|dict, Arguments|list, ArgumentsKw|dict]
type Yield struct {
	Request     ID
	Options     map[string]interface{}
	Arguments   []interface{}          `wamp:"omitempty"`
	ArgumentsKw map[string]interface{} `wamp:"omitempty"`
}

func (msg *Yield) MessageType() MessageType {
	return MessageTypeYield
}

// [CANCEL, CALL.Request|id, Options|dict]
type Cancel struct {
	Request ID
	Options map[string]interface{}
}

func (msg *Cancel) MessageType() MessageType {
	return MessageTypeCancel
}

// [INTERRUPT, INVOCATION.Request|id, Options|dict]
type Interrupt struct {
	Request ID
	Options map[string]interface{}
}

func (msg *Interrupt) MessageType() MessageType {
	return MessageTypeInterrupt
}

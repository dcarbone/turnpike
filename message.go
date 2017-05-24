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
		sync.Locker

		MessageType() MessageType
		ToPayload() []interface{}
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

	MessageTypePublish   MessageType = 16
	MessageTypePublished MessageType = 17

	MessageTypeSubscribe    MessageType = 32
	MessageTypeSubscribed   MessageType = 33
	MessageTypeUnsubscribe  MessageType = 34
	MessageTypeUnsubscribed MessageType = 35

	MessageTypeEvent MessageType = 36

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
	msg := reflect.New(reflect.TypeOf(proto)).Interface().(ExtensionMessage)

	return msg, nil
}

// [HELLO, Realm|uri, Details|dict]
type Hello struct {
	sync.Mutex
	Realm   URI
	Details map[string]interface{}
}

func (msg *Hello) MessageType() MessageType {
	return MessageTypeHello
}

func (msg *Hello) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Realm, msg.Details}
}

// [WELCOME, Session|id, Details|dict]
type Welcome struct {
	sync.Mutex
	Id      ID
	Details map[string]interface{}
}

func (msg *Welcome) MessageType() MessageType {
	return MessageTypeWelcome
}

func (msg *Welcome) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Id, msg.Details}
}

// [ABORT, Details|dict, Reason|uri]
type Abort struct {
	sync.Mutex
	Details map[string]interface{}
	Reason  URI
}

func (msg *Abort) MessageType() MessageType {
	return MessageTypeAbort
}

func (msg *Abort) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Details, msg.Reason}
}

// [CHALLENGE, AuthMethod|string, Extra|dict]
type Challenge struct {
	sync.Mutex
	AuthMethod string
	Extra      map[string]interface{}
}

func (msg *Challenge) MessageType() MessageType {
	return MessageTypeChallenge
}

func (msg *Challenge) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.AuthMethod, msg.Extra}
}

// [AUTHENTICATE, Signature|string, Extra|dict]
type Authenticate struct {
	sync.Mutex
	Signature string
	Extra     map[string]interface{}
}

func (msg *Authenticate) MessageType() MessageType {
	return MessageTypeAuthenticate
}

func (msg *Authenticate) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Signature, msg.Extra}
}

// [GOODBYE, Details|dict, Reason|uri]
type Goodbye struct {
	sync.Mutex
	Details map[string]interface{}
	Reason  URI
}

func (msg *Goodbye) MessageType() MessageType {
	return MessageTypeGoodbye
}

func (msg *Goodbye) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Details, msg.Reason}
}

// [ERROR, REQUEST.Type|int, REQUEST.Request|id, Details|dict, Error|uri]
// [ERROR, REQUEST.Type|int, REQUEST.Request|id, Details|dict, Error|uri, Arguments|list]
// [ERROR, REQUEST.Type|int, REQUEST.Request|id, Details|dict, Error|uri, Arguments|list, ArgumentsKw|dict]
type Error struct {
	sync.Mutex
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

func (msg *Error) ToPayload() []interface{} {
	p := []interface{}{msg.MessageType(), msg.Type, msg.Details, msg.Details, msg.Error}
	if nil != msg.ArgumentsKw && 0 < len(msg.ArgumentsKw) {
		p = append(p, msg.Arguments, msg.ArgumentsKw)
	} else if nil != msg.Arguments && 0 < len(msg.Arguments) {
		p = append(p, msg.Arguments)
	}
	return p
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
	sync.Mutex  `json:"-"`
}

func (msg *Publish) MessageType() MessageType {
	return MessageTypePublish
}

func (msg *Publish) ToPayload() []interface{} {
	p := []interface{}{msg.MessageType(), msg.Request, msg.Options, msg.Topic}
	if nil != msg.ArgumentsKw && 0 < len(msg.ArgumentsKw) {
		p = append(p, msg.Arguments, msg.ArgumentsKw)
	} else if nil != msg.Arguments && 0 < len(msg.Arguments) {
		p = append(p, msg.Arguments)
	}
	return p
}

// [PUBLISHED, PUBLISH.Request|id, Publication|id]
type Published struct {
	Request     ID
	Publication ID
	sync.Mutex  `json:"-"`
}

func (msg *Published) MessageType() MessageType {
	return MessageTypePublished
}

func (msg *Published) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Request, msg.Publication}
}

// [SUBSCRIBE, Request|id, Options|dict, Topic|uri]
type Subscribe struct {
	Request    ID
	Options    map[string]interface{}
	Topic      URI
	sync.Mutex `json:"-"`
}

func (msg *Subscribe) MessageType() MessageType {
	return MessageTypeSubscribe
}

func (msg *Subscribe) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Request, msg.Options, msg.Topic}
}

// [SUBSCRIBED, SUBSCRIBE.Request|id, Subscription|id]
type Subscribed struct {
	Request      ID
	Subscription ID
	sync.Mutex   `json:"-"`
}

func (msg *Subscribed) MessageType() MessageType {
	return MessageTypeSubscribed
}

func (msg *Subscribed) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Request, msg.Subscription}
}

// [UNSUBSCRIBE, Request|id, SUBSCRIBED.Subscription|id]
type Unsubscribe struct {
	Request      ID
	Subscription ID
	sync.Mutex   `json:"-"`
}

func (msg *Unsubscribe) MessageType() MessageType {
	return MessageTypeUnsubscribe
}

func (msg *Unsubscribe) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Request, msg.Subscription}
}

// [UNSUBSCRIBED, UNSUBSCRIBE.Request|id]
type Unsubscribed struct {
	Request    ID
	sync.Mutex `json:"-"`
}

func (msg *Unsubscribed) MessageType() MessageType {
	return MessageTypeUnsubscribed
}

func (msg *Unsubscribed) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Request}
}

// [EVENT, SUBSCRIBED.Subscription|id, PUBLISHED.Publication|id, Details|dict]
// [EVENT, SUBSCRIBED.Subscription|id, PUBLISHED.Publication|id, Details|dict, PUBLISH.Arguments|list]
// [EVENT, SUBSCRIBED.Subscription|id, PUBLISHED.Publication|id, Details|dict, PUBLISH.Arguments|list, PUBLISH.ArgumentsKw|dict]
type Event struct {
	Subscription ID
	Publication  ID
	Details      map[string]interface{}
	Arguments    []interface{}          `wamp:"omitempty"`
	ArgumentsKw  map[string]interface{} `wamp:"omitempty"`
	sync.Mutex   `json:"-"`
}

func (msg *Event) MessageType() MessageType {
	return MessageTypeEvent
}

func (msg *Event) ToPayload() []interface{} {
	p := []interface{}{msg.MessageType(), msg.Subscription, msg.Publication, msg.Details}
	if nil != msg.ArgumentsKw && 0 < len(msg.ArgumentsKw) {
		p = append(p, msg.Arguments, msg.ArgumentsKw)
	} else if nil != msg.Arguments && 0 < len(msg.Arguments) {
		p = append(p, msg.Arguments)
	}
	return p
}

// CallResult represents the result of a CALL.
type CallResult struct {
	Args       []interface{}
	Kwargs     map[string]interface{}
	Err        URI
	sync.Mutex `json:"-"`
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
	sync.Mutex  `json:"-"`
}

func (msg *Call) MessageType() MessageType {
	return MessageTypeCall
}

func (msg *Call) ToPayload() []interface{} {
	p := []interface{}{msg.MessageType(), msg.Request, msg.Options, msg.Procedure}
	if nil != msg.ArgumentsKw && 0 < len(msg.ArgumentsKw) {
		p = append(p, msg.Arguments, msg.ArgumentsKw)
	} else if nil != msg.Arguments && 0 < len(msg.Arguments) {
		p = append(p, msg.Arguments)
	}
	return p
}

// [RESULT, CALL.Request|id, Details|dict]
// [RESULT, CALL.Request|id, Details|dict, YIELD.Arguments|list]
// [RESULT, CALL.Request|id, Details|dict, YIELD.Arguments|list, YIELD.ArgumentsKw|dict]
type Result struct {
	Request     ID
	Details     map[string]interface{}
	Arguments   []interface{}          `wamp:"omitempty"`
	ArgumentsKw map[string]interface{} `wamp:"omitempty"`
	sync.Mutex  `json:"-"`
}

func (msg *Result) MessageType() MessageType {
	return MessageTypeResult
}

func (msg *Result) ToPayload() []interface{} {
	p := []interface{}{msg.MessageType(), msg.Details}
	if nil != msg.ArgumentsKw && 0 < len(msg.ArgumentsKw) {
		p = append(p, msg.Arguments, msg.ArgumentsKw)
	} else if nil != msg.Arguments && 0 < len(msg.Arguments) {
		p = append(p, msg.Arguments)
	}
	return p
}

// [REGISTER, Request|id, Options|dict, Procedure|uri]
type Register struct {
	Request    ID
	Options    map[string]interface{}
	Procedure  URI
	sync.Mutex `json:"-"`
}

func (msg *Register) MessageType() MessageType {
	return MessageTypeRegister
}

func (msg *Register) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Request, msg.Options, msg.Procedure}
}

// [REGISTERED, REGISTER.Request|id, Registration|id]
type Registered struct {
	Request      ID
	Registration ID
	sync.Mutex   `json:"-"`
}

func (msg *Registered) MessageType() MessageType {
	return MessageTypeRegistered
}

func (msg *Registered) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Request, msg.Registration}
}

// [UNREGISTER, Request|id, REGISTERED.Registration|id]
type Unregister struct {
	Request      ID
	Registration ID
	sync.Mutex   `json:"-"`
}

func (msg *Unregister) MessageType() MessageType {
	return MessageTypeUnregister
}

func (msg *Unregister) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Request, msg.Registration}
}

// [UNREGISTERED, UNREGISTER.Request|id]
type Unregistered struct {
	Request    ID
	sync.Mutex `json:"-"`
}

func (msg *Unregistered) MessageType() MessageType {
	return MessageTypeUnregistered
}

func (msg *Unregistered) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Request}
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
	sync.Mutex   `json:"-"`
}

func (msg *Invocation) MessageType() MessageType {
	return MessageTypeInvocation
}

func (msg *Invocation) ToPayload() []interface{} {
	p := []interface{}{msg.MessageType(), msg.Request, msg.Registration, msg.Details}
	if nil != msg.ArgumentsKw && 0 < len(msg.ArgumentsKw) {
		p = append(p, msg.Arguments, msg.ArgumentsKw)
	} else if nil != msg.Arguments && 0 < len(msg.Arguments) {
		p = append(p, msg.Arguments)
	}
	return p
}

// [YIELD, INVOCATION.Request|id, Options|dict]
// [YIELD, INVOCATION.Request|id, Options|dict, Arguments|list]
// [YIELD, INVOCATION.Request|id, Options|dict, Arguments|list, ArgumentsKw|dict]
type Yield struct {
	Request     ID
	Options     map[string]interface{}
	Arguments   []interface{}          `wamp:"omitempty"`
	ArgumentsKw map[string]interface{} `wamp:"omitempty"`
	sync.Mutex  `json:"-"`
}

func (msg *Yield) MessageType() MessageType {
	return MessageTypeYield
}

func (msg *Yield) ToPayload() []interface{} {
	p := []interface{}{msg.MessageType(), msg.Request, msg.Options}
	if nil != msg.ArgumentsKw && 0 < len(msg.ArgumentsKw) {
		p = append(p, msg.Arguments, msg.ArgumentsKw)
	} else if nil != msg.Arguments && 0 < len(msg.Arguments) {
		p = append(p, msg.Arguments)
	}
	return p
}

// [CANCEL, CALL.Request|id, Options|dict]
type Cancel struct {
	Request    ID
	Options    map[string]interface{}
	sync.Mutex `json:"-"`
}

func (msg *Cancel) MessageType() MessageType {
	return MessageTypeCancel
}

func (msg *Cancel) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Request, msg.Options}
}

// [INTERRUPT, INVOCATION.Request|id, Options|dict]
type Interrupt struct {
	Request    ID
	Options    map[string]interface{}
	sync.Mutex `json:"-"`
}

func (msg *Interrupt) MessageType() MessageType {
	return MessageTypeInterrupt
}

func (msg *Interrupt) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Request, msg.Options}
}

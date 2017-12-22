package message

import (
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

	// Type values sourced from: http://wamp-proto.org/static/rfc/draft-oberstet-hybi-crossbar-wamp.html#rfc.section.6.5
	Type uint32

	// Message is a generic container for a WAMP message.
	Message interface {
		MessageType() Type
		ToPayload() []interface{}
	}

	// Extension is a generic container for WAMP message extensions
	Extension interface {
		Message
		TypeName() string
	}

	// ExtensionProviderFunc defines a function that will be called whenever a custom message is seen.
	ExtensionProviderFunc func(Type, ...interface{}) Extension
)

var (
	extensions = new(sync.Map)
)

const (
	TypeHello        Type = 1
	TypeWelcome      Type = 2
	TypeAbort        Type = 3
	TypeChallenge    Type = 4
	TypeAuthenticate Type = 5
	TypeGoodbye      Type = 6
	TypeError        Type = 8
	TypePublish      Type = 16
	TypePublished    Type = 17
	TypeSubscribe    Type = 32
	TypeSubscribed   Type = 33
	TypeUnsubscribe  Type = 34
	TypeUnsubscribed Type = 35
	TypeEvent        Type = 36
	TypeCall         Type = 48
	TypeCancel       Type = 49
	TypeResult       Type = 50
	TypeRegister     Type = 64
	TypeRegistered   Type = 65
	TypeUnregister   Type = 66
	TypeUnregistered Type = 67
	TypeInvocation   Type = 68
	TypeInterrupt    Type = 69
	TypeYield        Type = 70

	TypeExtensionMin Type = 256
	TypeExtensionMax Type = 1023
)

func (mt Type) New() Message {
	switch mt {
	case TypeHello:
		return new(Hello)
	case TypeWelcome:
		return new(Welcome)
	case TypeAbort:
		return new(Abort)
	case TypeChallenge:
		return new(Challenge)
	case TypeAuthenticate:
		return new(Authenticate)
	case TypeGoodbye:
		return new(Goodbye)
	case TypeError:
		return new(Error)

	case TypePublish:
		return new(Publish)
	case TypePublished:
		return new(Published)

	case TypeSubscribe:
		return new(Subscribe)
	case TypeSubscribed:
		return new(Subscribed)
	case TypeUnsubscribe:
		return new(Unsubscribe)
	case TypeUnsubscribed:
		return new(Unsubscribed)
	case TypeEvent:
		return new(Event)

	case TypeCall:
		return new(Call)
	case TypeCancel:
		return new(Cancel)
	case TypeResult:
		return new(Result)

	case TypeRegister:
		return new(Register)
	case TypeRegistered:
		return new(Registered)
	case TypeUnregister:
		return new(Unregister)
	case TypeUnregistered:
		return new(Unregistered)
	case TypeInvocation:
		return new(Invocation)
	case TypeInterrupt:
		return new(Interrupt)
	case TypeYield:
		return new(Yield)
	}

	if TypeExtensionMin <= mt && mt <= TypeExtensionMax {
		if f, ok := extensions.Load(mt); ok {
			return f.(ExtensionProviderFunc)(mt)
		}
		return nil
	} else {
		return nil
	}
}

func (mt Type) String() string {
	switch mt {
	case TypeHello:
		return "HELLO"
	case TypeWelcome:
		return "WELCOME"
	case TypeAbort:
		return "ABORT"
	case TypeChallenge:
		return "CHALLENGE"
	case TypeAuthenticate:
		return "AUTHENTICATE"
	case TypeGoodbye:
		return "GOODBYE"
	case TypeError:
		return "ERROR"

	case TypePublish:
		return "PUBLISH"
	case TypePublished:
		return "PUBLISHED"

	case TypeSubscribe:
		return "SUBSCRIBE"
	case TypeSubscribed:
		return "SUBSCRIBED"
	case TypeUnsubscribe:
		return "UNSUBSCRIBE"
	case TypeUnsubscribed:
		return "UNSUBSCRIBED"
	case TypeEvent:
		return "EVENT"

	case TypeCall:
		return "CALL"
	case TypeCancel:
		return "CANCEL"
	case TypeResult:
		return "RESULT"

	case TypeRegister:
		return "REGISTER"
	case TypeRegistered:
		return "REGISTERED"
	case TypeUnregister:
		return "UNREGISTER"
	case TypeUnregistered:
		return "UNREGISTERED"
	case TypeInvocation:
		return "INVOCATION"
	case TypeInterrupt:
		return "INTERRUPT"
	case TypeYield:
		return "YIELD"
	}

	if TypeExtensionMin <= mt && mt <= TypeExtensionMax {
		if f, ok := extensions.Load(mt); ok {
			return f.(ExtensionProviderFunc)(mt).TypeName()
		}
		return "UNKNOWN"
	} else {
		return "UNKNOWN"
	}
}

// [HELLO, Realm|uri, Details|dict]
type Hello struct {
	Realm   URI
	Details map[string]interface{}
}

func (Hello) MessageType() Type {
	return TypeHello
}

func (msg Hello) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Realm, msg.Details}
}

// [WELCOME, Session|id, Details|dict]
type Welcome struct {
	ID      ID
	Details map[string]interface{}
}

func (Welcome) MessageType() Type {
	return TypeWelcome
}

func (msg Welcome) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.ID, msg.Details}
}

// [ABORT, Details|dict, Reason|uri]
type Abort struct {
	Details map[string]interface{}
	Reason  URI
}

func (Abort) MessageType() Type {
	return TypeAbort
}

func (msg Abort) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Details, msg.Reason}
}

// [CHALLENGE, AuthMethod|string, Extra|dict]
type Challenge struct {
	AuthMethod string
	Extra      map[string]interface{}
}

func (Challenge) MessageType() Type {
	return TypeChallenge
}

func (msg Challenge) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.AuthMethod, msg.Extra}
}

// [AUTHENTICATE, Signature|string, Extra|dict]
type Authenticate struct {
	Signature string
	Extra     map[string]interface{}
}

func (Authenticate) MessageType() Type {
	return TypeAuthenticate
}

func (msg Authenticate) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Signature, msg.Extra}
}

// [GOODBYE, Details|dict, Reason|uri]
type Goodbye struct {
	Details map[string]interface{}
	Reason  URI
}

func (Goodbye) MessageType() Type {
	return TypeGoodbye
}

func (msg Goodbye) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Details, msg.Reason}
}

// [ERROR, REQUEST.Type|int, REQUEST.Request|id, Details|dict, Error|uri]
// [ERROR, REQUEST.Type|int, REQUEST.Request|id, Details|dict, Error|uri, Arguments|list]
// [ERROR, REQUEST.Type|int, REQUEST.Request|id, Details|dict, Error|uri, Arguments|list, ArgumentsKw|dict]
type Error struct {
	Type        Type
	Request     ID
	Details     map[string]interface{}
	Error       URI
	Arguments   []interface{}          `wamp:"omitempty"`
	ArgumentsKw map[string]interface{} `wamp:"omitempty"`
}

func (Error) MessageType() Type {
	return TypeError
}

func (msg Error) ToPayload() []interface{} {
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
}

func (Publish) MessageType() Type {
	return TypePublish
}

func (msg Publish) ToPayload() []interface{} {
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
}

func (Published) MessageType() Type {
	return TypePublished
}

func (msg Published) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Request, msg.Publication}
}

// [SUBSCRIBE, Request|id, Options|dict, Topic|uri]
type Subscribe struct {
	Request ID
	Options map[string]interface{}
	Topic   URI
}

func (Subscribe) MessageType() Type {
	return TypeSubscribe
}

func (msg Subscribe) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Request, msg.Options, msg.Topic}
}

// [SUBSCRIBED, SUBSCRIBE.Request|id, Subscription|id]
type Subscribed struct {
	Request      ID
	Subscription ID
}

func (Subscribed) MessageType() Type {
	return TypeSubscribed
}

func (msg Subscribed) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Request, msg.Subscription}
}

// [UNSUBSCRIBE, Request|id, SUBSCRIBED.Subscription|id]
type Unsubscribe struct {
	Request      ID
	Subscription ID
}

func (Unsubscribe) MessageType() Type {
	return TypeUnsubscribe
}

func (msg Unsubscribe) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Request, msg.Subscription}
}

// [UNSUBSCRIBED, UNSUBSCRIBE.Request|id]
type Unsubscribed struct {
	Request ID
}

func (Unsubscribed) MessageType() Type {
	return TypeUnsubscribed
}

func (msg Unsubscribed) ToPayload() []interface{} {
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
}

func (Event) MessageType() Type {
	return TypeEvent
}

func (msg Event) ToPayload() []interface{} {
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

func (Call) MessageType() Type {
	return TypeCall
}

func (msg Call) ToPayload() []interface{} {
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
}

func (Result) MessageType() Type {
	return TypeResult
}

func (msg Result) ToPayload() []interface{} {
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
	Request   ID
	Options   map[string]interface{}
	Procedure URI
}

func (Register) MessageType() Type {
	return TypeRegister
}

func (msg Register) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Request, msg.Options, msg.Procedure}
}

// [REGISTERED, REGISTER.Request|id, Registration|id]
type Registered struct {
	Request      ID
	Registration ID
}

func (Registered) MessageType() Type {
	return TypeRegistered
}

func (msg Registered) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Request, msg.Registration}
}

// [UNREGISTER, Request|id, REGISTERED.Registration|id]
type Unregister struct {
	Request      ID
	Registration ID
}

func (Unregister) MessageType() Type {
	return TypeUnregister
}

func (msg Unregister) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Request, msg.Registration}
}

// [UNREGISTERED, UNREGISTER.Request|id]
type Unregistered struct {
	Request ID
}

func (Unregistered) MessageType() Type {
	return TypeUnregistered
}

func (msg Unregistered) ToPayload() []interface{} {
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
}

func (Invocation) MessageType() Type {
	return TypeInvocation
}

func (msg Invocation) ToPayload() []interface{} {
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
}

func (Yield) MessageType() Type {
	return TypeYield
}

func (msg Yield) ToPayload() []interface{} {
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
	Request ID
	Options map[string]interface{}
}

func (Cancel) MessageType() Type {
	return TypeCancel
}

func (msg Cancel) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Request, msg.Options}
}

// [INTERRUPT, INVOCATION.Request|id, Options|dict]
type Interrupt struct {
	Request ID
	Options map[string]interface{}
}

func (Interrupt) MessageType() Type {
	return TypeInterrupt
}

func (msg Interrupt) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Request, msg.Options}
}

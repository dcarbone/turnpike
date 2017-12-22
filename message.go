package turnpike

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
	MessageType uint32

	// Message is a generic container for a WAMP message.
	Message interface {
		MessageType() MessageType
		ToPayload() []interface{}
	}

	// MessageExtension is a generic container for WAMP message extensions
	MessageExtension interface {
		Message
		TypeName() string
	}

	// MessageExtensionProviderFunc defines a function that will be called whenever a custom message is seen.
	MessageExtensionProviderFunc func(MessageType, ...interface{}) MessageExtension
)

var (
	extensions = new(sync.Map)
)

const (
	MessageTypeHello        MessageType = 1
	MessageTypeWelcome      MessageType = 2
	MessageTypeAbort        MessageType = 3
	MessageTypeChallenge    MessageType = 4
	MessageTypeAuthenticate MessageType = 5
	MessageTypeGoodbye      MessageType = 6
	MessageTypeError        MessageType = 8
	MessageTypePublish      MessageType = 16
	MessageTypePublished    MessageType = 17
	MessageTypeSubscribe    MessageType = 32
	MessageTypeSubscribed   MessageType = 33
	MessageTypeUnsubscribe  MessageType = 34
	MessageTypeUnsubscribed MessageType = 35
	MessageTypeEvent        MessageType = 36
	MessageTypeCall         MessageType = 48
	MessageTypeCancel       MessageType = 49
	MessageTypeResult       MessageType = 50
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
		return new(MessageHello)
	case MessageTypeWelcome:
		return new(MessageWelcome)
	case MessageTypeAbort:
		return new(MessageAbort)
	case MessageTypeChallenge:
		return new(MessageChallenge)
	case MessageTypeAuthenticate:
		return new(MessageAuthenticate)
	case MessageTypeGoodbye:
		return new(MessageGoodbye)
	case MessageTypeError:
		return new(MessageError)

	case MessageTypePublish:
		return new(MessagePublish)
	case MessageTypePublished:
		return new(MessagePublished)

	case MessageTypeSubscribe:
		return new(MessageSubscribe)
	case MessageTypeSubscribed:
		return new(MessageSubscribed)
	case MessageTypeUnsubscribe:
		return new(MessageUnsubscribe)
	case MessageTypeUnsubscribed:
		return new(MessageUnsubscribed)
	case MessageTypeEvent:
		return new(MessageEvent)

	case MessageTypeCall:
		return new(MessageCall)
	case MessageTypeCancel:
		return new(MessageCancel)
	case MessageTypeResult:
		return new(MessageResult)

	case MessageTypeRegister:
		return new(MessageRegister)
	case MessageTypeRegistered:
		return new(MessageRegistered)
	case MessageTypeUnregister:
		return new(MessageUnregister)
	case MessageTypeUnregistered:
		return new(MessageUnregistered)
	case MessageTypeInvocation:
		return new(MessageInvocation)
	case MessageTypeInterrupt:
		return new(MessageInterrupt)
	case MessageTypeYield:
		return new(MessageYield)
	}

	if MessageTypeExtensionMin <= mt && mt <= MessageTypeExtensionMax {
		if f, ok := extensions.Load(mt); ok {
			return f.(MessageExtensionProviderFunc)(mt)
		}
		return nil
	} else {
		return nil
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
	}

	if MessageTypeExtensionMin <= mt && mt <= MessageTypeExtensionMax {
		if f, ok := extensions.Load(mt); ok {
			return f.(MessageExtensionProviderFunc)(mt).TypeName()
		}
		return "UNKNOWN"
	} else {
		return "UNKNOWN"
	}
}

// [MessageHello, Realm|uri, Details|dict]
type MessageHello struct {
	Realm   URI
	Details map[string]interface{}
}

func (MessageHello) MessageType() MessageType {
	return MessageTypeHello
}

func (msg MessageHello) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Realm, msg.Details}
}

// [MessageWelcome, Session|id, Details|dict]
type MessageWelcome struct {
	ID      ID
	Details map[string]interface{}
}

func (MessageWelcome) MessageType() MessageType {
	return MessageTypeWelcome
}

func (msg MessageWelcome) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.ID, msg.Details}
}

// [MessageAbort, Details|dict, Reason|uri]
type MessageAbort struct {
	Details map[string]interface{}
	Reason  URI
}

func (MessageAbort) MessageType() MessageType {
	return MessageTypeAbort
}

func (msg MessageAbort) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Details, msg.Reason}
}

// [MessageChallenge, AuthMethod|string, Extra|dict]
type MessageChallenge struct {
	AuthMethod string
	Extra      map[string]interface{}
}

func (MessageChallenge) MessageType() MessageType {
	return MessageTypeChallenge
}

func (msg MessageChallenge) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.AuthMethod, msg.Extra}
}

// [MessageAuthenticate, Signature|string, Extra|dict]
type MessageAuthenticate struct {
	Signature string
	Extra     map[string]interface{}
}

func (MessageAuthenticate) MessageType() MessageType {
	return MessageTypeAuthenticate
}

func (msg MessageAuthenticate) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Signature, msg.Extra}
}

// [MessageGoodbye, Details|dict, Reason|uri]
type MessageGoodbye struct {
	Details map[string]interface{}
	Reason  URI
}

func (MessageGoodbye) MessageType() MessageType {
	return MessageTypeGoodbye
}

func (msg MessageGoodbye) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Details, msg.Reason}
}

// [MessageError, REQUEST.Type|int, REQUEST.Request|id, Details|dict, Error|uri]
// [MessageError, REQUEST.Type|int, REQUEST.Request|id, Details|dict, Error|uri, Arguments|list]
// [MessageError, REQUEST.Type|int, REQUEST.Request|id, Details|dict, Error|uri, Arguments|list, ArgumentsKw|dict]
type MessageError struct {
	Type        MessageType
	Request     ID
	Details     map[string]interface{}
	Error       URI
	Arguments   []interface{}          `wamp:"omitempty"`
	ArgumentsKw map[string]interface{} `wamp:"omitempty"`
}

func (MessageError) MessageType() MessageType {
	return MessageTypeError
}

func (msg MessageError) ToPayload() []interface{} {
	p := []interface{}{msg.MessageType(), msg.Type, msg.Details, msg.Details, msg.Error}
	if nil != msg.ArgumentsKw && 0 < len(msg.ArgumentsKw) {
		p = append(p, msg.Arguments, msg.ArgumentsKw)
	} else if nil != msg.Arguments && 0 < len(msg.Arguments) {
		p = append(p, msg.Arguments)
	}
	return p
}

// [MessagePublish, Request|id, Options|dict, Topic|uri]
// [MessagePublish, Request|id, Options|dict, Topic|uri, Arguments|list]
// [MessagePublish, Request|id, Options|dict, Topic|uri, Arguments|list, ArgumentsKw|dict]
type MessagePublish struct {
	Request     ID
	Options     map[string]interface{}
	Topic       URI
	Arguments   []interface{}          `wamp:"omitempty"`
	ArgumentsKw map[string]interface{} `wamp:"omitempty"`
}

func (MessagePublish) MessageType() MessageType {
	return MessageTypePublish
}

func (msg MessagePublish) ToPayload() []interface{} {
	p := []interface{}{msg.MessageType(), msg.Request, msg.Options, msg.Topic}
	if nil != msg.ArgumentsKw && 0 < len(msg.ArgumentsKw) {
		p = append(p, msg.Arguments, msg.ArgumentsKw)
	} else if nil != msg.Arguments && 0 < len(msg.Arguments) {
		p = append(p, msg.Arguments)
	}
	return p
}

// [MessagePublished, MessagePublish.Request|id, Publication|id]
type MessagePublished struct {
	Request     ID
	Publication ID
}

func (MessagePublished) MessageType() MessageType {
	return MessageTypePublished
}

func (msg MessagePublished) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Request, msg.Publication}
}

// [MessageSubscribe, Request|id, Options|dict, Topic|uri]
type MessageSubscribe struct {
	Request ID
	Options map[string]interface{}
	Topic   URI
}

func (MessageSubscribe) MessageType() MessageType {
	return MessageTypeSubscribe
}

func (msg MessageSubscribe) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Request, msg.Options, msg.Topic}
}

// [MessageSubscribed, MessageSubscribe.Request|id, Subscription|id]
type MessageSubscribed struct {
	Request      ID
	Subscription ID
}

func (MessageSubscribed) MessageType() MessageType {
	return MessageTypeSubscribed
}

func (msg MessageSubscribed) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Request, msg.Subscription}
}

// [MessageUnsubscribe, Request|id, MessageSubscribed.Subscription|id]
type MessageUnsubscribe struct {
	Request      ID
	Subscription ID
}

func (MessageUnsubscribe) MessageType() MessageType {
	return MessageTypeUnsubscribe
}

func (msg MessageUnsubscribe) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Request, msg.Subscription}
}

// [MessageUnsubscribed, MessageUnsubscribed.Request|id]
type MessageUnsubscribed struct {
	Request ID
}

func (MessageUnsubscribed) MessageType() MessageType {
	return MessageTypeUnsubscribed
}

func (msg MessageUnsubscribed) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Request}
}

// [MessageEvent, MessageSubscribed.Subscription|id, MessagePublished.Publication|id, Details|dict]
// [MessageEvent, MessageSubscribed.Subscription|id, MessagePublished.Publication|id, Details|dict, MessagePublish.Arguments|list]
// [MessageEvent, MessageSubscribed.Subscription|id, MessagePublished.Publication|id, Details|dict, MessagePublish.Arguments|list, MessagePublish.ArgumentsKw|dict]
type MessageEvent struct {
	Subscription ID
	Publication  ID
	Details      map[string]interface{}
	Arguments    []interface{}          `wamp:"omitempty"`
	ArgumentsKw  map[string]interface{} `wamp:"omitempty"`
}

func (MessageEvent) MessageType() MessageType {
	return MessageTypeEvent
}

func (msg MessageEvent) ToPayload() []interface{} {
	p := []interface{}{msg.MessageType(), msg.Subscription, msg.Publication, msg.Details}
	if nil != msg.ArgumentsKw && 0 < len(msg.ArgumentsKw) {
		p = append(p, msg.Arguments, msg.ArgumentsKw)
	} else if nil != msg.Arguments && 0 < len(msg.Arguments) {
		p = append(p, msg.Arguments)
	}
	return p
}

// CallResult represents the result of a MessageCall.
type CallResult struct {
	Args   []interface{}
	Kwargs map[string]interface{}
	Err    URI
}

// [MessageCall, Request|id, Options|dict, Procedure|uri]
// [MessageCall, Request|id, Options|dict, Procedure|uri, Arguments|list]
// [MessageCall, Request|id, Options|dict, Procedure|uri, Arguments|list, ArgumentsKw|dict]
type MessageCall struct {
	Request     ID
	Options     map[string]interface{}
	Procedure   URI
	Arguments   []interface{}          `wamp:"omitempty"`
	ArgumentsKw map[string]interface{} `wamp:"omitempty"`
}

func (MessageCall) MessageType() MessageType {
	return MessageTypeCall
}

func (msg MessageCall) ToPayload() []interface{} {
	p := []interface{}{msg.MessageType(), msg.Request, msg.Options, msg.Procedure}
	if nil != msg.ArgumentsKw && 0 < len(msg.ArgumentsKw) {
		p = append(p, msg.Arguments, msg.ArgumentsKw)
	} else if nil != msg.Arguments && 0 < len(msg.Arguments) {
		p = append(p, msg.Arguments)
	}
	return p
}

// [MessageResult, MessageCall.Request|id, Details|dict]
// [MessageResult, MessageCall.Request|id, Details|dict, MessageYield.Arguments|list]
// [MessageResult, MessageCall.Request|id, Details|dict, MessageYield.Arguments|list, MessageYield.ArgumentsKw|dict]
type MessageResult struct {
	Request     ID
	Details     map[string]interface{}
	Arguments   []interface{}          `wamp:"omitempty"`
	ArgumentsKw map[string]interface{} `wamp:"omitempty"`
}

func (MessageResult) MessageType() MessageType {
	return MessageTypeResult
}

func (msg MessageResult) ToPayload() []interface{} {
	p := []interface{}{msg.MessageType(), msg.Details}
	if nil != msg.ArgumentsKw && 0 < len(msg.ArgumentsKw) {
		p = append(p, msg.Arguments, msg.ArgumentsKw)
	} else if nil != msg.Arguments && 0 < len(msg.Arguments) {
		p = append(p, msg.Arguments)
	}
	return p
}

// [MessageRegister, Request|id, Options|dict, Procedure|uri]
type MessageRegister struct {
	Request   ID
	Options   map[string]interface{}
	Procedure URI
}

func (MessageRegister) MessageType() MessageType {
	return MessageTypeRegister
}

func (msg MessageRegister) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Request, msg.Options, msg.Procedure}
}

// [MessageRegistered, MessageRegister.Request|id, Registration|id]
type MessageRegistered struct {
	Request      ID
	Registration ID
}

func (MessageRegistered) MessageType() MessageType {
	return MessageTypeRegistered
}

func (msg MessageRegistered) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Request, msg.Registration}
}

// [MessageUnregister, Request|id, MessageRegistered.Registration|id]
type MessageUnregister struct {
	Request      ID
	Registration ID
}

func (MessageUnregister) MessageType() MessageType {
	return MessageTypeUnregister
}

func (msg MessageUnregister) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Request, msg.Registration}
}

// [MessageUnregistered, MessageUnregister.Request|id]
type MessageUnregistered struct {
	Request ID
}

func (MessageUnregistered) MessageType() MessageType {
	return MessageTypeUnregistered
}

func (msg MessageUnregistered) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Request}
}

// [MessageInvocation, Request|id, MessageRegistered.Registration|id, Details|dict]
// [MessageInvocation, Request|id, MessageRegistered.Registration|id, Details|dict, MessageCall.Arguments|list]
// [MessageInvocation, Request|id, MessageRegistered.Registration|id, Details|dict, MessageCall.Arguments|list, MessageCall.ArgumentsKw|dict]
type MessageInvocation struct {
	Request      ID
	Registration ID
	Details      map[string]interface{}
	Arguments    []interface{}          `wamp:"omitempty"`
	ArgumentsKw  map[string]interface{} `wamp:"omitempty"`
}

func (MessageInvocation) MessageType() MessageType {
	return MessageTypeInvocation
}

func (msg MessageInvocation) ToPayload() []interface{} {
	p := []interface{}{msg.MessageType(), msg.Request, msg.Registration, msg.Details}
	if nil != msg.ArgumentsKw && 0 < len(msg.ArgumentsKw) {
		p = append(p, msg.Arguments, msg.ArgumentsKw)
	} else if nil != msg.Arguments && 0 < len(msg.Arguments) {
		p = append(p, msg.Arguments)
	}
	return p
}

// [MessageYield, MessageInvocation.Request|id, Options|dict]
// [MessageYield, MessageInvocation.Request|id, Options|dict, Arguments|list]
// [MessageYield, MessageInvocation.Request|id, Options|dict, Arguments|list, ArgumentsKw|dict]
type MessageYield struct {
	Request     ID
	Options     map[string]interface{}
	Arguments   []interface{}          `wamp:"omitempty"`
	ArgumentsKw map[string]interface{} `wamp:"omitempty"`
}

func (MessageYield) MessageType() MessageType {
	return MessageTypeYield
}

func (msg MessageYield) ToPayload() []interface{} {
	p := []interface{}{msg.MessageType(), msg.Request, msg.Options}
	if nil != msg.ArgumentsKw && 0 < len(msg.ArgumentsKw) {
		p = append(p, msg.Arguments, msg.ArgumentsKw)
	} else if nil != msg.Arguments && 0 < len(msg.Arguments) {
		p = append(p, msg.Arguments)
	}
	return p
}

// [MessageCancel, MessageCall.Request|id, Options|dict]
type MessageCancel struct {
	Request ID
	Options map[string]interface{}
}

func (MessageCancel) MessageType() MessageType {
	return MessageTypeCancel
}

func (msg MessageCancel) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Request, msg.Options}
}

// [MessageInterrupt, MessageInvocation.Request|id, Options|dict]
type MessageInterrupt struct {
	Request ID
	Options map[string]interface{}
}

func (MessageInterrupt) MessageType() MessageType {
	return MessageTypeInterrupt
}

func (msg MessageInterrupt) ToPayload() []interface{} {
	return []interface{}{msg.MessageType(), msg.Request, msg.Options}
}

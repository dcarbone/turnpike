package turnpike

import (
	"fmt"
)

type WebSocketProtocol string

const (
	WebSocketProtocolJSON    WebSocketProtocol = "wamp.2.json"
	WebSocketProtocolMSGPack WebSocketProtocol = "wamp.2.msgpack"
)

type invalidPayload byte

func (e invalidPayload) Error() string {
	return fmt.Sprintf("Invalid payloadType: %d", e)
}

type protocolExists string

func (e protocolExists) Error() string {
	return "This protocol has already been registered: " + string(e)
}

type webSocketProtocol struct {
	payloadType int
	serializer  Serializer
}

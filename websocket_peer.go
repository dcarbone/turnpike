package turnpike

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type webSocketPeer struct {
	conn        *websocket.Conn
	serializer  Serializer
	messages    chan Message
	payloadType int
	closed      bool
	sendMutex   sync.Mutex
}

func NewWebSocketPeer(serialization Serialization, url string, tlscfg *tls.Config, dial DialFunc) (Peer, error) {
	var serializer Serializer
	var payloadType int
	var protocol string

	switch serialization {
	case JSON:
		serializer = new(JSONSerializer)
		payloadType = websocket.TextMessage
		protocol = jsonWebsocketProtocol
	case MSGPACK:
		serializer = new(MessagePackSerializer)
		payloadType = websocket.BinaryMessage
		protocol = msgpackWebsocketProtocol
	default:
		return nil, fmt.Errorf("Unsupported serialization: %v", serialization)
	}

	dialer := websocket.Dialer{
		Subprotocols:    []string{protocol},
		TLSClientConfig: tlscfg,
		Proxy:           http.ProxyFromEnvironment,
		NetDial:         dial,
	}
	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}

	return newWebSocketPeer(serializer, payloadType, conn)
}

func newWebSocketPeer(serializer Serializer, payloadType int, conn *websocket.Conn) (Peer, error) {
	ep := &webSocketPeer{
		conn:        conn,
		messages:    make(chan Message, 10),
		serializer:  serializer,
		payloadType: payloadType,
	}

	go ep.run()

	return ep, nil
}

// TODO: make this just add the message to a channel so we don't block
func (ep *webSocketPeer) Send(msg Message) error {
	b, err := ep.serializer.Serialize(msg)
	if err != nil {
		return err
	}
	ep.sendMutex.Lock()
	defer ep.sendMutex.Unlock()
	return ep.conn.WriteMessage(ep.payloadType, b)
}
func (ep *webSocketPeer) Receive() <-chan Message {
	return ep.messages
}
func (ep *webSocketPeer) Close() error {
	closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "goodbye")
	err := ep.conn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(5*time.Second))
	if err != nil {
		log.Println("error sending close message:", err)
	}
	ep.closed = true
	return ep.conn.Close()
}

func (ep *webSocketPeer) run() {
	for {
		// TODO: use conn.NextMessage() and stream
		// TODO: do something different based on binary/text frames
		if msgType, b, err := ep.conn.ReadMessage(); err != nil {
			if ep.closed {
				log.Println("peer connection closed")
			} else {
				log.Println("error reading from peer:", err)
				ep.conn.Close()
			}
			close(ep.messages)
			break
		} else if msgType == websocket.CloseMessage {
			ep.conn.Close()
			close(ep.messages)
			break
		} else {
			msg, err := ep.serializer.Deserialize(b)
			if err != nil {
				log.Println("error deserializing peer message:", err)
				// TODO: handle error
			} else {
				ep.messages <- msg
			}
		}
	}
}

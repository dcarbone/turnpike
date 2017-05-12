package turnpike

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"sync"
	"time"

	"context"
	"errors"
	"github.com/gorilla/websocket"
)

type webSocketPeer struct {
	ctx       context.Context
	ctxCancel context.CancelFunc

	closedLock sync.RWMutex
	closed     bool

	conn       *websocket.Conn
	serializer Serializer

	incomingMessages chan Message
	outgoingMessages chan Message

	payloadType int
}

func NewWebSocketPeer(ctx context.Context, serialization Serialization, url string, tlscfg *tls.Config, dial DialFunc) (Peer, error) {
	switch serialization {
	case JSON:
		return newWebSocketPeer(ctx, url, jsonWebsocketProtocol,
			new(JSONSerializer), websocket.TextMessage, tlscfg, dial,
		)
	case MSGPACK:
		return newWebSocketPeer(ctx, url, msgpackWebsocketProtocol,
			new(MessagePackSerializer), websocket.BinaryMessage, tlscfg, dial,
		)
	default:
		return nil, fmt.Errorf("Unsupported serialization: %v", serialization)
	}
}

func newWebSocketPeer(ctx context.Context, url, protocol string, serializer Serializer, payloadType int, tlsCfg *tls.Config, dial DialFunc) (Peer, error) {
	dialer := websocket.Dialer{
		Subprotocols:    []string{protocol},
		TLSClientConfig: tlsCfg,
		Proxy:           http.ProxyFromEnvironment,
		NetDial:         dial,
	}

	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}

	ep := &webSocketPeer{
		conn:             conn,
		incomingMessages: make(chan Message, 10),
		outgoingMessages: make(chan Message, 10),
		serializer:       serializer,
		payloadType:      payloadType,
	}

	ep.ctx, ep.ctxCancel = context.WithCancel(ctx)

	go ep.run()

	return ep, nil
}

// TODO: make this just add the message to a channel so we don't block
func (ep *webSocketPeer) Send(msg Message) error {
	ep.closedLock.RLock()
	defer ep.closedLock.RUnlock()

	if ep.closed {
		return errors.New("Peer is closed")
	}

	ep.outgoingMessages <- msg

	return nil
}
func (ep *webSocketPeer) Receive() <-chan Message {
	return ep.incomingMessages
}

func (ep *webSocketPeer) Close() error {
	ep.closedLock.RLock()
	defer ep.closedLock.RUnlock()

	if ep.closed {
		return errors.New("Peer is already closed.")
	}

	closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "goodbye")
	err := ep.conn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(5*time.Second))
	if err != nil {
		log.Println("error sending close message:", err)
	}

	ep.ctxCancel()

	return err
}

func (ep *webSocketPeer) run() {

	go func() {
		//var msgType int
		//var payload []byte
		//var err error
		//
		//for {
		//	// read from connection...
		//	msgType, payload, err := ep.conn.ReadMessage()
		//	if nil != err {
		//		// if we saw an error
		//		if ep.closed {
		//			// and the peer was closed
		//
		//		}
		//		ep.closedLock.RUnlock()
		//		return
		//	}
		//
		//	// TODO: use conn.NextMessage() and stream
		//	// TODO: do something different based on binary/text frames
		//	if msgType, b, err := ep.conn.ReadMessage(); err != nil {
		//		if ep.closed {
		//			log.Println("peer connection closed")
		//		} else {
		//			log.Println("error reading from peer:", err)
		//			ep.conn.Close()
		//		}
		//		close(ep.incomingMessages)
		//		break
		//	}
		//
		//	if msgType == websocket.CloseMessage {
		//		ep.closedPolitely = true
		//		ep.conn.Close()
		//		close(ep.incomingMessages)
		//		break
		//	} else {
		//		msg, err := ep.serializer.Deserialize(b)
		//		if err != nil {
		//			log.Println("error deserializing peer message:", err)
		//			// TODO: handle error
		//		} else {
		//			ep.incomingMessages <- msg
		//		}
		//	}
		//}
	}()

	for {
		select {
		case <-ep.ctx.Done():
			log.Printf("Context closed: %s", ep.ctx.Err())

			ep.closedLock.Lock()
			ep.closed = true
			ep.closedLock.Unlock()

			close(ep.outgoingMessages)
			close(ep.incomingMessages)

			err := ep.conn.Close()
			if nil != err {
			}

			return

		case msg := <-ep.outgoingMessages:
			if b, err := ep.serializer.Serialize(msg); nil == err {
				err = ep.conn.WriteMessage(ep.payloadType, b)
				if nil != err {
					log.Printf("Unable to write message \"%+v\": %s", msg, err)
				}
			} else {
				log.Printf("Unable to serialize message \"%+v\": %s", msg, err)
			}
		}
	}

}

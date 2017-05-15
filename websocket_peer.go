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

func (ep *webSocketPeer) Send(msg Message) error {
	ep.closedLock.RLock()
	defer ep.closedLock.RUnlock()

	// has peer already been closed?
	if ep.closed {
		return errors.New("Peer is closed")
	}

	select {
	// was context cancelled?
	case <-msg.Context().Done():
		return fmt.Errorf("Unable to send \"%s\", context has been closed: %s", msg.MessageType(), msg.Context().Err())

	// push message to outgoing queue
	case ep.outgoingMessages <- msg:
	}

	return nil
}
func (ep *webSocketPeer) Receive(t time.Duration) (Message, error) {
	ep.closedLock.RLock()
	defer ep.closedLock.RUnlock()
	if ep.closed {
		return nil, errors.New("Peer is closed")
	}

	if 0 == int64(t) {
		t = 1 * time.Second
	}

	ctx, _ := context.WithDeadline(ep.ctx, time.Now().Add(t))

	select {
	case msg, ok := <-ep.incomingMessages:
		if !ok {
			ep.Close()
			return nil, errors.New("session lost")
		}
		return msg, nil

	case <-ctx.Done():
		return nil, ErrPeerReceiveTimeout
	}
}

func (ep *webSocketPeer) Close() error {
	ep.closedLock.RLock()
	defer ep.closedLock.RUnlock()

	// has peer already been closed?
	if ep.closed {
		return errors.New("Peer is already closed.")
	}

	// cancel our context
	ep.ctxCancel()

	return nil
}

func (ep *webSocketPeer) run() {
	for {
		select {
		case <-ep.ctx.Done():
			// close us
			ep.closedLock.Lock()
			ep.closed = true
			ep.closedLock.Unlock()

			// close our channels
			close(ep.outgoingMessages)
			close(ep.incomingMessages)

			// construct close message
			closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "goodbye")

			// send with 1 second timeout
			err := ep.conn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(1*time.Second))
			if err != nil {
				log.Println("error sending close message:", err)
			}

			// attempt to close connection
			err = ep.conn.Close()
			if nil != err {
				log.Println("error closing connection: %s", err)
			}

			return

		case msg := <-ep.outgoingMessages:
			// attempt to serialize outgoing message
			if b, err := ep.serializer.Serialize(msg); nil == err {
				// attempt to send
				err = ep.conn.WriteMessage(ep.payloadType, b)
				if nil != err {
					// TODO: handle error
					log.Printf("Unable to write message \"%+v\": %s", msg, err)
				}
			} else {
				// TODO: Handle error?
				log.Printf("Unable to serialize message \"%+v\": %s", msg, err)
			}
		}
	}

}

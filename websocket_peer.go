package turnpike

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"net/http"
	"sync"
	"time"
)

type webSocketPeer struct {
	ctx context.Context

	conn *websocket.Conn

	closed     bool
	closedLock sync.RWMutex

	serializer  Serializer
	payloadType int

	in  chan Message
	out chan Message
}

// TODO: Hate this.  Change.
func NewWebSocketPeer(serialization SerializationFormat, url string, tlscfg *tls.Config, dial DialFunc) (Peer, error) {
	var serializer Serializer
	var payloadType int
	var protocol WebSocketProtocol

	switch serialization {
	case SerializationFormatJSON:
		serializer = new(JSONSerializer)
		payloadType = websocket.TextMessage
		protocol = WebSocketProtocolJSON
	case SerializationFormatMSGPack:
		serializer = new(MessagePackSerializer)
		payloadType = websocket.BinaryMessage
		protocol = WebSocketProtocolMSGPack
	default:
		return nil, fmt.Errorf("Unsupported serialization: %v", serialization)
	}

	dialer := websocket.Dialer{
		Subprotocols:    []string{string(protocol)},
		TLSClientConfig: tlscfg,
		Proxy:           http.ProxyFromEnvironment,
		NetDial:         dial,
	}

	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}

	return NewPeer(serializer, payloadType, conn), nil
}

func NewPeer(serializer Serializer, payloadType int, conn *websocket.Conn) Peer {
	ep := &webSocketPeer{
		ctx:         context.Background(),
		conn:        conn,
		in:          make(chan Message, 100),
		out:         make(chan Message, 100),
		serializer:  serializer,
		payloadType: payloadType,
	}

	go ep.read()
	go ep.write()

	return ep
}

func (ep *webSocketPeer) Send(msg Message) error {
	ep.closedLock.RLock()
	defer ep.closedLock.RUnlock()

	if ep.closed {
		return errors.New("Peer is closed")
	}

	ep.out <- msg

	return nil
}

func (ep *webSocketPeer) Receive() <-chan Message {
	return ep.in
}

func (ep *webSocketPeer) Close() error {
	ep.closedLock.Lock()
	defer ep.closedLock.Unlock()

	if ep.closed {
		return errors.New("Peer already closed")
	}

	// attempt to send polite goodbye message
	closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "goodbye")
	err := ep.conn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(5*time.Second))
	if err != nil {
		log.Println("error sending close message:", err)
	}

	// mark closed
	ep.closed = true

	// close chans
	ep.closeIOChans()

	return ep.conn.Close()
}

func (ep *webSocketPeer) Closed() bool {
	ep.closedLock.RLock()
	defer ep.closedLock.RUnlock()
	return ep.closed
}

func (ep *webSocketPeer) closeIOChans() {
	close(ep.in)
	close(ep.out)
}

func (ep *webSocketPeer) write() {
	for msg := range ep.out {
		b, err := ep.serializer.Serialize(msg)
		if nil != err {
			logErr(err)
			continue
		}

		errChan := make(chan error)
		go func() { errChan <- ep.conn.WriteMessage(ep.payloadType, b) }()

		ctx, cancel := context.WithTimeout(ep.ctx, 2*time.Second)

		select {
		case <-ctx.Done():
			ep.closedLock.Lock()
			ep.closed = true
			ep.closedLock.Unlock()

			logErr(errors.New("Message write timeout exceeded"))

			ep.closeIOChans()
			ep.conn.Close()
			return

		case err = <-errChan:
			cancel()
			if nil != err {
				log.Printf("unable to write message \"%s\": %s", msg.MessageType(), err)
			}
		}
	}
}

func (ep *webSocketPeer) read() {
	for {
		// read from connection...
		msgType, b, err := ep.conn.ReadMessage()

		// if errored...
		if err != nil {
			ep.closedLock.Lock()

			// are we expecting an error?
			if ep.closed {
				log.Println("peer connection closed")
			} else {
				log.Println("error reading from peer:", err)
				ep.closed = true
				ep.closeIOChans()
				ep.conn.Close()
			}
			ep.closedLock.Unlock()

			// break read loop
			return
		}

		// did the client close the connection?
		if msgType == websocket.CloseMessage {
			ep.closedLock.Lock()
			ep.closeIOChans()
			ep.conn.Close()
			ep.closed = true
			ep.closedLock.Unlock()
			return
		}

		// otherwise, attempt to process message
		msg, err := ep.serializer.Deserialize(b)
		if err != nil {
			// TODO: handle error
			log.Println("error deserializing peer message:", err)
		} else {
			ep.in <- msg
		}
	}
}

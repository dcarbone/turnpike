package turnpike

import (
	"context"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"sync"
	"time"
)

type webSocketPeer struct {
	ctx context.Context

	closedLock sync.RWMutex
	closed     bool

	conn       *websocket.Conn
	serializer Serializer

	incomingMessages chan Message
	outgoingMessages chan Message

	payloadType int
}

func NewWebSocketPeer(serializer Serializer, payloadType int, conn *websocket.Conn) Peer {
	ep := &webSocketPeer{
		ctx:              context.Background(),
		conn:             conn,
		incomingMessages: make(chan Message, 10),
		outgoingMessages: make(chan Message, 10),
		serializer:       serializer,
		payloadType:      payloadType,
	}

	go ep.readLoop()
	go ep.writeLoop()

	return ep
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

	ctx, cancel := context.WithDeadline(ep.ctx, time.Now().Add(t))

	select {
	case msg, ok := <-ep.incomingMessages:
		cancel()
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
	ep.closedLock.Lock()
	defer ep.closedLock.Unlock()

	// has peer already been closed?
	if ep.closed {
		return errors.New("Peer is already closed.")
	}

	ep.closed = true

	// construct close message
	closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "goodbye")

	// send with 1 second timeout
	err := ep.conn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(1*time.Second))
	if err != nil {
		log.Println("error sending close message:", err)
	}

	ep.closeIOChans()
	return ep.conn.Close()
}

func (ep *webSocketPeer) closeIOChans() {
	// close our channels
	close(ep.outgoingMessages)
	close(ep.incomingMessages)
}

func (ep *webSocketPeer) readLoop() {
	for {
		// attempt to read message from connection
		msgType, b, err := ep.conn.ReadMessage()
		if nil != err {
			// unable to read message, terminate peer
			ep.closedLock.RLock()
			ep.closeIOChans()
			if ep.closed {
				log.Println("peer connection closed")
			} else {
				log.Println("error reading from peer:", err)
				ep.conn.Close()
			}
			ep.closedLock.RUnlock()
			return
		}

		// if we got a close..
		if websocket.CloseMessage == msgType {
			// terminate peer
			ep.closedLock.Lock()
			ep.closed = true
			ep.closeIOChans()
			ep.conn.Close()
			ep.closedLock.Unlock()
			return
		}

		// attempt to unmarshal into something and append to incoming message queue
		msg, err := ep.serializer.Deserialize(b)
		if nil != err {
			log.Printf("error deserializing peer message: %s", err)
		} else {
			ep.incomingMessages <- msg
		}
	}
}

func (ep *webSocketPeer) writeLoop() {
	// loop so long as channel remains open
	for msg := range ep.outgoingMessages {
		// attempt to marshal
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

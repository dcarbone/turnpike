package turnpike

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/gorilla/websocket"
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

	dialer := DefaultDialer([]string{string(protocol)}, tlscfg, dial)

	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}

	return NewPeer(serializer, payloadType, conn), nil
}

func NewPeer(serializer Serializer, payloadType int, conn *websocket.Conn) Peer {
	p := &webSocketPeer{
		ctx:         context.Background(),
		conn:        conn,
		in:          make(chan Message, 100),
		out:         make(chan Message, 100),
		serializer:  serializer,
		payloadType: payloadType,
	}

	go p.read()
	go p.write()

	return p
}

func (p *webSocketPeer) Closed() bool {
	p.closedLock.RLock()
	defer p.closedLock.RUnlock()
	return p.closed
}

// Close will attempt to politely close the connection after sending a Goodbye message
func (p *webSocketPeer) Close() error {
	p.closedLock.Lock()
	defer p.closedLock.Unlock()

	// are we already closed?
	if p.closed {
		return fmt.Errorf("Peer is already closed")
	}

	// set closed to true
	p.closed = true

	closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "goodbye")
	err := p.conn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(5*time.Second))
	if err != nil {
		log.Printf("Unable to send \"Goodbye\": %s", err)
	}

	// close connection
	err = p.conn.Close()
	if nil != err {
		return fmt.Errorf("Close() Error closing peer connection: %s", err)
	}

	return nil
}

// Receive returns the underlying "in" channel.  The closure of this channel indicates the peer is defunct.
func (p *webSocketPeer) Receive() <-chan Message {
	return p.in
}

// Send will attempt to add a message to the "to be sent" channel.
func (p *webSocketPeer) Send(msg Message) error {
	p.closedLock.RLock()
	defer p.closedLock.RUnlock()

	if p.closed {
		return fmt.Errorf("Peer is closed, cannot write \"%v\"", msg)
	}

	select {
	case p.out <- msg:
		return nil
	default:
		go p.hardClose()
		return fmt.Errorf("Peer out queue is full! Something is wrong.  Closing peer")
	}
}

func (p *webSocketPeer) hardClose() {
	p.closedLock.Lock()
	defer p.closedLock.Unlock()

	// if not already closed...
	if !p.closed {
		p.closed = true
		close(p.in)
		close(p.out)

		err := p.conn.Close()
		if nil != err {
			log.Printf("hardClose() error closing peer connection: %s", err)
		}
	}
}

func (p *webSocketPeer) read() {
	for {
		// attempt to read message from connection
		msgType, message, err := p.conn.ReadMessage()

		// if errored...
		if nil != err {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure) {
				log.Printf("Error reading from peer conn: %s", err)
			}
			go p.hardClose()
			return
		}

		if msgType == websocket.CloseMessage {
			go p.hardClose()
			return
		}

		msg, err := p.serializer.Deserialize(message)
		if nil != err {
			log.Printf("Error deserializing message: %s", err)
		} else {
		TryPush:
			select {
			case p.in <- msg:
			default:
				dmsg := <-p.in
				log.Printf("Peer in queue is full, nothing is reading from it.  Discarding message \"%v\"", dmsg)
				goto TryPush
			}
		}
	}
}

func (p *webSocketPeer) write() {
	for msg := range p.out {
		b, err := p.serializer.Serialize(msg)
		if nil != err {
			log.Printf("Unable to serialize message: %s; Message: \"%v\"", err, msg)
			continue
		}

		err = p.conn.WriteMessage(p.payloadType, b)
		if nil != err {
			log.Printf("Unable to write message to connection: %s; Message: \"%v\"", err, msg)
		}
	}
}

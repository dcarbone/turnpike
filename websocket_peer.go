package turnpike

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/gorilla/websocket"
	"sync"
	"time"
)

type packet struct {
	ctx  context.Context
	msg  Message
	err  error
	done chan *packet
}

func (pack *packet) finish() {
	select {
	case pack.done <- pack:
	default:
		// discard
	}
}

type packetPool struct {
	*sync.Pool
}

func newPacketPool() *packetPool {
	pp := &packetPool{
		Pool: new(sync.Pool),
	}
	pp.Pool.New = pp.New
	return pp
}

func (pp *packetPool) Get() *packet {
	pack := pp.Pool.Get().(*packet)
	pack.msg = nil
	pack.err = nil
	pack.ctx = nil
	pack.done = make(chan *packet, 1) // TODO: clean up old chan, if necessary...
	return pack
}

func (pp *packetPool) New() interface{} {
	pack := packet{
		done: make(chan *packet, 1),
	}
	return &pack
}

type webSocketPeer struct {
	mu sync.Mutex

	conn   *websocket.Conn
	closed bool

	serializer Serializer
	msgType    int

	packetPool *packetPool
	in         chan *packet
	out        chan *packet
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
		return nil, fmt.Errorf("unsupported serialization: %v", serialization)
	}

	dialer := DefaultDialer([]string{string(protocol)}, tlscfg, dial)

	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}

	return NewPeer(serializer, payloadType, conn), nil
}

func NewPeer(serializer Serializer, msgType int, conn *websocket.Conn) Peer {
	p := &webSocketPeer{
		conn:       conn,
		serializer: serializer,
		msgType:    msgType,
		packetPool: newPacketPool(),
		in:         make(chan *packet),
		out:        make(chan *packet, 1000),
	}

	p.packetPool.New = func() interface{} {
		pack := packet{
			done: make(chan *packet),
		}
		return &pack
	}

	go p.read()
	go p.write()

	return p
}

func (p *webSocketPeer) Closed() bool {
	p.mu.Lock()
	b := p.closed
	p.mu.Unlock()
	return b
}

// Close will attempt to politely close the connection after sending a MessageGoodbye message
func (p *webSocketPeer) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	if p.conn == nil {
		p.mu.Unlock()
		return nil
	}

	// mark closed, localize conn, close chans
	p.closed = true
	conn := p.conn
	close(p.in)
	close(p.out)

	p.mu.Unlock()

	// try to be nice
	closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "goodbye")
	err := conn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(5*time.Second))
	if err != nil {
		log.Printf("Unable to send \"MessageGoodbye\": %s", err)
	}

	// terminate!
	return conn.Close()
}

func (p *webSocketPeer) Receive() (Message, error) {
	return p.ReceiveUntil(context.Background())
}

func (p *webSocketPeer) ReceiveUntil(ctx context.Context) (Message, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, errPeerClosed{}
	}
	pack := p.packetPool.Get()
	pack.ctx = ctx
	in := p.in
	p.mu.Unlock()
	in <- pack
	<-pack.done
	msg, err := pack.msg, pack.err
	p.packetPool.Put(pack)
	return msg, err
}

// Send will block until the send is attempted
func (p *webSocketPeer) Send(ctx context.Context, msg Message) error {
	return <-p.SendAsync(ctx, msg, nil)
}

// SendAsync will not block until the send is attempted.
func (p *webSocketPeer) SendAsync(ctx context.Context, msg Message, errChan chan error) <-chan error {
	p.mu.Lock()
	if errChan == nil {
		errChan = make(chan error, 1)
	}
	if p.closed {
		errChan <- errPeerClosed{}
		p.mu.Unlock()
		return errChan
	}
	pack := p.packetPool.Get()
	pack.msg = msg
	pack.ctx = ctx
	go func(pool *packetPool, pack *packet, out chan *packet) {
		out <- pack
		<-pack.done
		errChan <- pack.err
		pool.Put(pack)
	}(p.packetPool, pack, p.out)
	p.mu.Unlock()
	return errChan
}

func (p *webSocketPeer) read() {
	var conn *websocket.Conn
	var serializer Serializer

	var msgType int
	var b []byte
	var msg Message
	var err error
	var closePeer bool

	for pack := range p.in {
		p.mu.Lock()

		// lock while checking for closed...
		if p.closed {
			p.mu.Unlock()
			pack.err = errPeerClosed{}
			goto finish
		}

		// localize stuff...
		conn = p.conn
		serializer = p.serializer

		p.mu.Unlock()

		if err = pack.ctx.Err(); err != nil {
			pack.err = errMessageContextFinished{err}
		} else if msgType, b, err = conn.ReadMessage(); err != nil {
			pack.err = errSocketRead{err}
			closePeer = true
		} else if msgType == websocket.CloseMessage {
			log.Printf("Close message: %s", string(b))
			closePeer = true
		} else if msg, err = serializer.Deserialize(b); err != nil {
			pack.err = errMessageDeserialize{err}
		} else {
			pack.msg = msg
		}

	finish:
		if closePeer {
			if err = p.Close(); err != nil {
				log.Printf("read() Error closing socket: %s", err)
			}
		}

		pack.finish()
	}
}

func (p *webSocketPeer) write() {
	var conn *websocket.Conn
	var serializer Serializer
	var msgType int

	var b []byte
	var err error
	var closePeer bool

	for pack := range p.out {
		p.mu.Lock()

		// lock while checking for closed...
		if p.closed {
			p.mu.Unlock()
			pack.err = errPeerClosed{}
			goto finish
		}

		// localize stuff...
		conn = p.conn
		serializer = p.serializer
		msgType = p.msgType

		p.mu.Unlock()

		if err = pack.ctx.Err(); err != nil {
			pack.err = &errMessageContextFinished{pack.ctx.Err()}
		} else if pack.msg == nil {
			pack.err = errMessageIsNil{}
		} else if b, err = serializer.Serialize(pack.msg); err != nil {
			pack.err = errMessageSerialize{err}
		} else if err = conn.WriteMessage(msgType, b); err != nil {
			pack.err = errSocketWrite{err}
			closePeer = true
		} else if pack.msg.MessageType() == MessageTypeAbort {
			closePeer = true
		}

	finish:
		if closePeer {
			if err = p.Close(); err != nil {
				log.Printf("write() Error closing socket: %s", err)
			}
		}

		pack.finish()
	}
}

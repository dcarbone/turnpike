package turnpike

import (
	"fmt"
	"net/http"

	"context"
	"errors"
	"github.com/gorilla/websocket"
	"sync"
	"time"
)

const (
	jsonWebsocketProtocol    = "wamp.2.json"
	msgpackWebsocketProtocol = "wamp.2.msgpack"
)

type invalidPayload byte

func (e invalidPayload) Error() string {
	return fmt.Sprintf("Invalid payloadType: %d", e)
}

type protocolExists string

func (e protocolExists) Error() string {
	return "This protocol has already been registered: " + string(e)
}

type protocol struct {
	payloadType int
	serializer  Serializer
}

type WebSocketServer struct {
	Upgrader *websocket.Upgrader

	// The serializer to use for text frames. Defaults to JSONSerializer.
	TextSerializer Serializer
	// The serializer to use for binary frames. Defaults to JSONSerializer.
	BinarySerializer Serializer

	ctx       context.Context
	ctxCancel context.CancelFunc

	realms                map[URI]Realm
	closing               bool
	closeLock             sync.Mutex
	sessionOpenCallbacks  []func(*Session, string)
	sessionCloseCallbacks []func(*Session, string)

	protocols map[string]protocol
}

func NewWebSocketServer(ctx context.Context) (*WebSocketServer, error) {
	log.Println("NewWebSocketServer")
	if nil == ctx {
		return nil, errors.New("Context cannot be nil")
	}

	// Initialize new WS Server
	s := &WebSocketServer{
		realms:                make(map[URI]Realm),
		sessionOpenCallbacks:  []func(*Session, string){},
		sessionCloseCallbacks: []func(*Session, string){},
		protocols:             make(map[string]protocol),
	}

	// create cancellable context off of parent...
	s.ctx, s.ctxCancel = context.WithCancel(ctx)

	// initialize new upgrader
	s.Upgrader = &websocket.Upgrader{}

	// register default protocols...
	s.RegisterProtocol(jsonWebsocketProtocol, websocket.TextMessage, new(JSONSerializer))
	s.RegisterProtocol(msgpackWebsocketProtocol, websocket.BinaryMessage, new(MessagePackSerializer))

	return s, nil
}

// NewWebSocketServerWithRealms creates a new WebSocketServer from a map of realms
func NewWebSocketServerWithRealms(ctx context.Context, realms map[string]Realm) (*WebSocketServer, error) {
	log.Println("NewWebSocketServerWithRealms")

	ws, err := NewWebSocketServer(ctx)
	if nil != err {
		return nil, err
	}

	for uri, realm := range realms {
		if err := ws.RegisterRealm(URI(uri), realm); err != nil {
			return nil, err
		}
	}

	return ws, nil
}

// RegisterProtocol registers a serializer that should be used for a given protocol string and payload type.
func (ws *WebSocketServer) RegisterProtocol(proto string, payloadType int, serializer Serializer) error {
	log.Println("RegisterProtocol:", proto)
	if payloadType != websocket.TextMessage && payloadType != websocket.BinaryMessage {
		return invalidPayload(payloadType)
	}
	if _, ok := ws.protocols[proto]; ok {
		return protocolExists(proto)
	}
	ws.protocols[proto] = protocol{payloadType, serializer}
	ws.Upgrader.Subprotocols = append(ws.Upgrader.Subprotocols, proto)
	return nil
}

// GetLocalClient returns a client connected to the specified realm
func (ws *WebSocketServer) GetLocalClient(realm string, details map[string]interface{}) (*Client, error) {
	peer, err := ws.GetLocalPeer(URI(realm), details)
	if err != nil {
		return nil, err
	}
	c := NewClient(peer)
	go c.Receive()
	return c, nil
}

// ServeHTTP handles a new HTTP connection.
func (ws *WebSocketServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println("WebSocketServer.ServeHTTP", r.Method, r.RequestURI)
	// TODO: subprotocol?
	conn, err := ws.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Error upgrading to websocket connection:", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ws.handleWebsocket(conn)
}

func (ws *WebSocketServer) AddSessionOpenCallback(fn func(*Session, string)) {
	ws.sessionOpenCallbacks = append(ws.sessionOpenCallbacks, fn)
}

func (ws *WebSocketServer) AddSessionCloseCallback(fn func(*Session, string)) {
	ws.sessionCloseCallbacks = append(ws.sessionCloseCallbacks, fn)
}

func (ws *WebSocketServer) RegisterRealm(uri URI, realm Realm) error {
	if _, ok := ws.realms[uri]; ok {
		return RealmExistsError(uri)
	}

	err := realm.init(ws.ctx)
	if nil != err {
		return err
	}

	ws.realms[uri] = realm

	log.Println("registered realm:", uri)

	return nil
}

func (ws *WebSocketServer) Accept(client Peer) error {
	if ws.closing {
		logErr(client.Send(&Abort{Reason: ErrSystemShutdown}))
		logErr(client.Close())
		return fmt.Errorf("Router is closing, no new connections are allowed")
	}

	msg, err := GetMessageTimeout(client, 5*time.Second)
	if err != nil {
		return err
	}
	log.Printf("%s: %+v", msg.MessageType(), msg)

	hello, ok := msg.(*Hello)
	if !ok {
		logErr(client.Send(&Abort{Reason: URI("wamp.error.protocol_violation")}))
		logErr(client.Close())
		return fmt.Errorf("protocol violation: expected HELLO, received %s", msg.MessageType())
	}

	realm, ok := ws.realms[hello.Realm]
	if !ok {
		logErr(client.Send(&Abort{Reason: ErrNoSuchRealm}))
		logErr(client.Close())
		return NoSuchRealmError(hello.Realm)
	}

	welcome, err := realm.handleAuth(client, hello.Details)
	if err != nil {
		abort := &Abort{
			Reason:  ErrAuthorizationFailed, // TODO: should this be AuthenticationFailed?
			Details: map[string]interface{}{"error": err.Error()},
		}
		logErr(client.Send(abort))
		logErr(client.Close())
		return AuthenticationError(err.Error())
	}

	welcome.Id = NewID()

	if welcome.Details == nil {
		welcome.Details = make(map[string]interface{})
	}
	// add default details to welcome message
	for k, v := range defaultWelcomeDetails {
		if _, ok := welcome.Details[k]; !ok {
			welcome.Details[k] = v
		}
	}
	if err := client.Send(welcome); err != nil {
		return err
	}
	log.Println("Established session:", welcome.Id)

	// session details
	welcome.Details["session"] = welcome.Id
	welcome.Details["realm"] = hello.Realm
	sess := &Session{
		Peer:    client,
		Id:      welcome.Id,
		Details: welcome.Details,
		kill:    make(chan URI, 1),
	}
	for _, callback := range ws.sessionOpenCallbacks {
		go callback(sess, string(hello.Realm))
	}
	go func() {
		realm.handleSession(sess)
		sess.Close()
		for _, callback := range ws.sessionCloseCallbacks {
			go callback(sess, string(hello.Realm))
		}
	}()
	return nil
}

func (ws *WebSocketServer) Close() error {
	ws.closeLock.Lock()
	if ws.closing {
		ws.closeLock.Unlock()
		return errors.New("already closed")
	}
	ws.closing = true
	ws.closeLock.Unlock()
	for _, realm := range ws.realms {
		realm.Close()
	}
	return nil
}

// GetLocalPeer returns an internal peer connected to the specified realm.
func (ws *WebSocketServer) GetLocalPeer(realmURI URI, details map[string]interface{}) (Peer, error) {
	realm, ok := ws.realms[realmURI]
	if !ok {
		return nil, NoSuchRealmError(realmURI)
	}
	// TODO: session open/close callbacks?
	return realm.getPeer(details)
}

func (ws *WebSocketServer) handleWebsocket(conn *websocket.Conn) {
	var serializer Serializer
	var payloadType int
	if proto, ok := ws.protocols[conn.Subprotocol()]; ok {
		serializer = proto.serializer
		payloadType = proto.payloadType
	} else {
		// TODO: this will not currently ever be hit because
		//       gorilla/websocket will reject the connection
		//       if the subprotocol isn't registered
		switch conn.Subprotocol() {
		case jsonWebsocketProtocol:
			serializer = new(JSONSerializer)
			payloadType = websocket.TextMessage
		case msgpackWebsocketProtocol:
			serializer = new(MessagePackSerializer)
			payloadType = websocket.BinaryMessage
		default:
			conn.Close()
			return
		}
	}

	peer := webSocketPeer{
		conn:             conn,
		serializer:       serializer,
		incomingMessages: make(chan Message, 10),
		payloadType:      payloadType,
	}
	go peer.run()

	logErr(ws.Accept(&peer))
}

func (ws *WebSocketServer) getTestPeer() Peer {
	peerA, peerB := localPipe()
	go ws.Accept(peerA)
	return peerB
}

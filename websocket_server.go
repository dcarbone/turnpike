package turnpike

import (
	"fmt"
	"github.com/gorilla/websocket"
	"net/http"
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
	Router
	Upgrader *websocket.Upgrader

	// The serializer to use for text frames. Defaults to JSONSerializer.
	TextSerializer Serializer
	// The serializer to use for binary frames. Defaults to JSONSerializer.
	BinarySerializer Serializer

	protocols map[string]protocol
}

func NewWebSocketServer() (*WebSocketServer, error) {
	log.Println("NewWebSocketServer")

	// Initialize new WS Server
	s := &WebSocketServer{
		Router:    NewDefaultRouter(),
		Upgrader:  &websocket.Upgrader{},
		protocols: make(map[string]protocol),
	}

	// register default protocols...
	s.RegisterProtocol(jsonWebsocketProtocol, websocket.TextMessage, new(JSONSerializer))
	s.RegisterProtocol(msgpackWebsocketProtocol, websocket.BinaryMessage, new(MessagePackSerializer))

	return s, nil
}

// NewWebSocketServerWithRealms creates a new WebSocketServer from a map of realms
func NewWebSocketServerWithRealms(realms map[string]Realm) (*WebSocketServer, error) {
	log.Println("NewWebSocketServerWithRealms")

	ws, err := NewWebSocketServer()
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
	go peer.writeLoop()

	logErr(ws.Accept(&peer))
}

func (ws *WebSocketServer) getTestPeer() Peer {
	peerA, peerB := localPipe()
	go ws.Accept(peerA)
	return peerB
}

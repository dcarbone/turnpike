package turnpike

import (
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
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

// WebSocketServer handles websocket connections.
type WebSocketServer struct {
	Router
	Upgrader *websocket.Upgrader

	protocols map[string]protocol

	// The serializer to use for text frames. Defaults to JSONSerializer.
	TextSerializer Serializer
	// The serializer to use for binary frames. Defaults to JSONSerializer.
	BinarySerializer Serializer
}

// NewWebSocketServer creates a new WebSocketServer from a map of realms
func NewWebSocketServer(realms map[string]Realm) (*WebSocketServer, error) {
	log.Println("NewWebSocketServer")
	r := NewDefaultRouter()
	for uri, realm := range realms {
		if err := r.RegisterRealm(URI(uri), realm); err != nil {
			return nil, err
		}
	}
	s := newWebSocketServer(r)
	return s, nil
}

// NewBasicWebSocketServer creates a new WebSocketServer with a single basic realm
func NewBasicWebSocketServer(uri string) *WebSocketServer {
	log.Println("NewBasicWebSocketServer")
	s, _ := NewWebSocketServer(map[string]Realm{uri: {}})
	return s
}

func newWebSocketServer(r Router) *WebSocketServer {
	s := &WebSocketServer{
		Router:    r,
		protocols: make(map[string]protocol),
	}
	s.Upgrader = &websocket.Upgrader{}
	s.RegisterProtocol(jsonWebsocketProtocol, websocket.TextMessage, new(JSONSerializer))
	s.RegisterProtocol(msgpackWebsocketProtocol, websocket.BinaryMessage, new(MessagePackSerializer))
	return s
}

// RegisterProtocol registers a serializer that should be used for a given protocol string and payload type.
func (s *WebSocketServer) RegisterProtocol(proto string, payloadType int, serializer Serializer) error {
	log.Println("RegisterProtocol:", proto)
	if payloadType != websocket.TextMessage && payloadType != websocket.BinaryMessage {
		return invalidPayload(payloadType)
	}
	if _, ok := s.protocols[proto]; ok {
		return protocolExists(proto)
	}
	s.protocols[proto] = protocol{payloadType, serializer}
	s.Upgrader.Subprotocols = append(s.Upgrader.Subprotocols, proto)
	return nil
}

// GetLocalClient returns a client connected to the specified realm
func (s *WebSocketServer) GetLocalClient(realm string, details map[string]interface{}) (*Client, error) {
	peer, err := s.Router.GetLocalPeer(URI(realm), details)
	if err != nil {
		return nil, err
	}
	c := NewClient(peer)
	go c.Receive()
	return c, nil
}

// ServeHTTP handles a new HTTP connection.
func (s *WebSocketServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println("WebSocketServer.ServeHTTP", r.Method, r.RequestURI)
	// TODO: subprotocol?
	conn, err := s.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Error upgrading to websocket connection:", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.handleWebsocket(conn)
}

func (s *WebSocketServer) handleWebsocket(conn *websocket.Conn) {
	var serializer Serializer
	var payloadType int
	if proto, ok := s.protocols[conn.Subprotocol()]; ok {
		serializer = proto.serializer
		payloadType = proto.payloadType
	} else {
		// TODO: this will not currently ever be hit because
		//       gorilla/websocket will reject the conncetion
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
		conn:        conn,
		serializer:  serializer,
		messages:    make(chan Message, 10),
		payloadType: payloadType,
	}
	go peer.run()

	logErr(s.Router.Accept(&peer))
}

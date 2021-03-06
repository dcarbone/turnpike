package turnpike

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
)

func newTestWebsocketServer(t *testing.T) (int, Router, io.Closer) {
	r := NewDefaultRouter()
	r.RegisterRealm(&Realm{URI: testRealm})
	s := newWebSocketServer(r)
	server := &http.Server{
		Handler: s,
	}

	var addr net.TCPAddr
	l, err := net.ListenTCP("tcp", &addr)
	if err != nil {
		t.Fatal(err)
	}
	go server.Serve(l)
	return l.Addr().(*net.TCPAddr).Port, r, l
}

func TestWSHandshakeJSON(t *testing.T) {
	port, r, closer := newTestWebsocketServer(t)
	defer closer.Close()

	client, err := NewWebSocketPeer(SerializationFormatJSON, fmt.Sprintf("ws://localhost:%d/", port), nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	client.Send(&HelloMessage{Realm: testRealm})
	go r.Accept(client)

	if msg, ok := <-client.Receive(); !ok {
		t.Fatal("Receive buffer closed")
	} else if _, ok := msg.(*Welcome); !ok {
		t.Errorf("Message not Welcome message: %T, %+v", msg, msg)
	}
}

func TestWSHandshakeMsgpack(t *testing.T) {
	port, r, closer := newTestWebsocketServer(t)
	defer closer.Close()

	client, err := NewWebSocketPeer(SerializationFormatMSGPack, fmt.Sprintf("ws://localhost:%d/", port), nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	client.Send(&HelloMessage{Realm: testRealm})
	go r.Accept(client)

	if msg, ok := <-client.Receive(); !ok {
		t.Fatal("Receive buffer closed")
	} else if _, ok := msg.(*Welcome); !ok {
		t.Errorf("Message not Welcome message: %T, %+v", msg, msg)
	}
}

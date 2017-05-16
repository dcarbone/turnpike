package turnpike

import (
	"crypto/tls"
	"github.com/gorilla/websocket"
	"net"
	"net/http"
)

type DialFunc func(network, addr string) (net.Conn, error)

func DefaultDialer(subProtocols []string, tlsConfig *tls.Config, dialFunc DialFunc) *websocket.Dialer {
	return &websocket.Dialer{
		Subprotocols:    subProtocols,
		TLSClientConfig: tlsConfig,
		Proxy:           http.ProxyFromEnvironment,
		NetDial:         dialFunc,
	}
}

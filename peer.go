package turnpike

import (
	"context"
)

// A Sender can send a message to its peer.
//
// For clients, this sends a message to the router, and for routers,
// this sends a message to the client.
type Sender interface {
	// Send a message to the peer
	Send(context.Context, Message) error
	SendAsync(context.Context, Message, chan error) <-chan error
}

// Peer is the interface that must be implemented by all WAMP peers.
// It must be a single in, multi-out writer.
type Peer interface {
	Sender

	// Closes the peer connection and any channel returned from Receive().
	// Multiple calls to Close() will have no effect.
	Close() error
	Closed() bool

	// Receive will wait
	Receive() (Message, error)
	// ReceiveUntil will attempt to return a message if the provided context is not done'd prior to message retrieval.
	ReceiveUntil(context.Context) (Message, error)
}

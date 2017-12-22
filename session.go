package turnpike

import (
	"context"
	"fmt"
)

// Session represents an active WAMP session
type Session struct {
	Peer
	ID      ID
	Details map[string]interface{}
}

func (s Session) String() string {
	return fmt.Sprintf("%d", s.ID)
}

// localPipe creates two linked sessions. Messages sent to one will
// appear in the Receive of the other. This is useful for implementing
// client sessions
func localPipe() (*localPeer, *localPeer) {
	aToB := make(chan Message, 10)
	bToA := make(chan Message, 10)

	a := &localPeer{
		incoming: bToA,
		outgoing: aToB,
	}
	b := &localPeer{
		incoming: aToB,
		outgoing: bToA,
	}

	return a, b
}

type localPeer struct {
	outgoing chan<- Message
	incoming <-chan Message
}

func (s *localPeer) Closed() bool {
	return false
}

func (s *localPeer) Receive() (Message, error) {
	return s.ReceiveUntil(context.Background())
}

func (s *localPeer) ReceiveUntil(ctx context.Context) (Message, error) {
	select {
	case <-ctx.Done():
		return nil, errMessageContextFinished{ctx.Err()}
	case msg := <-s.incoming:
		return msg, nil
	}
}

func (s *localPeer) Send(ctx context.Context, msg Message) error {
	return <-s.SendAsync(ctx, msg, nil)
}

func (s *localPeer) SendAsync(ctx context.Context, msg Message, errChan chan error) <-chan error {
	if errChan == nil {
		errChan = make(chan error, 1)
	}
	go func() {
		select {
		case <-ctx.Done():
			errChan <- errMessageContextFinished{ctx.Err()}
		case s.outgoing <- msg:
			errChan <- nil
		}
	}()
	return errChan
}

func (s *localPeer) Close() error {
	close(s.outgoing)
	return nil
}

package websocket

import (
	"context"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/matchmaking"
)

const sessionOutboundBufferSize = 16

type clientSession struct {
	ctx             context.Context
	clientID        matchmaking.ClientID
	conn            *websocket.Conn
	outbound        chan any
	cancel          context.CancelFunc
	searchRequestID string
	mtx             sync.RWMutex
}

func newClientSession(
	ctx context.Context,
	clientID matchmaking.ClientID,
	conn *websocket.Conn,
) *clientSession {
	sessionCtx, cancel := context.WithCancel(ctx)

	session := clientSession{
		clientID: clientID,
		conn:     conn,
		outbound: make(chan any, sessionOutboundBufferSize),
		cancel:   cancel,
		ctx:      sessionCtx,
	}

	return &session
}

func (session *clientSession) setSearchRequestID(requestID string) {
	session.mtx.Lock()
	defer session.mtx.Unlock()

	session.searchRequestID = requestID
}

func (session *clientSession) takeSearchRequestID() string {
	session.mtx.Lock()
	defer session.mtx.Unlock()

	requestID := session.searchRequestID
	session.searchRequestID = ""

	return requestID
}

func (session *clientSession) clearSearchRequestID() {
	session.mtx.Lock()
	defer session.mtx.Unlock()

	session.searchRequestID = ""
}

func (session *clientSession) send(message any) error {
	return session.sendWithContext(session.ctx, message)
}

func (session *clientSession) sendWithContext(ctx context.Context, message any) error {
	if err := session.ctx.Err(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	select {
	case <-session.ctx.Done():
		return session.ctx.Err()
	case <-ctx.Done():
		return ctx.Err()
	case session.outbound <- message:
		return nil
	}
}

func (session *clientSession) writeLoop() {
	defer session.cancel()
	defer session.conn.Close()

	for {
		select {
		case <-session.ctx.Done():
			return

		case message := <-session.outbound:
			if err := session.conn.WriteJSON(message); err != nil {
				return
			}
		}
	}
}

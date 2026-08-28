package websocket

import (
	"context"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/matchmaking"
)

const sessionOutboundBufferSize = 16
const sessionEnqueueTimeout = 10 * time.Second
const sessionWriteTimeout = 10 * time.Second

type clientSession struct {
	ctx             context.Context
	clientID        matchmaking.ClientID
	conn            *websocket.Conn
	outbound        chan any
	enqueueTimeout  time.Duration
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
		clientID:       clientID,
		conn:           conn,
		outbound:       make(chan any, sessionOutboundBufferSize),
		enqueueTimeout: sessionEnqueueTimeout,
		cancel:         cancel,
		ctx:            sessionCtx,
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

	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, session.enqueueTimeout)
	defer timeoutCancel()

	select {
	case <-session.ctx.Done():
		return session.ctx.Err()
	case <-timeoutCtx.Done():
		return timeoutCtx.Err()
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
			if err := session.conn.SetWriteDeadline(time.Now().Add(sessionWriteTimeout)); err != nil {
				return
			}
			if err := session.conn.WriteJSON(message); err != nil {
				return
			}
		}
	}
}

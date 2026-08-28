package websocket

import (
	"sync"

	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/matchmaking"
)

type SessionRegistry struct {
	clientSessions map[matchmaking.ClientID]*clientSession
	mtx            sync.RWMutex
}

func NewSessionRegistry() *SessionRegistry {
	return &SessionRegistry{
		clientSessions: make(map[matchmaking.ClientID]*clientSession),
	}
}

func (registry *SessionRegistry) register(session *clientSession) bool {
	registry.mtx.Lock()
	defer registry.mtx.Unlock()

	if _, exists := registry.clientSessions[session.clientID]; exists {
		return false
	}

	registry.clientSessions[session.clientID] = session
	return true
}

func (registry *SessionRegistry) get(clientID matchmaking.ClientID) (*clientSession, bool) {
	registry.mtx.RLock()
	defer registry.mtx.RUnlock()

	var session *clientSession
	if session = registry.clientSessions[clientID]; session == nil {
		return nil, false
	}
	return session, true
}

func (registry *SessionRegistry) remove(session *clientSession) {
	registry.mtx.Lock()
	defer registry.mtx.Unlock()

	var foundedSession *clientSession
	if foundedSession = registry.clientSessions[session.clientID]; foundedSession == session {
		delete(registry.clientSessions, session.clientID)
	}
}

package websocket

import (
	"github.com/google/uuid"

	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/matchmaking"
)

func GenerateClientID() (matchmaking.ClientID, error) {
	value := uuid.NewString()
	clientID, err := matchmaking.NewClientID(value)
	if err != nil {
		return matchmaking.ClientID{}, err
	}
	return clientID, nil
}

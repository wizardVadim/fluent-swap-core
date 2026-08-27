package websocket

import (
	"context"

	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/matchmaking"
	matchmakingservice "github.com/wizardVadim/fluent-swap-core/internal/features/matchmaking/service"
)

type ClientIDGenerator func() (matchmaking.ClientID, error)

type Service interface {
	FindPartner(ctx context.Context, user matchmaking.WaitingUser) (matchmakingservice.MatchResult, error)
	CancelSearch(ctx context.Context, clientID matchmaking.ClientID) error
}

type MatchIDGenerator func() string

package websocket

import (
	"context"

	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/matchmaking"
	"github.com/wizardVadim/fluent-swap-core/internal/features/matchmaking/repository"
)

type ClientIDGenerator func() (matchmaking.ClientID, error)

type Service interface {
	FindPartner(ctx context.Context, user matchmaking.WaitingUser) (repository.MatchResult, error)
	CancelSearch(ctx context.Context, clientID matchmaking.ClientID) error
}

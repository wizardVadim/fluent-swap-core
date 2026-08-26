package repository

import (
	"context"

	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/matchmaking"
)

type Repository interface {
	MatchOrEnqueue(ctx context.Context, wu matchmaking.WaitingUser) (MatchResult, error)
	RemoveFromQueue(ctx context.Context, clientID matchmaking.ClientID) error
}

type MatchResult struct {
	Partner matchmaking.WaitingUser
	Matched bool
}

package service

import (
	"context"

	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/matchmaking"
)

type MatchResult struct {
	Partner matchmaking.WaitingUser
	Matched bool
}

type Repository interface {
	MatchOrEnqueue(
		ctx context.Context,
		user matchmaking.WaitingUser,
	) (MatchResult, error)

	RemoveFromQueue(
		ctx context.Context,
		clientID matchmaking.ClientID,
	) error
}

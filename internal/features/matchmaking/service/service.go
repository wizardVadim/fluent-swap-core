package service

import (
	"context"

	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/matchmaking"
)

type Service struct {
	repository Repository
}

func New(repo Repository) *Service {
	return &Service{
		repository: repo,
	}
}

func (s *Service) FindPartner(ctx context.Context, user matchmaking.WaitingUser) (MatchResult, error) {
	return s.repository.MatchOrEnqueue(ctx, user)
}

func (s *Service) CancelSearch(ctx context.Context, clientID matchmaking.ClientID) error {
	return s.repository.RemoveFromQueue(ctx, clientID)
}

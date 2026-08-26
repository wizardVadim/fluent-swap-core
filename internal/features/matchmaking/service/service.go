package service

import (
	"context"

	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/matchmaking"
	"github.com/wizardVadim/fluent-swap-core/internal/features/matchmaking/repository"
)

type Service struct {
	repository repository.Repository
}

func New(repo repository.Repository) *Service {
	return &Service{
		repository: repo,
	}
}

func (s *Service) FindPartner(ctx context.Context, user matchmaking.WaitingUser) (repository.MatchResult, error) {
	return s.repository.MatchOrEnqueue(ctx, user)
}

func (s *Service) CancelSearch(ctx context.Context, clientID matchmaking.ClientID) error {
	return s.repository.RemoveFromQueue(ctx, clientID)
}

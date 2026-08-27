package repository

import (
	"context"
	"sync"

	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/matchmaking"
	matchmakingservice "github.com/wizardVadim/fluent-swap-core/internal/features/matchmaking/service"
)

var _ matchmakingservice.Repository = (*MemoryRepository)(nil)

type queue []matchmaking.WaitingUser

type MemoryRepository struct {
	queue queue
	mtx   sync.Mutex
}

func NewMemoryRepository() *MemoryRepository {
	var queue queue
	repository := MemoryRepository{queue: queue}

	return &repository
}

func (repository *MemoryRepository) MatchOrEnqueue(ctx context.Context, wu matchmaking.WaitingUser) (matchmakingservice.MatchResult, error) {
	repository.mtx.Lock()
	defer repository.mtx.Unlock()

	if err := ctx.Err(); err != nil {
		return matchmakingservice.MatchResult{Matched: false}, err
	}

	var partner matchmaking.WaitingUser
	matched := false
	partnerIndex := -1
	for idx, v := range repository.queue {
		if v.ClientID().IsEqual(wu.ClientID()) {
			if v.LanguagePair().IsEqual(wu.LanguagePair()) {
				return matchmakingservice.MatchResult{Matched: false}, nil
			}
			return matchmakingservice.MatchResult{Matched: false}, matchmakingservice.ErrClientAlreadyQueued
		}
		if !matched && v.LanguagePair().IsCompatibleWith(wu.LanguagePair()) {
			partner = v
			matched = true

			partnerIndex = idx
		}
	}

	if !matched {
		repository.queue = append(repository.queue, wu)
	} else {
		repository.removeFromQueueByIndex(partnerIndex)
	}

	return matchmakingservice.MatchResult{Matched: matched, Partner: partner}, nil
}

func (repository *MemoryRepository) removeFromQueueByIndex(idx int) {
	repository.queue = append(repository.queue[:idx], repository.queue[idx+1:]...)
}

func (repository *MemoryRepository) removeFromQueue(clientID matchmaking.ClientID) {
	for idx, v := range repository.queue {
		if v.ClientID().IsEqual(clientID) {
			repository.removeFromQueueByIndex(idx)
			return
		}
	}
}

func (repository *MemoryRepository) RemoveFromQueue(ctx context.Context, clientID matchmaking.ClientID) error {
	repository.mtx.Lock()
	defer repository.mtx.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}
	repository.removeFromQueue(clientID)
	return nil
}

package repository

import (
	"context"
	"sync"

	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/matchmaking"
)

var _ Repository = (*MemoryRepository)(nil)

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

func (repository *MemoryRepository) MatchOrEnqueue(ctx context.Context, wu matchmaking.WaitingUser) (MatchResult, error) {
	repository.mtx.Lock()
	defer repository.mtx.Unlock()

	if err := ctx.Err(); err != nil {
		return MatchResult{Matched: false}, err
	}

	var partner matchmaking.WaitingUser
	matched := false
	partnerIndex := -1
	for idx, v := range repository.queue {
		if v.ClientID().IsEqual(wu.ClientID()) {
			if v.LanguagePair().IsEqual(wu.LanguagePair()) {
				return MatchResult{Matched: false}, nil
			}
			return MatchResult{Matched: false}, ErrClientAlreadyQueued
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

	return MatchResult{Matched: matched, Partner: partner}, nil
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

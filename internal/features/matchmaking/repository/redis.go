package repository

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/matchmaking"
)

//go:embed lua_scripts/remove_from_queue.lua
var removeFromQueueLua string

var removeFromQueueScript = redis.NewScript(removeFromQueueLua)

const clientStateKeyPrefix = "fluent-swap:queue:client:"

const (
	removeResultQueueKeyMissing int64 = -2
	removeResultInvalidState    int64 = -1
	removeResultNoop            int64 = 0
	removeResultRemoved         int64 = 1
)

type RedisRepository struct {
	client *redis.Client
}

func NewRedisRepository(client *redis.Client) *RedisRepository {
	return &RedisRepository{
		client: client,
	}
}

func (repository *RedisRepository) RemoveFromQueue(ctx context.Context, clientID matchmaking.ClientID) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	keys := []string{
		clientStateKey(clientID),
	}

	args := []any{
		clientID.Value(),
	}

	result, err := removeFromQueueScript.Run(ctx, repository.client, keys, args...).Int64()
	if err != nil {
		return fmt.Errorf("remove client from Redis queue: %w", err)
	}

	switch result {
	case removeResultNoop, removeResultRemoved:
		return nil
	case removeResultQueueKeyMissing:
		return errQueueKeyMissing
	case removeResultInvalidState:
		return errInvalidClientState
	default:
		return fmt.Errorf("%w: %d", errInvalidRedisResponseCode, result)
	}
}

func clientStateKey(clientID matchmaking.ClientID) string {
	return clientStateKeyPrefix + clientID.Value()
}

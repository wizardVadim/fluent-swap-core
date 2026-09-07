package repository

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/matchmaking"
	matchmakingservice "github.com/wizardVadim/fluent-swap-core/internal/features/matchmaking/service"
)

//go:embed lua_scripts/remove_from_queue.lua
var removeFromQueueLua string

//go:embed lua_scripts/match_or_enqueue.lua
var matchOrEnqueue string

var removeFromQueueScript = redis.NewScript(removeFromQueueLua)

var matchOrEnqueueScript = redis.NewScript(matchOrEnqueue)

const clientStateKeyPrefix = "fluent-swap:queue:client:"
const queueKeyPrefix = "fluent-swap:queue:"

const (
	removeResultQueueKeyMissing int64 = -2
	removeResultInvalidState    int64 = -1
	removeResultNoop            int64 = 0
	removeResultRemoved         int64 = 1
)

const (
	matchOrEnqueueWaitingState               int64 = 0
	matchOrEnqueueWaitingAnotherLanguagePair int64 = -1
	matchOrEnqueueMatchedState               int64 = 1
	matchOrEnqueueInvalidState               int64 = -2
	matchOrEnqueueContinueState              int64 = 2
)

const (
	waitingSecondsTTL int = 600
	matchedSecondsTTL int = 7200
	stepCount         int = 30
)

type RedisRepository struct {
	client *redis.Client
}

var _ matchmakingservice.Repository = (*RedisRepository)(nil)

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

func (repository *RedisRepository) MatchOrEnqueue(ctx context.Context, wu matchmaking.WaitingUser) (matchmakingservice.MatchResult, error) {
	for {
		if err := ctx.Err(); err != nil {
			return matchmakingservice.MatchResult{Matched: false}, err
		}

		partnerLanguagePair, err := matchmaking.NewLanguagePair(wu.LanguagePair().LearningLanguage(), wu.LanguagePair().NativeLanguage())
		if err != nil {
			return matchmakingservice.MatchResult{Matched: false}, err
		}

		partnerQueueKey := fmt.Sprintf("%s%s:%s", queueKeyPrefix, partnerLanguagePair.NativeLanguage().Code(), partnerLanguagePair.LearningLanguage().Code())
		clientQueueKey := fmt.Sprintf("%s%s:%s", queueKeyPrefix, wu.LanguagePair().NativeLanguage().Code(), wu.LanguagePair().LearningLanguage().Code())

		keys := []string{
			clientStateKey(wu.ClientID()),
			partnerQueueKey,
			clientQueueKey,
		}

		args := []any{
			wu.ClientID().Value(),
			waitingSecondsTTL,
			matchedSecondsTTL,
			clientStateKeyPrefix,
			stepCount,
		}

		result, err := matchOrEnqueueScript.Run(ctx, repository.client, keys, args...).Result()
		if err != nil {
			return matchmakingservice.MatchResult{Matched: false}, fmt.Errorf("match client or enqueue error: %w", err)
		}
		values, ok := result.([]any)
		if !ok {
			return matchmakingservice.MatchResult{Matched: false}, fmt.Errorf("match client or enqueue error: %w", errInvalidRedisResult)
		}
		if len(values) < 1 {
			return matchmakingservice.MatchResult{Matched: false}, fmt.Errorf("match client or enqueue error: %w", errInvalidRedisResult)
		}

		resultCode, ok := values[0].(int64)
		if !ok {
			return matchmakingservice.MatchResult{Matched: false}, fmt.Errorf("match client or enqueue error: %w", errInvalidRedisResult)
		}

		switch resultCode {
		case matchOrEnqueueContinueState:
			if len(values) != 1 {
				return matchmakingservice.MatchResult{}, fmt.Errorf("match client or enqueue error: %w", errInvalidRedisResult)
			}
			continue
		case matchOrEnqueueWaitingState:
			if len(values) != 1 {
				return matchmakingservice.MatchResult{}, fmt.Errorf("match client or enqueue error: %w", errInvalidRedisResult)
			}
			return matchmakingservice.MatchResult{}, nil
		case matchOrEnqueueWaitingAnotherLanguagePair:
			if len(values) != 1 {
				return matchmakingservice.MatchResult{}, fmt.Errorf("match client or enqueue error: %w", errInvalidRedisResult)
			}
			return matchmakingservice.MatchResult{}, matchmakingservice.ErrClientAlreadyQueued
		case matchOrEnqueueInvalidState:
			if len(values) != 1 {
				return matchmakingservice.MatchResult{}, fmt.Errorf("match client or enqueue error: %w", errInvalidRedisResult)
			}
			return matchmakingservice.MatchResult{}, errInvalidClientState
		case matchOrEnqueueMatchedState:
			if len(values) != 3 {
				return matchmakingservice.MatchResult{Matched: false}, fmt.Errorf("match client or enqueue error: %w", errInvalidRedisResult)
			}
			return helpMatchOrEnqueueMatchedState(values)
		default:
			return matchmakingservice.MatchResult{}, fmt.Errorf("%w: %d", errInvalidRedisResponseCode, resultCode)
		}
	}
}

func helpMatchOrEnqueueMatchedState(values []any) (matchmakingservice.MatchResult, error) {
	partnerIDString, ok := values[1].(string)
	if !ok {
		return matchmakingservice.MatchResult{Matched: false}, fmt.Errorf("match client or enqueue error: %w", errInvalidRedisResult)
	}
	partnerQueueKey, ok := values[2].(string)
	if !ok {
		return matchmakingservice.MatchResult{Matched: false}, fmt.Errorf("match client or enqueue error: %w", errInvalidRedisResult)
	}

	partnerLanguagePair, err := languagePairFromQueueKey(partnerQueueKey)
	if err != nil {
		return matchmakingservice.MatchResult{Matched: false}, err
	}

	partnerID, err := matchmaking.NewClientID(partnerIDString)
	if err != nil {
		return matchmakingservice.MatchResult{Matched: false}, err
	}

	partnerUser, err := matchmaking.NewWaitingUser(partnerID, partnerLanguagePair)
	if err != nil {
		return matchmakingservice.MatchResult{Matched: false}, err
	}

	return matchmakingservice.MatchResult{Matched: true, Partner: partnerUser}, nil
}

func clientStateKey(clientID matchmaking.ClientID) string {
	return clientStateKeyPrefix + clientID.Value()
}

func languagePairFromQueueKey(queueKey string) (matchmaking.LanguagePair, error) {
	lPairString, found := strings.CutPrefix(queueKey, queueKeyPrefix)
	if !found {
		return matchmaking.LanguagePair{}, errInvalidRedisResult
	}

	codes := strings.Split(lPairString, ":")
	if len(codes) != 2 {
		return matchmaking.LanguagePair{}, errInvalidRedisResult
	}

	native, err := matchmaking.NewLanguage(matchmaking.LanguageCode(codes[0]))
	if err != nil {
		return matchmaking.LanguagePair{}, err
	}
	learning, err := matchmaking.NewLanguage(matchmaking.LanguageCode(codes[1]))
	if err != nil {
		return matchmaking.LanguagePair{}, err
	}

	lPair, err := matchmaking.NewLanguagePair(native, learning)
	if err != nil {
		return matchmaking.LanguagePair{}, err
	}

	return lPair, nil
}

//go:build integration

package repository

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/matchmaking"
)

const (
	defaultRedisAddr     = "localhost:6379"
	defaultRedisPassword = "test_pass"
	defaultRedisDB       = 15
)

func envIntOrDefault(t *testing.T, key string, fallback int) int {
	t.Helper()

	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		t.Fatalf("parse %s=%q: %v", key, value, err)
	}

	return parsed
}

func envOrDefault(key, fallback string) string {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback
	}

	return value
}

func mustRedisClient(t *testing.T) *redis.Client {
	t.Helper()

	addr := envOrDefault("REDIS_ADDR", defaultRedisAddr)
	password := envOrDefault("REDIS_PASSWORD", defaultRedisPassword)
	db := envIntOrDefault(t, "REDIS_DB", defaultRedisDB)

	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close Redis client: %v", err)
		}
	})

	return client
}

func TestClientConnection(t *testing.T) {
	client := mustRedisClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	pong, err := client.Ping(ctx).Result()
	if err != nil {
		t.Fatalf("could not connect to Redis: %v", err)
	}
	if pong != "PONG" {
		t.Fatalf("unexpected ping response: %q", pong)
	}
}

func TestRedisStateIsolation(t *testing.T) {
	client := mustRedisClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	key := "fluent-swap:test:" + uuid.NewString()
	value := "test"

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(
			context.Background(),
			3*time.Second,
		)
		defer cleanupCancel()

		if err := client.Del(cleanupCtx, key).Err(); err != nil {
			t.Errorf("delete test Redis key: %v", err)
		}
	})

	if err := client.Set(ctx, key, value, time.Minute).Err(); err != nil {
		t.Fatalf("could not insert key: %v", err)
	}

	val, err := client.Get(ctx, key).Result()
	if err != nil {
		t.Fatalf("could not get by key: %v", err)
	}
	if val != value {
		t.Fatalf("unexpected value: got %q, want %q", val, value)
	}
}

func TestRemoveFromQueueSuccess(t *testing.T) {
	client := mustRedisClient(t)
	repo := NewRedisRepository(client)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientID := uuid.NewString()

	id, err := matchmaking.NewClientID(clientID)
	if err != nil {
		t.Fatalf("create client ID: %v", err)
	}

	keyIndex := clientStateKey(id)
	stateParam := "state"
	state := "waiting"
	queueKeyParam := "queue_key"
	queueKey := uuid.NewString()

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cleanupCancel()

		if err := repo.client.Del(cleanupCtx, keyIndex).Err(); err != nil {
			t.Errorf("remove_from_queue test Redis key index: %v", err)
		}
		if err := repo.client.Del(cleanupCtx, queueKey).Err(); err != nil {
			t.Errorf("remove_from_queue test Redis key queue: %v", err)
		}
	})

	if err := repo.client.HSet(ctx, keyIndex, stateParam, state, queueKeyParam, queueKey).Err(); err != nil {
		t.Fatalf("could not insert into hash: %v", err)
	}

	if err := repo.client.LPush(ctx, queueKey, clientID).Err(); err != nil {
		t.Fatalf("could not insert into list: %v", err)
	}

	if err := repo.RemoveFromQueue(ctx, id); err != nil {
		t.Fatalf("error removing from queue: %v", err)
	}

	length, err := client.LLen(ctx, queueKey).Result()
	if err != nil {
		t.Fatalf("get queue length: %v", err)
	}
	const wantLength int64 = 0
	if length != wantLength {
		t.Fatalf("unexpected queue length: got %d, want %d", length, wantLength)
	}

	exists, err := client.Exists(ctx, keyIndex).Result()
	if err != nil {
		t.Fatalf("get key exists: %v", err)
	}
	const wantExists int64 = 0
	if exists != wantExists {
		t.Fatalf("unexpected client state existence: got %d, want %d", exists, wantExists)
	}
}

func TestRemoveFromQueueMatchedState(t *testing.T) {
	client := mustRedisClient(t)
	repo := NewRedisRepository(client)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientID := uuid.NewString()

	id, err := matchmaking.NewClientID(clientID)
	if err != nil {
		t.Fatalf("create client ID: %v", err)
	}

	keyIndex := clientStateKey(id)
	stateParam := "state"
	state := "matched"
	queueKeyParam := "queue_key"
	queueKey := uuid.NewString()

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cleanupCancel()

		if err := repo.client.Del(cleanupCtx, keyIndex).Err(); err != nil {
			t.Errorf("remove_from_queue test Redis key index: %v", err)
		}
		if err := repo.client.Del(cleanupCtx, queueKey).Err(); err != nil {
			t.Errorf("remove_from_queue test Redis key queue: %v", err)
		}
	})

	if err := repo.client.HSet(ctx, keyIndex, stateParam, state, queueKeyParam, queueKey).Err(); err != nil {
		t.Fatalf("could not insert into hash: %v", err)
	}

	if err := repo.client.LPush(ctx, queueKey, clientID).Err(); err != nil {
		t.Fatalf("could not insert into list: %v", err)
	}

	if err := repo.RemoveFromQueue(ctx, id); err != nil {
		t.Fatalf("error removing from queue: %v", err)
	}

	length, err := client.LLen(ctx, queueKey).Result()
	if err != nil {
		t.Fatalf("get queue length: %v", err)
	}
	const wantLength int64 = 1
	if length != wantLength {
		t.Fatalf("unexpected queue length: got %d, want %d", length, wantLength)
	}

	exists, err := client.Exists(ctx, keyIndex).Result()
	if err != nil {
		t.Fatalf("get key exists: %v", err)
	}
	const wantExists int64 = 1
	if exists != wantExists {
		t.Fatalf("unexpected client state existence: got %d, want %d", exists, wantExists)
	}

	actualState, err := client.HGet(ctx, keyIndex, stateParam).Result()
	if err != nil {
		t.Fatalf("get client state: %v", err)
	}
	if actualState != state {
		t.Fatalf("unexpected client state: got %q, want %q", actualState, state)
	}
}

func TestRemoveFromQueueAbsent(t *testing.T) {
	client := mustRedisClient(t)
	repo := NewRedisRepository(client)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientID := uuid.NewString()

	id, err := matchmaking.NewClientID(clientID)
	if err != nil {
		t.Fatalf("create client ID: %v", err)
	}

	if err := repo.RemoveFromQueue(ctx, id); err != nil {
		t.Fatalf("error removing from queue: %v", err)
	}
}

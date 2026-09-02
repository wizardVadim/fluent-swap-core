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

//go:build integration

package repository

import (
	"context"
	"errors"
	matchmakingservice "github.com/wizardVadim/fluent-swap-core/internal/features/matchmaking/service"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
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

func TestRemoveFromQueueWithoutQueueKey(t *testing.T) {
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

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cleanupCancel()

		if err := repo.client.Del(cleanupCtx, keyIndex).Err(); err != nil {
			t.Errorf("remove_from_queue test Redis key index: %v", err)
		}
	})

	if err := repo.client.HSet(ctx, keyIndex, stateParam, state).Err(); err != nil {
		t.Fatalf("could not insert into hash: %v", err)
	}

	err = repo.RemoveFromQueue(ctx, id)
	if !errors.Is(err, errQueueKeyMissing) {
		t.Fatalf("unexpected error: got %v, want %v", err, errQueueKeyMissing)
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

func TestRemoveFromQueueInvalidState(t *testing.T) {
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
	state := "removing"
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

	err = repo.RemoveFromQueue(ctx, id)
	if !errors.Is(err, errInvalidClientState) {
		t.Fatalf("unexpected error: got %v, want %v", err, errInvalidClientState)
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

func TestRemoveFromQueueContextCanceled(t *testing.T) {
	client := mustRedisClient(t)
	repo := NewRedisRepository(client)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cancelledCtx, cancelCancelledContext := context.WithCancel(context.Background())
	defer cancelCancelledContext()

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

	cancelCancelledContext()

	err = repo.RemoveFromQueue(cancelledCtx, id)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected error: got %v, want %v", err, context.Canceled)
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

// Direct Lua tests cover script-specific arguments and state transitions.
type matchScriptFixture struct {
	t                        *testing.T
	client                   *redis.Client
	ctx                      context.Context
	prefix, forward, reverse string
	keys                     []string
}

func newMatchScriptFixture(t *testing.T) *matchScriptFixture {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	prefix := "fluent-swap:test:" + uuid.NewString() + ":"
	f := &matchScriptFixture{t: t, client: mustRedisClient(t), ctx: ctx, prefix: prefix + "client:", forward: prefix + "ru:en", reverse: prefix + "en:ru"}
	f.keys = []string{f.forward, f.reverse}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := f.client.Del(ctx, f.keys...).Err(); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	return f
}

func (f *matchScriptFixture) key(id string) string {
	key := f.prefix + id
	f.keys = append(f.keys, key)
	return key
}

func (f *matchScriptFixture) run(id, own, opposite string, waiting, matched any) (any, error) {
	f.t.Helper()
	return f.client.Eval(f.ctx, matchOrEnqueue, []string{f.key(id), opposite, own}, id, waiting, matched, f.prefix).Result()
}

func (f *matchScriptFixture) wantResult(id, own, opposite string, want ...any) {
	f.t.Helper()
	got, err := f.run(id, own, opposite, 60, 120)
	if err != nil {
		f.t.Fatalf("run script: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		f.t.Fatalf("response: got %#v, want %#v", got, want)
	}
}

func (f *matchScriptFixture) wantHash(id string, want map[string]string) {
	f.t.Helper()
	got, err := f.client.HGetAll(f.ctx, f.key(id)).Result()
	if err != nil {
		f.t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		f.t.Fatalf("hash %s: got %v, want %v", id, got, want)
	}
}

func (f *matchScriptFixture) wantQueue(key string, want ...string) {
	f.t.Helper()
	got, err := f.client.LRange(f.ctx, key, 0, -1).Result()
	if err != nil {
		f.t.Fatal(err)
	}
	if len(got) != len(want) {
		f.t.Fatalf("queue: got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			f.t.Fatalf("queue: got %v, want %v", got, want)
		}
	}
}

func (f *matchScriptFixture) wantTTL(id string, maximum time.Duration) {
	f.t.Helper()
	ttl, err := f.client.PTTL(f.ctx, f.key(id)).Result()
	if err != nil {
		f.t.Fatal(err)
	}
	if ttl <= maximum-10*time.Second || ttl > maximum {
		f.t.Fatalf("TTL %s: got %s, expected near %s", id, ttl, maximum)
	}
}

// Compare absolute expiration deadlines, not decreasing remaining TTLs.
func (f *matchScriptFixture) snapshot() map[string]any {
	f.t.Helper()
	result := map[string]any{}
	for _, key := range f.keys {
		value, err := f.client.Dump(f.ctx, key).Result()
		if err != nil && err != redis.Nil {
			f.t.Fatal(err)
		}
		expiry, err := f.client.Do(f.ctx, "PEXPIRETIME", key).Int64()
		if err != nil {
			f.t.Fatal(err)
		}
		result[key] = []any{value, expiry}
	}
	return result
}

func TestMatchOrEnqueueLuaEnqueueMatchAndRetry(t *testing.T) {
	f := newMatchScriptFixture(t)
	for _, id := range []string{"a", "b", "c"} {
		f.key(id)
	}
	f.wantResult("a", f.forward, f.reverse, int64(0))
	f.wantHash("a", map[string]string{"state": "waiting", "queue_key": f.forward})
	f.wantTTL("a", time.Minute)
	f.wantQueue(f.forward, "a")
	f.wantResult("c", f.forward, f.reverse, int64(0))
	before := f.snapshot()
	f.wantResult("a", f.forward, f.reverse, int64(0))
	f.wantResult("a", f.reverse, f.forward, int64(-1))
	if !reflect.DeepEqual(before, f.snapshot()) {
		t.Fatal("waiting retry/conflict changed Redis state or expiration")
	}
	// FIFO must choose a, although c was enqueued more recently.
	f.wantResult("b", f.reverse, f.forward, int64(1), "a", f.forward)
	f.wantQueue(f.forward, "c")
	f.wantQueue(f.reverse)
	f.wantHash("a", map[string]string{"state": "matched", "partner_client_id": "b", "partner_queue_key": f.reverse})
	f.wantHash("b", map[string]string{"state": "matched", "partner_client_id": "a", "partner_queue_key": f.forward})
	f.wantTTL("a", 2*time.Minute)
	f.wantTTL("b", 2*time.Minute)
	before = f.snapshot()
	// Even changed input settings must return the saved match.
	f.wantResult("a", f.reverse, f.forward, int64(1), "b", f.reverse)
	f.wantResult("b", f.reverse, f.forward, int64(1), "a", f.forward)
	if !reflect.DeepEqual(before, f.snapshot()) {
		t.Fatal("matched retry changed Redis state or expiration")
	}
}

func TestMatchOrEnqueueLuaInvalidClientState(t *testing.T) {
	cases := []struct {
		name   string
		fields map[string]string
	}{
		{"missing state", map[string]string{"queue_key": "queue"}},
		{"empty state", map[string]string{"state": ""}},
		{"unknown state", map[string]string{"state": "unknown"}},
		{"waiting missing queue", map[string]string{"state": "waiting"}},
		{"waiting empty queue", map[string]string{"state": "waiting", "queue_key": ""}},
		{"matched missing partner", map[string]string{"state": "matched", "partner_queue_key": "queue"}},
		{"matched empty partner", map[string]string{"state": "matched", "partner_client_id": "", "partner_queue_key": "queue"}},
		{"matched missing queue", map[string]string{"state": "matched", "partner_client_id": "b"}},
		{"matched empty queue", map[string]string{"state": "matched", "partner_client_id": "b", "partner_queue_key": ""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newMatchScriptFixture(t)
			key := f.key("a")
			if err := f.client.HSet(f.ctx, key, tc.fields).Err(); err != nil {
				t.Fatal(err)
			}
			if err := f.client.Expire(f.ctx, key, time.Minute).Err(); err != nil {
				t.Fatal(err)
			}
			before := f.snapshot()
			f.wantResult("a", f.forward, f.reverse, int64(-2))
			if !reflect.DeepEqual(before, f.snapshot()) {
				t.Fatal("invalid state handling mutated Redis")
			}
		})
	}
}

func TestMatchOrEnqueueLuaInvalidTTL(t *testing.T) {
	for _, arg := range []string{"waiting", "matched"} {
		for _, value := range []string{"0", "-1", "1.5", "invalid", ""} {
			t.Run(arg+"/"+value, func(t *testing.T) {
				f := newMatchScriptFixture(t)
				f.key("a")
				f.wantResult("b", f.reverse, f.forward, int64(0))
				before := f.snapshot()
				waiting, matched := "60", "120"
				if arg == "waiting" {
					waiting = value
				} else {
					matched = value
				}
				_, err := f.run("a", f.forward, f.reverse, waiting, matched)
				if err == nil {
					t.Fatal("expected invalid TTL error")
				}
				if !strings.Contains(err.Error(), "invalid "+arg+"TTL") {
					t.Fatalf("unexpected TTL error: %v", err)
				}
				if _, ok := err.(redis.Error); !ok {
					t.Fatalf("expected Redis error, got %T: %v", err, err)
				}
				if !reflect.DeepEqual(before, f.snapshot()) {
					t.Fatal("invalid TTL mutated Redis")
				}
			})
		}
	}
}

func TestMatchOrEnqueueLuaSkipsStaleCandidates(t *testing.T) {
	f := newMatchScriptFixture(t)
	f.key("expired") // Missing state models a client-key removed by TTL.
	for id, fields := range map[string]map[string]string{
		"matched": {"state": "matched", "partner_client_id": "other", "partner_queue_key": f.forward},
		"moved":   {"state": "waiting", "queue_key": f.forward},
	} {
		if err := f.client.HSet(f.ctx, f.key(id), fields).Err(); err != nil {
			t.Fatal(err)
		}
	}
	f.wantResult("live", f.reverse, f.forward, int64(0))
	// RPOP examines expired, matched, moved, and finally live.
	if err := f.client.RPush(f.ctx, f.reverse, "moved", "matched", "expired").Err(); err != nil {
		t.Fatal(err)
	}
	f.wantResult("a", f.forward, f.reverse, int64(1), "live", f.reverse)
	f.wantQueue(f.reverse)
	f.wantHash("matched", map[string]string{"state": "matched", "partner_client_id": "other", "partner_queue_key": f.forward})
	f.wantHash("moved", map[string]string{"state": "waiting", "queue_key": f.forward})
}

// Public repository tests use its fixed queue names. Run against a dedicated test
// Redis DB and never in parallel: refuse to touch pre-existing queues.
func newMatchRepositoryFixture(t *testing.T) (*matchScriptFixture, *RedisRepository) {
	t.Helper()
	f := newMatchScriptFixture(t)
	f.prefix = clientStateKeyPrefix
	f.forward, f.reverse = queueKeyPrefix+"ru:en", queueKeyPrefix+"en:ru"
	exists, err := f.client.Exists(f.ctx, f.forward, f.reverse).Result()
	if err != nil {
		t.Fatal(err)
	}
	if exists != 0 {
		t.Fatal("repository tests require empty matchmaking queues in a dedicated Redis test DB")
	}
	f.keys = append(f.keys, f.forward, f.reverse)
	return f, NewRedisRepository(f.client)
}

func repositoryTestUser(t *testing.T, id string, reverse bool) matchmaking.WaitingUser {
	t.Helper()
	clientID, err := matchmaking.NewClientID(id)
	if err != nil {
		t.Fatal(err)
	}
	ru, err := matchmaking.NewLanguage(matchmaking.LanguageCodeRU)
	if err != nil {
		t.Fatal(err)
	}
	en, err := matchmaking.NewLanguage(matchmaking.LanguageCodeEN)
	if err != nil {
		t.Fatal(err)
	}
	if reverse {
		ru, en = en, ru
	}
	pair, err := matchmaking.NewLanguagePair(ru, en)
	if err != nil {
		t.Fatal(err)
	}
	user, err := matchmaking.NewWaitingUser(clientID, pair)
	if err != nil {
		t.Fatal(err)
	}
	return user
}

func TestRedisMatchOrEnqueueLifecycle(t *testing.T) {
	f, repo := newMatchRepositoryFixture(t)
	a, b, c := uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, id := range []string{a, b, c} {
		f.key(id)
	}
	userA, userB, userC := repositoryTestUser(t, a, false), repositoryTestUser(t, b, true), repositoryTestUser(t, c, false)
	for _, user := range []matchmaking.WaitingUser{userA, userC} {
		result, err := repo.MatchOrEnqueue(f.ctx, user)
		if err != nil || result.Matched {
			t.Fatalf("enqueue: %+v, %v", result, err)
		}
	}
	f.wantQueue(f.forward, c, a)
	f.wantHash(a, map[string]string{"state": "waiting", "queue_key": f.forward})
	f.wantTTL(a, time.Duration(waitingSecondsTTL)*time.Second)
	before := f.snapshot()
	result, err := repo.MatchOrEnqueue(f.ctx, userA)
	if err != nil || result.Matched {
		t.Fatalf("retry: %+v, %v", result, err)
	}
	_, err = repo.MatchOrEnqueue(f.ctx, repositoryTestUser(t, a, true))
	if !errors.Is(err, matchmakingservice.ErrClientAlreadyQueued) {
		t.Fatalf("conflict: %v", err)
	}
	if !reflect.DeepEqual(before, f.snapshot()) {
		t.Fatal("waiting retry/conflict mutated Redis")
	}
	result, err = repo.MatchOrEnqueue(f.ctx, userB)
	if err != nil || !result.Matched || !reflect.DeepEqual(result.Partner, userA) {
		t.Fatalf("match: %+v, %v", result, err)
	}
	f.wantQueue(f.forward, c)
	f.wantQueue(f.reverse)
	f.wantHash(a, map[string]string{"state": "matched", "partner_client_id": b, "partner_queue_key": f.reverse})
	f.wantHash(b, map[string]string{"state": "matched", "partner_client_id": a, "partner_queue_key": f.forward})
	f.wantTTL(a, time.Duration(matchedSecondsTTL)*time.Second)
	f.wantTTL(b, time.Duration(matchedSecondsTTL)*time.Second)
	before = f.snapshot()
	result, err = repo.MatchOrEnqueue(f.ctx, repositoryTestUser(t, a, true))
	if err != nil || !result.Matched || !reflect.DeepEqual(result.Partner, userB) {
		t.Fatalf("saved match: %+v, %v", result, err)
	}
	if !reflect.DeepEqual(before, f.snapshot()) {
		t.Fatal("matched retry mutated Redis")
	}
}

func TestRedisMatchOrEnqueueCanceledContext(t *testing.T) {
	f, repo := newMatchRepositoryFixture(t)
	id := uuid.NewString()
	f.key(id)
	before := f.snapshot()
	ctx, cancel := context.WithCancel(f.ctx)
	cancel()
	_, err := repo.MatchOrEnqueue(ctx, repositoryTestUser(t, id, false))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled context: %v", err)
	}
	if !reflect.DeepEqual(before, f.snapshot()) {
		t.Fatal("canceled request mutated Redis")
	}
}

func TestRedisMatchOrEnqueueConcurrentRetry(t *testing.T) {
	f, repo := newMatchRepositoryFixture(t)
	a, b, c := uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, id := range []string{a, b, c} {
		f.key(id)
	}
	userA := repositoryTestUser(t, a, false)
	for _, id := range []string{b, c} {
		if _, err := repo.MatchOrEnqueue(f.ctx, repositoryTestUser(t, id, true)); err != nil {
			t.Fatal(err)
		}
	}
	// Separate Redis connections model two backend instances sharing the same DB.
	second := NewRedisRepository(mustRedisClient(t))
	const workers = 12
	results := make(chan matchmakingservice.MatchResult, workers)
	failures := make(chan error, workers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		target := repo
		if i%2 == 1 {
			target = second
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			result, err := target.MatchOrEnqueue(f.ctx, userA)
			results <- result
			failures <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	expected := repositoryTestUser(t, b, true)
	for result := range results {
		if !result.Matched || !reflect.DeepEqual(result.Partner, expected) {
			t.Fatalf("double match/inconsistent retry: %+v", result)
		}
	}
	f.wantQueue(f.reverse, c)
	f.wantHash(a, map[string]string{"state": "matched", "partner_client_id": b, "partner_queue_key": f.reverse})
}

func TestRedisMatchOrEnqueueCorruptedMarker(t *testing.T) {
	for _, queue := range []string{"", "bad", "wrong-prefix:en:ru", queueKeyPrefix + "en:invalid"} {
		t.Run(queue, func(t *testing.T) {
			f, repo := newMatchRepositoryFixture(t)
			id := uuid.NewString()
			if err := f.client.HSet(f.ctx, f.key(id), "state", "matched", "partner_client_id", uuid.NewString(), "partner_queue_key", queue).Err(); err != nil {
				t.Fatal(err)
			}
			before := f.snapshot()
			result, err := repo.MatchOrEnqueue(f.ctx, repositoryTestUser(t, id, false))
			if err == nil || result.Matched {
				t.Fatalf("corrupted marker accepted: %+v, %v", result, err)
			}
			if queue == "" && !errors.Is(err, errInvalidClientState) {
				t.Fatalf("invalid state mapping: %v", err)
			}
			if !reflect.DeepEqual(before, f.snapshot()) {
				t.Fatal("corrupted marker mutated Redis")
			}
		})
	}
}

func TestRedisMatchOrEnqueueConcurrentPartnerSelection(t *testing.T) {
	f, repo := newMatchRepositoryFixture(t)
	partner, a, b := uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, id := range []string{partner, a, b} {
		f.key(id)
	}
	waiting := repositoryTestUser(t, partner, true)
	if _, err := repo.MatchOrEnqueue(f.ctx, waiting); err != nil {
		t.Fatal(err)
	}
	second := NewRedisRepository(mustRedisClient(t))
	type outcome struct {
		id     string
		result matchmakingservice.MatchResult
		err    error
	}
	outcomes := make(chan outcome, 2)
	start := make(chan struct{})
	for i, id := range []string{a, b} {
		user := repositoryTestUser(t, id, false)
		target := repo
		if i == 1 {
			target = second
		}
		go func() {
			<-start
			result, err := target.MatchOrEnqueue(f.ctx, user)
			outcomes <- outcome{id, result, err}
		}()
	}
	close(start)
	winner, loser := "", ""
	for i := 0; i < 2; i++ {
		out := <-outcomes
		if out.err != nil {
			t.Errorf("concurrent request: %v", out.err)
			continue
		}
		if out.result.Matched {
			if winner != "" {
				t.Error("same partner assigned twice")
			}
			winner = out.id
			if !reflect.DeepEqual(out.result.Partner, waiting) {
				t.Errorf("unexpected partner: %+v", out.result.Partner)
			}
		} else {
			loser = out.id
		}
	}
	if t.Failed() {
		return
	}
	if winner == "" || loser == "" {
		t.Fatal("expected one match and one waiting client")
	}
	f.wantQueue(f.reverse)
	f.wantQueue(f.forward, loser)
	f.wantHash(partner, map[string]string{"state": "matched", "partner_client_id": winner, "partner_queue_key": f.forward})
	f.wantHash(winner, map[string]string{"state": "matched", "partner_client_id": partner, "partner_queue_key": f.reverse})
	f.wantHash(loser, map[string]string{"state": "waiting", "queue_key": f.forward})
}

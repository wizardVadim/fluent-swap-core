package repository

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/matchmaking"
)

func TestMemoryRepository_Enqueue(t *testing.T) {
	ctx := context.Background()
	repository := NewMemoryRepository()

	native := mustLanguage(t, matchmaking.LanguageCodeEN)
	other := mustLanguage(t, matchmaking.LanguageCodeRU)
	pair := mustLanguagePair(t, native, other)
	clientID := mustClientID(t, "test-identificator-1")
	wu := mustWaitingUser(t, clientID, pair)

	queueLength := len(repository.queue)

	result, err := repository.MatchOrEnqueue(ctx, wu)

	if err != nil {
		t.Fatalf("MatchOrEnqueue(%q, %q) returned unexpected error: %v", ctx, wu, err)
	}

	if result.Matched {
		t.Errorf("MatchResult.Matched got %v, want %v", result.Matched, false)
	}

	if got, want := len(repository.queue), queueLength+1; got != want {
		t.Fatalf("len(queue) = %d, want %d", got, want)
	}

	if repository.queue[0].ClientID() != wu.ClientID() {
		t.Errorf("clientID got %q, want %q", repository.queue[0].ClientID(), wu.ClientID())
	}
}

func TestMemoryRepository_MatchOrEnqueue(t *testing.T) {
	ctx := context.Background()
	repository := NewMemoryRepository()
	waiting := mustWaitingUserForCodes(t, "waiting", matchmaking.LanguageCodeRU, matchmaking.LanguageCodeEN)
	incoming := mustWaitingUserForCodes(t, "incoming", matchmaking.LanguageCodeEN, matchmaking.LanguageCodeRU)

	firstResult, err := repository.MatchOrEnqueue(ctx, waiting)
	if err != nil {
		t.Fatalf("enqueue waiting user: %v", err)
	}
	if firstResult.Matched {
		t.Fatal("first user unexpectedly matched")
	}

	result, err := repository.MatchOrEnqueue(ctx, incoming)
	if err != nil {
		t.Fatalf("match incoming user: %v", err)
	}
	if !result.Matched {
		t.Fatal("compatible user was not matched")
	}
	if !result.Partner.ClientID().IsEqual(waiting.ClientID()) {
		t.Errorf("partner ClientID = %q, want %q", result.Partner.ClientID().Value(), waiting.ClientID().Value())
	}
	if got := len(repository.queue); got != 0 {
		t.Errorf("len(queue) = %d, want 0", got)
	}
}

func TestMemoryRepository_SelectsFirstCompatibleUser(t *testing.T) {
	ctx := context.Background()
	repository := NewMemoryRepository()
	first := mustWaitingUserForCodes(t, "first", matchmaking.LanguageCodeRU, matchmaking.LanguageCodeEN)
	second := mustWaitingUserForCodes(t, "second", matchmaking.LanguageCodeRU, matchmaking.LanguageCodeEN)
	incoming := mustWaitingUserForCodes(t, "incoming", matchmaking.LanguageCodeEN, matchmaking.LanguageCodeRU)

	for _, user := range []matchmaking.WaitingUser{first, second} {
		result, err := repository.MatchOrEnqueue(ctx, user)
		if err != nil {
			t.Fatalf("enqueue %q: %v", user.ClientID().Value(), err)
		}
		if result.Matched {
			t.Fatalf("user %q unexpectedly matched", user.ClientID().Value())
		}
	}

	result, err := repository.MatchOrEnqueue(ctx, incoming)
	if err != nil {
		t.Fatalf("match incoming user: %v", err)
	}
	if !result.Matched {
		t.Fatal("compatible user was not matched")
	}
	if !result.Partner.ClientID().IsEqual(first.ClientID()) {
		t.Errorf("partner ClientID = %q, want first queued user %q", result.Partner.ClientID().Value(), first.ClientID().Value())
	}
	if got := len(repository.queue); got != 1 {
		t.Fatalf("len(queue) = %d, want 1", got)
	}
	if !repository.queue[0].ClientID().IsEqual(second.ClientID()) {
		t.Errorf("remaining ClientID = %q, want %q", repository.queue[0].ClientID().Value(), second.ClientID().Value())
	}
}

func TestMemoryRepository_DuplicateRequest(t *testing.T) {
	t.Run("same language pair is idempotent", func(t *testing.T) {
		repository := NewMemoryRepository()
		user := mustWaitingUserForCodes(t, "client", matchmaking.LanguageCodeRU, matchmaking.LanguageCodeEN)

		if _, err := repository.MatchOrEnqueue(context.Background(), user); err != nil {
			t.Fatalf("first request: %v", err)
		}
		result, err := repository.MatchOrEnqueue(context.Background(), user)
		if err != nil {
			t.Fatalf("duplicate request: %v", err)
		}
		if result.Matched {
			t.Fatal("duplicate request unexpectedly matched")
		}
		if got := len(repository.queue); got != 1 {
			t.Errorf("len(queue) = %d, want 1", got)
		}
	})

	t.Run("different language pair returns conflict", func(t *testing.T) {
		repository := NewMemoryRepository()
		original := mustWaitingUserForCodes(t, "client", matchmaking.LanguageCodeRU, matchmaking.LanguageCodeEN)
		changed := mustWaitingUserForCodes(t, "client", matchmaking.LanguageCodeEN, matchmaking.LanguageCodeRU)

		if _, err := repository.MatchOrEnqueue(context.Background(), original); err != nil {
			t.Fatalf("first request: %v", err)
		}
		_, err := repository.MatchOrEnqueue(context.Background(), changed)
		if !errors.Is(err, ErrClientAlreadyQueued) {
			t.Fatalf("duplicate request error = %v, want %v", err, ErrClientAlreadyQueued)
		}
		if got := len(repository.queue); got != 1 {
			t.Fatalf("len(queue) = %d, want 1", got)
		}
		if !repository.queue[0].LanguagePair().IsEqual(original.LanguagePair()) {
			t.Error("duplicate request changed the original language pair")
		}
	})
}

func TestMemoryRepository_RemoveFromQueue(t *testing.T) {
	t.Run("removes existing user", func(t *testing.T) {
		repository := NewMemoryRepository()
		user := mustWaitingUserForCodes(t, "client", matchmaking.LanguageCodeRU, matchmaking.LanguageCodeEN)

		if _, err := repository.MatchOrEnqueue(context.Background(), user); err != nil {
			t.Fatalf("enqueue user: %v", err)
		}
		if err := repository.RemoveFromQueue(context.Background(), user.ClientID()); err != nil {
			t.Fatalf("remove user: %v", err)
		}
		if got := len(repository.queue); got != 0 {
			t.Errorf("len(queue) = %d, want 0", got)
		}
	})

	t.Run("missing user is idempotent", func(t *testing.T) {
		repository := NewMemoryRepository()
		clientID := mustClientID(t, "missing")

		if err := repository.RemoveFromQueue(context.Background(), clientID); err != nil {
			t.Fatalf("remove missing user: %v", err)
		}
	})
}

func TestMemoryRepository_CanceledContextDoesNotMutateQueue(t *testing.T) {
	t.Run("match or enqueue", func(t *testing.T) {
		repository := NewMemoryRepository()
		user := mustWaitingUserForCodes(t, "client", matchmaking.LanguageCodeRU, matchmaking.LanguageCodeEN)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := repository.MatchOrEnqueue(ctx, user)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("MatchOrEnqueue error = %v, want %v", err, context.Canceled)
		}
		if got := len(repository.queue); got != 0 {
			t.Errorf("len(queue) = %d, want 0", got)
		}
	})

	t.Run("remove", func(t *testing.T) {
		repository := NewMemoryRepository()
		user := mustWaitingUserForCodes(t, "client", matchmaking.LanguageCodeRU, matchmaking.LanguageCodeEN)
		if _, err := repository.MatchOrEnqueue(context.Background(), user); err != nil {
			t.Fatalf("enqueue user: %v", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := repository.RemoveFromQueue(ctx, user.ClientID())
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("RemoveFromQueue error = %v, want %v", err, context.Canceled)
		}
		if got := len(repository.queue); got != 1 {
			t.Fatalf("len(queue) = %d, want 1", got)
		}
		if !repository.queue[0].ClientID().IsEqual(user.ClientID()) {
			t.Error("canceled removal changed the queued user")
		}
	})
}

func TestMemoryRepository_ConcurrentMatchDoesNotReusePartner(t *testing.T) {
	repository := NewMemoryRepository()
	waiting := mustWaitingUserForCodes(t, "waiting", matchmaking.LanguageCodeRU, matchmaking.LanguageCodeEN)
	firstIncoming := mustWaitingUserForCodes(t, "incoming-1", matchmaking.LanguageCodeEN, matchmaking.LanguageCodeRU)
	secondIncoming := mustWaitingUserForCodes(t, "incoming-2", matchmaking.LanguageCodeEN, matchmaking.LanguageCodeRU)

	if _, err := repository.MatchOrEnqueue(context.Background(), waiting); err != nil {
		t.Fatalf("enqueue waiting user: %v", err)
	}

	type callResult struct {
		requester matchmaking.ClientID
		match     MatchResult
		err       error
	}

	start := make(chan struct{})
	results := make(chan callResult, 2)
	var wg sync.WaitGroup
	for _, user := range []matchmaking.WaitingUser{firstIncoming, secondIncoming} {
		wg.Add(1)
		go func(user matchmaking.WaitingUser) {
			defer wg.Done()
			<-start
			result, err := repository.MatchOrEnqueue(context.Background(), user)
			results <- callResult{requester: user.ClientID(), match: result, err: err}
		}(user)
	}

	close(start)
	wg.Wait()
	close(results)

	matchedCount := 0
	for result := range results {
		if result.err != nil {
			t.Fatalf("MatchOrEnqueue for %q: %v", result.requester.Value(), result.err)
		}
		if result.match.Matched {
			matchedCount++
			if !result.match.Partner.ClientID().IsEqual(waiting.ClientID()) {
				t.Errorf("partner ClientID = %q, want %q", result.match.Partner.ClientID().Value(), waiting.ClientID().Value())
			}
		}
	}

	if matchedCount != 1 {
		t.Errorf("matched calls = %d, want 1", matchedCount)
	}
	if got := len(repository.queue); got != 1 {
		t.Fatalf("len(queue) = %d, want 1", got)
	}
}

func mustWaitingUserForCodes(
	t *testing.T,
	id string,
	native matchmaking.LanguageCode,
	learning matchmaking.LanguageCode,
) matchmaking.WaitingUser {
	t.Helper()

	clientID := mustClientID(t, id)
	nativeLanguage := mustLanguage(t, native)
	learningLanguage := mustLanguage(t, learning)
	pair := mustLanguagePair(t, nativeLanguage, learningLanguage)
	return mustWaitingUser(t, clientID, pair)
}

func mustWaitingUser(t *testing.T, clientID matchmaking.ClientID, languagePair matchmaking.LanguagePair) matchmaking.WaitingUser {
	t.Helper()

	wu, err := matchmaking.NewWaitingUser(clientID, languagePair)
	if err != nil {
		t.Fatalf("NewWaitingUser(%q, %q) returned unexpected error: %v", clientID, languagePair, err)
	}
	return wu
}

func mustClientID(t *testing.T, id string) matchmaking.ClientID {
	t.Helper()

	clientID, err := matchmaking.NewClientID(id)
	if err != nil {
		t.Fatalf("NewClientID(%q) returned unexpected error: %v", id, err)
	}
	return clientID
}

func mustLanguagePair(
	t *testing.T,
	native matchmaking.Language,
	learning matchmaking.Language,
) matchmaking.LanguagePair {
	t.Helper()

	pair, err := matchmaking.NewLanguagePair(native, learning)
	if err != nil {
		t.Fatalf("NewLanguagePair() returned unexpected error: %v", err)
	}

	return pair
}

func mustLanguage(t *testing.T, code matchmaking.LanguageCode) matchmaking.Language {
	t.Helper()

	language, err := matchmaking.NewLanguage(code)
	if err != nil {
		t.Fatalf("NewLanguage(%q) returned unexpected error: %v", code, err)
	}

	return language
}

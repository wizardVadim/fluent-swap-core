package service

import (
	"context"
	"errors"
	"testing"

	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/matchmaking"
	"github.com/wizardVadim/fluent-swap-core/internal/features/matchmaking/repository"
)

var _ repository.Repository = (*fakeRepository)(nil)

type fakeRepository struct {
	matchOrEnqueueFunc  func(context.Context, matchmaking.WaitingUser) (repository.MatchResult, error)
	removeFromQueueFunc func(context.Context, matchmaking.ClientID) error
}

func (f *fakeRepository) MatchOrEnqueue(
	ctx context.Context,
	user matchmaking.WaitingUser,
) (repository.MatchResult, error) {
	return f.matchOrEnqueueFunc(ctx, user)
}

func (f *fakeRepository) RemoveFromQueue(ctx context.Context, clientID matchmaking.ClientID) error {
	return f.removeFromQueueFunc(ctx, clientID)
}

func TestService_FindPartner(t *testing.T) {
	user := newWaitingUser(t, "client-1", matchmaking.LanguageCodeRU, matchmaking.LanguageCodeEN)
	partner := newWaitingUser(t, "client-2", matchmaking.LanguageCodeEN, matchmaking.LanguageCodeRU)
	expectedResult := repository.MatchResult{
		Partner: partner,
		Matched: true,
	}
	expectedCtx := context.WithValue(context.Background(), contextKey("request-id"), "request-1")

	repo := &fakeRepository{
		matchOrEnqueueFunc: func(ctx context.Context, actualUser matchmaking.WaitingUser) (repository.MatchResult, error) {
			if ctx != expectedCtx {
				t.Error("FindPartner() passed a different context")
			}
			if !actualUser.ClientID().IsEqual(user.ClientID()) || !actualUser.LanguagePair().IsEqual(user.LanguagePair()) {
				t.Errorf("FindPartner() user = %#v, want %#v", actualUser, user)
			}

			return expectedResult, nil
		},
	}

	result, err := New(repo).FindPartner(expectedCtx, user)
	if err != nil {
		t.Fatalf("FindPartner() error = %v, want nil", err)
	}
	if result.Matched != expectedResult.Matched {
		t.Errorf("FindPartner() Matched = %v, want %v", result.Matched, expectedResult.Matched)
	}
	if !result.Partner.ClientID().IsEqual(expectedResult.Partner.ClientID()) ||
		!result.Partner.LanguagePair().IsEqual(expectedResult.Partner.LanguagePair()) {
		t.Errorf("FindPartner() Partner = %#v, want %#v", result.Partner, expectedResult.Partner)
	}
}

func TestService_FindPartnerReturnsRepositoryError(t *testing.T) {
	user := newWaitingUser(t, "client-1", matchmaking.LanguageCodeRU, matchmaking.LanguageCodeEN)
	expectedErr := errors.New("repository failure")

	repo := &fakeRepository{
		matchOrEnqueueFunc: func(context.Context, matchmaking.WaitingUser) (repository.MatchResult, error) {
			return repository.MatchResult{}, expectedErr
		},
	}

	_, err := New(repo).FindPartner(context.Background(), user)
	if !errors.Is(err, expectedErr) {
		t.Errorf("FindPartner() error = %v, want %v", err, expectedErr)
	}
}

func TestService_CancelSearch(t *testing.T) {
	clientID := newClientID(t, "client-1")
	expectedCtx := context.WithValue(context.Background(), contextKey("request-id"), "request-1")

	repo := &fakeRepository{
		removeFromQueueFunc: func(ctx context.Context, actualClientID matchmaking.ClientID) error {
			if ctx != expectedCtx {
				t.Error("CancelSearch() passed a different context")
			}
			if !actualClientID.IsEqual(clientID) {
				t.Errorf("CancelSearch() clientID = %q, want %q", actualClientID.Value(), clientID.Value())
			}

			return nil
		},
	}

	if err := New(repo).CancelSearch(expectedCtx, clientID); err != nil {
		t.Fatalf("CancelSearch() error = %v, want nil", err)
	}
}

func TestService_CancelSearchReturnsRepositoryError(t *testing.T) {
	clientID := newClientID(t, "client-1")
	expectedErr := errors.New("repository failure")

	repo := &fakeRepository{
		removeFromQueueFunc: func(context.Context, matchmaking.ClientID) error {
			return expectedErr
		},
	}

	err := New(repo).CancelSearch(context.Background(), clientID)
	if !errors.Is(err, expectedErr) {
		t.Errorf("CancelSearch() error = %v, want %v", err, expectedErr)
	}
}

type contextKey string

func newWaitingUser(
	t *testing.T,
	clientID string,
	nativeLanguage matchmaking.LanguageCode,
	learningLanguage matchmaking.LanguageCode,
) matchmaking.WaitingUser {
	t.Helper()

	native := newLanguage(t, nativeLanguage)
	learning := newLanguage(t, learningLanguage)
	pair, err := matchmaking.NewLanguagePair(native, learning)
	if err != nil {
		t.Fatalf("NewLanguagePair() error = %v", err)
	}

	user, err := matchmaking.NewWaitingUser(newClientID(t, clientID), pair)
	if err != nil {
		t.Fatalf("NewWaitingUser() error = %v", err)
	}

	return user
}

func newClientID(t *testing.T, value string) matchmaking.ClientID {
	t.Helper()

	clientID, err := matchmaking.NewClientID(value)
	if err != nil {
		t.Fatalf("NewClientID() error = %v", err)
	}

	return clientID
}

func newLanguage(t *testing.T, code matchmaking.LanguageCode) matchmaking.Language {
	t.Helper()

	language, err := matchmaking.NewLanguage(code)
	if err != nil {
		t.Fatalf("NewLanguage() error = %v", err)
	}

	return language
}

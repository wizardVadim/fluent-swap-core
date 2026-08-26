package matchmaking_test

import (
	"errors"
	"testing"

	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/matchmaking"
)

func TestNewClientID(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantValue string
		wantErr   error
	}{
		{
			name:      "valid ID",
			input:     "client-1",
			wantValue: "client-1",
		},
		{
			name:      "trim surrounding spaces and preserve case",
			input:     "  Client-A  ",
			wantValue: "Client-A",
		},
		{
			name:    "empty ID",
			input:   "",
			wantErr: matchmaking.ErrInvalidClientID,
		},
		{
			name:    "spaces only",
			input:   "   ",
			wantErr: matchmaking.ErrInvalidClientID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientID, err := matchmaking.NewClientID(tt.input)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("NewClientID() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("NewClientID() returned unexpected error: %v", err)
			}

			if clientID.Value() != tt.wantValue {
				t.Errorf("Value() = %q, want %q", clientID.Value(), tt.wantValue)
			}
		})
	}
}

func TestNewWaitingUser(t *testing.T) {
	clientID := mustClientID(t, "client-1")
	languagePair := mustLanguagePair(
		t,
		mustLanguage(t, matchmaking.LanguageCodeRU),
		mustLanguage(t, matchmaking.LanguageCodeEN),
	)

	waitingUser, err := matchmaking.NewWaitingUser(clientID, languagePair)
	if err != nil {
		t.Fatalf("NewWaitingUser() returned unexpected error: %v", err)
	}

	if waitingUser.ClientID() != clientID {
		t.Errorf("ClientID() = %v, want %v", waitingUser.ClientID(), clientID)
	}

	if waitingUser.LanguagePair() != languagePair {
		t.Errorf("LanguagePair() = %v, want %v", waitingUser.LanguagePair(), languagePair)
	}
}

func TestNewWaitingUser_InvalidValue(t *testing.T) {
	validClientID := mustClientID(t, "client-1")
	validLanguagePair := mustLanguagePair(
		t,
		mustLanguage(t, matchmaking.LanguageCodeRU),
		mustLanguage(t, matchmaking.LanguageCodeEN),
	)

	tests := []struct {
		name         string
		clientID     matchmaking.ClientID
		languagePair matchmaking.LanguagePair
		wantErr      error
	}{
		{
			name:         "zero-value client ID",
			languagePair: validLanguagePair,
			wantErr:      matchmaking.ErrInvalidClientID,
		},
		{
			name:     "zero-value language pair",
			clientID: validClientID,
			wantErr:  matchmaking.ErrInvalidLanguageCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := matchmaking.NewWaitingUser(tt.clientID, tt.languagePair)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewWaitingUser() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func mustClientID(t *testing.T, value string) matchmaking.ClientID {
	t.Helper()

	clientID, err := matchmaking.NewClientID(value)
	if err != nil {
		t.Fatalf("NewClientID(%q) returned unexpected error: %v", value, err)
	}

	return clientID
}

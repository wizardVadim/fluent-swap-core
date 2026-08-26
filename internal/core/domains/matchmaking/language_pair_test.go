package matchmaking_test

import (
	"errors"
	"testing"

	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/matchmaking"
)

func TestNewLanguage(t *testing.T) {
	tests := []struct {
		name     string
		input    matchmaking.LanguageCode
		wantCode matchmaking.LanguageCode
		wantErr  error
	}{
		{name: "Russian code", input: matchmaking.LanguageCodeRU, wantCode: matchmaking.LanguageCodeRU},
		{name: "English code", input: matchmaking.LanguageCodeEN, wantCode: matchmaking.LanguageCodeEN},
		{name: "normalized Russian code", input: " Ru ", wantCode: matchmaking.LanguageCodeRU},
		{name: "normalized English code", input: " eN", wantCode: matchmaking.LanguageCodeEN},
		{name: "unsupported code", input: "du", wantErr: matchmaking.ErrInvalidLanguageCode},
		{name: "normalized unsupported code", input: "dU   ", wantErr: matchmaking.ErrInvalidLanguageCode},
		{name: "spaces only", input: "   ", wantErr: matchmaking.ErrInvalidLanguageCode},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			language, err := matchmaking.NewLanguage(tt.input)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("NewLanguage() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("NewLanguage() returned unexpected error: %v", err)
			}

			if language.Code() != tt.wantCode {
				t.Errorf("Code() = %q, want %q", language.Code(), tt.wantCode)
			}
		})
	}
}

func TestNewLanguagePair_SameLanguages(t *testing.T) {
	tests := []struct {
		name    string
		input1  matchmaking.Language
		input2  matchmaking.Language
		wantErr error
	}{
		{
			name:    "Same languages",
			input1:  mustLanguage(t, matchmaking.LanguageCodeRU),
			input2:  mustLanguage(t, matchmaking.LanguageCodeRU),
			wantErr: matchmaking.ErrEqualLanguages,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := matchmaking.NewLanguagePair(tt.input1, tt.input2)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("NewLanguagePair() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("NewLanguagePair() returned unexpected error: %v", err)
			}

		})
	}
}

func TestNewLanguagePair_IsCompatibleWith(t *testing.T) {
	tests := []struct {
		name           string
		input1         matchmaking.Language
		input2         matchmaking.Language
		input3         matchmaking.Language
		input4         matchmaking.Language
		wantCompatible bool
	}{
		{
			name:           "Compatatible languages",
			input1:         mustLanguage(t, matchmaking.LanguageCodeRU),
			input2:         mustLanguage(t, matchmaking.LanguageCodeEN),
			input3:         mustLanguage(t, matchmaking.LanguageCodeEN),
			input4:         mustLanguage(t, matchmaking.LanguageCodeRU),
			wantCompatible: true,
		},
		{
			name:           "Not compatible languages",
			input1:         mustLanguage(t, matchmaking.LanguageCodeRU),
			input2:         mustLanguage(t, matchmaking.LanguageCodeEN),
			input3:         mustLanguage(t, matchmaking.LanguageCodeRU),
			input4:         mustLanguage(t, matchmaking.LanguageCodeEN),
			wantCompatible: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lp1 := mustLanguagePair(t, tt.input1, tt.input2)

			lp2 := mustLanguagePair(t, tt.input3, tt.input4)

			compatible := lp2.IsCompatibleWith(lp1)

			if compatible != tt.wantCompatible {
				t.Fatalf("IsCompatibleWith() result: %v, want: %v", compatible, tt.wantCompatible)
			}

		})
	}
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

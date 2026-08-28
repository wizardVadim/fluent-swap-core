package chat_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/chat"
)

func TestNewMessageText(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantValue string
		wantErr   error
	}{
		{
			name:      "valid text",
			input:     "Hello!",
			wantValue: "Hello!",
		},
		{
			name:      "preserves surrounding whitespace",
			input:     "  Hello!\t",
			wantValue: "  Hello!\t",
		},
		{
			name:    "empty text",
			input:   "",
			wantErr: chat.ErrInvalidText,
		},
		{
			name:    "unicode whitespace only",
			input:   " \t\n\u2003 ",
			wantErr: chat.ErrInvalidText,
		},
		{
			name:      "exactly 4096 ASCII bytes",
			input:     strings.Repeat("a", 4096),
			wantValue: strings.Repeat("a", 4096),
		},
		{
			name:    "more than 4096 bytes",
			input:   strings.Repeat("a", 4097),
			wantErr: chat.ErrInvalidText,
		},
		{
			name:      "multibyte text is limited by bytes",
			input:     strings.Repeat("я", 2048),
			wantValue: strings.Repeat("я", 2048),
		},
		{
			name:    "multibyte text exceeding byte limit",
			input:   strings.Repeat("я", 2049),
			wantErr: chat.ErrInvalidText,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, err := chat.NewMessageText(tt.input)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("NewMessageText() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("NewMessageText() returned unexpected error: %v", err)
			}
			if text.Value() != tt.wantValue {
				t.Errorf("Value() = %q, want %q", text.Value(), tt.wantValue)
			}
			if !text.IsValid() {
				t.Error("IsValid() = false, want true")
			}
		})
	}
}

func TestMessageTextZeroValueIsInvalid(t *testing.T) {
	if (chat.MessageText{}).IsValid() {
		t.Error("zero MessageText IsValid() = true, want false")
	}
}

package chat

import (
	"fmt"
	"strings"
)

const maxTextLength = 4096

type MessageText struct {
	value string
}

func NewMessageText(text string) (MessageText, error) {
	mText := MessageText{
		value: text,
	}
	if err := mText.validate(); err != nil {
		return MessageText{}, err
	}
	return mText, nil
}

func (mText MessageText) validate() error {
	text := mText.value
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("%w; cause: %s", ErrInvalidText, "empty")
	}
	textLen := len(text)
	if textLen > maxTextLength {
		return fmt.Errorf("%w; text length: %d, max: %d", ErrInvalidText, textLen, maxTextLength)
	}
	return nil
}

func (mText MessageText) Value() string {
	return mText.value
}

func (mText MessageText) IsValid() bool {
	if err := mText.validate(); err != nil {
		return false
	}
	return true
}

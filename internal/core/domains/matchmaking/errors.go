package matchmaking

import "errors"

var (
	ErrInvalidLanguageCode = errors.New("invalid language code")
	ErrEqualLanguages      = errors.New("languages are same")
)

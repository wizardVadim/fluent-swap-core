package matchmaking

import (
	"strings"
)

type LanguageCode string

type Language struct {
	code LanguageCode // Example: "ru", "en", etc.
}

const (
	LanguageCodeRU LanguageCode = "ru"
	LanguageCodeEN LanguageCode = "en"
)

func NewLanguage(code LanguageCode) (Language, error) {
	language := Language{code: LanguageCode(strings.ToLower(strings.TrimSpace(string(code))))}

	if err := language.validate(); err != nil {
		return Language{}, err
	}

	return language, nil
}

func (l Language) Code() LanguageCode {
	return l.code
}

func (l Language) validate() error {
	switch l.code {
	case LanguageCodeEN, LanguageCodeRU:
		return nil
	default:
		return ErrInvalidLanguageCode
	}
}

func (l Language) IsEqual(comparable Language) bool {
	return l.code == comparable.code
}

type LanguagePair struct {
	nativeLanguage   Language
	learningLanguage Language
}

func NewLanguagePair(nativeLanguage Language, learningLanguage Language) (LanguagePair, error) {
	languagePair := LanguagePair{
		nativeLanguage:   nativeLanguage,
		learningLanguage: learningLanguage,
	}

	if err := languagePair.validate(); err != nil {
		return LanguagePair{}, err
	}

	return languagePair, nil
}

func (lp LanguagePair) validate() error {
	if err := lp.nativeLanguage.validate(); err != nil {
		return err
	}

	if err := lp.learningLanguage.validate(); err != nil {
		return err
	}

	if lp.nativeLanguage.IsEqual(lp.learningLanguage) {
		return ErrEqualLanguages
	}

	return nil
}

func (lp LanguagePair) NativeLanguage() Language {
	return lp.nativeLanguage
}

func (lp LanguagePair) LearningLanguage() Language {
	return lp.learningLanguage
}

func (lp LanguagePair) IsCompatibleWith(lp2 LanguagePair) bool {
	if lp.learningLanguage.IsEqual(lp2.nativeLanguage) && lp.nativeLanguage.IsEqual(lp2.learningLanguage) {
		return true
	}

	return false
}

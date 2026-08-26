package matchmaking

import "strings"

type ClientID struct {
	value string
}

func (id ClientID) validate() error {
	if strings.TrimSpace(id.value) == "" {
		return ErrInvalidClientID
	}
	return nil
}

func NewClientID(id string) (ClientID, error) {
	clientID := ClientID{
		value: strings.TrimSpace(id),
	}

	if err := clientID.validate(); err != nil {
		return ClientID{}, err
	}

	return clientID, nil
}

func (id ClientID) Value() string {
	return id.value
}

type WaitingUser struct {
	clientID     ClientID
	languagePair LanguagePair
}

func NewWaitingUser(clientID ClientID, languagePair LanguagePair) (WaitingUser, error) {

	waitingUser := WaitingUser{
		clientID:     clientID,
		languagePair: languagePair,
	}

	if err := waitingUser.validate(); err != nil {
		return WaitingUser{}, err
	}

	return waitingUser, nil
}

func (wu WaitingUser) validate() error {
	if err := wu.clientID.validate(); err != nil {
		return err
	}
	if err := wu.languagePair.validate(); err != nil {
		return err
	}
	return nil
}

func (wu WaitingUser) ClientID() ClientID {
	return wu.clientID
}

func (wu WaitingUser) LanguagePair() LanguagePair {
	return wu.languagePair
}

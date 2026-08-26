package repository

import "errors"

var (
	ErrClientAlreadyQueued = errors.New("user is already in the queue using another language pair")
)

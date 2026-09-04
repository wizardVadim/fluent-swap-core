package repository

import "errors"

var (
	errInvalidRedisResponseCode = errors.New("invalid redis response code")
	errQueueKeyMissing          = errors.New("queue_key is missing")
	errInvalidClientState       = errors.New("invalid client hash state")
)

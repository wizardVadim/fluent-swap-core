package service

import "errors"

var (
	ErrSenderNotInRoom      = errors.New("client doesn't have a room")
	ErrRoomMismatch         = errors.New("room ID does not match sender's active room")
	ErrRecipientUnavailable = errors.New("recipient is unavailable")
)

package room

import "errors"

var (
	ErrInvalidRoomID       = errors.New("invalid room ID")
	ErrSameClientInOneRoom = errors.New("same client is trying to connect one room")
	ErrClientNotInRoom     = errors.New("current client doesn't exist in the room")
)

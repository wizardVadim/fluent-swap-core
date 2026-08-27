package room

import "errors"

var (
	ErrInvalidRoomID       = errors.New("invalid room ID")
	ErrSameClientInOneRoom = errors.New("same client is trying to connect one room")
)

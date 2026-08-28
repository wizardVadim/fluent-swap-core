package repository

import "errors"

var (
	errInvalidRoomIndex = errors.New("client room index points to a missing room")
)

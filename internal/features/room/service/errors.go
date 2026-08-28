package service

import "errors"

var (
	ErrRoomAlreadyExists   = errors.New("room already exists")
	ErrClientAlreadyInRoom = errors.New("client already belongs to an active room")
)

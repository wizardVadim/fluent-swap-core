package service

import (
	"github.com/google/uuid"
	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/room"
)

func GenerateRoomID() (room.RoomID, error) {
	value := uuid.NewString()
	roomID, err := room.NewRoomID(value)
	if err != nil {
		return room.RoomID{}, err
	}
	return roomID, nil
}

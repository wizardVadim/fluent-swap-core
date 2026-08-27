package service

import "github.com/wizardVadim/fluent-swap-core/internal/core/domains/room"

type RoomIDGenerator func() (room.RoomID, error)

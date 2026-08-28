package service

import (
	"context"

	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/matchmaking"
	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/room"
)

// If context has been cancelled, an operation will return ctx.Err() and it won't change the state
type Repository interface {
	// Create atomically persists target if its ID is unused and neither
	// participant belongs to another active room.
	// It returns ErrRoomAlreadyExists or ErrClientAlreadyInRoom on conflict.
	Create(ctx context.Context, target room.Room) error

	// FindByClientID returns the client's active room.
	// It returns found=false and a zero Room when no room exists.
	FindByClientID(
		ctx context.Context,
		clientID matchmaking.ClientID,
	) (room.Room, bool, error)

	// Delete atomically removes the room and both participant associations.
	// It returns nil when the room does not exist.
	Delete(ctx context.Context, roomID room.RoomID) error
}

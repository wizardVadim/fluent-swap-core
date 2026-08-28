package service

import (
	"context"

	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/chat"
	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/matchmaking"
	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/room"
)

type Delivery interface {
	Deliver(ctx context.Context, recipientID matchmaking.ClientID, roomID room.RoomID, text chat.MessageText) error
}

type RoomFinder interface {
	FindRoomByClientID(ctx context.Context, senderID matchmaking.ClientID) (room.Room, bool, error)
}

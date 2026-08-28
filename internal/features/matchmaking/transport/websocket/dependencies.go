package websocket

import (
	"context"

	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/matchmaking"
	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/room"
	matchmakingservice "github.com/wizardVadim/fluent-swap-core/internal/features/matchmaking/service"
)

type ClientIDGenerator func() (matchmaking.ClientID, error)

type Service interface {
	FindPartner(ctx context.Context, user matchmaking.WaitingUser) (matchmakingservice.MatchResult, error)
	CancelSearch(ctx context.Context, clientID matchmaking.ClientID) error
}

type RoomService interface {
	CreateRoom(ctx context.Context, firstClientID matchmaking.ClientID, secondClientID matchmaking.ClientID) (room.Room, error)
	CloseRoom(ctx context.Context, roomID room.RoomID) error
	FindRoomByClientID(ctx context.Context, clientID matchmaking.ClientID) (room.Room, bool, error)
}

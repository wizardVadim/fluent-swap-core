package service

import (
	"context"

	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/matchmaking"
	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/room"
)

type Service struct {
	repository      Repository
	roomIDGenerator RoomIDGenerator
}

func New(repository Repository, roomIDGenerator RoomIDGenerator) *Service {
	return &Service{
		repository:      repository,
		roomIDGenerator: roomIDGenerator,
	}
}

func (s *Service) CreateRoom(
	ctx context.Context,
	firstClientID matchmaking.ClientID,
	secondClientID matchmaking.ClientID,
) (room.Room, error) {
	if err := ctx.Err(); err != nil {
		return room.Room{}, err
	}

	clients, err := room.NewConnectedClientsPair(firstClientID, secondClientID)
	if err != nil {
		return room.Room{}, err
	}

	roomID, err := s.roomIDGenerator()
	if err != nil {
		return room.Room{}, err
	}

	inputRoom, err := room.NewRoom(clients, roomID)
	if err != nil {
		return room.Room{}, err
	}

	if err := s.repository.Create(ctx, inputRoom); err != nil {
		return room.Room{}, err
	}
	return inputRoom, nil
}

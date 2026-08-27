package repository

import (
	"context"
	"sync"

	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/matchmaking"
	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/room"
	"github.com/wizardVadim/fluent-swap-core/internal/features/room/service"
)

var _ service.Repository = (*MemoryRepository)(nil)

type roomStorage map[room.RoomID]room.Room
type clientsRoomsStorage map[matchmaking.ClientID]room.RoomID

type MemoryRepository struct {
	rooms        roomStorage
	clientsRooms clientsRoomsStorage
	mtx          sync.RWMutex
}

func NewMemoryRepository() *MemoryRepository {
	repo := MemoryRepository{
		rooms:        make(roomStorage),
		clientsRooms: make(clientsRoomsStorage),
	}
	return &repo
}

func (repository *MemoryRepository) Create(ctx context.Context, target room.Room) error {
	repository.mtx.Lock()
	defer repository.mtx.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}

	if _, exists := repository.rooms[target.RoomID()]; exists {
		return service.ErrRoomAlreadyExists
	}

	clients := target.Clients()
	firstClientID := clients.FirstClientID()
	secondClientID := clients.SecondClientID()

	if _, exists := repository.clientsRooms[firstClientID]; exists {
		return service.ErrClientAlreadyInRoom
	}
	if _, exists := repository.clientsRooms[secondClientID]; exists {
		return service.ErrClientAlreadyInRoom
	}

	repository.rooms[target.RoomID()] = target
	repository.clientsRooms[firstClientID] = target.RoomID()
	repository.clientsRooms[secondClientID] = target.RoomID()

	return nil
}

func (repository *MemoryRepository) FindByClientID(
	ctx context.Context,
	clientID matchmaking.ClientID,
) (room.Room, bool, error) {
	repository.mtx.RLock()
	defer repository.mtx.RUnlock()

	if err := ctx.Err(); err != nil {
		return room.Room{}, false, err
	}

	roomID, exists := repository.clientsRooms[clientID]
	if !exists {
		return room.Room{}, false, nil
	}

	foundRoom, exists := repository.rooms[roomID]
	if !exists {
		return room.Room{}, false, errInvalidRoomIndex
	}

	return foundRoom, true, nil
}

func (repository *MemoryRepository) Delete(ctx context.Context, roomID room.RoomID) error {
	repository.mtx.Lock()
	defer repository.mtx.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}

	foundRoom, exists := repository.rooms[roomID]
	if !exists {
		return nil
	}
	delete(repository.rooms, roomID)

	firstClientID := foundRoom.Clients().FirstClientID()
	secondClientID := foundRoom.Clients().SecondClientID()

	_, exists = repository.clientsRooms[firstClientID]
	if exists {
		delete(repository.clientsRooms, firstClientID)
	}

	_, exists = repository.clientsRooms[secondClientID]
	if exists {
		delete(repository.clientsRooms, secondClientID)
	}

	return nil
}

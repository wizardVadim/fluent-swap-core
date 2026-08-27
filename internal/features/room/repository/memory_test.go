package repository

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/matchmaking"
	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/room"
	"github.com/wizardVadim/fluent-swap-core/internal/features/room/service"
)

func TestMemoryRepositoryCreateAndFindByBothClients(t *testing.T) {
	repository := NewMemoryRepository()
	target := mustRoom(t, "room-1", "client-1", "client-2")

	if err := repository.Create(context.Background(), target); err != nil {
		t.Fatalf("Create() returned unexpected error: %v", err)
	}

	clients := target.Clients()
	assertRoomFound(t, repository, clients.FirstClientID(), target)
	assertRoomFound(t, repository, clients.SecondClientID(), target)
}

func TestMemoryRepositoryCreateRejectsDuplicateRoomIDWithoutPartialWrite(t *testing.T) {
	repository := NewMemoryRepository()
	existingRoom := mustRoom(t, "room-1", "client-1", "client-2")
	conflictingRoom := mustRoom(t, "room-1", "client-3", "client-4")

	if err := repository.Create(context.Background(), existingRoom); err != nil {
		t.Fatalf("create existing room: %v", err)
	}

	err := repository.Create(context.Background(), conflictingRoom)
	if !errors.Is(err, service.ErrRoomAlreadyExists) {
		t.Fatalf("Create() error = %v, want %v", err, service.ErrRoomAlreadyExists)
	}

	assertRoomFound(t, repository, existingRoom.Clients().FirstClientID(), existingRoom)
	assertRoomNotFound(t, repository, conflictingRoom.Clients().FirstClientID())
	assertRoomNotFound(t, repository, conflictingRoom.Clients().SecondClientID())
}

func TestMemoryRepositoryCreateRejectsOccupiedClientWithoutPartialWrite(t *testing.T) {
	tests := []struct {
		name              string
		conflictingFirst  string
		conflictingSecond string
		freeClient        string
	}{
		{
			name:              "first client is occupied",
			conflictingFirst:  "client-1",
			conflictingSecond: "client-3",
			freeClient:        "client-3",
		},
		{
			name:              "second client is occupied",
			conflictingFirst:  "client-3",
			conflictingSecond: "client-2",
			freeClient:        "client-3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewMemoryRepository()
			existingRoom := mustRoom(t, "room-1", "client-1", "client-2")
			conflictingRoom := mustRoom(
				t,
				"room-2",
				tt.conflictingFirst,
				tt.conflictingSecond,
			)

			if err := repository.Create(context.Background(), existingRoom); err != nil {
				t.Fatalf("create existing room: %v", err)
			}

			err := repository.Create(context.Background(), conflictingRoom)
			if !errors.Is(err, service.ErrClientAlreadyInRoom) {
				t.Fatalf("Create() error = %v, want %v", err, service.ErrClientAlreadyInRoom)
			}

			assertRoomFound(t, repository, existingRoom.Clients().FirstClientID(), existingRoom)
			assertRoomNotFound(t, repository, mustClientID(t, tt.freeClient))
		})
	}
}

func TestMemoryRepositoryFindByClientIDReturnsNotFound(t *testing.T) {
	repository := NewMemoryRepository()

	assertRoomNotFound(t, repository, mustClientID(t, "client-1"))
}

func TestMemoryRepositoryDeleteRemovesRoomAndReleasesBothClients(t *testing.T) {
	repository := NewMemoryRepository()
	target := mustRoom(t, "room-1", "client-1", "client-2")

	if err := repository.Create(context.Background(), target); err != nil {
		t.Fatalf("Create() returned unexpected error: %v", err)
	}
	if err := repository.Delete(context.Background(), target.RoomID()); err != nil {
		t.Fatalf("Delete() returned unexpected error: %v", err)
	}

	clients := target.Clients()
	assertRoomNotFound(t, repository, clients.FirstClientID())
	assertRoomNotFound(t, repository, clients.SecondClientID())

	firstClientRoom := mustRoom(t, "room-2", "client-1", "client-3")
	if err := repository.Create(context.Background(), firstClientRoom); err != nil {
		t.Fatalf("Create() with released first client returned unexpected error: %v", err)
	}

	secondClientRoom := mustRoom(t, "room-3", "client-2", "client-4")
	if err := repository.Create(context.Background(), secondClientRoom); err != nil {
		t.Fatalf("Create() with released second client returned unexpected error: %v", err)
	}
}

func TestMemoryRepositoryDeleteIsIdempotent(t *testing.T) {
	repository := NewMemoryRepository()
	roomID := mustRoomID(t, "room-1")

	if err := repository.Delete(context.Background(), roomID); err != nil {
		t.Fatalf("first Delete() returned unexpected error: %v", err)
	}
	if err := repository.Delete(context.Background(), roomID); err != nil {
		t.Fatalf("second Delete() returned unexpected error: %v", err)
	}
}

func TestMemoryRepositoryCancelledContextDoesNotChangeState(t *testing.T) {
	repository := NewMemoryRepository()
	existingRoom := mustRoom(t, "room-1", "client-1", "client-2")
	if err := repository.Create(context.Background(), existingRoom); err != nil {
		t.Fatalf("create existing room: %v", err)
	}

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	newRoom := mustRoom(t, "room-2", "client-3", "client-4")
	if err := repository.Create(cancelledCtx, newRoom); !errors.Is(err, context.Canceled) {
		t.Fatalf("Create() error = %v, want %v", err, context.Canceled)
	}
	assertRoomNotFound(t, repository, newRoom.Clients().FirstClientID())

	gotRoom, found, err := repository.FindByClientID(
		cancelledCtx,
		existingRoom.Clients().FirstClientID(),
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("FindByClientID() error = %v, want %v", err, context.Canceled)
	}
	if found {
		t.Errorf("FindByClientID() found = true, want false for cancelled context")
	}
	if gotRoom != (room.Room{}) {
		t.Errorf("FindByClientID() room = %v, want zero Room", gotRoom)
	}

	if err := repository.Delete(cancelledCtx, existingRoom.RoomID()); !errors.Is(err, context.Canceled) {
		t.Fatalf("Delete() error = %v, want %v", err, context.Canceled)
	}
	assertRoomFound(t, repository, existingRoom.Clients().FirstClientID(), existingRoom)
}

func TestMemoryRepositoryConcurrentCreateAllowsOnlyOneRoomPerClient(t *testing.T) {
	repository := NewMemoryRepository()
	firstRoom := mustRoom(t, "room-1", "shared-client", "client-1")
	secondRoom := mustRoom(t, "room-2", "client-2", "shared-client")

	start := make(chan struct{})
	results := make(chan error, 2)
	var workers sync.WaitGroup

	for _, target := range []room.Room{firstRoom, secondRoom} {
		workers.Add(1)
		go func(target room.Room) {
			defer workers.Done()
			<-start
			results <- repository.Create(context.Background(), target)
		}(target)
	}

	close(start)
	workers.Wait()
	close(results)

	var successfulCreates int
	var clientConflicts int
	for err := range results {
		switch {
		case err == nil:
			successfulCreates++
		case errors.Is(err, service.ErrClientAlreadyInRoom):
			clientConflicts++
		default:
			t.Fatalf("Create() returned unexpected error: %v", err)
		}
	}

	if successfulCreates != 1 {
		t.Errorf("successful Create() calls = %d, want 1", successfulCreates)
	}
	if clientConflicts != 1 {
		t.Errorf("client conflicts = %d, want 1", clientConflicts)
	}

	sharedClientID := mustClientID(t, "shared-client")
	_, found, err := repository.FindByClientID(context.Background(), sharedClientID)
	if err != nil {
		t.Fatalf("FindByClientID() returned unexpected error: %v", err)
	}
	if !found {
		t.Fatal("shared client has no active room after one successful Create()")
	}
}

func assertRoomFound(
	t *testing.T,
	repository *MemoryRepository,
	clientID matchmaking.ClientID,
	want room.Room,
) {
	t.Helper()

	got, found, err := repository.FindByClientID(context.Background(), clientID)
	if err != nil {
		t.Fatalf("FindByClientID() returned unexpected error: %v", err)
	}
	if !found {
		t.Fatal("FindByClientID() found = false, want true")
	}
	if got != want {
		t.Errorf("FindByClientID() room = %v, want %v", got, want)
	}
}

func assertRoomNotFound(
	t *testing.T,
	repository *MemoryRepository,
	clientID matchmaking.ClientID,
) {
	t.Helper()

	got, found, err := repository.FindByClientID(context.Background(), clientID)
	if err != nil {
		t.Fatalf("FindByClientID() returned unexpected error: %v", err)
	}
	if found {
		t.Errorf("FindByClientID() found = true, want false")
	}
	if got != (room.Room{}) {
		t.Errorf("FindByClientID() room = %v, want zero Room", got)
	}
}

func mustRoom(t *testing.T, roomID, firstClientID, secondClientID string) room.Room {
	t.Helper()

	clients, err := room.NewConnectedClientsPair(
		mustClientID(t, firstClientID),
		mustClientID(t, secondClientID),
	)
	if err != nil {
		t.Fatalf("NewConnectedClientsPair() returned unexpected error: %v", err)
	}

	target, err := room.NewRoom(clients, mustRoomID(t, roomID))
	if err != nil {
		t.Fatalf("NewRoom() returned unexpected error: %v", err)
	}

	return target
}

func mustRoomID(t *testing.T, value string) room.RoomID {
	t.Helper()

	roomID, err := room.NewRoomID(value)
	if err != nil {
		t.Fatalf("NewRoomID(%q) returned unexpected error: %v", value, err)
	}

	return roomID
}

func mustClientID(t *testing.T, value string) matchmaking.ClientID {
	t.Helper()

	clientID, err := matchmaking.NewClientID(value)
	if err != nil {
		t.Fatalf("NewClientID(%q) returned unexpected error: %v", value, err)
	}

	return clientID
}

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/matchmaking"
	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/room"
)

type fakeRepository struct {
	create         func(context.Context, room.Room) error
	findByClientID func(context.Context, matchmaking.ClientID) (room.Room, bool, error)
	delete         func(context.Context, room.RoomID) error
}

func (f *fakeRepository) Create(ctx context.Context, target room.Room) error {
	if f.create == nil {
		panic("unexpected Repository.Create call")
	}
	return f.create(ctx, target)
}

func (f *fakeRepository) FindByClientID(
	ctx context.Context,
	clientID matchmaking.ClientID,
) (room.Room, bool, error) {
	if f.findByClientID == nil {
		panic("unexpected Repository.FindByClientID call")
	}
	return f.findByClientID(ctx, clientID)
}

func (f *fakeRepository) Delete(ctx context.Context, roomID room.RoomID) error {
	if f.delete == nil {
		panic("unexpected Repository.Delete call")
	}
	return f.delete(ctx, roomID)
}

func TestServiceCreateRoom(t *testing.T) {
	firstClientID := mustClientID(t, "client-1")
	secondClientID := mustClientID(t, "client-2")
	roomID := mustRoomID(t, "room-1")
	type contextKey string
	ctx := context.WithValue(context.Background(), contextKey("test"), "value")

	repository := &fakeRepository{
		create: func(gotCtx context.Context, target room.Room) error {
			if gotCtx != ctx {
				t.Error("Repository.Create() received a different context")
			}
			if target.RoomID() != roomID {
				t.Errorf("created RoomID = %v, want %v", target.RoomID(), roomID)
			}
			if target.Clients().FirstClientID() != firstClientID {
				t.Errorf("first client = %v, want %v", target.Clients().FirstClientID(), firstClientID)
			}
			if target.Clients().SecondClientID() != secondClientID {
				t.Errorf("second client = %v, want %v", target.Clients().SecondClientID(), secondClientID)
			}
			return nil
		},
	}

	createdRoom, err := New(repository, func() (room.RoomID, error) {
		return roomID, nil
	}).CreateRoom(ctx, firstClientID, secondClientID)
	if err != nil {
		t.Fatalf("CreateRoom() returned unexpected error: %v", err)
	}
	if createdRoom.RoomID() != roomID {
		t.Errorf("CreateRoom() RoomID = %v, want %v", createdRoom.RoomID(), roomID)
	}
}

func TestServiceCreateRoomRejectsCancelledContextWithoutCallingDependencies(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	generatorCalled := false
	service := New(&fakeRepository{}, func() (room.RoomID, error) {
		generatorCalled = true
		return room.RoomID{}, nil
	})

	_, err := service.CreateRoom(
		ctx,
		mustClientID(t, "client-1"),
		mustClientID(t, "client-2"),
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("CreateRoom() error = %v, want %v", err, context.Canceled)
	}
	if generatorCalled {
		t.Error("RoomIDGenerator was called for cancelled context")
	}
}

func TestServiceCreateRoomRejectsInvalidClientsWithoutCallingDependencies(t *testing.T) {
	generatorCalled := false
	service := New(&fakeRepository{}, func() (room.RoomID, error) {
		generatorCalled = true
		return room.RoomID{}, nil
	})

	_, err := service.CreateRoom(
		context.Background(),
		matchmaking.ClientID{},
		mustClientID(t, "client-2"),
	)
	if !errors.Is(err, matchmaking.ErrInvalidClientID) {
		t.Fatalf("CreateRoom() error = %v, want %v", err, matchmaking.ErrInvalidClientID)
	}
	if generatorCalled {
		t.Error("RoomIDGenerator was called for invalid clients")
	}
}

func TestServiceCreateRoomPropagatesGeneratorError(t *testing.T) {
	wantErr := errors.New("generate room ID")
	service := New(&fakeRepository{}, func() (room.RoomID, error) {
		return room.RoomID{}, wantErr
	})

	_, err := service.CreateRoom(
		context.Background(),
		mustClientID(t, "client-1"),
		mustClientID(t, "client-2"),
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("CreateRoom() error = %v, want %v", err, wantErr)
	}
}

func TestServiceCreateRoomRejectsInvalidGeneratedRoomID(t *testing.T) {
	service := New(&fakeRepository{}, func() (room.RoomID, error) {
		return room.RoomID{}, nil
	})

	_, err := service.CreateRoom(
		context.Background(),
		mustClientID(t, "client-1"),
		mustClientID(t, "client-2"),
	)
	if !errors.Is(err, room.ErrInvalidRoomID) {
		t.Fatalf("CreateRoom() error = %v, want %v", err, room.ErrInvalidRoomID)
	}
}

func TestServiceCreateRoomPropagatesRepositoryError(t *testing.T) {
	wantErr := ErrClientAlreadyInRoom
	repository := &fakeRepository{
		create: func(context.Context, room.Room) error {
			return wantErr
		},
	}
	service := New(repository, func() (room.RoomID, error) {
		return mustRoomID(t, "room-1"), nil
	})

	_, err := service.CreateRoom(
		context.Background(),
		mustClientID(t, "client-1"),
		mustClientID(t, "client-2"),
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("CreateRoom() error = %v, want %v", err, wantErr)
	}
}

func TestServiceFindRoomByClientID(t *testing.T) {
	clientID := mustClientID(t, "client-1")
	wantRoom := mustRoom(t, "room-1", "client-1", "client-2")
	wantErr := errors.New("find room")

	tests := []struct {
		name  string
		room  room.Room
		found bool
		err   error
	}{
		{name: "found", room: wantRoom, found: true},
		{name: "not found"},
		{name: "repository error", err: wantErr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type contextKey string
			ctx := context.WithValue(context.Background(), contextKey("test"), tt.name)
			repository := &fakeRepository{
				findByClientID: func(gotCtx context.Context, gotClientID matchmaking.ClientID) (room.Room, bool, error) {
					if gotCtx != ctx {
						t.Error("Repository.FindByClientID() received a different context")
					}
					if gotClientID != clientID {
						t.Errorf("client ID = %v, want %v", gotClientID, clientID)
					}
					return tt.room, tt.found, tt.err
				},
			}

			gotRoom, found, err := New(repository, nil).FindRoomByClientID(ctx, clientID)
			if !errors.Is(err, tt.err) {
				t.Fatalf("FindRoomByClientID() error = %v, want %v", err, tt.err)
			}
			if found != tt.found {
				t.Errorf("FindRoomByClientID() found = %v, want %v", found, tt.found)
			}
			if gotRoom != tt.room {
				t.Errorf("FindRoomByClientID() room = %v, want %v", gotRoom, tt.room)
			}
		})
	}
}

func TestServiceFindRoomByClientIDRejectsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	gotRoom, found, err := New(&fakeRepository{}, nil).FindRoomByClientID(
		ctx,
		mustClientID(t, "client-1"),
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("FindRoomByClientID() error = %v, want %v", err, context.Canceled)
	}
	if found || gotRoom != (room.Room{}) {
		t.Errorf("FindRoomByClientID() = (%v, %v), want zero Room and false", gotRoom, found)
	}
}

func TestServiceCloseRoom(t *testing.T) {
	roomID := mustRoomID(t, "room-1")
	wantErr := errors.New("delete room")

	tests := []struct {
		name string
		err  error
	}{
		{name: "success"},
		{name: "repository error", err: wantErr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type contextKey string
			ctx := context.WithValue(context.Background(), contextKey("test"), tt.name)
			repository := &fakeRepository{
				delete: func(gotCtx context.Context, gotRoomID room.RoomID) error {
					if gotCtx != ctx {
						t.Error("Repository.Delete() received a different context")
					}
					if gotRoomID != roomID {
						t.Errorf("room ID = %v, want %v", gotRoomID, roomID)
					}
					return tt.err
				},
			}

			err := New(repository, nil).CloseRoom(ctx, roomID)
			if !errors.Is(err, tt.err) {
				t.Fatalf("CloseRoom() error = %v, want %v", err, tt.err)
			}
		})
	}
}

func TestServiceCloseRoomRejectsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := New(&fakeRepository{}, nil).CloseRoom(ctx, mustRoomID(t, "room-1"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("CloseRoom() error = %v, want %v", err, context.Canceled)
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

package room_test

import (
	"errors"
	"testing"

	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/matchmaking"
	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/room"
)

func TestNewRoomID(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantValue string
		wantErr   error
	}{
		{
			name:      "valid ID",
			input:     "client-1",
			wantValue: "client-1",
		},
		{
			name:      "trim surrounding spaces and preserve case",
			input:     "  Client-A  ",
			wantValue: "Client-A",
		},
		{
			name:    "empty ID",
			input:   "",
			wantErr: room.ErrInvalidRoomID,
		},
		{
			name:    "spaces only",
			input:   "   ",
			wantErr: room.ErrInvalidRoomID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			roomID, err := room.NewRoomID(tt.input)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("NewRoomID() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("NewRoomID() returned unexpected error: %v", err)
			}

			if roomID.Value() != tt.wantValue {
				t.Errorf("Value() = %q, want %q", roomID.Value(), tt.wantValue)
			}
		})
	}
}

func TestNewConnectedClientsPair(t *testing.T) {
	firstClientID := mustClientID(t, "client-1")
	secondClientID := mustClientID(t, "client-2")

	clients, err := room.NewConnectedClientsPair(firstClientID, secondClientID)
	if err != nil {
		t.Fatalf("NewConnectedClientsPair() returned unexpected error: %v", err)
	}

	if clients.FirstClientID() != firstClientID {
		t.Errorf("FirstClientID() = %v, want %v", clients.FirstClientID(), firstClientID)
	}
	if clients.SecondClientID() != secondClientID {
		t.Errorf("SecondClientID() = %v, want %v", clients.SecondClientID(), secondClientID)
	}
}

func TestNewConnectedClientsPair_RejectsSameClient(t *testing.T) {
	clientID := mustClientID(t, "client-1")

	_, err := room.NewConnectedClientsPair(clientID, clientID)
	if !errors.Is(err, room.ErrSameClientInOneRoom) {
		t.Fatalf("NewConnectedClientsPair() error = %v, want %v", err, room.ErrSameClientInOneRoom)
	}
}

func TestNewConnectedClientsPair_RejectsZeroClientID(t *testing.T) {
	validClientID := mustClientID(t, "client-1")

	tests := []struct {
		name     string
		firstID  matchmaking.ClientID
		secondID matchmaking.ClientID
	}{
		{
			name:     "zero first client ID",
			secondID: validClientID,
		},
		{
			name:    "zero second client ID",
			firstID: validClientID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := room.NewConnectedClientsPair(tt.firstID, tt.secondID)
			if !errors.Is(err, matchmaking.ErrInvalidClientID) {
				t.Fatalf("NewConnectedClientsPair() error = %v, want %v", err, matchmaking.ErrInvalidClientID)
			}
		})
	}
}

func TestNewRoom(t *testing.T) {
	clients := mustConnectedClientsPair(t, "client-1", "client-2")
	roomID := mustRoomID(t, "room-1")

	gotRoom, err := room.NewRoom(clients, roomID)
	if err != nil {
		t.Fatalf("NewRoom() returned unexpected error: %v", err)
	}

	if gotRoom.RoomID() != roomID {
		t.Errorf("RoomID() = %v, want %v", gotRoom.RoomID(), roomID)
	}
	if gotRoom.Clients() != clients {
		t.Errorf("Clients() = %v, want %v", gotRoom.Clients(), clients)
	}
}

func TestNewRoom_RejectsZeroRoomID(t *testing.T) {
	clients := mustConnectedClientsPair(t, "client-1", "client-2")

	_, err := room.NewRoom(clients, room.RoomID{})
	if !errors.Is(err, room.ErrInvalidRoomID) {
		t.Fatalf("NewRoom() error = %v, want %v", err, room.ErrInvalidRoomID)
	}
}

func TestRoomOtherClientID(t *testing.T) {
	firstClientID := mustClientID(t, "client-1")
	secondClientID := mustClientID(t, "client-2")
	otherClientID := mustClientID(t, "client-3")
	clients, err := room.NewConnectedClientsPair(firstClientID, secondClientID)
	if err != nil {
		t.Fatalf("NewConnectedClientsPair() returned unexpected error: %v", err)
	}
	targetRoom, err := room.NewRoom(clients, mustRoomID(t, "room-1"))
	if err != nil {
		t.Fatalf("NewRoom() returned unexpected error: %v", err)
	}

	tests := []struct {
		name         string
		clientID     matchmaking.ClientID
		wantClientID matchmaking.ClientID
		wantErr      error
	}{
		{
			name:         "first participant returns second participant",
			clientID:     firstClientID,
			wantClientID: secondClientID,
		},
		{
			name:         "second participant returns first participant",
			clientID:     secondClientID,
			wantClientID: firstClientID,
		},
		{
			name:     "client outside room",
			clientID: otherClientID,
			wantErr:  room.ErrClientNotInRoom,
		},
		{
			name:    "zero client ID",
			wantErr: matchmaking.ErrInvalidClientID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotClientID, err := targetRoom.OtherClientID(tt.clientID)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("OtherClientID() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("OtherClientID() returned unexpected error: %v", err)
			}
			if !gotClientID.IsEqual(tt.wantClientID) {
				t.Errorf("OtherClientID() = %v, want %v", gotClientID, tt.wantClientID)
			}
		})
	}
}

func mustClientID(t *testing.T, value string) matchmaking.ClientID {
	t.Helper()

	clientID, err := matchmaking.NewClientID(value)
	if err != nil {
		t.Fatalf("NewClientID(%q) returned unexpected error: %v", value, err)
	}

	return clientID
}

func mustConnectedClientsPair(t *testing.T, first, second string) room.ConnectedClients {
	t.Helper()

	clients, err := room.NewConnectedClientsPair(
		mustClientID(t, first),
		mustClientID(t, second),
	)
	if err != nil {
		t.Fatalf("NewConnectedClientsPair() returned unexpected error: %v", err)
	}

	return clients
}

func mustRoomID(t *testing.T, value string) room.RoomID {
	t.Helper()

	roomID, err := room.NewRoomID(value)
	if err != nil {
		t.Fatalf("NewRoomID(%q) returned unexpected error: %v", value, err)
	}

	return roomID
}

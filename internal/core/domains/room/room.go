package room

import (
	"strings"

	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/matchmaking"
)

type RoomID struct {
	value string
}

func NewRoomID(id string) (RoomID, error) {
	roomID := RoomID{
		value: strings.TrimSpace(id),
	}
	if err := roomID.validate(); err != nil {
		return RoomID{}, err
	}

	return roomID, nil
}

func (roomID RoomID) validate() error {
	if roomID.value == "" {
		return ErrInvalidRoomID
	}
	return nil
}

func (roomID RoomID) Value() string {
	return roomID.value
}

type ConnectedClients struct {
	firstClientID  matchmaking.ClientID
	secondClientID matchmaking.ClientID
}

func NewConnectedClientsPair(firstClientID matchmaking.ClientID, secondClientID matchmaking.ClientID) (ConnectedClients, error) {

	clients := ConnectedClients{
		firstClientID:  firstClientID,
		secondClientID: secondClientID,
	}

	if err := clients.validate(); err != nil {
		return ConnectedClients{}, err
	}

	return clients, nil
}

func (clients ConnectedClients) validate() error {
	if !clients.firstClientID.IsValid() {
		return matchmaking.ErrInvalidClientID
	}
	if !clients.secondClientID.IsValid() {
		return matchmaking.ErrInvalidClientID
	}
	if clients.firstClientID.IsEqual(clients.secondClientID) {
		return ErrSameClientInOneRoom
	}
	return nil
}

func (clients ConnectedClients) FirstClientID() matchmaking.ClientID {
	return clients.firstClientID
}

func (clients ConnectedClients) SecondClientID() matchmaking.ClientID {
	return clients.secondClientID
}

type Room struct {
	connectedClients ConnectedClients
	roomID           RoomID
}

func NewRoom(clients ConnectedClients, roomID RoomID) (Room, error) {
	room := Room{
		connectedClients: clients,
		roomID:           roomID,
	}
	if err := room.validate(); err != nil {
		return Room{}, err
	}
	return room, nil
}

func (room Room) validate() error {
	if err := room.roomID.validate(); err != nil {
		return err
	}
	if err := room.connectedClients.validate(); err != nil {
		return err
	}
	return nil
}

func (room Room) Clients() ConnectedClients {
	return room.connectedClients
}

func (room Room) RoomID() RoomID {
	return room.roomID
}

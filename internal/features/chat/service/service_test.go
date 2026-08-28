package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/chat"
	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/matchmaking"
	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/room"
	chatservice "github.com/wizardVadim/fluent-swap-core/internal/features/chat/service"
)

type fakeRoomFinder struct {
	find func(context.Context, matchmaking.ClientID) (room.Room, bool, error)
}

func (f *fakeRoomFinder) FindRoomByClientID(
	ctx context.Context,
	clientID matchmaking.ClientID,
) (room.Room, bool, error) {
	return f.find(ctx, clientID)
}

type fakeDelivery struct {
	deliver func(context.Context, matchmaking.ClientID, room.RoomID, chat.MessageText) error
}

func (f *fakeDelivery) Deliver(
	ctx context.Context,
	recipientID matchmaking.ClientID,
	roomID room.RoomID,
	text chat.MessageText,
) error {
	return f.deliver(ctx, recipientID, roomID, text)
}

func TestServiceSendMessageRejectsCancelledContextBeforeDependencies(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	service := chatservice.New(
		&fakeDelivery{deliver: func(context.Context, matchmaking.ClientID, room.RoomID, chat.MessageText) error {
			t.Fatal("Deliver() called for cancelled context")
			return nil
		}},
		&fakeRoomFinder{find: func(context.Context, matchmaking.ClientID) (room.Room, bool, error) {
			t.Fatal("FindRoomByClientID() called for cancelled context")
			return room.Room{}, false, nil
		}},
	)

	err := service.SendMessage(ctx, matchmaking.ClientID{}, room.RoomID{}, chat.MessageText{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("SendMessage() error = %v, want %v", err, context.Canceled)
	}
}

func TestServiceSendMessageRejectsInvalidTextBeforeDependencies(t *testing.T) {
	service := chatservice.New(
		&fakeDelivery{deliver: func(context.Context, matchmaking.ClientID, room.RoomID, chat.MessageText) error {
			t.Fatal("Deliver() called for invalid text")
			return nil
		}},
		&fakeRoomFinder{find: func(context.Context, matchmaking.ClientID) (room.Room, bool, error) {
			t.Fatal("FindRoomByClientID() called for invalid text")
			return room.Room{}, false, nil
		}},
	)

	err := service.SendMessage(context.Background(), matchmaking.ClientID{}, room.RoomID{}, chat.MessageText{})
	if !errors.Is(err, chat.ErrInvalidText) {
		t.Fatalf("SendMessage() error = %v, want %v", err, chat.ErrInvalidText)
	}
}

func TestServiceSendMessagePropagatesRoomFinderError(t *testing.T) {
	finderErr := errors.New("room finder failed")
	senderID := mustClientID(t, "sender")
	text := mustMessageText(t, "hello")
	service := chatservice.New(
		&fakeDelivery{deliver: func(context.Context, matchmaking.ClientID, room.RoomID, chat.MessageText) error {
			t.Fatal("Deliver() called after room finder error")
			return nil
		}},
		&fakeRoomFinder{find: func(context.Context, matchmaking.ClientID) (room.Room, bool, error) {
			return room.Room{}, false, finderErr
		}},
	)

	err := service.SendMessage(context.Background(), senderID, mustRoomID(t, "room-1"), text)
	if !errors.Is(err, finderErr) {
		t.Fatalf("SendMessage() error = %v, want %v", err, finderErr)
	}
}

func TestServiceSendMessageReturnsSenderNotInRoom(t *testing.T) {
	senderID := mustClientID(t, "sender")
	service := chatservice.New(
		&fakeDelivery{deliver: func(context.Context, matchmaking.ClientID, room.RoomID, chat.MessageText) error {
			t.Fatal("Deliver() called when sender has no room")
			return nil
		}},
		&fakeRoomFinder{find: func(context.Context, matchmaking.ClientID) (room.Room, bool, error) {
			return room.Room{}, false, nil
		}},
	)

	err := service.SendMessage(
		context.Background(),
		senderID,
		mustRoomID(t, "room-1"),
		mustMessageText(t, "hello"),
	)
	if !errors.Is(err, chatservice.ErrSenderNotInRoom) {
		t.Fatalf("SendMessage() error = %v, want %v", err, chatservice.ErrSenderNotInRoom)
	}
}

func TestServiceSendMessageReturnsRoomMismatch(t *testing.T) {
	targetRoom, senderID, _ := mustRoom(t)
	service := chatservice.New(
		&fakeDelivery{deliver: func(context.Context, matchmaking.ClientID, room.RoomID, chat.MessageText) error {
			t.Fatal("Deliver() called for mismatched room")
			return nil
		}},
		&fakeRoomFinder{find: func(context.Context, matchmaking.ClientID) (room.Room, bool, error) {
			return targetRoom, true, nil
		}},
	)

	err := service.SendMessage(
		context.Background(),
		senderID,
		mustRoomID(t, "another-room"),
		mustMessageText(t, "hello"),
	)
	if !errors.Is(err, chatservice.ErrRoomMismatch) {
		t.Fatalf("SendMessage() error = %v, want %v", err, chatservice.ErrRoomMismatch)
	}
}

func TestServiceSendMessageDeliversToOtherParticipant(t *testing.T) {
	targetRoom, senderID, recipientID := mustRoom(t)
	text := mustMessageText(t, "  hello  ")
	ctx := context.WithValue(context.Background(), contextKey{}, "expected-context")

	service := chatservice.New(
		&fakeDelivery{deliver: func(
			gotCtx context.Context,
			gotRecipientID matchmaking.ClientID,
			gotRoomID room.RoomID,
			gotText chat.MessageText,
		) error {
			if gotCtx != ctx {
				t.Error("Deliver() received a different context")
			}
			if !gotRecipientID.IsEqual(recipientID) {
				t.Errorf("Deliver() recipient ID = %v, want %v", gotRecipientID, recipientID)
			}
			if !gotRoomID.IsEqual(targetRoom.RoomID()) {
				t.Errorf("Deliver() room ID = %v, want %v", gotRoomID, targetRoom.RoomID())
			}
			if gotText.Value() != text.Value() {
				t.Errorf("Deliver() text = %q, want %q", gotText.Value(), text.Value())
			}
			return nil
		}},
		&fakeRoomFinder{find: func(gotCtx context.Context, gotSenderID matchmaking.ClientID) (room.Room, bool, error) {
			if gotCtx != ctx {
				t.Error("FindRoomByClientID() received a different context")
			}
			if !gotSenderID.IsEqual(senderID) {
				t.Errorf("FindRoomByClientID() sender ID = %v, want %v", gotSenderID, senderID)
			}
			return targetRoom, true, nil
		}},
	)

	if err := service.SendMessage(ctx, senderID, targetRoom.RoomID(), text); err != nil {
		t.Fatalf("SendMessage() returned unexpected error: %v", err)
	}
}

func TestServiceSendMessageJoinsRecipientUnavailableWithDeliveryError(t *testing.T) {
	targetRoom, senderID, _ := mustRoom(t)
	deliveryErr := errors.New("delivery failed")
	service := chatservice.New(
		&fakeDelivery{deliver: func(context.Context, matchmaking.ClientID, room.RoomID, chat.MessageText) error {
			return deliveryErr
		}},
		&fakeRoomFinder{find: func(context.Context, matchmaking.ClientID) (room.Room, bool, error) {
			return targetRoom, true, nil
		}},
	)

	err := service.SendMessage(context.Background(), senderID, targetRoom.RoomID(), mustMessageText(t, "hello"))
	if !errors.Is(err, chatservice.ErrRecipientUnavailable) {
		t.Errorf("SendMessage() error = %v, want it to contain %v", err, chatservice.ErrRecipientUnavailable)
	}
	if !errors.Is(err, deliveryErr) {
		t.Errorf("SendMessage() error = %v, want it to contain %v", err, deliveryErr)
	}
}

func TestServiceSendMessageReturnsContextErrorWhenCancelledDuringDelivery(t *testing.T) {
	targetRoom, senderID, _ := mustRoom(t)
	ctx, cancel := context.WithCancel(context.Background())
	deliveryErr := errors.New("delivery failed")
	service := chatservice.New(
		&fakeDelivery{deliver: func(context.Context, matchmaking.ClientID, room.RoomID, chat.MessageText) error {
			cancel()
			return deliveryErr
		}},
		&fakeRoomFinder{find: func(context.Context, matchmaking.ClientID) (room.Room, bool, error) {
			return targetRoom, true, nil
		}},
	)

	err := service.SendMessage(ctx, senderID, targetRoom.RoomID(), mustMessageText(t, "hello"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("SendMessage() error = %v, want %v", err, context.Canceled)
	}
	if errors.Is(err, chatservice.ErrRecipientUnavailable) {
		t.Errorf("SendMessage() error = %v, must not contain %v", err, chatservice.ErrRecipientUnavailable)
	}
}

type contextKey struct{}

func mustRoom(t *testing.T) (room.Room, matchmaking.ClientID, matchmaking.ClientID) {
	t.Helper()

	senderID := mustClientID(t, "sender")
	recipientID := mustClientID(t, "recipient")
	clients, err := room.NewConnectedClientsPair(senderID, recipientID)
	if err != nil {
		t.Fatalf("NewConnectedClientsPair() returned unexpected error: %v", err)
	}
	targetRoom, err := room.NewRoom(clients, mustRoomID(t, "room-1"))
	if err != nil {
		t.Fatalf("NewRoom() returned unexpected error: %v", err)
	}
	return targetRoom, senderID, recipientID
}

func mustClientID(t *testing.T, value string) matchmaking.ClientID {
	t.Helper()

	clientID, err := matchmaking.NewClientID(value)
	if err != nil {
		t.Fatalf("NewClientID(%q) returned unexpected error: %v", value, err)
	}
	return clientID
}

func mustRoomID(t *testing.T, value string) room.RoomID {
	t.Helper()

	roomID, err := room.NewRoomID(value)
	if err != nil {
		t.Fatalf("NewRoomID(%q) returned unexpected error: %v", value, err)
	}
	return roomID
}

func mustMessageText(t *testing.T, value string) chat.MessageText {
	t.Helper()

	text, err := chat.NewMessageText(value)
	if err != nil {
		t.Fatalf("NewMessageText(%q) returned unexpected error: %v", value, err)
	}
	return text
}

package websocket

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/chat"
	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/room"
)

func TestChatDeliveryDeliversReceiveMessage(t *testing.T) {
	recipientID := newTestClientID(t, "recipient")
	roomID := newChatDeliveryTestRoomID(t, "room-1")
	text := newChatDeliveryTestMessageText(t, "  hello  ")
	session := newClientSession(context.Background(), recipientID, nil)
	t.Cleanup(session.cancel)
	sessions := NewSessionRegistry()
	if !sessions.register(session) {
		t.Fatal("register() = false, want true")
	}

	delivery := NewChatDelivery(sessions)
	if err := delivery.Deliver(context.Background(), recipientID, roomID, text); err != nil {
		t.Fatalf("Deliver() returned unexpected error: %v", err)
	}

	message := <-session.outbound
	received, ok := message.(ReceiveMessage)
	if !ok {
		t.Fatalf("outbound message type = %T, want ReceiveMessage", message)
	}
	if received.Type != TypeReceiveMessage {
		t.Errorf("ReceiveMessage.Type = %q, want %q", received.Type, TypeReceiveMessage)
	}
	if received.Payload.MatchID != roomID.Value() {
		t.Errorf("ReceiveMessage match ID = %q, want %q", received.Payload.MatchID, roomID.Value())
	}
	if received.Payload.Text != text.Value() {
		t.Errorf("ReceiveMessage text = %q, want %q", received.Payload.Text, text.Value())
	}
}

func TestChatDeliveryReturnsErrorWhenSessionIsMissing(t *testing.T) {
	delivery := NewChatDelivery(NewSessionRegistry())

	err := delivery.Deliver(
		context.Background(),
		newTestClientID(t, "recipient"),
		newChatDeliveryTestRoomID(t, "room-1"),
		newChatDeliveryTestMessageText(t, "hello"),
	)
	if !errors.Is(err, errCannotFindClientSession) {
		t.Fatalf("Deliver() error = %v, want %v", err, errCannotFindClientSession)
	}
}

func TestChatDeliveryReturnsErrorWhenSessionIsCancelled(t *testing.T) {
	recipientID := newTestClientID(t, "recipient")
	session := newClientSession(context.Background(), recipientID, nil)
	sessions := NewSessionRegistry()
	if !sessions.register(session) {
		t.Fatal("register() = false, want true")
	}
	session.cancel()
	delivery := NewChatDelivery(sessions)

	err := delivery.Deliver(
		context.Background(),
		recipientID,
		newChatDeliveryTestRoomID(t, "room-1"),
		newChatDeliveryTestMessageText(t, "hello"),
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Deliver() error = %v, want %v", err, context.Canceled)
	}
}

func TestChatDeliveryRejectsCancelledOperationContextBeforeRegistryAccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	delivery := NewChatDelivery(nil)

	err := delivery.Deliver(
		ctx,
		newTestClientID(t, "recipient"),
		newChatDeliveryTestRoomID(t, "room-1"),
		newChatDeliveryTestMessageText(t, "hello"),
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Deliver() error = %v, want %v", err, context.Canceled)
	}
}

func TestChatDeliveryFullOutboundUnblocksWhenOperationContextIsCancelled(t *testing.T) {
	recipientID := newTestClientID(t, "recipient")
	session := newClientSession(context.Background(), recipientID, nil)
	t.Cleanup(session.cancel)
	for i := 0; i < cap(session.outbound); i++ {
		session.outbound <- struct{}{}
	}
	sessions := NewSessionRegistry()
	if !sessions.register(session) {
		t.Fatal("register() = false, want true")
	}
	delivery := NewChatDelivery(sessions)
	ctx, cancel := context.WithCancel(context.Background())
	roomID := newChatDeliveryTestRoomID(t, "room-1")
	text := newChatDeliveryTestMessageText(t, "hello")
	done := make(chan error, 1)
	go func() {
		done <- delivery.Deliver(
			ctx,
			recipientID,
			roomID,
			text,
		)
	}()

	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Deliver() error = %v, want %v", err, context.Canceled)
		}
	case <-time.After(time.Second):
		t.Fatal("Deliver() did not return after operation context cancellation")
	}
}

func newChatDeliveryTestRoomID(t *testing.T, value string) room.RoomID {
	t.Helper()

	roomID, err := room.NewRoomID(value)
	if err != nil {
		t.Fatalf("NewRoomID(%q): %v", value, err)
	}
	return roomID
}

func newChatDeliveryTestMessageText(t *testing.T, value string) chat.MessageText {
	t.Helper()

	text, err := chat.NewMessageText(value)
	if err != nil {
		t.Fatalf("NewMessageText(%q): %v", value, err)
	}
	return text
}

package websocket

import (
	"context"

	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/chat"
	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/matchmaking"
	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/room"
)

type ChatDelivery struct {
	sessions *SessionRegistry
}

func NewChatDelivery(sessions *SessionRegistry) *ChatDelivery {
	return &ChatDelivery{
		sessions: sessions,
	}
}

func (chatDelivery *ChatDelivery) Deliver(ctx context.Context, recipientID matchmaking.ClientID, roomID room.RoomID, text chat.MessageText) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	session, ok := chatDelivery.sessions.get(recipientID)
	if !ok {
		return errCannotFindClientSession
	}

	payload := MessagePayload{
		Text:    text.Value(),
		MatchID: roomID.Value(),
	}
	receiveMessage := ReceiveMessage{
		Type:    TypeReceiveMessage,
		Payload: payload,
	}

	return session.sendWithContext(ctx, receiveMessage)
}

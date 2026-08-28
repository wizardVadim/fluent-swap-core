package service

import (
	"context"
	"errors"

	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/chat"
	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/matchmaking"
	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/room"
)

type Service struct {
	delivery   Delivery
	roomFinder RoomFinder
}

func New(delivery Delivery, roomFinder RoomFinder) *Service {
	return &Service{
		delivery:   delivery,
		roomFinder: roomFinder,
	}
}

func (s *Service) SendMessage(ctx context.Context, senderID matchmaking.ClientID, roomID room.RoomID, text chat.MessageText) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if !text.IsValid() {
		return chat.ErrInvalidText
	}

	activeRoom, ok, err := s.roomFinder.FindRoomByClientID(ctx, senderID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrSenderNotInRoom
	}
	if !activeRoom.RoomID().IsEqual(roomID) {
		return ErrRoomMismatch
	}

	recipientID, err := activeRoom.OtherClientID(senderID)
	if err != nil {
		return err
	}

	if err := s.delivery.Deliver(ctx, recipientID, activeRoom.RoomID(), text); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return errors.Join(ErrRecipientUnavailable, err)
	}
	return nil
}

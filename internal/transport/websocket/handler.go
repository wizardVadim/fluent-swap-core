package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/matchmaking"
	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/room"
)

type WebsocketHandler struct {
	upgrader          websocket.Upgrader
	service           MatchmakingService
	clientIDGenerator ClientIDGenerator
	sessions          *sessionRegistry
	roomService       RoomService
}

func NewWebsocketHandler(
	service MatchmakingService,
	clientIDGenerator ClientIDGenerator,
	roomService RoomService,
) *WebsocketHandler {
	var upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	return &WebsocketHandler{
		upgrader:          upgrader,
		clientIDGenerator: clientIDGenerator,
		service:           service,
		sessions:          newSessionRegistry(),
		roomService:       roomService,
	}
}

func (h *WebsocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	clientID, err := h.clientIDGenerator()
	if err != nil {
		errorDTO := NewErrorWithoutRequestID("cannot create client ID", ErrorInternalServerError)
		if err := conn.WriteJSON(errorDTO); err != nil {
			return
		}
		return
	}

	session := newClientSession(
		r.Context(),
		clientID,
		conn,
	)

	if !h.sessions.register(session) {
		session.cancel()
		errorDTO := NewErrorWithoutRequestID(
			"internal server error",
			ErrorInternalServerError,
		)

		if err := conn.WriteJSON(errorDTO); err != nil {
			return
		}

		return
	}

	defer h.cleanUp(session)

	go session.writeLoop()

	//messages reading
	for {
		select {
		case <-session.ctx.Done():
			return
		default:
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var envelope incomingEnvelope

		if err := json.Unmarshal(message, &envelope); err != nil {
			errorDTO := NewErrorWithoutRequestID("cannot unmarshal json", ErrorInvalidJSON)
			if err := session.send(errorDTO); err != nil {
				return
			}
			continue
		}

		if err := envelope.validate(); err != nil {
			var errorDTO Error
			if errors.Is(err, errRequestIDNotFound) {
				errorDTO = NewErrorWithoutRequestID("cannot find request id, or it is empty", ErrorInvalidPayload)
			} else {
				errorDTO = NewError("invalid payloads", ErrorInvalidPayload, envelope.RequestID)
			}
			if err := session.send(errorDTO); err != nil {
				return
			}
			continue
		}

		switch envelope.Type {
		case TypeFindPartner:
			if err := h.handleFindPartner(session, envelope); err != nil {
				return
			}
		case TypeCancelSearch:
			if err := h.handleCancelSearch(session, envelope); err != nil {
				return
			}
		default:
			errorDTO := NewError("unknown message type: "+string(envelope.Type), ErrorUnknownMessageType, envelope.RequestID)
			if err := session.send(errorDTO); err != nil {
				return
			}
			continue
		}
	}

}

func (h *WebsocketHandler) cleanUp(session *clientSession) {
	session.cancel()
	h.sessions.remove(session)
	session.clearSearchRequestID()
	currentCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = h.service.CancelSearch(currentCtx, session.clientID)
	// TODO: log cleanup error when structured logging is added
	activeRoom, ok, _ := h.roomService.FindRoomByClientID(currentCtx, session.clientID)
	// TODO: log cleanup error when structured logging is added
	if ok {
		_ = h.closeRoomWithTimeout(activeRoom.RoomID())
		// TODO: log cleanup error when structured logging is added
	}
}

func (h *WebsocketHandler) handleCancelSearch(
	session *clientSession,
	envelope incomingEnvelope,
) error {
	cs := CancelSearch{
		Type:      envelope.Type,
		RequestID: envelope.RequestID,
	}

	if err := h.service.CancelSearch(session.ctx, session.clientID); err != nil {
		errorDTO := NewError("internal server error", ErrorInternalServerError, envelope.RequestID)
		if err := session.send(errorDTO); err != nil {
			return err
		}
		return nil
	}

	session.clearSearchRequestID()

	sc := SearchCancelled{
		Type:      TypeSearchCancelled,
		RequestID: cs.RequestID,
	}
	if err := session.send(sc); err != nil {
		return err
	}
	return nil
}

func (h *WebsocketHandler) handleFindPartner(
	session *clientSession,
	envelope incomingEnvelope,
) error {
	fp, err := incomingEnvelopeToFindPartner(envelope)
	if err != nil {
		errorDTO := NewError("invalid find_partner payloads", ErrorInvalidPayload, envelope.RequestID)
		if err := session.send(errorDTO); err != nil {
			return err
		}
		return nil
	}

	nativeLanguage, err := matchmaking.NewLanguage(
		matchmaking.LanguageCode(fp.Payload.NativeLanguageCode),
	)
	if err != nil {
		errorDTO := NewError("invalid find_partner payloads", ErrorInvalidPayload, envelope.RequestID)
		if err := session.send(errorDTO); err != nil {
			return err
		}
		return nil
	}
	learningLanguage, err := matchmaking.NewLanguage(
		matchmaking.LanguageCode(fp.Payload.LearningLanguageCode),
	)
	if err != nil {
		errorDTO := NewError("invalid find_partner payloads", ErrorInvalidPayload, envelope.RequestID)
		if err := session.send(errorDTO); err != nil {
			return err
		}
		return nil
	}

	languagePair, err := matchmaking.NewLanguagePair(
		nativeLanguage,
		learningLanguage,
	)
	if err != nil {
		errorDTO := NewError("invalid find_partner payloads", ErrorInvalidPayload, envelope.RequestID)
		if err := session.send(errorDTO); err != nil {
			return err
		}
		return nil
	}

	wu, err := matchmaking.NewWaitingUser(session.clientID, languagePair)
	if err != nil {
		errorDTO := NewError("internal server error", ErrorInternalServerError, envelope.RequestID)
		if err := session.send(errorDTO); err != nil {
			return err
		}
		return nil
	}

	session.setSearchRequestID(fp.RequestID)

	result, err := h.service.FindPartner(session.ctx, wu)
	if err != nil {
		session.clearSearchRequestID()
		errorDTO := NewError("internal server error", ErrorInternalServerError, envelope.RequestID)
		if err := session.send(errorDTO); err != nil {
			return err
		}
		return nil
	}

	if result.Matched {
		session.clearSearchRequestID()
		partnerSession, success := h.sessions.get(result.Partner.ClientID())
		if !success {
			errorDTO := NewError("internal server error", ErrorInternalServerError, envelope.RequestID)
			if err := session.send(errorDTO); err != nil {
				return err
			}
			return nil
		}

		currentRequestID := fp.RequestID
		partnerRequestID := partnerSession.takeSearchRequestID()
		if partnerRequestID == "" {
			errorDTO := NewError("internal server error", ErrorInternalServerError, envelope.RequestID)
			if err := session.send(errorDTO); err != nil {
				return err
			}
			return nil
		}

		createdRoom, err := h.roomService.CreateRoom(session.ctx, session.clientID, partnerSession.clientID)
		if err != nil {
			errorDTO := NewError("internal server error", ErrorInternalServerError, envelope.RequestID)
			var savedErr error
			if err := session.send(errorDTO); err != nil {
				savedErr = err
			}
			errorDTO = NewError("internal server error", ErrorInternalServerError, partnerRequestID)
			if err := partnerSession.send(errorDTO); err != nil {
				savedErr = err
			}
			return savedErr
		}

		currentMatchFound := MatchFound{
			Type:      TypeMatchFound,
			RequestID: currentRequestID,
			Payload: MatchFoundPayload{
				MatchID: createdRoom.RoomID().Value(),
			},
		}

		partnerMatchFound := MatchFound{
			Type:      TypeMatchFound,
			RequestID: partnerRequestID,
			Payload: MatchFoundPayload{
				MatchID: createdRoom.RoomID().Value(),
			},
		}

		if err := partnerSession.send(partnerMatchFound); err != nil {
			closeErr := h.closeRoomWithTimeout(createdRoom.RoomID())
			errorDTO := NewError("internal server error", ErrorInternalServerError, envelope.RequestID)
			if err := session.send(errorDTO); err != nil || closeErr != nil {
				return errors.Join(err, closeErr)
			}
			return nil
		}

		if err := session.send(currentMatchFound); err != nil {
			closeErr := h.closeRoomWithTimeout(createdRoom.RoomID())
			return errors.Join(err, closeErr)
		}

		return nil
	}

	searchWaitingDTO := SearchWaiting{
		Type:      TypeSearchWaiting,
		RequestID: fp.RequestID,
	}
	if err := session.send(searchWaitingDTO); err != nil {
		return err
	}
	return nil
}

func (h *WebsocketHandler) closeRoomWithTimeout(
	roomID room.RoomID,
) error {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	return h.roomService.CloseRoom(ctx, roomID)
}

type incomingEnvelope struct {
	Type      Type            `json:"type"`
	RequestID string          `json:"request_id"`
	Payload   json.RawMessage `json:"payload"`
}

func (ie incomingEnvelope) validate() error {
	if ie.RequestID == "" {
		return errRequestIDNotFound
	}
	if ie.Type == "" {
		return errTypeIsEmpty
	}
	return nil
}

func incomingEnvelopePayloadToFindPartnerPayload(iep json.RawMessage) (FindPartnerPayload, error) {
	var fpp FindPartnerPayload
	if err := json.Unmarshal(iep, &fpp); err != nil {
		return FindPartnerPayload{}, err
	}
	return fpp, nil
}

func incomingEnvelopeToFindPartner(ie incomingEnvelope) (FindPartner, error) {
	fpp, err := incomingEnvelopePayloadToFindPartnerPayload(ie.Payload)
	if err != nil {
		return FindPartner{}, err
	}
	fp := FindPartner{
		Type:      ie.Type,
		RequestID: ie.RequestID,
		Payload:   fpp,
	}
	if err := fp.validate(); err != nil {
		return FindPartner{}, err
	}
	return fp, nil
}

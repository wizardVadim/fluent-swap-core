package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/matchmaking"
)

type WebsocketHandler struct {
	upgrader          websocket.Upgrader
	service           Service
	clientIDGenerator ClientIDGenerator
}

func NewWebsocketHandler(service Service, clientIDGenerator ClientIDGenerator) *WebsocketHandler {
	var upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	return &WebsocketHandler{upgrader: upgrader, clientIDGenerator: clientIDGenerator, service: service}
}

func (h *WebsocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	handlerCtx, cancel := context.WithCancel(r.Context())
	defer cancel()

	clientID, err := h.clientIDGenerator()
	if err != nil {
		errorDTO := NewErrorWithoutRequestID("cannot create client ID", ErrorInternalServerError)
		if err := conn.WriteJSON(errorDTO); err != nil {
			return
		}
		return
	}

	//messages reading
	for {
		select {
		case <-handlerCtx.Done():
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
			if err := conn.WriteJSON(errorDTO); err != nil {
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
			if err := conn.WriteJSON(errorDTO); err != nil {
				return
			}
			continue
		}

		switch envelope.Type {
		case TypeFindPartner:
			if err := h.handleFindPartner(handlerCtx, conn, clientID, envelope); err != nil {
				return
			}
		case TypeCancelSearch:
			if err := h.handleCancelSearch(
				handlerCtx,
				conn,
				clientID,
				envelope,
			); err != nil {
				return
			}
		default:
			errorDTO := NewError("unknown message type: "+string(envelope.Type), ErrorUnknownMessageType, envelope.RequestID)
			if err := conn.WriteJSON(errorDTO); err != nil {
				return
			}
			continue
		}
	}

}

func (h *WebsocketHandler) handleCancelSearch(
	ctx context.Context,
	conn *websocket.Conn,
	clientID matchmaking.ClientID,
	envelope incomingEnvelope,
) error {
	cs := CancelSearch{
		Type:      envelope.Type,
		RequestID: envelope.RequestID,
	}

	if err := h.service.CancelSearch(ctx, clientID); err != nil {
		errorDTO := NewError("internal server error", ErrorInternalServerError, envelope.RequestID)
		if err := conn.WriteJSON(errorDTO); err != nil {
			return err
		}
		return nil
	}

	sc := SearchCancelled{
		Type:      TypeSearchCancelled,
		RequestID: cs.RequestID,
	}
	if err := conn.WriteJSON(sc); err != nil {
		return err
	}
	return nil
}

func (h *WebsocketHandler) handleFindPartner(
	ctx context.Context,
	conn *websocket.Conn,
	clientID matchmaking.ClientID,
	envelope incomingEnvelope,
) error {
	fp, err := incomingEnvelopeToFindPartner(envelope)
	if err != nil {
		errorDTO := NewError("invalid find_partner payloads", ErrorInvalidPayload, envelope.RequestID)
		if err := conn.WriteJSON(errorDTO); err != nil {
			return err
		}
		return nil
	}

	nativeLanguage, err := matchmaking.NewLanguage(
		matchmaking.LanguageCode(fp.Payload.NativeLanguageCode),
	)
	if err != nil {
		errorDTO := NewError("invalid find_partner payloads", ErrorInvalidPayload, envelope.RequestID)
		if err := conn.WriteJSON(errorDTO); err != nil {
			return err
		}
		return nil
	}
	learningLanguage, err := matchmaking.NewLanguage(
		matchmaking.LanguageCode(fp.Payload.LearningLanguageCode),
	)
	if err != nil {
		errorDTO := NewError("invalid find_partner payloads", ErrorInvalidPayload, envelope.RequestID)
		if err := conn.WriteJSON(errorDTO); err != nil {
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
		if err := conn.WriteJSON(errorDTO); err != nil {
			return err
		}
		return nil
	}

	wu, err := matchmaking.NewWaitingUser(clientID, languagePair)
	if err != nil {
		errorDTO := NewError("internal server error", ErrorInternalServerError, envelope.RequestID)
		if err := conn.WriteJSON(errorDTO); err != nil {
			return err
		}
		return nil
	}

	result, err := h.service.FindPartner(ctx, wu)
	if err != nil {
		errorDTO := NewError("internal server error", ErrorInternalServerError, envelope.RequestID)
		if err := conn.WriteJSON(errorDTO); err != nil {
			return err
		}
		return nil
	}

	if result.Matched {
		// TODO: notify both matched clients
		return nil
	}

	searchWaitingDTO := SearchWaiting{
		Type:      TypeSearchWaiting,
		RequestID: fp.RequestID,
	}
	if err := conn.WriteJSON(searchWaitingDTO); err != nil {
		return err
	}
	return nil
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

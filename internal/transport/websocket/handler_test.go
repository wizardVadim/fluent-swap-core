package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gorilla "github.com/gorilla/websocket"
	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/chat"
	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/matchmaking"
	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/room"
	chatservice "github.com/wizardVadim/fluent-swap-core/internal/features/chat/service"
	"github.com/wizardVadim/fluent-swap-core/internal/features/matchmaking/repository"
	matchmakingservice "github.com/wizardVadim/fluent-swap-core/internal/features/matchmaking/service"
	roomrepository "github.com/wizardVadim/fluent-swap-core/internal/features/room/repository"
	roomservice "github.com/wizardVadim/fluent-swap-core/internal/features/room/service"
)

type fakeMatchmakingService struct {
	findPartner  func(context.Context, matchmaking.WaitingUser) (matchmakingservice.MatchResult, error)
	cancelSearch func(context.Context, matchmaking.ClientID) error
}

type fakeRoomService struct {
	createRoom         func(context.Context, matchmaking.ClientID, matchmaking.ClientID) (room.Room, error)
	closeRoom          func(context.Context, room.RoomID) error
	findRoomByClientID func(context.Context, matchmaking.ClientID) (room.Room, bool, error)
}

type fakeChatService struct {
	sendMessage func(context.Context, matchmaking.ClientID, room.RoomID, chat.MessageText) error
}

type cancelSearchCall struct {
	clientID matchmaking.ClientID
	ctxErr   error
}

func (f *fakeMatchmakingService) FindPartner(
	ctx context.Context,
	user matchmaking.WaitingUser,
) (matchmakingservice.MatchResult, error) {
	if f.findPartner == nil {
		return matchmakingservice.MatchResult{}, nil
	}
	return f.findPartner(ctx, user)
}

func (f *fakeMatchmakingService) CancelSearch(ctx context.Context, clientID matchmaking.ClientID) error {
	if f.cancelSearch == nil {
		return nil
	}
	return f.cancelSearch(ctx, clientID)
}

func (f *fakeRoomService) CreateRoom(ctx context.Context, firstClientID matchmaking.ClientID, secondClientID matchmaking.ClientID) (room.Room, error) {
	if f.createRoom == nil {
		panic("unexpected RoomService.CreateRoom call")
	}
	return f.createRoom(ctx, firstClientID, secondClientID)
}

func (f *fakeRoomService) CloseRoom(ctx context.Context, room room.RoomID) error {
	if f.closeRoom == nil {
		panic("unexpected RoomService.CloseRoom call")
	}
	return f.closeRoom(ctx, room)
}

func (f *fakeRoomService) FindRoomByClientID(ctx context.Context, clientID matchmaking.ClientID) (room.Room, bool, error) {
	if f.findRoomByClientID == nil {
		return room.Room{}, false, nil
	}
	return f.findRoomByClientID(ctx, clientID)
}

func (f *fakeChatService) SendMessage(ctx context.Context, senderID matchmaking.ClientID, roomID room.RoomID, text chat.MessageText) error {
	if f.sendMessage == nil {
		return nil
	}
	return f.sendMessage(ctx, senderID, roomID, text)
}

func TestWebsocketHandlerSendMessageCallsChatService(t *testing.T) {
	clientID := newTestClientID(t, "chat-sender")
	session := newClientSession(context.Background(), clientID, nil)
	t.Cleanup(session.cancel)
	request := newTestSendMessageEnvelope(t, "req-chat-1", "room-1", "  hello  ")

	handler := &WebsocketHandler{
		chatService: &fakeChatService{
			sendMessage: func(
				ctx context.Context,
				senderID matchmaking.ClientID,
				roomID room.RoomID,
				text chat.MessageText,
			) error {
				if ctx != session.ctx {
					t.Error("SendMessage() received a different context")
				}
				if !senderID.IsEqual(clientID) {
					t.Errorf("SendMessage() sender ID = %v, want %v", senderID, clientID)
				}
				if roomID.Value() != "room-1" {
					t.Errorf("SendMessage() room ID = %q, want %q", roomID.Value(), "room-1")
				}
				if text.Value() != "  hello  " {
					t.Errorf("SendMessage() text = %q, want %q", text.Value(), "  hello  ")
				}
				return nil
			},
		},
	}

	if err := handler.handleSendMessage(session, request); err != nil {
		t.Fatalf("handleSendMessage() returned unexpected error: %v", err)
	}
	select {
	case message := <-session.outbound:
		t.Fatalf("successful send_message produced unexpected response: %T", message)
	default:
	}
}

func TestWebsocketHandlerSendMessageRejectsInvalidDomainPayload(t *testing.T) {
	tests := []struct {
		name      string
		matchID   string
		text      string
		wantCode  ErrorCode
		requestID string
	}{
		{
			name:      "invalid text",
			matchID:   "room-1",
			text:      "   ",
			wantCode:  ErrorInvalidPayload,
			requestID: "req-invalid-text",
		},
		{
			name:      "invalid match ID",
			matchID:   "   ",
			text:      "hello",
			wantCode:  ErrorInvalidMatchID,
			requestID: "req-invalid-match",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientID := newTestClientID(t, "chat-sender")
			session := newClientSession(context.Background(), clientID, nil)
			t.Cleanup(session.cancel)
			handler := &WebsocketHandler{
				chatService: &fakeChatService{sendMessage: func(context.Context, matchmaking.ClientID, room.RoomID, chat.MessageText) error {
					t.Fatal("SendMessage() called for invalid domain payload")
					return nil
				}},
			}

			err := handler.handleSendMessage(
				session,
				newTestSendMessageEnvelope(t, tt.requestID, tt.matchID, tt.text),
			)
			if err != nil {
				t.Fatalf("handleSendMessage() returned unexpected error: %v", err)
			}
			assertOutboundError(t, session, tt.requestID, tt.wantCode)
		})
	}
}

func TestWebsocketHandlerSendMessageMapsServiceErrors(t *testing.T) {
	unknownErr := errors.New("chat service failed")
	tests := []struct {
		name     string
		inputErr error
		wantCode ErrorCode
	}{
		{
			name:     "sender is not in room",
			inputErr: chatservice.ErrSenderNotInRoom,
			wantCode: ErrorInvalidMatchID,
		},
		{
			name:     "room mismatch",
			inputErr: chatservice.ErrRoomMismatch,
			wantCode: ErrorInvalidMatchID,
		},
		{
			name:     "unknown service error",
			inputErr: unknownErr,
			wantCode: ErrorInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientID := newTestClientID(t, "chat-sender")
			session := newClientSession(context.Background(), clientID, nil)
			t.Cleanup(session.cancel)
			handler := &WebsocketHandler{
				chatService: &fakeChatService{sendMessage: func(context.Context, matchmaking.ClientID, room.RoomID, chat.MessageText) error {
					return tt.inputErr
				}},
			}

			requestID := "req-service-error"
			if err := handler.handleSendMessage(
				session,
				newTestSendMessageEnvelope(t, requestID, "room-1", "hello"),
			); err != nil {
				t.Fatalf("handleSendMessage() returned unexpected error: %v", err)
			}
			assertOutboundError(t, session, requestID, tt.wantCode)
		})
	}
}

func TestWebsocketHandlerRecipientUnavailableClosesRoom(t *testing.T) {
	clientID := newTestClientID(t, "chat-sender")
	session := newClientSession(context.Background(), clientID, nil)
	t.Cleanup(session.cancel)

	const (
		requestID = "req-recipient-unavailable"
		matchID   = "room-1"
	)
	closeCalls := 0
	handler := &WebsocketHandler{
		chatService: &fakeChatService{
			sendMessage: func(context.Context, matchmaking.ClientID, room.RoomID, chat.MessageText) error {
				return chatservice.ErrRecipientUnavailable
			},
		},
		roomService: &fakeRoomService{
			closeRoom: func(ctx context.Context, roomID room.RoomID) error {
				closeCalls++
				if err := ctx.Err(); err != nil {
					t.Errorf("CloseRoom() received inactive context: %v", err)
				}
				if roomID.Value() != matchID {
					t.Errorf("CloseRoom() room ID = %q, want %q", roomID.Value(), matchID)
				}
				return nil
			},
		},
	}

	err := handler.handleSendMessage(
		session,
		newTestSendMessageEnvelope(t, requestID, matchID, "hello"),
	)
	if err != nil {
		t.Fatalf("handleSendMessage() returned unexpected error: %v", err)
	}
	if closeCalls != 1 {
		t.Errorf("CloseRoom() calls = %d, want 1", closeCalls)
	}
	assertOutboundError(t, session, requestID, ErrorInternalServerError)
}

func TestWebsocketHandlerRecipientUnavailablePreservesRoomCleanupError(t *testing.T) {
	clientID := newTestClientID(t, "chat-sender")
	session := newClientSession(context.Background(), clientID, nil)
	t.Cleanup(session.cancel)
	cleanupErr := errors.New("room cleanup failed")
	handler := &WebsocketHandler{
		chatService: &fakeChatService{
			sendMessage: func(context.Context, matchmaking.ClientID, room.RoomID, chat.MessageText) error {
				return chatservice.ErrRecipientUnavailable
			},
		},
		roomService: &fakeRoomService{
			closeRoom: func(context.Context, room.RoomID) error {
				return cleanupErr
			},
		},
	}

	const requestID = "req-cleanup-error"
	err := handler.handleSendMessage(
		session,
		newTestSendMessageEnvelope(t, requestID, "room-1", "hello"),
	)
	if !errors.Is(err, cleanupErr) {
		t.Fatalf("handleSendMessage() error = %v, want %v", err, cleanupErr)
	}
	assertOutboundError(t, session, requestID, ErrorInternalServerError)
}

func TestWebsocketHandlerContinuesAfterInvalidSendMessage(t *testing.T) {
	clientID := newTestClientID(t, "chat-sender")
	chatCalls := make(chan struct{}, 1)
	handler := NewWebsocketHandler(
		&fakeMatchmakingService{},
		fixedClientIDGenerator(clientID),
		&fakeRoomService{},
		NewSessionRegistry(),
		&fakeChatService{sendMessage: func(context.Context, matchmaking.ClientID, room.RoomID, chat.MessageText) error {
			chatCalls <- struct{}{}
			return nil
		}},
	)
	conn := openTestConnection(t, handler)

	invalidRequest := SendMessage{
		Type:      TypeSendMessage,
		RequestID: "req-invalid",
		Payload: MessagePayload{
			MatchID: "room-1",
			Text:    "   ",
		},
	}
	if err := conn.WriteJSON(invalidRequest); err != nil {
		t.Fatalf("write invalid send_message: %v", err)
	}
	var errorResponse Error
	if err := conn.ReadJSON(&errorResponse); err != nil {
		t.Fatalf("read invalid_payload response: %v", err)
	}
	if errorResponse.Payload.Code != ErrorInvalidPayload {
		t.Fatalf("error code = %q, want %q", errorResponse.Payload.Code, ErrorInvalidPayload)
	}

	validRequest := SendMessage{
		Type:      TypeSendMessage,
		RequestID: "req-valid",
		Payload: MessagePayload{
			MatchID: "room-1",
			Text:    "hello",
		},
	}
	if err := conn.WriteJSON(validRequest); err != nil {
		t.Fatalf("write valid send_message: %v", err)
	}
	select {
	case <-chatCalls:
	case <-time.After(time.Second):
		t.Fatal("SendMessage() was not called after client error")
	}
}

func TestWebsocketHandlerClosesConnectionWhenInboundMessageExceedsLimit(t *testing.T) {
	clientID := newTestClientID(t, "oversized-message-client")
	handler := NewWebsocketHandler(
		&fakeMatchmakingService{},
		fixedClientIDGenerator(clientID),
		&fakeRoomService{},
		NewSessionRegistry(),
		&fakeChatService{},
	)
	conn := openTestConnection(t, handler)

	oversizedMessage := []byte(strings.Repeat("x", int(maxInboundMessageBytes)+1))
	if err := conn.WriteMessage(gorilla.TextMessage, oversizedMessage); err != nil {
		t.Fatalf("write oversized message: %v", err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}

	_, _, err := conn.ReadMessage()
	if err == nil {
		t.Fatal("ReadMessage() error = nil, want connection close")
	}
	if !gorilla.IsCloseError(err, gorilla.CloseMessageTooBig) {
		t.Fatalf("ReadMessage() error = %v, want close code %d", err, gorilla.CloseMessageTooBig)
	}
}

func TestWebsocketHandlerFindPartnerReturnsSearchWaiting(t *testing.T) {
	clientID := newTestClientID(t, "client-test-1")
	findCalls := make(chan matchmaking.WaitingUser, 1)

	service := &fakeMatchmakingService{
		findPartner: func(ctx context.Context, user matchmaking.WaitingUser) (matchmakingservice.MatchResult, error) {
			if err := ctx.Err(); err != nil {
				t.Errorf("FindPartner() received cancelled context: %v", err)
			}
			findCalls <- user
			return matchmakingservice.MatchResult{Matched: false}, nil
		},
	}

	roomService := &fakeRoomService{
		createRoom: nil,
	}

	conn := openTestConnection(t, NewWebsocketHandler(
		service,
		fixedClientIDGenerator(clientID),
		roomService,
		NewSessionRegistry(),
		&fakeChatService{},
	))

	request := FindPartner{
		Type:      TypeFindPartner,
		RequestID: "req-find-1",
		Payload: FindPartnerPayload{
			NativeLanguageCode:   "ru",
			LearningLanguageCode: "en",
		},
	}
	if err := conn.WriteJSON(request); err != nil {
		t.Fatalf("write find_partner: %v", err)
	}

	var response SearchWaiting
	if err := conn.ReadJSON(&response); err != nil {
		t.Fatalf("read search_waiting: %v", err)
	}
	if response.Type != TypeSearchWaiting {
		t.Errorf("response type = %q, want %q", response.Type, TypeSearchWaiting)
	}
	if response.RequestID != request.RequestID {
		t.Errorf("response request_id = %q, want %q", response.RequestID, request.RequestID)
	}

	select {
	case user := <-findCalls:
		if !user.ClientID().IsEqual(clientID) {
			t.Errorf("FindPartner() client ID = %q, want %q", user.ClientID().Value(), clientID.Value())
		}
		if got := user.LanguagePair().NativeLanguage().Code(); got != matchmaking.LanguageCodeRU {
			t.Errorf("native language = %q, want %q", got, matchmaking.LanguageCodeRU)
		}
		if got := user.LanguagePair().LearningLanguage().Code(); got != matchmaking.LanguageCodeEN {
			t.Errorf("learning language = %q, want %q", got, matchmaking.LanguageCodeEN)
		}
	default:
		t.Fatal("FindPartner() was not called")
	}
}

func TestWebsocketHandlerCancelSearchReturnsSearchCancelled(t *testing.T) {
	clientID := newTestClientID(t, "client-test-2")
	cancelCalls := make(chan matchmaking.ClientID, 1)

	service := &fakeMatchmakingService{
		cancelSearch: func(ctx context.Context, gotClientID matchmaking.ClientID) error {
			if err := ctx.Err(); err != nil {
				t.Errorf("CancelSearch() received cancelled context: %v", err)
			}
			cancelCalls <- gotClientID
			return nil
		},
	}

	roomService := &fakeRoomService{
		createRoom: nil,
	}

	conn := openTestConnection(t, NewWebsocketHandler(
		service,
		fixedClientIDGenerator(clientID),
		roomService,
		NewSessionRegistry(),
		&fakeChatService{},
	))

	request := CancelSearch{Type: TypeCancelSearch, RequestID: "req-cancel-1"}
	if err := conn.WriteJSON(request); err != nil {
		t.Fatalf("write cancel_search: %v", err)
	}

	var response SearchCancelled
	if err := conn.ReadJSON(&response); err != nil {
		t.Fatalf("read search_cancelled: %v", err)
	}
	if response.Type != TypeSearchCancelled {
		t.Errorf("response type = %q, want %q", response.Type, TypeSearchCancelled)
	}
	if response.RequestID != request.RequestID {
		t.Errorf("response request_id = %q, want %q", response.RequestID, request.RequestID)
	}

	select {
	case gotClientID := <-cancelCalls:
		if !gotClientID.IsEqual(clientID) {
			t.Errorf("CancelSearch() client ID = %q, want %q", gotClientID.Value(), clientID.Value())
		}
	default:
		t.Fatal("CancelSearch() was not called")
	}
}

func TestWebsocketHandlerInvalidJSONKeepsConnectionOpen(t *testing.T) {
	clientID := newTestClientID(t, "client-test-3")
	cancelCalls := make(chan struct{}, 1)
	service := &fakeMatchmakingService{
		cancelSearch: func(context.Context, matchmaking.ClientID) error {
			cancelCalls <- struct{}{}
			return nil
		},
	}

	roomService := &fakeRoomService{
		createRoom: nil,
	}

	conn := openTestConnection(t, NewWebsocketHandler(
		service,
		fixedClientIDGenerator(clientID),
		roomService,
		NewSessionRegistry(),
		&fakeChatService{},
	))

	if err := conn.WriteMessage(gorilla.TextMessage, []byte(`{"type":`)); err != nil {
		t.Fatalf("write invalid JSON: %v", err)
	}

	var errorResponse Error
	if err := conn.ReadJSON(&errorResponse); err != nil {
		t.Fatalf("read invalid_json response: %v", err)
	}
	if errorResponse.Type != TypeError {
		t.Errorf("response type = %q, want %q", errorResponse.Type, TypeError)
	}
	if errorResponse.RequestID != nil {
		t.Errorf("invalid_json request_id = %q, want omitted", *errorResponse.RequestID)
	}
	if errorResponse.Payload.Code != ErrorInvalidJSON {
		t.Errorf("error code = %q, want %q", errorResponse.Payload.Code, ErrorInvalidJSON)
	}

	request := CancelSearch{Type: TypeCancelSearch, RequestID: "req-after-invalid-json"}
	if err := conn.WriteJSON(request); err != nil {
		t.Fatalf("write cancel_search after invalid JSON: %v", err)
	}

	var response SearchCancelled
	if err := conn.ReadJSON(&response); err != nil {
		t.Fatalf("read search_cancelled after invalid JSON: %v", err)
	}
	if response.Type != TypeSearchCancelled || response.RequestID != request.RequestID {
		t.Errorf("response = %+v, want type %q and request_id %q", response, TypeSearchCancelled, request.RequestID)
	}

	select {
	case <-cancelCalls:
	default:
		t.Fatal("connection did not process command after invalid JSON")
	}
}

func TestWebsocketHandlerDisconnectCancelsActiveSearch(t *testing.T) {
	clientID := newTestClientID(t, "client-disconnect-1")
	cancelCalls := make(chan cancelSearchCall, 1)
	service := &fakeMatchmakingService{
		findPartner: func(context.Context, matchmaking.WaitingUser) (matchmakingservice.MatchResult, error) {
			return matchmakingservice.MatchResult{Matched: false}, nil
		},
		cancelSearch: func(ctx context.Context, gotClientID matchmaking.ClientID) error {
			cancelCalls <- cancelSearchCall{clientID: gotClientID, ctxErr: ctx.Err()}
			return nil
		},
	}

	roomService := &fakeRoomService{
		createRoom: nil,
	}

	conn := openTestConnection(t, NewWebsocketHandler(
		service,
		fixedClientIDGenerator(clientID),
		roomService,
		NewSessionRegistry(),
		&fakeChatService{},
	))

	request := FindPartner{
		Type:      TypeFindPartner,
		RequestID: "req-disconnect-1",
		Payload: FindPartnerPayload{
			NativeLanguageCode:   "ru",
			LearningLanguageCode: "en",
		},
	}
	if err := conn.WriteJSON(request); err != nil {
		t.Fatalf("write find_partner: %v", err)
	}

	var waiting SearchWaiting
	if err := conn.ReadJSON(&waiting); err != nil {
		t.Fatalf("read search_waiting: %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("close WebSocket: %v", err)
	}

	select {
	case call := <-cancelCalls:
		if !call.clientID.IsEqual(clientID) {
			t.Errorf("CancelSearch() client ID = %q, want %q", call.clientID.Value(), clientID.Value())
		}
		if call.ctxErr != nil {
			t.Errorf("CancelSearch() received cancelled cleanup context: %v", call.ctxErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("CancelSearch() was not called after disconnect")
	}
}

func TestWebsocketHandlerMatchFoundNotifiesBothClients(t *testing.T) {
	firstClientID := newTestClientID(t, "client-match-1")
	secondClientID := newTestClientID(t, "client-match-2")
	const matchID = "match-test-1"

	roomService := &fakeRoomService{
		createRoom: func(
			ctx context.Context,
			gotFirstClientID matchmaking.ClientID,
			gotSecondClientID matchmaking.ClientID,
		) (room.Room, error) {
			if err := ctx.Err(); err != nil {
				t.Errorf("CreateRoom() received cancelled context: %v", err)
			}
			if !gotFirstClientID.IsEqual(secondClientID) {
				t.Errorf("first client ID = %q, want %q", gotFirstClientID.Value(), secondClientID.Value())
			}
			if !gotSecondClientID.IsEqual(firstClientID) {
				t.Errorf("second client ID = %q, want %q", gotSecondClientID.Value(), firstClientID.Value())
			}
			return newTestRoom(t, matchID, gotFirstClientID, gotSecondClientID), nil
		},
	}

	service := matchmakingservice.New(repository.NewMemoryRepository())
	handler := NewWebsocketHandler(
		service,
		sequenceClientIDGenerator(firstClientID, secondClientID),
		roomService,
		NewSessionRegistry(),
		&fakeChatService{},
	)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	firstConn := dialTestConnection(t, server.URL)
	secondConn := dialTestConnection(t, server.URL)

	firstRequest := FindPartner{
		Type:      TypeFindPartner,
		RequestID: "req-match-1",
		Payload: FindPartnerPayload{
			NativeLanguageCode:   "ru",
			LearningLanguageCode: "en",
		},
	}
	if err := firstConn.WriteJSON(firstRequest); err != nil {
		t.Fatalf("write first find_partner: %v", err)
	}

	var waiting SearchWaiting
	if err := firstConn.ReadJSON(&waiting); err != nil {
		t.Fatalf("read first search_waiting: %v", err)
	}
	if waiting.Type != TypeSearchWaiting || waiting.RequestID != firstRequest.RequestID {
		t.Fatalf("waiting response = %+v, want type %q and request_id %q", waiting, TypeSearchWaiting, firstRequest.RequestID)
	}

	secondRequest := FindPartner{
		Type:      TypeFindPartner,
		RequestID: "req-match-2",
		Payload: FindPartnerPayload{
			NativeLanguageCode:   "en",
			LearningLanguageCode: "ru",
		},
	}
	if err := secondConn.WriteJSON(secondRequest); err != nil {
		t.Fatalf("write second find_partner: %v", err)
	}

	var firstMatch MatchFound
	if err := firstConn.ReadJSON(&firstMatch); err != nil {
		t.Fatalf("read first match_found: %v", err)
	}
	var secondMatch MatchFound
	if err := secondConn.ReadJSON(&secondMatch); err != nil {
		t.Fatalf("read second match_found: %v", err)
	}

	if firstMatch.Type != TypeMatchFound || secondMatch.Type != TypeMatchFound {
		t.Errorf("match types = (%q, %q), want %q", firstMatch.Type, secondMatch.Type, TypeMatchFound)
	}
	if firstMatch.RequestID != firstRequest.RequestID {
		t.Errorf("first request_id = %q, want %q", firstMatch.RequestID, firstRequest.RequestID)
	}
	if secondMatch.RequestID != secondRequest.RequestID {
		t.Errorf("second request_id = %q, want %q", secondMatch.RequestID, secondRequest.RequestID)
	}
	if firstMatch.Payload.MatchID != matchID || secondMatch.Payload.MatchID != matchID {
		t.Errorf("match IDs = (%q, %q), want %q", firstMatch.Payload.MatchID, secondMatch.Payload.MatchID, matchID)
	}
}

func TestWebsocketHandlerMatchedClientsRelayMessagesBothWays(t *testing.T) {
	firstClientID := newTestClientID(t, "client-chat-1")
	secondClientID := newTestClientID(t, "client-chat-2")
	sessions := NewSessionRegistry()
	rooms := roomservice.New(roomrepository.NewMemoryRepository(), roomservice.GenerateRoomID)
	chatDelivery := NewChatDelivery(sessions)
	chatService := chatservice.New(chatDelivery, rooms)
	handler := NewWebsocketHandler(
		matchmakingservice.New(repository.NewMemoryRepository()),
		sequenceClientIDGenerator(firstClientID, secondClientID),
		rooms,
		sessions,
		chatService,
	)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	firstConn := dialTestConnection(t, server.URL)
	secondConn := dialTestConnection(t, server.URL)

	firstFind := FindPartner{
		Type:      TypeFindPartner,
		RequestID: "req-chat-find-1",
		Payload: FindPartnerPayload{
			NativeLanguageCode:   "ru",
			LearningLanguageCode: "en",
		},
	}
	if err := firstConn.WriteJSON(firstFind); err != nil {
		t.Fatalf("first client write find_partner: %v", err)
	}
	var waiting SearchWaiting
	if err := firstConn.ReadJSON(&waiting); err != nil {
		t.Fatalf("first client read search_waiting: %v", err)
	}
	if waiting.Type != TypeSearchWaiting || waiting.RequestID != firstFind.RequestID {
		t.Fatalf("search_waiting = %+v, want type %q and request ID %q", waiting, TypeSearchWaiting, firstFind.RequestID)
	}

	secondFind := FindPartner{
		Type:      TypeFindPartner,
		RequestID: "req-chat-find-2",
		Payload: FindPartnerPayload{
			NativeLanguageCode:   "en",
			LearningLanguageCode: "ru",
		},
	}
	if err := secondConn.WriteJSON(secondFind); err != nil {
		t.Fatalf("second client write find_partner: %v", err)
	}

	var firstMatch MatchFound
	if err := firstConn.ReadJSON(&firstMatch); err != nil {
		t.Fatalf("first client read match_found: %v", err)
	}
	var secondMatch MatchFound
	if err := secondConn.ReadJSON(&secondMatch); err != nil {
		t.Fatalf("second client read match_found: %v", err)
	}
	if firstMatch.Type != TypeMatchFound || secondMatch.Type != TypeMatchFound {
		t.Fatalf("match types = (%q, %q), want %q", firstMatch.Type, secondMatch.Type, TypeMatchFound)
	}
	if firstMatch.RequestID != firstFind.RequestID || secondMatch.RequestID != secondFind.RequestID {
		t.Errorf(
			"match request IDs = (%q, %q), want (%q, %q)",
			firstMatch.RequestID,
			secondMatch.RequestID,
			firstFind.RequestID,
			secondFind.RequestID,
		)
	}
	if firstMatch.Payload.MatchID == "" || firstMatch.Payload.MatchID != secondMatch.Payload.MatchID {
		t.Fatalf(
			"match IDs = (%q, %q), want same non-empty value",
			firstMatch.Payload.MatchID,
			secondMatch.Payload.MatchID,
		)
	}

	matchID := firstMatch.Payload.MatchID
	firstText := "Hello from first client"
	if err := firstConn.WriteJSON(SendMessage{
		Type:      TypeSendMessage,
		RequestID: "req-chat-message-1",
		Payload: MessagePayload{
			MatchID: matchID,
			Text:    firstText,
		},
	}); err != nil {
		t.Fatalf("first client write send_message: %v", err)
	}
	assertReceivedChatMessage(t, secondConn, matchID, firstText)

	secondText := "Hello from second client"
	if err := secondConn.WriteJSON(SendMessage{
		Type:      TypeSendMessage,
		RequestID: "req-chat-message-2",
		Payload: MessagePayload{
			MatchID: matchID,
			Text:    secondText,
		},
	}); err != nil {
		t.Fatalf("second client write send_message: %v", err)
	}
	assertReceivedChatMessage(t, firstConn, matchID, secondText)
}

func TestWebsocketHandlerDisconnectClosesActiveRoom(t *testing.T) {
	firstClientID := newTestClientID(t, "client-room-disconnect-1")
	secondClientID := newTestClientID(t, "client-room-disconnect-2")
	wantRoomID, err := room.NewRoomID("room-disconnect-1")
	if err != nil {
		t.Fatalf("NewRoomID(): %v", err)
	}

	rooms := roomservice.New(roomrepository.NewMemoryRepository(), func() (room.RoomID, error) {
		return wantRoomID, nil
	})
	handler := NewWebsocketHandler(
		matchmakingservice.New(repository.NewMemoryRepository()),
		sequenceClientIDGenerator(firstClientID, secondClientID),
		rooms,
		NewSessionRegistry(),
		&fakeChatService{},
	)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	firstConn := dialTestConnection(t, server.URL)
	secondConn := dialTestConnection(t, server.URL)
	firstRequest := FindPartner{
		Type:      TypeFindPartner,
		RequestID: "req-room-disconnect-1",
		Payload: FindPartnerPayload{
			NativeLanguageCode:   "ru",
			LearningLanguageCode: "en",
		},
	}
	secondRequest := FindPartner{
		Type:      TypeFindPartner,
		RequestID: "req-room-disconnect-2",
		Payload: FindPartnerPayload{
			NativeLanguageCode:   "en",
			LearningLanguageCode: "ru",
		},
	}

	if err := firstConn.WriteJSON(firstRequest); err != nil {
		t.Fatalf("write first find_partner: %v", err)
	}
	var waiting SearchWaiting
	if err := firstConn.ReadJSON(&waiting); err != nil {
		t.Fatalf("read first search_waiting: %v", err)
	}
	if err := secondConn.WriteJSON(secondRequest); err != nil {
		t.Fatalf("write second find_partner: %v", err)
	}
	var firstMatch MatchFound
	if err := firstConn.ReadJSON(&firstMatch); err != nil {
		t.Fatalf("read first match_found: %v", err)
	}
	var secondMatch MatchFound
	if err := secondConn.ReadJSON(&secondMatch); err != nil {
		t.Fatalf("read second match_found: %v", err)
	}

	if err := firstConn.Close(); err != nil {
		t.Fatalf("close first connection: %v", err)
	}

	deadline := time.Now().Add(time.Second)
	for {
		_, found, err := rooms.FindRoomByClientID(context.Background(), secondClientID)
		if err != nil {
			t.Fatalf("FindRoomByClientID(): %v", err)
		}
		if !found {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("room was not closed after client disconnect")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestWebsocketHandlerCreateRoomFailureNotifiesBothClients(t *testing.T) {
	firstClientID := newTestClientID(t, "client-room-error-1")
	secondClientID := newTestClientID(t, "client-room-error-2")
	wantErr := errors.New("create room")

	roomService := &fakeRoomService{
		createRoom: func(
			context.Context,
			matchmaking.ClientID,
			matchmaking.ClientID,
		) (room.Room, error) {
			return room.Room{}, wantErr
		},
	}

	handler := NewWebsocketHandler(
		matchmakingservice.New(repository.NewMemoryRepository()),
		sequenceClientIDGenerator(firstClientID, secondClientID),
		roomService,
		NewSessionRegistry(),
		&fakeChatService{},
	)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	firstConn := dialTestConnection(t, server.URL)
	secondConn := dialTestConnection(t, server.URL)
	firstRequest := FindPartner{
		Type:      TypeFindPartner,
		RequestID: "req-room-error-1",
		Payload: FindPartnerPayload{
			NativeLanguageCode:   "ru",
			LearningLanguageCode: "en",
		},
	}
	secondRequest := FindPartner{
		Type:      TypeFindPartner,
		RequestID: "req-room-error-2",
		Payload: FindPartnerPayload{
			NativeLanguageCode:   "en",
			LearningLanguageCode: "ru",
		},
	}

	if err := firstConn.WriteJSON(firstRequest); err != nil {
		t.Fatalf("write first find_partner: %v", err)
	}
	var waiting SearchWaiting
	if err := firstConn.ReadJSON(&waiting); err != nil {
		t.Fatalf("read first search_waiting: %v", err)
	}
	if err := secondConn.WriteJSON(secondRequest); err != nil {
		t.Fatalf("write second find_partner: %v", err)
	}

	assertInternalServerError(t, secondConn, secondRequest.RequestID)
	assertInternalServerError(t, firstConn, firstRequest.RequestID)
}

func TestWebsocketHandlerMatchDeliveryFailureClosesCreatedRoom(t *testing.T) {
	currentClientID := newTestClientID(t, "client-delivery-current")
	partnerClientID := newTestClientID(t, "client-delivery-partner")
	createdRoom := newTestRoom(t, "room-delivery-1", currentClientID, partnerClientID)
	closeCalls := make(chan room.RoomID, 1)

	partner := newTestWaitingUser(t, partnerClientID, matchmaking.LanguageCodeEN, matchmaking.LanguageCodeRU)
	matchmakingService := &fakeMatchmakingService{
		findPartner: func(context.Context, matchmaking.WaitingUser) (matchmakingservice.MatchResult, error) {
			return matchmakingservice.MatchResult{Matched: true, Partner: partner}, nil
		},
	}
	roomService := &fakeRoomService{
		createRoom: func(context.Context, matchmaking.ClientID, matchmaking.ClientID) (room.Room, error) {
			return createdRoom, nil
		},
		closeRoom: func(ctx context.Context, roomID room.RoomID) error {
			if err := ctx.Err(); err != nil {
				t.Errorf("CloseRoom() received cancelled context: %v", err)
			}
			closeCalls <- roomID
			return nil
		},
	}
	handler := NewWebsocketHandler(
		matchmakingService,
		nil, roomService,
		NewSessionRegistry(),
		&fakeChatService{},
	)

	currentSession := newClientSession(context.Background(), currentClientID, nil)
	t.Cleanup(currentSession.cancel)
	partnerSession := newClientSession(context.Background(), partnerClientID, nil)
	partnerSession.setSearchRequestID("req-delivery-partner")
	if !handler.sessions.register(partnerSession) {
		t.Fatal("register partner session")
	}
	for i := 0; i < cap(partnerSession.outbound); i++ {
		partnerSession.outbound <- struct{}{}
	}
	partnerSession.cancel()

	envelope := incomingEnvelope{
		Type:      TypeFindPartner,
		RequestID: "req-delivery-current",
		Payload: []byte(`{
			"native_language_code":"ru",
			"learning_language_code":"en"
		}`),
	}
	if err := handler.handleFindPartner(currentSession, envelope); err != nil {
		t.Fatalf("handleFindPartner() returned unexpected error: %v", err)
	}

	select {
	case gotRoomID := <-closeCalls:
		if gotRoomID != createdRoom.RoomID() {
			t.Errorf("CloseRoom() room ID = %v, want %v", gotRoomID, createdRoom.RoomID())
		}
	default:
		t.Fatal("CloseRoom() was not called after match delivery failure")
	}

	select {
	case message := <-currentSession.outbound:
		errorResponse, ok := message.(Error)
		if !ok {
			t.Fatalf("current client message type = %T, want Error", message)
		}
		if errorResponse.RequestID == nil || *errorResponse.RequestID != envelope.RequestID {
			t.Errorf("error request_id = %v, want %q", errorResponse.RequestID, envelope.RequestID)
		}
	case <-time.After(time.Second):
		t.Fatal("current client did not receive delivery failure notification")
	}
}

func openTestConnection(t *testing.T, handler *WebsocketHandler) *gorilla.Conn {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return dialTestConnection(t, server.URL)
}

func dialTestConnection(t *testing.T, serverURL string) *gorilla.Conn {
	t.Helper()

	wsURL := "ws" + strings.TrimPrefix(serverURL, "http")
	conn, response, err := gorilla.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		if response != nil {
			t.Fatalf("dial WebSocket: %v (HTTP status %s)", err, response.Status)
		}
		t.Fatalf("dial WebSocket: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
	})

	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}

	return conn
}

func fixedClientIDGenerator(clientID matchmaking.ClientID) ClientIDGenerator {
	return func() (matchmaking.ClientID, error) {
		return clientID, nil
	}
}

func sequenceClientIDGenerator(clientIDs ...matchmaking.ClientID) ClientIDGenerator {
	ids := make(chan matchmaking.ClientID, len(clientIDs))
	for _, clientID := range clientIDs {
		ids <- clientID
	}
	close(ids)

	return func() (matchmaking.ClientID, error) {
		clientID, ok := <-ids
		if !ok {
			return matchmaking.ClientID{}, errors.New("test client ID sequence exhausted")
		}
		return clientID, nil
	}
}

func newTestClientID(t *testing.T, value string) matchmaking.ClientID {
	t.Helper()

	clientID, err := matchmaking.NewClientID(value)
	if err != nil {
		t.Fatalf("NewClientID(%q): %v", value, err)
	}
	return clientID
}

func newTestRoom(
	t *testing.T,
	roomIDValue string,
	firstClientID matchmaking.ClientID,
	secondClientID matchmaking.ClientID,
) room.Room {
	t.Helper()

	clients, err := room.NewConnectedClientsPair(firstClientID, secondClientID)
	if err != nil {
		t.Fatalf("NewConnectedClientsPair(): %v", err)
	}
	roomID, err := room.NewRoomID(roomIDValue)
	if err != nil {
		t.Fatalf("NewRoomID(%q): %v", roomIDValue, err)
	}
	target, err := room.NewRoom(clients, roomID)
	if err != nil {
		t.Fatalf("NewRoom(): %v", err)
	}
	return target
}

func newTestWaitingUser(
	t *testing.T,
	clientID matchmaking.ClientID,
	nativeCode matchmaking.LanguageCode,
	learningCode matchmaking.LanguageCode,
) matchmaking.WaitingUser {
	t.Helper()

	native, err := matchmaking.NewLanguage(nativeCode)
	if err != nil {
		t.Fatalf("NewLanguage(%q): %v", nativeCode, err)
	}
	learning, err := matchmaking.NewLanguage(learningCode)
	if err != nil {
		t.Fatalf("NewLanguage(%q): %v", learningCode, err)
	}
	pair, err := matchmaking.NewLanguagePair(native, learning)
	if err != nil {
		t.Fatalf("NewLanguagePair(): %v", err)
	}
	user, err := matchmaking.NewWaitingUser(clientID, pair)
	if err != nil {
		t.Fatalf("NewWaitingUser(): %v", err)
	}
	return user
}

func newTestSendMessageEnvelope(
	t *testing.T,
	requestID string,
	matchID string,
	text string,
) incomingEnvelope {
	t.Helper()

	payload, err := json.Marshal(MessagePayload{MatchID: matchID, Text: text})
	if err != nil {
		t.Fatalf("marshal send_message payload: %v", err)
	}
	return incomingEnvelope{
		Type:      TypeSendMessage,
		RequestID: requestID,
		Payload:   payload,
	}
}

func assertOutboundError(
	t *testing.T,
	session *clientSession,
	requestID string,
	wantCode ErrorCode,
) {
	t.Helper()

	select {
	case message := <-session.outbound:
		errorResponse, ok := message.(Error)
		if !ok {
			t.Fatalf("outbound message type = %T, want Error", message)
		}
		if errorResponse.RequestID == nil || *errorResponse.RequestID != requestID {
			t.Errorf("error request ID = %v, want %q", errorResponse.RequestID, requestID)
		}
		if errorResponse.Payload.Code != wantCode {
			t.Errorf("error code = %q, want %q", errorResponse.Payload.Code, wantCode)
		}
	default:
		t.Fatal("session outbound does not contain error response")
	}
}

func assertReceivedChatMessage(
	t *testing.T,
	conn *gorilla.Conn,
	wantMatchID string,
	wantText string,
) {
	t.Helper()

	var received ReceiveMessage
	if err := conn.ReadJSON(&received); err != nil {
		t.Fatalf("read receive_message: %v", err)
	}
	if received.Type != TypeReceiveMessage {
		t.Errorf("receive_message type = %q, want %q", received.Type, TypeReceiveMessage)
	}
	if received.Payload.MatchID != wantMatchID {
		t.Errorf("receive_message match ID = %q, want %q", received.Payload.MatchID, wantMatchID)
	}
	if received.Payload.Text != wantText {
		t.Errorf("receive_message text = %q, want %q", received.Payload.Text, wantText)
	}
}

func assertInternalServerError(t *testing.T, conn *gorilla.Conn, requestID string) {
	t.Helper()

	var response Error
	if err := conn.ReadJSON(&response); err != nil {
		t.Fatalf("read internal_server_error: %v", err)
	}
	if response.Type != TypeError {
		t.Errorf("response type = %q, want %q", response.Type, TypeError)
	}
	if response.RequestID == nil || *response.RequestID != requestID {
		t.Errorf("response request_id = %v, want %q", response.RequestID, requestID)
	}
	if response.Payload.Code != ErrorInternalServerError {
		t.Errorf("error code = %q, want %q", response.Payload.Code, ErrorInternalServerError)
	}
}

package websocket

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gorilla "github.com/gorilla/websocket"
	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/matchmaking"
	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/room"
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
	handler := NewWebsocketHandler(matchmakingService, nil, roomService)

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

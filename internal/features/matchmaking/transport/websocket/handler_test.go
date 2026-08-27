package websocket

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gorilla "github.com/gorilla/websocket"
	"github.com/wizardVadim/fluent-swap-core/internal/core/domains/matchmaking"
	"github.com/wizardVadim/fluent-swap-core/internal/features/matchmaking/repository"
)

type fakeService struct {
	findPartner  func(context.Context, matchmaking.WaitingUser) (repository.MatchResult, error)
	cancelSearch func(context.Context, matchmaking.ClientID) error
}

func (f *fakeService) FindPartner(
	ctx context.Context,
	user matchmaking.WaitingUser,
) (repository.MatchResult, error) {
	if f.findPartner == nil {
		return repository.MatchResult{}, nil
	}
	return f.findPartner(ctx, user)
}

func (f *fakeService) CancelSearch(ctx context.Context, clientID matchmaking.ClientID) error {
	if f.cancelSearch == nil {
		return nil
	}
	return f.cancelSearch(ctx, clientID)
}

func TestWebsocketHandlerFindPartnerReturnsSearchWaiting(t *testing.T) {
	clientID := newTestClientID(t, "client-test-1")
	findCalls := make(chan matchmaking.WaitingUser, 1)

	service := &fakeService{
		findPartner: func(ctx context.Context, user matchmaking.WaitingUser) (repository.MatchResult, error) {
			if err := ctx.Err(); err != nil {
				t.Errorf("FindPartner() received cancelled context: %v", err)
			}
			findCalls <- user
			return repository.MatchResult{Matched: false}, nil
		},
	}

	conn := openTestConnection(t, NewWebsocketHandler(service, fixedClientIDGenerator(clientID)))

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

	service := &fakeService{
		cancelSearch: func(ctx context.Context, gotClientID matchmaking.ClientID) error {
			if err := ctx.Err(); err != nil {
				t.Errorf("CancelSearch() received cancelled context: %v", err)
			}
			cancelCalls <- gotClientID
			return nil
		},
	}

	conn := openTestConnection(t, NewWebsocketHandler(service, fixedClientIDGenerator(clientID)))

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
	service := &fakeService{
		cancelSearch: func(context.Context, matchmaking.ClientID) error {
			cancelCalls <- struct{}{}
			return nil
		},
	}

	conn := openTestConnection(t, NewWebsocketHandler(service, fixedClientIDGenerator(clientID)))

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

func openTestConnection(t *testing.T, handler *WebsocketHandler) *gorilla.Conn {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
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

func newTestClientID(t *testing.T, value string) matchmaking.ClientID {
	t.Helper()

	clientID, err := matchmaking.NewClientID(value)
	if err != nil {
		t.Fatalf("NewClientID(%q): %v", value, err)
	}
	return clientID
}

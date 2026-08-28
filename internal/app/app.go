package app

import (
	"net/http"
	"time"

	chatservice "github.com/wizardVadim/fluent-swap-core/internal/features/chat/service"
	matchmakingrepository "github.com/wizardVadim/fluent-swap-core/internal/features/matchmaking/repository"
	matchmakingservice "github.com/wizardVadim/fluent-swap-core/internal/features/matchmaking/service"
	roomrepository "github.com/wizardVadim/fluent-swap-core/internal/features/room/repository"
	roomservice "github.com/wizardVadim/fluent-swap-core/internal/features/room/service"
	"github.com/wizardVadim/fluent-swap-core/internal/transport/websocket"
)

func NewHTTPServer(addr string, readHeaderTimeout time.Duration) *http.Server {

	sessions := websocket.NewSessionRegistry()
	chatDelivery := websocket.NewChatDelivery(sessions)

	matchmakingRepository := matchmakingrepository.NewMemoryRepository()
	matchmakingService := matchmakingservice.New(matchmakingRepository)

	roomRepository := roomrepository.NewMemoryRepository()
	roomService := roomservice.New(roomRepository, roomservice.GenerateRoomID)

	chatService := chatservice.New(chatDelivery, roomService)

	handler := websocket.NewWebsocketHandler(matchmakingService, websocket.GenerateClientID, roomService, sessions, chatService)

	mux := http.NewServeMux()
	mux.Handle("/ws/matchmaking", handler)

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	return server
}

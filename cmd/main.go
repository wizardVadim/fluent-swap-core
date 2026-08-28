package main

import (
	"errors"
	"net/http"
	"time"

	matchmakingrepository "github.com/wizardVadim/fluent-swap-core/internal/features/matchmaking/repository"
	matchmakingservice "github.com/wizardVadim/fluent-swap-core/internal/features/matchmaking/service"
	roomrepository "github.com/wizardVadim/fluent-swap-core/internal/features/room/repository"
	roomservice "github.com/wizardVadim/fluent-swap-core/internal/features/room/service"
	"github.com/wizardVadim/fluent-swap-core/internal/transport/websocket"
)

func main() {
	matchmakingRepository := matchmakingrepository.NewMemoryRepository()
	matchmakingService := matchmakingservice.New(matchmakingRepository)

	roomRepository := roomrepository.NewMemoryRepository()
	roomService := roomservice.New(roomRepository, roomservice.GenerateRoomID)
	sessions := websocket.NewSessionRegistry()
	handler := websocket.NewWebsocketHandler(matchmakingService, websocket.GenerateClientID, roomService, sessions)

	mux := http.NewServeMux()
	mux.Handle("/ws/matchmaking", handler)

	server := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	err := server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}

package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/wizardVadim/fluent-swap-core/internal/features/matchmaking/repository"
	matchmakingservice "github.com/wizardVadim/fluent-swap-core/internal/features/matchmaking/service"
	"github.com/wizardVadim/fluent-swap-core/internal/features/matchmaking/transport/websocket"
)

func main() {
	matchmakingRepository := repository.NewMemoryRepository()
	matchmakingService := matchmakingservice.New(matchmakingRepository)
	handler := websocket.NewWebsocketHandler(matchmakingService, websocket.GenerateClientID, websocket.GenerateMatchID)

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

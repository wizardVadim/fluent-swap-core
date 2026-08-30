package app

import (
	"errors"
	"net/http"
	"time"

	logger_pack "github.com/wizardVadim/fluent-swap-core/internal/core/logger"
	chatservice "github.com/wizardVadim/fluent-swap-core/internal/features/chat/service"
	matchmakingrepository "github.com/wizardVadim/fluent-swap-core/internal/features/matchmaking/repository"
	matchmakingservice "github.com/wizardVadim/fluent-swap-core/internal/features/matchmaking/service"
	roomrepository "github.com/wizardVadim/fluent-swap-core/internal/features/room/repository"
	roomservice "github.com/wizardVadim/fluent-swap-core/internal/features/room/service"
	"github.com/wizardVadim/fluent-swap-core/internal/transport/websocket"
)

type App struct {
	Server      *http.Server
	closeLogger func() error
}

func New(addr string, readHeaderTimeout time.Duration) (*App, error) {
	logger, closeLogger, err := logger_pack.NewLogger("INFO")
	if err != nil {
		return nil, errors.Join(ErrCannotInitLogger, err)
	}

	sessions := websocket.NewSessionRegistry()
	chatDelivery := websocket.NewChatDelivery(sessions)

	matchmakingRepository := matchmakingrepository.NewMemoryRepository()
	matchmakingService := matchmakingservice.New(matchmakingRepository)

	roomRepository := roomrepository.NewMemoryRepository()
	roomService := roomservice.New(roomRepository, roomservice.GenerateRoomID)

	chatService := chatservice.New(chatDelivery, roomService)

	handler := websocket.NewWebsocketHandler(
		matchmakingService,
		websocket.GenerateClientID,
		roomService,
		sessions,
		chatService,
		logger,
	)

	logger.Info("Application has been built")

	return &App{
		Server:      newHTTPServer(addr, readHeaderTimeout, handler),
		closeLogger: closeLogger,
	}, nil
}

func newHTTPServer(addr string, readHeaderTimeout time.Duration, handler *websocket.WebsocketHandler) *http.Server {

	mux := http.NewServeMux()
	mux.Handle("/ws/matchmaking", handler)

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	return server
}

func (a *App) Close() error {
	return a.closeLogger()
}

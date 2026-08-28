package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/wizardVadim/fluent-swap-core/internal/app"
)

func main() {
	server := app.NewHTTPServer(":9090", 5*time.Second)

	err := server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}

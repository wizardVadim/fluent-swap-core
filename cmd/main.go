package main

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/wizardVadim/fluent-swap-core/internal/app"
)

const port = ":9090"
const timeout = 5 * time.Second

func main() {
	application, err := app.New(port, timeout)
	if err != nil {
		fmt.Printf("cannot initialize application: %v\n", err)
		return
	}
	defer func() {
		if err := application.Close(); err != nil {
			fmt.Printf("close application: %v\n", err)
		}
	}()

	if err := application.Server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}

package main

import "github.com/wizardVadim/fluent-swap-core/internal/core"

func main() {
	logger, close, err := core.NewLogger("INFO")

	if err != nil {
		panic(err)
	}

	defer close()

	logger.Info("check")
}

package main

import logger_pack "github.com/wizardVadim/fluent-swap-core/internal/core/logger"

func main() {
	logger, close, err := logger_pack.NewLogger("INFO")

	if err != nil {
		panic(err)
	}

	defer close()

	logger.Info("check")
}

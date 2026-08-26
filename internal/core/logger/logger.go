package logger_pack

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var LOG_DIR string = "out/logs"
var LOG_DIR_TIME_FORMAT string = "2006-01-02T15-04-05.000000"
var LOG_FILE_TIME_FORMAT string = "2006-01-02T15:04:05.000000"

func NewLogger(logLevel string) (*zap.Logger, func() error, error) {
	lvl := zap.NewAtomicLevel()
	if err := lvl.UnmarshalText([]byte(logLevel)); err != nil {
		return nil, nil, fmt.Errorf("unmarshal log lever: %w", err)
	}

	if err := os.MkdirAll(LOG_DIR, 0755); err != nil {
		return nil, nil, fmt.Errorf("mkdir log folder: %w", err)

	}

	timestamp := time.Now().UTC().Format(LOG_DIR_TIME_FORMAT)
	logFilePath := filepath.Join(LOG_DIR, fmt.Sprintf("%s.log", timestamp))

	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, nil, fmt.Errorf("open log file: %w", err)
	}

	cfg := zap.NewDevelopmentEncoderConfig()
	cfg.EncodeTime = zapcore.TimeEncoderOfLayout(LOG_FILE_TIME_FORMAT)

	encoder := zapcore.NewConsoleEncoder(cfg)

	core := zapcore.NewTee(
		zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), lvl),
		zapcore.NewCore(encoder, zapcore.AddSync(logFile), lvl),
	)

	logger := zap.New(
		core,
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)

	return logger, logFile.Close, nil
}

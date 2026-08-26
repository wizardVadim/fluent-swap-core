package core_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wizardVadim/fluent-swap-core/internal/core"
	"go.uber.org/zap"
)

func TestNewLogger_WritesMessageToFile(t *testing.T) {

	originalLogDir := core.LOG_DIR
	core.LOG_DIR = t.TempDir()

	t.Cleanup(func() {
		core.LOG_DIR = originalLogDir
	})

	logger, closeLogger, err := core.NewLogger("INFO")
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}

	t.Cleanup(func() {
		_ = logger.Sync()

		if err := closeLogger(); err != nil {
			t.Errorf("close logger: %v", err)
		}
	})

	logger.Info(
		"match created",
		zap.String("room_id", "room-123"),
	)

	// if err := logger.Sync(); err != nil {
	// 	t.Fatalf("sync error: %v", err)
	// }

	files, err := filepath.Glob(filepath.Join(core.LOG_DIR, "*.log"))
	if err != nil {
		t.Fatalf("find log files: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("log files count = %d, want 1", len(files))
	}

	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}

	logEntry := string(data)

	if !strings.Contains(logEntry, "match created") {
		t.Errorf("log does not contain message: %q", logEntry)
	}

	if !strings.Contains(logEntry, "room-123") {
		t.Errorf("log does not contain room_id: %q", logEntry)
	}
}

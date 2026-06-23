package messaging

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTxRx(t *testing.T) {
	tx, rx := NewChannels(slog.LevelDebug)
	defer tx.Close()
	tx.Infof("Info Message: %d", 123)

	msg := rx.NextMessage(context.Background())
	msg.timestamp = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	require.Equal(t, " 2026-01-01 12:00:00 [INFO ]: Info Message: 123", msg.FormatMessageNoColor())
}

func TestFormatting(t *testing.T) {
	msg := newMessage(slog.LevelDebug, "Test Debug")
	msg.timestamp = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	require.Equal(t, " 2026-01-01 12:00:00 [TRACE]: Test Debug", msg.FormatMessageNoColor())
}

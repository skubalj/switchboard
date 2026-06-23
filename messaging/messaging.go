package messaging

import (
	"context"
	"fmt"
	"image/color"
	"log/slog"
	"time"

	"charm.land/lipgloss/v2"
)

func NewChannels(logLevel slog.Level) (Tx, Rx) {
	errMessages := make(chan Message, 5)
	warningMessages := make(chan Message, 5)
	infoMessages := make(chan Message, 5)
	traceMessages := make(chan Message, 5)

	return Tx{logLevel, errMessages, warningMessages, infoMessages, traceMessages},
		Rx{errMessages, warningMessages, infoMessages, traceMessages}
}

type Tx struct {
	logLevel        slog.Level
	errMessages     chan<- Message
	warningMessages chan<- Message
	infoMessages    chan<- Message
	traceMessages   chan<- Message
}

func (tx Tx) Errorf(template string, args ...any) {
	if tx.logLevel <= slog.LevelError {
		tx.errMessages <- newMessage(slog.LevelError, fmt.Errorf(template, args...).Error())
	}
}

func (tx Tx) SendError(err error) {
	if tx.logLevel <= slog.LevelError {
		tx.errMessages <- newMessage(slog.LevelError, err.Error())
	}
}

func (tx Tx) Warnf(template string, args ...any) {
	if tx.logLevel <= slog.LevelWarn {
		tx.warningMessages <- newMessage(slog.LevelWarn, fmt.Sprintf(template, args...))
	}
}

func (tx Tx) Infof(template string, args ...any) {
	if tx.logLevel <= slog.LevelInfo {
		tx.infoMessages <- newMessage(slog.LevelInfo, fmt.Sprintf(template, args...))
	}
}

func (tx Tx) Tracef(template string, args ...any) {
	if tx.logLevel <= slog.LevelDebug {
		tx.traceMessages <- newMessage(slog.LevelDebug, fmt.Sprintf(template, args...))
	}
}

func (tx Tx) Close() {
	close(tx.errMessages)
	close(tx.warningMessages)
	close(tx.infoMessages)
	close(tx.traceMessages)
}

type Rx struct {
	errMessages     <-chan Message
	warningMessages <-chan Message
	infoMessages    <-chan Message
	traceMessages   <-chan Message
}

func (rx Rx) NextMessage(ctx context.Context) Message {
	select {
	case <-ctx.Done():
		return Message{}
	case err := <-rx.errMessages:
		return err
	case warn := <-rx.warningMessages:
		return warn
	case info := <-rx.infoMessages:
		return info
	case trace := <-rx.traceMessages:
		return trace
	}
}

type Message struct {
	timestamp time.Time
	color     color.Color
	tag       string
	message   string
}

func newMessage(level slog.Level, message string) Message {
	var color color.Color
	var tag string
	switch level {
	case slog.LevelDebug:
		color = lipgloss.BrightBlue
		tag = "TRACE"
	case slog.LevelInfo:
		color = lipgloss.Green
		tag = "INFO "
	case slog.LevelWarn:
		color = lipgloss.Yellow
		tag = "WARN "
	case slog.LevelError:
		color = lipgloss.Red
		tag = "ERROR"
	}

	return Message{
		timestamp: time.Now().Local(),
		color:     color,
		tag:       tag,
		message:   message,
	}
}

func (m Message) IsZero() bool {
	return m.timestamp.IsZero()
}

func (m Message) FormatMessage() string {
	return fmt.Sprintf(
		" %s [%s]: %s",
		m.timestamp.Format(time.DateTime),
		lipgloss.NewStyle().Foreground(m.color).Render(m.tag),
		m.message,
	)
}

func (m Message) FormatMessageNoColor() string {
	return fmt.Sprintf(" %s [%s]: %s", m.timestamp.Format(time.DateTime), m.tag, m.message)
}

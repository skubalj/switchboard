package messaging

import (
	"context"
	"fmt"
)

func NewChannels() (Tx, Rx) {
	errMessages := make(chan error, 5)
	warningMessages := make(chan string, 5)
	infoMessages := make(chan string, 5)
	debugMessages := make(chan string, 5)

	return Tx{errMessages, warningMessages, infoMessages, debugMessages},
		Rx{errMessages, warningMessages, infoMessages, debugMessages}
}

type Tx struct {
	errMessages     chan<- error
	warningMessages chan<- string
	infoMessages    chan<- string
	debugMessages   chan<- string
}

func (tx Tx) Errorf(template string, args ...any) {
	tx.errMessages <- fmt.Errorf(template, args...)
}

func (tx Tx) SendError(err error) {
	tx.errMessages <- err
}

func (tx Tx) Warnf(template string, args ...any) {
	tx.warningMessages <- fmt.Sprintf(template, args...)
}

func (tx Tx) Infof(template string, args ...any) {
	tx.infoMessages <- fmt.Sprintf(template, args...)
}

func (tx Tx) Debugf(template string, args ...any) {
	tx.debugMessages <- fmt.Sprintf(template, args...)
}

type Rx struct {
	errMessages     <-chan error
	warningMessages <-chan string
	infoMessages    <-chan string
	debugMessages   <-chan string
}

func (rx Rx) NextMessage(ctx context.Context) string {
	select {
	case <-ctx.Done():
		return ""
	case err := <-rx.errMessages:
		return "⛔: " + err.Error()
	case warn := <-rx.warningMessages:
		return "⚠️: " + warn
	case info := <-rx.infoMessages:
		return "📨: " + info
	case debug := <-rx.debugMessages:
		return "🪲: " + debug
	}
}

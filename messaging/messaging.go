package messaging

import (
	"context"
)

func NewChannels() (Tx, Rx) {
	errMessages := make(chan error, 5)

	return Tx{errMessages}, Rx{errMessages}
}

type Tx struct {
	errMessages chan<- error
}

func (tx Tx) SendError(err error) {
	tx.errMessages <- err
}

type Rx struct {
	errMessages <-chan error
}

func (rx Rx) NextMessage(ctx context.Context) string {
	select {
	case <-ctx.Done():
		return ""
	case err := <-rx.errMessages:
		return "⛔: " + err.Error()
		// case warn := <-rx.warnMessages:
		// 	return "⚠️: " + warn
	}
}

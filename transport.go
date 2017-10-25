package engine_io

import (
	"errors"
	"fmt"

	"github.com/jjeffcaii/engine.io/parser"
)

type messageOK struct {
	Sid          string      `json:"sid"`
	Upgrades     []Transport `json:"upgrades"`
	PingInterval uint32      `json:"pingInterval"`
	PingTimeout  uint32      `json:"pingTimeout"`
}

type transport interface {
	//OnFlush() Socket
	//OnDrain() Socket
	//OnPacket() Socket
	transport(ctx *context) error
	write(packet *parser.Packet) error
	flush() error
	close() error
	getEngine() *engineImpl
	upgrading() error
	upgrade() error
}

func newTransport(engine *engineImpl, transport Transport) (transport, error) {
	switch transport {
	default:
		return nil, errors.New(fmt.Sprintf("invalid transport '%s'", transport))
	case WEBSOCKET:
		return newWebsocketTransport(engine), nil
	case POLLING:
		return newXhrTransport(engine), nil
	}
}

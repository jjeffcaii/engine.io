package engine_io

import (
	"errors"
	"fmt"
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
	write(packet *Packet) error
	flush() error
	upgrade() (transport, error)
	close() error
	getEngine() *engineImpl
}

func newTransport(engine *engineImpl, transport string) (transport, error) {
	switch Transport(transport) {
	default:
		return nil, errors.New(fmt.Sprintf("invalid transport '%s'", transport))
	case WEBSOCKET:
		return newWebsocketTransport(engine), nil
	case POLLING:
		return newXhrTransport(engine), nil
	}
}

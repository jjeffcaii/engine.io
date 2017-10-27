package eio

import (
	"net/http"

	"github.com/jjeffcaii/engine.io/parser"
)

// TransportType define the type of transport.
type TransportType int8

const (
	// POLLING use Polling-XHR as Transport.
	POLLING TransportType = iota
	// WEBSOCKET use Websocket as Transport.
	WEBSOCKET TransportType = iota
)

// DefaultPath for engine.io http router.
var DefaultPath = "/engine.io/"

// Engine is the main server/manager.
type Engine interface {
	Router() func(http.ResponseWriter, *http.Request)
	Listen(addr string) error
	GetProtocol() uint8
	GetClients() map[string]Socket
	CountClients() int
	OnConnect(func(socket Socket)) Engine
	Close()
	Debug() string
}

// Transport is used to control socket.
type Transport interface {
	GetType() TransportType
	GetEngine() Engine
	GetSocket() Socket

	setSocket(socket Socket)

	ready(writer http.ResponseWriter, request *http.Request) error
	doReq(writer http.ResponseWriter, request *http.Request)
	doUpgrade() error
	write(packet *parser.Packet) error
	flush() error
	close() error
}

// Socket is a representation of a client.
type Socket interface {
	ID() string
	Server() Engine
	OnClose(func(reason string)) Socket
	OnMessage(func(data []byte)) Socket
	OnError(func(err error)) Socket
	OnUpgrade(func()) Socket
	Send(message interface{}) error
	SendCustom(message interface{}, options parser.PacketOption) error
	Close()
}

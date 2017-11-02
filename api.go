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
	// Router returns a std golang http handler.
	Router() func(http.ResponseWriter, *http.Request)
	// Listen engine server.
	Listen(addr string) error
	// GetProtocol returns engine protocol version.
	GetProtocol() uint8
	// GetClients returns current socket map. (SocketID -> Socket)
	GetClients() map[string]Socket
	// CountClients returns current socket count.
	CountClients() int
	// OnConnect bind handler when sockets created.
	OnConnect(func(socket Socket)) Engine
	// Close current engine server.
	Close()
}

// Transport is used to control socket.
type Transport interface {
	// GetType returns transport type.
	GetType() TransportType
	// GetEngine returns engine of current transport.
	GetEngine() Engine
	// GetSocket returns current socket.
	GetSocket() Socket
	// GetRequest returns native http request.
	GetRequest() *http.Request

	// inner functions.
	setSocket(socket Socket)
	init(writer http.ResponseWriter, request *http.Request) error
	receive(writer http.ResponseWriter, request *http.Request)

	upgradeStart() error
	//the active transport will be used to send rest packet.
	upgradeEnd(tActive Transport) error

	write(packet *parser.Packet) error
	flush() error
	close() error
}

// Socket is a representation of a client.
type Socket interface {
	// ID returns SessionID of socket.
	ID() string
	// Server returns engine of current socket.
	Server() Engine
	// Transport returns the active transport of socket.
	Transport() Transport
	// OnClose bind handler when socket closed.
	OnClose(func(reason string)) Socket
	// OnMessage bind handler when message income.
	OnMessage(func(data []byte)) Socket
	// OnError bind handler when error appeared.
	OnError(func(err error)) Socket
	// OnUpgrade bind handler when socket upgraded.
	OnUpgrade(func()) Socket
	// Send a message.
	Send(message interface{}) error
	// Close current socket.
	Close()
}

package engine_io

import (
	"net/http"

	"github.com/jjeffcaii/engine.io/parser"
)

type TransportType int8

const (
	POLLING   TransportType = iota
	WEBSOCKET TransportType = iota
)

var DEFAULT_PATH = "/engine.io/"

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

type Socket interface {
	Id() string
	Server() Engine
	OnClose(func(reason string)) Socket
	OnMessage(func(data []byte)) Socket
	OnError(func(err error)) Socket
	OnUpgrade(func()) Socket
	Send(message interface{}) error
	SendCustom(message interface{}, options parser.PacketOption) error
	Close()
}

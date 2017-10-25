package engine_io

import (
	"net/http"

	"github.com/jjeffcaii/engine.io/parser"
)

type Transport string

var (
	DEFAULT_PATH string    = "/engine.io/"
	POLLING      Transport = "polling"
	WEBSOCKET    Transport = "websocket"
)

type Engine interface {
	Router() func(http.ResponseWriter, *http.Request)
	Listen(addr string) error
	GetProtocol() uint8
	GetClients() map[string]Socket
	CountClients() int
	OnConnect(func(socket Socket)) Engine
	Close()
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

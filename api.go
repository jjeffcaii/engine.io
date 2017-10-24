package engine_io

import "net/http"

type SendOption uint8
type Transport string

const (
	BINARY   SendOption = 0x01
	COMPRESS SendOption = 0x01 << 1
)

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
	Send(message interface{}) error
	SendCustom(message interface{}, options SendOption) error
	Close()
}

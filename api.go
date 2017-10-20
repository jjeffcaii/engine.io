package engine_io

type SendOption uint8

const (
	BINARY   SendOption = 0x01
	COMPRESS SendOption = 0x01 << 1
)

type Engine interface {
	Listen(addr string) error
	GetProtocol() uint8
	GetClients() map[string]Socket
	OnConnect(func(socket Socket)) Engine
}

type Socket interface {
	Id() string
	Server() Engine

	OnClose(func(reason string)) Socket
	OnMessage(func(data []byte)) Socket
	OnError(func(err error)) Socket
	//OnFlush() Socket
	//OnDrain() Socket
	//OnPacket() Socket
	Send(message interface{}) error
	SendCustom(message interface{}, options SendOption) error
	Close()
}

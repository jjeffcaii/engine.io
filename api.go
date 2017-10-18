package engine_io

type MessageOptions struct {
}

type Engine interface {
	Listen(addr string) error
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
	SendCustom(message interface{}, options *MessageOptions) error
	Close()
}

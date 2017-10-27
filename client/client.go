package eio

type clientOptions struct {
	addr       string
	path       string
	binaryType string
}

//define MessageType
type MessageType uint8

const (
	//message type is string
	StringMsg MessageType = 0x01
	//message type is binary
	BinbaryMsg MessageType = 0x01 << 1
)

//define a web socket client
type WebSocketClient interface {
	Connect(options *clientOptions)
}

type ClientSocket interface {
	OnOpen(func()) ClientSocket
	OnMessage(func(messageType MessageType, data interface{})) ClientSocket
	OnClose(func()) ClientSocket
	OnPing(func()) ClientSocket
	OnPong(func()) ClientSocket
	OnFlush(func()) ClientSocket
	OnDrain(func()) ClientSocket
	OnError(func()) ClientSocket
	OnUpgrade(func()) ClientSocket
	OnUpgradeError(func()) ClientSocket
}

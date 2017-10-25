package engine_io

type clientOptions struct {
	addr       string
	path       string
	binaryType string
}

type MESSAGE_TYPE uint8

const (
	STRING_MSG MESSAGE_TYPE = 0x01
	BINARY_MSG MESSAGE_TYPE = 0x01 << 1
)

type WSClient interface {
	Connect(options *clientOptions)
}

type ClientSocket interface {
	OnOpen(func()) ClientSocket
	OnMessage(func(messageType MESSAGE_TYPE, data interface{})) ClientSocket
	OnClose(func()) ClientSocket
	OnPing(func()) ClientSocket
	OnPong(func()) ClientSocket
	OnFlush(func()) ClientSocket
	OnDrain(func()) ClientSocket
	OnError(func()) ClientSocket
	OnUpgrade(func()) ClientSocket
	OnUpgradeError(func()) ClientSocket
}

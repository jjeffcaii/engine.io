package eio

import (
	"log"
	"net/url"

	"github.com/gorilla/websocket"
	"golang.org/x/tools/go/gcimporter15/testdata"
	"unicode"
)

type ClientOptions struct {
	upgrade           bool
	forceJSONP        bool
	jsonp             bool
	forceBase64       bool
	timestampRequests bool
	timestampParam    string
	path              string
	transports        []string
	transportOptions  map[string]interface{}
	rememberUpgrade   bool
}

//MessageType
type MessageType uint8

const (
	//message type is string
	StringMsg MessageType = 0x01
	//message type is binary
	BinbaryMsg MessageType = 0x01 << 1
)

type WebSocketClient interface {
	Connect(addr string, options *ClientOptions) (*ClientSocket, error)
	createClientSocket(conn *websocket.Conn) *ClientSocket
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

	Send(interface{}) error
	Close()
}

type webSocketClientImpl struct {
	options      *ClientOptions
	clientSocket *ClientSocket
}

type clientSocketImpl struct {
	wsConnection *websocket.Conn
	openHandler func()
	messageHandler func(messageType MessageType, data interface{})
	closeHandler func()
	pingHandler func()
	pongHandler func()
	flushHandler func()
	drainHandler func()
	errorHandler func()
	upgradeHandler func()
	upgradeErrorHandler func()
}

func (p *clientSocketImpl) Send(interface{}) error {
	panic("implement me")
}

func (p *clientSocketImpl) Close() {
	err := p.wsConnection.Close()
	if err != nil {
		log.Error("close connection err %s", err)
	}
}

func (p *clientSocketImpl) OnOpen(handler func()) ClientSocket {
	p.openHandler = handler
	return p
}

func (p *clientSocketImpl) OnMessage(handler func(messageType MessageType, data interface{})) ClientSocket {
	p.messageHandler = handler
	return p
}

func (p *clientSocketImpl) OnClose(handler func()) ClientSocket {
	p.closeHandler = handler
	return p
}

func (p *clientSocketImpl) OnPing(handler func()) ClientSocket {
	p.pingHandler = handler
	return p
}


func (p *clientSocketImpl) OnPong(handler func()) ClientSocket {
	p.pongHandler = handler
	return p
}

func (p *clientSocketImpl) OnFlush(handler func()) ClientSocket {
	p.flushHandler = handler
	return p
}

func (p *clientSocketImpl) OnDrain(handler func()) ClientSocket {
	p.drainHandler = handler
	return p
}

func (p *clientSocketImpl) OnError(handler func()) ClientSocket {
	p.errorHandler = handler
	return p
}

func (p *clientSocketImpl) OnUpgrade(handler func()) ClientSocket {
	p.upgradeHandler = handler
	return p
}

func (p *clientSocketImpl) OnUpgradeError(handler func()) ClientSocket {
	p.upgradeErrorHandler = handler
	return p
}

func (p *webSocketClientImpl) Connect(addr string, options *ClientOptions) (*ClientSocket, error) {

	var t string
	for _, v = range options.transports {
		if v == "websocket" {
			t = v
			break
		}
	}
	if len(t) == 0 {
		return nil, Errors.New("we only suport websocket for now.")
	}

	u := url.URL{Scheme: "ws", Host: addr, Path: options.path, RawQuery: "transport=" + t + "&EIO=3"}

	log.Printf("connection to %s\n", u.String())
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return nil, err
	}
	clientSocket := createClientSocket(conn)

	return clientSocket, nil
}

func (p *webSocketClientImpl) createClientSocket(conn *websocket.Conn) ClientSocket {
	cs := clientSocketImpl{
		wsConnection.conn,
	}
return &cs
}

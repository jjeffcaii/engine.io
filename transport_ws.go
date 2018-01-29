package eio

import (
	"errors"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/jjeffcaii/engine.io/parser"
)

var (
	libWebsocket          *websocket.Upgrader
	errUpgradeWsTransport error
)

func init() {
	libWebsocket = &websocket.Upgrader{
		CheckOrigin:       func(r *http.Request) bool { return true },
		ReadBufferSize:    1024,
		WriteBufferSize:   1024,
		EnableCompression: true,
	}
	errUpgradeWsTransport = errors.New("transport: cannot upgrade websocket transport")
}

type wsTransport struct {
	tinyTransport
	req     *http.Request
	connect *websocket.Conn
	outbox  *queue
}

func (p *wsTransport) GetRequest() *http.Request {
	return p.req
}

func (p *wsTransport) GetType() TransportType {
	return WEBSOCKET
}

func (p *wsTransport) GetEngine() Engine {
	return p.eng
}

func (p *wsTransport) GetSocket() Socket {
	return p.socket
}

func (p *wsTransport) ensureWebsocket(writer http.ResponseWriter, request *http.Request) error {
	if p.connect != nil {
		return nil
	}
	// upgrade to websocket.
	conn, err := libWebsocket.Upgrade(writer, request, nil)
	if err != nil {
		if p.eng.logErr != nil {
			p.eng.logErr("websocket upgrade failed: %s\n", err)
		}
		return err
	}
	p.connect = conn
	p.req = request
	p.onWrite(func() { p.flush() }, false)
	return nil
}

func (p *wsTransport) ready(writer http.ResponseWriter, request *http.Request) error {
	if err := p.ensureWebsocket(writer, request); err != nil {
		return err
	}
	msgOpen := parser.NewPacketByJSON(parser.OPEN, &messageOK{
		Sid:          p.socket.id,
		Upgrades:     emptyStringArray,
		PingInterval: int64(1000 * p.eng.options.pingInterval.Seconds()),
		PingTimeout:  int64(1000 * p.eng.options.pingTimeout.Seconds()),
	})
	return p.write(msgOpen)
}

func (p *wsTransport) doAccept(msg []byte, opt parser.PacketOption) {
	pack, err := parser.Decode(msg, opt)
	if err != nil {
		if p.eng.logErr != nil {
			p.eng.logErr("decode packet failed: %s\n", err)
		}
		panic(err)
	}

	err = p.socket.accept(pack)
	if err != nil {
		panic(err)
	}
}

func (p *wsTransport) doReq(writer http.ResponseWriter, request *http.Request) {
	defer func() {
		p.req = nil
		p.GetSocket().Close()
		e := recover()
		if e == nil {
			return
		}
		if _, ok := e.(*websocket.CloseError); ok {
			return
		}
		if p.eng.logErr != nil {
			p.eng.logErr("do request failed: %s\n", e)
		}
	}()

	if err := p.ensureWebsocket(writer, request); err != nil {
		panic(err)
	}

	// read messages
	for {
		t, message, err := p.connect.ReadMessage()
		if err != nil {
			panic(err)
		}
		switch t {
		default:
			break
		case websocket.TextMessage:
			p.doAccept(message, 0)
			break
		case websocket.BinaryMessage:
			p.doAccept(message, parser.BINARY)
			break
		}
	}
}

func (p *wsTransport) upgradeStart(dest Transport) error {
	return errUpgradeWsTransport
}
func (p *wsTransport) upgradeEnd(dest Transport) error {
	return errUpgradeWsTransport
}

func (p *wsTransport) write(packet *parser.Packet) error {
	p.outbox.append(packet)
	if p.handlerWrite != nil {
		p.handlerWrite()
	}
	return nil
}

func (p *wsTransport) flush() error {
	for {
		item, ok := p.outbox.pop()
		if !ok {
			break
		}
		out := item.(*parser.Packet)
		var msgType int
		if out.Option&parser.BINARY == parser.BINARY {
			msgType = websocket.BinaryMessage
		} else {
			msgType = websocket.TextMessage
		}
		bs, err := parser.Encode(out)
		if err != nil {
			return err
		}
		p.locker.Lock()
		err = p.connect.WriteMessage(msgType, bs)
		p.locker.Unlock()
		if err != nil {
			return err
		}
		if out.Type == parser.PONG && out.Data != nil && len(out.Data) == 5 && string(out.Data[:5]) == "probe" {
			p.socket.getTransportOld().upgradeStart(p)
		}
	}
	if p.handlerFlush != nil {
		p.handlerFlush()
	}
	return nil
}

func (p *wsTransport) close() error {
	if p.connect == nil {
		return nil
	}
	return p.connect.Close()
}

func newWebsocketTransport(eng *engineImpl) Transport {
	return &wsTransport{
		tinyTransport: tinyTransport{
			eng:    eng,
			locker: new(sync.RWMutex),
		},
		outbox: newQueue(),
	}
}

package eio

import (
	"errors"
	"net/http"
	"sync"

	"github.com/golang/glog"
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
	connect *websocket.Conn
	outbox  *queue
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
		glog.Errorln("websocket upgrade failed:", err)
		return err
	}
	p.connect = conn
	p.onWrite(func() { p.flush() })
	return nil
}

func (p *wsTransport) init(writer http.ResponseWriter, request *http.Request) error {
	if err := p.ensureWebsocket(writer, request); err != nil {
		return err
	}
	msgOpen := parser.NewPacketByJSON(parser.OPEN, &messageOK{
		Sid:          p.socket.id,
		Upgrades:     emptyStringArray,
		PingInterval: p.eng.options.pingInterval,
		PingTimeout:  p.eng.options.pingTimeout,
	})
	return p.write(msgOpen)
}

func (p *wsTransport) accept(msg []byte, opt parser.PacketOption) {
	pack, err := parser.Decode(msg, opt)
	if err != nil {
		glog.Errorln("decode packet failed:", err)
		panic(err)
	}

	err = p.socket.accept(pack)
	if err != nil {
		panic(err)
	}
}

func (p *wsTransport) receive(writer http.ResponseWriter, request *http.Request) {
	defer func() {
		p.GetSocket().Close()
		e := recover()
		if e == nil {
			return
		}
		if _, ok := e.(*websocket.CloseError); ok {
			return
		}
		glog.Errorln("transport:", e)
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
			p.accept(message, 0)
			break
		case websocket.BinaryMessage:
			p.accept(message, parser.BINARY)
			break
		}
	}
}

func (p *wsTransport) upgradeStart() error {
	return errUpgradeWsTransport
}
func (p *wsTransport) upgradeEnd(tActive Transport) error {
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
			p.socket.getTransportBackup().upgradeStart()
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
	t := &wsTransport{
		tinyTransport: tinyTransport{
			eng:    eng,
			locker: new(sync.RWMutex),
		},
		outbox: newQueue(),
	}
	return t
}

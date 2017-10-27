package engine_io

import (
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/gorilla/websocket"
	"github.com/jjeffcaii/engine.io/parser"
)

var (
	libWebsocket          *websocket.Upgrader
	ErrUpgradeWsTransport error
)

func init() {
	libWebsocket = &websocket.Upgrader{
		CheckOrigin:       func(r *http.Request) bool { return true },
		ReadBufferSize:    1024,
		WriteBufferSize:   1024,
		EnableCompression: true,
	}
	ErrUpgradeWsTransport = errors.New("transport: cannot upgrade websocket transport")
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

func (p *wsTransport) ready(writer http.ResponseWriter, request *http.Request) error {
	// upgrade to websocket.
	conn, err := libWebsocket.Upgrade(writer, request, nil)
	if err != nil {
		glog.Errorln("websocket upgrade failed:", err)
		return err
	}
	p.connect = conn
	p.onWrite(func() { p.flush() })

	msgOpen := parser.NewPacketByJSON(parser.OPEN, &messageOK{
		Sid:          p.socket.id,
		Upgrades:     emptyStringArray,
		PingInterval: p.eng.options.pingInterval,
		PingTimeout:  p.eng.options.pingTimeout,
	})
	err = p.write(msgOpen)
	return err
}

func (p *wsTransport) doAccept(msg []byte, opt parser.PacketOption) {
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

func (p *wsTransport) doReq(writer http.ResponseWriter, request *http.Request) {
	defer func() {
		p.GetSocket().Close()
		if e := recover(); e != nil {
			if _, ok := e.(*websocket.CloseError); !ok {
				glog.Errorln(e)
			}
		}
	}()
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

func (p *wsTransport) doUpgrade() error {
	return ErrUpgradeWsTransport
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
		if out.Type == parser.PONG && string(out.Data[:5]) == "probe" {
			// ensure upgrade
			time.AfterFunc(100*time.Millisecond, func() {
				// TODO: make upgrade
				// p.socket.getFirstTransport().upgrade()
			})
		}
	}
	if p.handlerFlush != nil {
		p.handlerFlush()
	}
	return nil
}

func (p *wsTransport) close() error {
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

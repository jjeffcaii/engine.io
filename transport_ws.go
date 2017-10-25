package engine_io

import (
	"errors"
	"fmt"
	"net/http"
	"sync"

	"time"

	"github.com/golang/glog"
	"github.com/gorilla/websocket"
	"github.com/jjeffcaii/engine.io/parser"
)

var upgrader *websocket.Upgrader

func init() {
	upgrader = &websocket.Upgrader{
		CheckOrigin:       func(r *http.Request) bool { return true },
		ReadBufferSize:    1024,
		WriteBufferSize:   1024,
		EnableCompression: true,
	}
}

type wsTransport struct {
	eng     *engineImpl
	socket  *socketImpl
	connect *websocket.Conn
	outbox  *queue
	locker  *sync.Mutex
	onWrite func()
	onFlush func()
}

func (p *wsTransport) close() error {
	return p.connect.Close()
}

func (p *wsTransport) getEngine() *engineImpl {
	return p.eng
}

func (p *wsTransport) write(packet *parser.Packet) error {
	defer func() {
		if p.onWrite != nil {
			go p.onWrite()
		}
	}()
	p.outbox.append(packet)
	return nil
}

func (p *wsTransport) flush() error {
	defer func() {
		if p.onFlush != nil {
			go p.onFlush()
		}
	}()

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
				p.socket.getFirstTransport().upgrade()
			})
		}
	}
	return nil
}

func (p *wsTransport) isUpgradable() bool {
	return false
}

func (p *wsTransport) upgrading() error {
	return nil
}

func (p *wsTransport) upgrade() error {
	return nil
}

func (p *wsTransport) transport(ctx *context) error {
	// ensure socket
	isNew := len(ctx.sid) < 1
	var socket *socketImpl
	if isNew {
		ctx.sid = p.eng.generateId()
		socket = newSocket(ctx.sid, p)
	} else if it, ok := p.eng.getSocket(ctx.sid); ok {
		old := it.getFirstTransport()
		old.upgrading()
		it.setTransport(p)
		socket = it
	} else {
		return errors.New(fmt.Sprintf("no such socket#%s", ctx.sid))
	}
	p.socket = socket

	p.socket.OnUpgrade(func() {
		p.socket.clearTransports()
	})

	defer socket.Close()

	// upgrade to websocket.
	if conn, err := upgrader.Upgrade(ctx.res, ctx.req, nil); err != nil {
		glog.Errorln("websocket upgrade failed:", err)
		return err
	} else {
		p.connect = conn
	}

	// flush after write
	p.onWrite = func() {
		p.flush()
	}

	// send connect ok.
	if isNew {
		msgOpen := parser.NewPacketByJSON(parser.OPEN, &messageOK{
			Sid:          ctx.sid,
			Upgrades:     make([]Transport, 0),
			PingInterval: p.eng.options.pingInterval,
			PingTimeout:  p.eng.options.pingTimeout,
		})
		if err := p.write(msgOpen); err != nil {
			return err
		}
		// add socket
		p.eng.putSocket(socket)
		for _, cb := range p.eng.onSockets {
			go cb(socket)
		}
	}

	// begin reading
	return p.foreverRead(socket)
}

func (p *wsTransport) foreverRead(socket *socketImpl) error {
	// read messages
	for {
		t, message, err := p.connect.ReadMessage()
		if err != nil {
			return err
		}
		switch t {
		default:
			break
		case websocket.TextMessage:
			if pack, err := parser.Decode(message, 0); err != nil {
				glog.Errorln("decode packet failed:", err)
				return err
			} else if err := socket.accept(pack); err != nil {
				return err
			}
			break
		case websocket.BinaryMessage:
			if pack, err := parser.Decode(message, parser.BINARY); err != nil {
				glog.Errorln("decode packet failed:", err)
				return err
			} else if err := socket.accept(pack); err != nil {
				return err
			}
			break
		}
	}
	return nil
}

func newWebsocketTransport(eng *engineImpl) transport {
	t := wsTransport{
		eng:    eng,
		outbox: newQueue(),
		locker: new(sync.Mutex),
	}
	return &t
}

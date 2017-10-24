package engine_io

import (
	"net/http"

	"errors"

	"sync"

	"github.com/golang/glog"
	"github.com/gorilla/websocket"
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
	connect *websocket.Conn
	outbox  *queue
	locker  *sync.Mutex
	onWrite func()
	onFlush func()
}

func (p *wsTransport) upgrade() (transport, error) {
	return nil, errors.New("cannot upgrade websocket transport")
}

func (p *wsTransport) close() error {
	return p.connect.Close()
}

func (p *wsTransport) getEngine() *engineImpl {
	return p.eng
}

func (p *wsTransport) write(packet *Packet) error {
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
		out := item.(*Packet)
		var codec packetCodec
		msgType := websocket.TextMessage
		if out.option&BINARY == BINARY {
			msgType = websocket.BinaryMessage
			codec = binaryEncoder
		} else {
			codec = stringEncoder
		}
		bs, err := codec.encode(out)
		if err != nil {
			return err
		}
		p.locker.Lock()
		err = p.connect.WriteMessage(msgType, bs)
		p.locker.Unlock()
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *wsTransport) transport(ctx *context) error {
	// upgrade to websocket.
	if conn, err := upgrader.Upgrade(ctx.res, ctx.req, nil); err != nil {
		glog.Errorln("websocket upgrade failed:", err)
		return err
	} else {
		p.connect = conn
	}

	// create socket
	if len(ctx.sid) < 1 {
		ctx.sid = p.eng.generateId()
	}
	socket := newSocket(ctx.sid, p)

	// flush after write
	p.onWrite = func() {
		p.flush()
	}

	// send connect ok.
	if err := p.write(newPacketByJSON(typeOpen, &messageOK{
		Sid:          ctx.sid,
		Upgrades:     make([]Transport, 0),
		PingInterval: p.eng.options.pingInterval,
		PingTimeout:  p.eng.options.pingTimeout,
	})); err != nil {
		return err
	}

	// add socket
	p.eng.putSocket(socket)
	defer socket.Close()

	for _, cb := range p.eng.onSockets {
		go cb(socket)
	}

	// begin read
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
			if pack, err := stringEncoder.decode(message); err != nil {
				glog.Errorln("decode packet failed:", err)
				return err
			} else {
				socket.accept(pack)
			}
			break
		case websocket.BinaryMessage:
			if pack, err := binaryEncoder.decode(message); err != nil {
				glog.Errorln("decode packet failed:", err)
				return err
			} else {
				socket.accept(pack)
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

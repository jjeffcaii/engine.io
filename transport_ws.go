package engine_io

import (
	"net/http"

	"github.com/golang/glog"
	"github.com/gorilla/websocket"
)

type wsTransport struct {
	server   *engineImpl
	upgrader *websocket.Upgrader
}

func (p *wsTransport) newUpgradeSuccess(socket Socket) *Packet {
	us := initMsg{
		Sid:          socket.Id(),
		Upgrades:     make([]string, 0),
		PingInterval: p.server.options.pingInterval,
		PingTimeout:  p.server.options.pingTimeout,
	}
	return newPacketByJSON(typeOpen, &us)
}

func (p *wsTransport) transport(ctx *context) error {
	conn, err := p.upgrader.Upgrade(ctx.res, ctx.req, nil)
	if err != nil {
		glog.Errorln("websocket upgrade failed:", err)
		return err
	}

	if len(ctx.sid) < 1 {
		ctx.sid = p.server.generateId()
	}
	socket := newSocket(ctx, p.server, 128, 128)

	socket.OnClose(func(reason string) {
		conn.Close()
	})

	mailman := func(packet *Packet) error {
		bs, err := stringEncoder.encode(packet)
		if err != nil {
			return err
		}
		return conn.WriteMessage(websocket.TextMessage, bs)
	}

	if err := mailman(p.newUpgradeSuccess(socket)); err != nil {
		return err
	}

	// consume outbox packets.
	go func() {
		for packet := range socket.outbox {
			mailman(packet)
		}
	}()

	socket.fire()

	defer socket.Close()

	// add socket
	p.server.putSocket(socket)

	for _, cb := range p.server.onSockets {
		go cb(socket)
	}

	// listen messages
	for {
		t, message, err := conn.ReadMessage()
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
				socket.inbox <- pack
			}
			break
		case websocket.BinaryMessage:
			if pack, err := binaryEncoder.decode(message); err != nil {
				glog.Errorln("decode packet failed:", err)
				return err
			} else {
				socket.inbox <- pack
			}
			break
		}
	}
	return nil
}

func newWebsocketTransport(server *engineImpl) *wsTransport {
	upgrader := websocket.Upgrader{
		CheckOrigin:       func(r *http.Request) bool { return true },
		ReadBufferSize:    1024,
		WriteBufferSize:   1024,
		EnableCompression: true,
	}
	trans := wsTransport{
		server:   server,
		upgrader: &upgrader,
	}
	return &trans
}

package engine_io

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"sync"

	"io"
)

type xhrTransport struct {
	eng         *engineImpl
	sk          *socketImpl
	pollingTime time.Duration
	ctx         *context
	outbox      chan *Packet
	onFlush     func()
	onWrite     func()
	locker      *sync.RWMutex
}

func (p *xhrTransport) setSocket(socket *socketImpl) {
	p.locker.Lock()
	p.sk = socket
	p.locker.Unlock()
}

func (p *xhrTransport) getSocket() *socketImpl {
	p.locker.RLock()
	defer p.locker.RUnlock()
	socket := p.sk
	if socket == nil {
		panic(errors.New("cached socket is nil"))
	}
	return socket
}

func (p *xhrTransport) write(packet *Packet) error {
	defer func() {
		if p.onWrite != nil {
			p.onWrite()
		}
	}()
	p.outbox <- packet
	return nil
}

func (p *xhrTransport) flush() error {
	defer func() {
		if p.onFlush != nil {
			p.onFlush()
		}
	}()
	ctx := p.ctx
	closeNotifier := ctx.res.(http.CloseNotifier)
	queue := make([]*Packet, 0)
	// 1. check current packets inbox chan buffer.
	end := false
	for {
		select {
		case pk := <-p.outbox:
			queue = append(queue, pk)
			break
		default:
			end = true
			break
		}
		if end {
			break
		}
	}
	// 2. waiting packet inbox chan until timeout if queue is empty.
	var quit bool
	if len(queue) < 1 {
		select {
		case <-closeNotifier.CloseNotify():
			queue = append(queue, newPacket(typeClose, make([]byte, 0), 0))
			quit = true
			break
		case pk := <-p.outbox:
			queue = append(queue, pk)
			break
		case <-time.After(time.Millisecond * time.Duration(p.eng.options.pingTimeout)):
			kill := Packet{
				typo: typeClose,
				data: make([]byte, 0),
			}
			queue = append(queue, &kill)
			quit = true
			break
		}
	}
	if bs, err := payloader.encode(queue...); err != nil {
		return err
	} else if _, err := ctx.res.Write(bs); err != nil {
		return err
	} else if quit {
		return io.ErrUnexpectedEOF
	} else {
		return nil
	}
}

func (p *xhrTransport) upgrade() (transport, error) {
	// TODO: do upgrade
	return nil, errors.New("TODO")
}

func (p *xhrTransport) close() error {
	var err error
	func() {
		defer func() {
			e := recover()
			if ex, ok := e.(error); ok {
				err = ex
			}
		}()
		close(p.outbox)
	}()
	return err
}

func (p *xhrTransport) getEngine() *engineImpl {
	return p.eng
}

func (p *xhrTransport) transport(ctx *context) error {
	defer ctx.req.Body.Close()
	if p.eng.options.cookie {
		ctx.res.Header().Set("Set-Cookie", fmt.Sprintf("io=%s; Path=/; HttpOnly", ctx.sid))
	}
	if _, ok := ctx.req.Header["User-Agent"]; ok {
		ctx.res.Header().Set("Access-Control-Allow-Origin", "*")
	}
	switch ctx.req.Method {
	default:
		return errors.New(fmt.Sprintf("Unsupported Method: %s", ctx.req.Method))
	case http.MethodGet:
		err := p.doGet(ctx)
		if err == io.ErrUnexpectedEOF {
			p.getSocket().Close()
		}
		return err
	case http.MethodPost:
		err := p.doPost(ctx)
		return err
	}
}

func (p *xhrTransport) doPost(ctx *context) error {
	var ex error
	defer func() {
		if ex == nil {
			ctx.res.WriteHeader(http.StatusOK)
			ctx.res.Header().Set("Content-Type", "text/html; charset=UTF-8")
			ctx.res.Write([]byte("ok"))
		} else {
			ctx.res.WriteHeader(http.StatusInternalServerError)
			ctx.res.Header().Set("Content-Type", "application/json; charset=UTF-8")
			errmsg := engineError{Code: 0, Message: ex.Error()}
			bs, _ := json.Marshal(&errmsg)
			ctx.res.Write(bs)
		}
	}()

	if body, err := ioutil.ReadAll(ctx.req.Body); err != nil {
		ex = err
	} else if packets, err := payloader.decode(body); err != nil {
		ex = err
	} else {
		socket := p.getSocket()
		for _, pack := range packets {
			if err := socket.accept(pack); err != nil {
				ex = err
				break
			}
		}
	}
	return ex

}

func (p *xhrTransport) doGet(ctx *context) error {
	p.locker.Lock()
	p.ctx = ctx
	p.locker.Unlock()
	if len(ctx.sid) < 1 {
		if err := p.asNewborn(ctx); err != nil {
			return err
		}
	}
	p.locker.Lock()
	defer func() {
		p.ctx = nil
		p.locker.Unlock()
	}()
	return p.flush()
}

func (p *xhrTransport) asNewborn(ctx *context) error {
	ctx.sid = p.eng.generateId()
	socket := newSocket(ctx.sid, p)
	p.setSocket(socket)
	p.eng.putSocket(socket)
	for _, fn := range p.eng.onSockets {
		go fn(socket)
	}
	okMsg := messageOK{
		Sid:          ctx.sid,
		Upgrades:     make([]Transport, 0),
		PingInterval: p.eng.options.pingInterval,
		PingTimeout:  p.eng.options.pingTimeout,
	}
	if p.eng.options.allowUpgrades {
		okMsg.Upgrades = append(okMsg.Upgrades, WEBSOCKET)
	}
	return p.write(newPacketByJSON(typeOpen, &okMsg))
}

func newXhrTransport(server *engineImpl) transport {
	trans := xhrTransport{
		eng:    server,
		outbox: make(chan *Packet, 1024),
		locker: new(sync.RWMutex),
	}
	return &trans
}

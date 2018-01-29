package eio

import (
	"fmt"
	"sync"
)

type messageOK struct {
	Sid          string   `json:"sid"`
	Upgrades     []string `json:"upgrades"`
	PingInterval int64    `json:"pingInterval"`
	PingTimeout  int64    `json:"pingTimeout"`
}

type tinyTransport struct {
	eng          *engineImpl
	socket       *socketImpl
	locker       *sync.RWMutex
	handlerWrite func()
	handlerFlush func()
}

func (p *tinyTransport) onWrite(fn func(), async bool) {
	if fn == nil {
		return
	}
	fn2 := func() {
		defer func() {
			if e := recover(); e != nil {
				if p.eng.logErr != nil {
					p.eng.logErr("handle write failed: %s\n", e)
				}
			}
		}()
		fn()
	}
	if async {
		p.handlerWrite = func() { go fn2() }
	} else {
		p.handlerWrite = fn2
	}
}

func (p *tinyTransport) onFlush(fn func(), async bool) {
	if fn == nil {
		return
	}
	fn2 := func() {
		defer func() {
			if e := recover(); e != nil {
				if p.eng.logErr != nil {
					p.eng.logErr("handle flush failed: %s\n", e)
				}
			}
		}()
		fn()
	}
	if async {
		p.handlerFlush = func() { go fn2() }
	} else {
		p.handlerFlush = fn2
	}
}

func (p *tinyTransport) setSocket(socket Socket) {
	p.locker.Lock()
	p.socket = socket.(*socketImpl)
	p.locker.Unlock()
}

func (p *tinyTransport) clearSocket() {
	p.socket = nil
}

func newTransport(engine *engineImpl, transport TransportType) Transport {
	switch transport {
	default:
		panic(fmt.Errorf("invalid transport '%d'", transport))
	case WEBSOCKET:
		return newWebsocketTransport(engine)
	case POLLING:
		return newXhrTransport(engine)
	}
}

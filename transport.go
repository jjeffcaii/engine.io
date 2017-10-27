package eio

import (
	"fmt"

	"sync"

	"github.com/golang/glog"
)

type messageOK struct {
	Sid          string   `json:"sid"`
	Upgrades     []string `json:"upgrades"`
	PingInterval uint32   `json:"pingInterval"`
	PingTimeout  uint32   `json:"pingTimeout"`
}

type tinyTransport struct {
	eng          *engineImpl
	socket       *socketImpl
	locker       *sync.RWMutex
	handlerWrite func()
	handlerFlush func()
}

func (p *tinyTransport) onWrite(fn func()) {
	if fn == nil {
		return
	}
	p.handlerWrite = func() {
		go func() {
			defer func() {
				if e := recover(); e != nil {
					glog.Errorln("handle write failed:", e)
				}
			}()
			fn()
		}()
	}
}

func (p *tinyTransport) onFlush(fn func()) {
	if fn == nil {
		return
	}
	p.handlerFlush = func() {
		go func() {
			defer func() {
				if e := recover(); e != nil {
					glog.Errorln("handle flush failed:", e)
				}
			}()
			fn()
		}()
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

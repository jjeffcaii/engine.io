package engine_io

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/golang/glog"
	"github.com/jjeffcaii/engine.io/parser"
)

type socketImpl struct {
	id        string
	heartbeat uint32
	engine    *engineImpl

	msgHanders      []func([]byte)
	upgradeHandlers []func()
	errorHandlers   []func(err error)
	closeHandlers   []func(reason string)

	locker *sync.RWMutex

	// try read B first. if failed, read A.
	transportA Transport
	transportB Transport
}

func (p *socketImpl) Id() string {
	return p.id
}

func (p *socketImpl) Server() Engine {
	return p.engine
}

func (p *socketImpl) OnClose(handler func(string)) Socket {
	if handler == nil {
		return p
	}
	p.closeHandlers = append(p.closeHandlers, func(reason string) {
		go func() {
			defer func() {
				if e := recover(); e != nil {
					glog.Error("handle socket close event failed:", e)
				}
			}()
			handler(reason)
		}()
	})
	return p
}

func (p *socketImpl) OnMessage(handler func([]byte)) Socket {
	if handler == nil {
		return p
	}
	p.msgHanders = append(p.msgHanders, func(data []byte) {
		go func() {
			defer func() {
				if e := recover(); e != nil {
					p.shit(e)
					glog.Errorln("handle socket message event failed:", e)
				}
			}()
			handler(data)
		}()
	})
	return p
}

func (p *socketImpl) OnError(handler func(error)) Socket {
	if handler == nil {
		return p
	}
	p.errorHandlers = append(p.errorHandlers, func(err error) {
		go func() {
			defer func() {
				if e := recover(); e != nil {
					glog.Errorln("handle socket error event failed:", e)
				}
			}()
			handler(err)
		}()
	})
	return p
}

func (p *socketImpl) OnUpgrade(handler func()) Socket {
	if handler == nil {
		return p
	}
	p.upgradeHandlers = append(p.upgradeHandlers, func() {
		go func() {
			defer func() {
				if e := recover(); e != nil {
					glog.Errorln("handle socket upgrade event failed:", e)
				}
			}()
			handler()
		}()
	})
	return p
}

func (p *socketImpl) Send(message interface{}) error {
	return p.SendCustom(message, 0)
}

func (p *socketImpl) SendCustom(message interface{}, options parser.PacketOption) error {
	if atomic.LoadUint32(&(p.heartbeat)) == 0 {
		return errors.New(fmt.Sprintf("socket#%s is closed", p.id))
	}
	packet := parser.NewPacketAuto(parser.MESSAGE, message)
	packet.Option |= options
	return p.getTransport().write(packet)
}

func (p *socketImpl) Close() {
	if atomic.LoadUint32(&(p.heartbeat)) == 0 {
		return
	}
	atomic.StoreUint32(&(p.heartbeat), 0)
	p.locker.Lock()
	defer p.locker.Unlock()

	var reason string
	if p.transportB != nil {
		if err := p.transportB.close(); err != nil {
			reason += err.Error()
		}
	}
	if p.transportA != nil {
		if err := p.transportA.close(); err != nil {
			if len(reason) > 0 {
				reason += ", "
			}
			reason += err.Error()
		}
	}
	for _, fn := range p.closeHandlers {
		fn(reason)
	}
}

func (p *socketImpl) clearTransports() {
	p.locker.Lock()
	defer p.locker.Unlock()

	if p.transportB == nil {
		return
	}
	p.transportA.close()
	p.transportA = nil
}

func (p *socketImpl) setTransport(t Transport) error {
	p.locker.Lock()
	defer p.locker.Unlock()
	if p.transportB != nil {
		return errors.New("transports is full")
	}
	if p.transportA == nil {
		p.transportA = t
	} else {
		p.transportB = t
	}
	return nil
}

func (p *socketImpl) getTransport() Transport {
	p.locker.RLock()
	defer p.locker.RUnlock()
	if p.transportB != nil {
		return p.transportB
	} else if p.transportA != nil {
		return p.transportA
	} else {
		panic(errors.New("transport unavailable"))
	}
}

func (p *socketImpl) getTransportOld() Transport {
	p.locker.RLock()
	defer p.locker.RUnlock()
	if p.transportB == nil || p.transportA == nil {
		panic("old transport unavailable")
	}
	return p.transportA
}

func (p *socketImpl) shit(e interface{}) {
	if e == nil {
		return
	}
	err, ok := e.(error)
	if !ok {
		return
	}
	if p.errorHandlers != nil {
		for _, fn := range p.errorHandlers {
			fn(err)
		}
	}
}

func (p *socketImpl) accept(packet *parser.Packet) error {
	switch packet.Type {
	default:
		return errors.New(fmt.Sprintf("unsupport packet: %d", packet.Type))
	case parser.CLOSE:
		p.Close()
		break
	case parser.UPGRADE:
		for _, fn := range p.upgradeHandlers {
			fn()
		}
		break
	case parser.PING:
		go func() {
			// refresh heartbeat then pong it.
			if atomic.LoadUint32(&(p.heartbeat)) != 0 {
				atomic.StoreUint32(&(p.heartbeat), now32())
			}
			pong := parser.NewPacketCustom(parser.PONG, packet.Data, 0)
			p.getTransport().write(pong)
		}()
		break
	case parser.MESSAGE:
		for _, fn := range p.msgHanders {
			fn(packet.Data)
		}
		break
	}
	return nil
}

func (p *socketImpl) isLost() bool {
	d := 1000 * (now32() - atomic.LoadUint32(&(p.heartbeat)))
	return d > p.engine.options.pingTimeout
}

func newSocket(id string, eng *engineImpl) *socketImpl {
	socket := &socketImpl{
		id:              id,
		engine:          eng,
		heartbeat:       now32(),
		upgradeHandlers: make([]func(), 0),
		msgHanders:      make([]func([]byte), 0),
		errorHandlers:   make([]func(error), 0),
		locker:          new(sync.RWMutex),
	}
	return socket
}

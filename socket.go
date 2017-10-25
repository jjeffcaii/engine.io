package engine_io

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang/glog"
	"github.com/jjeffcaii/engine.io/parser"
)

type socketImpl struct {
	id              string
	heart           uint32
	msgHanders      []func([]byte)
	upgradeHandlers []func()
	errorHandlers   []func(err error)
	closeHandlers   []func(reason string)
	locker          *sync.RWMutex
	transports      []transport
}

func (p *socketImpl) clearTransports() {
	p.locker.Lock()
	if len(p.transports) > 1 {
		var deads []transport
		sp := len(p.transports) - 1
		deads, p.transports = p.transports[0:sp], p.transports[sp:]
		for _, dead := range deads {
			dead.close()
		}
	}
	p.locker.Unlock()
}

func (p *socketImpl) setTransport(t transport) {
	p.locker.Lock()
	p.transports = append(p.transports, t)
	p.locker.Unlock()
}

func (p *socketImpl) getFirstTransport() transport {
	p.locker.RLock()
	defer p.locker.RUnlock()
	return p.transports[0]
}

func (p *socketImpl) getTransport() transport {
	p.locker.RLock()
	defer p.locker.RUnlock()
	return p.transports[len(p.transports)-1]
}

func (p *socketImpl) shit(e interface{}) {
	if e == nil {
		return
	}
	err, ok := e.(error)
	if !ok {
		return
	}
	for _, fn := range p.errorHandlers {
		go func() {
			defer func() {
				ee := recover()
				if ee != nil {
					glog.Errorln("handle error failed:", ee)
				}
			}()
			fn(err)
		}()
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
		if p.upgradeHandlers != nil {
			for _, fn := range p.upgradeHandlers {
				go fn()
			}
		}
		break
	case parser.PING:
		go func() {
			// refresh heartbeat then pong it.
			if atomic.LoadUint32(&(p.heart)) != 0 {
				now := uint32(time.Now().Unix())
				atomic.StoreUint32(&(p.heart), now)
			}
			pong := parser.NewPacketCustom(parser.PONG, packet.Data, 0)
			p.getTransport().write(pong)
		}()
		break
	case parser.MESSAGE:
		for _, fn := range p.msgHanders {
			go func() {
				defer func() {
					e := recover()
					p.shit(e)
				}()
				fn(packet.Data)
			}()
		}
		break
	}
	return nil
}

func (p *socketImpl) Id() string {
	return p.id
}

func (p *socketImpl) Server() Engine {
	return p.getTransport().getEngine()
}

func (p *socketImpl) OnClose(handler func(reason string)) Socket {
	p.closeHandlers = append(p.closeHandlers, handler)
	return p
}

func (p *socketImpl) OnMessage(handler func(data []byte)) Socket {
	p.msgHanders = append(p.msgHanders, handler)
	return p
}

func (p *socketImpl) OnError(handler func(err error)) Socket {
	p.errorHandlers = append(p.errorHandlers, handler)
	return p
}

func (p *socketImpl) OnUpgrade(handler func()) Socket {
	p.upgradeHandlers = append(p.upgradeHandlers, handler)
	return p
}

func (p *socketImpl) Send(message interface{}) error {
	return p.SendCustom(message, 0)
}

func (p *socketImpl) SendCustom(message interface{}, options parser.PacketOption) error {
	if atomic.LoadUint32(&(p.heart)) == 0 {
		return errors.New(fmt.Sprintf("socket#%s is closed", p.id))
	}
	packet := parser.NewPacketAuto(parser.MESSAGE, message)
	packet.Option |= options
	return p.getTransport().write(packet)
}

func (p *socketImpl) Close() {
	if atomic.LoadUint32(&(p.heart)) == 0 {
		return
	}
	atomic.StoreUint32(&(p.heart), 0)
	var reason string
	t := p.getTransport()
	if err := t.close(); err != nil {
		reason = err.Error()
	}
	t.getEngine().removeSocket(p)
	for _, fn := range p.closeHandlers {
		go fn(reason)
	}
}

func (p *socketImpl) isLost() bool {
	now := uint32(time.Now().Unix())
	d := 1000 * (now - atomic.LoadUint32(&(p.heart)))
	return d > p.getTransport().getEngine().options.pingTimeout
}

func newSocket(id string, t transport) *socketImpl {
	now := uint32(time.Now().Unix())
	socket := socketImpl{
		id:              id,
		heart:           now,
		upgradeHandlers: make([]func(), 0),
		msgHanders:      make([]func([]byte), 0),
		errorHandlers:   make([]func(error), 0),
		locker:          new(sync.RWMutex),
		transports:      []transport{t},
	}
	return &socket
}

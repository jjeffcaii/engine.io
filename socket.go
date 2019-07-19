package eio

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/jjeffcaii/engine.io/internal/parser"
)

type socketImpl struct {
	id        string
	heartbeat int64
	engine    *engineImpl

	msgHanders      []func([]byte)
	upgradeHandlers []func()
	errorHandlers   []func(err error)
	closeHandlers   []func(reason string)

	transportBackup, transportPrimary Transport
}

func (p *socketImpl) Transport() Transport {
	if p.transportPrimary != nil {
		return p.transportPrimary
	}
	return p.transportBackup
}

func (p *socketImpl) ID() string {
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
					if p.engine.logErr != nil {
						p.engine.logErr("handle socket close event failed: %s\n", e)
					}
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
		defer func() {
			e := recover()
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
			if p.engine.logErr != nil {
				p.engine.logErr("handle socket message event failed: %s\n", e)
			}
		}()
		handler(data)
	})
	return p
}

func (p *socketImpl) OnError(handler func(error)) Socket {
	if handler == nil {
		return p
	}
	p.errorHandlers = append(p.errorHandlers, func(err error) {
		defer func() {
			if e := recover(); e != nil {
				if p.engine.logErr != nil {
					p.engine.logErr("handle socket error event failed: %s\n", e)
				}
			}
		}()
		handler(err)
	})
	return p
}

func (p *socketImpl) OnUpgrade(handler func()) Socket {
	if handler == nil {
		return p
	}
	p.upgradeHandlers = append(p.upgradeHandlers, func() {
		defer func() {
			if e := recover(); e != nil {
				if p.engine.logErr != nil {
					p.engine.logErr("handle socket upgrade event failed: %s\n", e)
				}
			}
		}()
		handler()
	})
	return p
}

func (p *socketImpl) Send(message interface{}) error {
	if atomic.LoadInt64(&(p.heartbeat)) == 0 {
		return fmt.Errorf("socket#%s is closed", p.id)
	}
	packet := parser.NewPacket(parser.MESSAGE, message)
	if p.transportBackup != nil {
		return p.transportBackup.write(packet)
	}
	return p.transportPrimary.write(packet)
}

func (p *socketImpl) Close() {
	if atomic.LoadInt64(&(p.heartbeat)) == 0 {
		return
	}
	atomic.StoreInt64(&(p.heartbeat), 0)
	var reason string
	if p.transportPrimary != nil {
		if err := p.transportPrimary.close(); err != nil {
			reason += err.Error()
		}
	}
	if p.transportBackup != nil {
		if err := p.transportBackup.close(); err != nil {
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

func (p *socketImpl) setTransport(t Transport) error {
	if p.transportPrimary != nil {
		return errors.New("transports is full")
	}
	if p.transportBackup == nil {
		p.transportBackup = t
	} else {
		p.transportPrimary = t
	}
	return nil
}

func (p *socketImpl) getTransport() Transport {
	if p.transportPrimary == nil && p.transportBackup == nil {
		panic(errors.New("transport unavailable"))
	}
	if p.transportPrimary != nil {
		return p.transportPrimary
	}
	return p.transportBackup
}

func (p *socketImpl) getTransportOld() Transport {
	if p.transportPrimary == nil || p.transportBackup == nil {
		panic("old transport unavailable")
	}
	return p.transportBackup
}

func (p *socketImpl) accept(packet *parser.Packet) (err error) {
	switch packet.Type {

	case parser.CLOSE:
		p.Close()
	case parser.UPGRADE:
		if p.transportPrimary != nil && p.transportBackup != nil {
			old := p.transportBackup
			err = old.upgradeEnd(p.transportPrimary)
			if err != nil {
				return
			}
			p.transportBackup = nil
			err = old.close()
			if err != nil {
				return
			}
		}
		for _, fn := range p.upgradeHandlers {
			fn()
		}
	case parser.PING:
		go func() {
			// refresh heartbeat then pong it.
			if atomic.LoadInt64(&(p.heartbeat)) != 0 {
				atomic.StoreInt64(&(p.heartbeat), time.Now().Unix())
			}
			pong := parser.NewPacketCustom(parser.PONG, packet.Data, 0)
			_ = p.getTransport().write(pong)
		}()
	case parser.MESSAGE:
		for _, fn := range p.msgHanders {
			fn(packet.Data)
		}
	default:
		err = fmt.Errorf("unsupport packet: %d", packet.Type)
	}
	return
}

func (p *socketImpl) isLost() bool {
	d := time.Now().Unix() - atomic.LoadInt64(&(p.heartbeat))
	return d > int64(p.engine.options.pingTimeout.Seconds())
}

func newSocket(id string, eng *engineImpl) *socketImpl {
	socket := &socketImpl{
		id:              id,
		engine:          eng,
		heartbeat:       time.Now().Unix(),
		upgradeHandlers: make([]func(), 0),
		msgHanders:      make([]func([]byte), 0),
		errorHandlers:   make([]func(error), 0),
	}
	return socket
}

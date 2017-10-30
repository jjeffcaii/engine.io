package eio

import (
	"errors"
	"fmt"
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

	transportBackup, transportPrimary Transport
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
				glog.Errorln("handle socket message event failed:", e)
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
		defer func() {
			if e := recover(); e != nil {
				glog.Errorln("handle socket error event failed:", e)
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
	return p.sendCustom(message, 0)
}

func (p *socketImpl) sendCustom(message interface{}, options parser.PacketOption) error {
	if atomic.LoadUint32(&(p.heartbeat)) == 0 {
		return fmt.Errorf("socket#%s is closed", p.id)
	}
	packet := parser.NewPacket(parser.MESSAGE, message)
	packet.Option |= options
	if p.transportBackup != nil {
		return p.transportBackup.write(packet)
	}
	return p.transportPrimary.write(packet)
}

func (p *socketImpl) Close() {
	if atomic.LoadUint32(&(p.heartbeat)) == 0 {
		return
	}
	atomic.StoreUint32(&(p.heartbeat), 0)
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
	if p.transportPrimary != nil {
		return p.transportPrimary
	} else if p.transportBackup != nil {
		return p.transportBackup
	} else {
		panic(errors.New("transport unavailable"))
	}
}

func (p *socketImpl) getTransportOld() Transport {
	if p.transportPrimary == nil || p.transportBackup == nil {
		panic("old transport unavailable")
	}
	return p.transportBackup
}

func (p *socketImpl) accept(packet *parser.Packet) error {
	switch packet.Type {
	default:
		return fmt.Errorf("unsupport packet: %d", packet.Type)
	case parser.CLOSE:
		p.Close()
		break
	case parser.UPGRADE:
		// clean transports
		if p.transportPrimary != nil {
			p.transportBackup.close()
			p.transportBackup = nil
		}
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
	}
	return socket
}

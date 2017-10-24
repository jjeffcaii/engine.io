package engine_io

import (
	"errors"
	"time"

	"fmt"

	"sync/atomic"

	"github.com/golang/glog"
)

type socketImpl struct {
	id         string
	heart      uint32
	onMessages []func([]byte)
	onErrors   []func(err error)
	onCloses   []func(reason string)
	t          transport
}

func (p *socketImpl) shit(e interface{}) {
	if e == nil {
		return
	}
	err, ok := e.(error)
	if !ok {
		return
	}
	for _, fn := range p.onErrors {
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

func (p *socketImpl) accept(packet *Packet) error {
	switch packet.typo {
	default:
		return errors.New(fmt.Sprintf("unsupport packet: %d", packet.typo))
	case typePing:
		go func() {
			// refresh heartbeat then pong it.
			if atomic.LoadUint32(&(p.heart)) != 0 {
				now := uint32(time.Now().Unix())
				atomic.StoreUint32(&(p.heart), now)
			}
			pong := newPacket(typePong, packet.data, 0)
			p.t.write(pong)
		}()
		break
	case typeMessage:
		for _, fn := range p.onMessages {
			go func() {
				defer func() {
					e := recover()
					p.shit(e)
				}()
				fn(packet.data)
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
	return p.t.getEngine()
}

func (p *socketImpl) OnClose(handler func(reason string)) Socket {
	p.onCloses = append(p.onCloses, handler)
	return p
}

func (p *socketImpl) OnMessage(handler func(data []byte)) Socket {
	p.onMessages = append(p.onMessages, handler)
	return p
}

func (p *socketImpl) OnError(handler func(err error)) Socket {
	p.onErrors = append(p.onErrors, handler)
	return p
}

func (p *socketImpl) Send(message interface{}) error {
	return p.SendCustom(message, 0)
}

func (p *socketImpl) SendCustom(message interface{}, options SendOption) error {
	if atomic.LoadUint32(&(p.heart)) == 0 {
		return errors.New(fmt.Sprintf("socket#%s is closed", p.id))
	}
	packet := newPacketAuto(typeMessage, message)
	packet.option |= options
	return p.t.write(packet)
}

func (p *socketImpl) Close() {
	if atomic.LoadUint32(&(p.heart)) == 0 {
		return
	}
	atomic.StoreUint32(&(p.heart), 0)
	var reason string
	if err := p.t.close(); err != nil {
		reason = err.Error()
	}
	p.t.getEngine().removeSocket(p)
	for _, fn := range p.onCloses {
		go fn(reason)
	}
}

func (p *socketImpl) isLost() bool {
	now := uint32(time.Now().Unix())
	d := 1000 * (now - atomic.LoadUint32(&(p.heart)))
	return d > p.t.getEngine().options.pingTimeout
}

func newSocket(id string, transport transport) *socketImpl {
	now := uint32(time.Now().Unix())
	socket := socketImpl{
		id:         id,
		heart:      now,
		onMessages: make([]func([]byte), 0),
		onErrors:   make([]func(error), 0),
		t:          transport,
	}
	return &socket
}

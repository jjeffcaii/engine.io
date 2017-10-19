package engine_io

import (
	"errors"
	"sync"
	"time"

	"fmt"

	"github.com/golang/glog"
)

type socketImpl struct {
	heart      uint32
	ctx        *context
	engine     *engineImpl
	onMessages []func([]byte)
	onErrors   []func(err error)
	onCloses   []func(reason string)
	outbox     chan *Packet
	inbox      chan *Packet
	locker     *sync.Mutex
}

func (p *socketImpl) Id() string {
	return p.ctx.sid
}

func (p *socketImpl) Server() Engine {
	return p.engine
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
	return p.SendCustom(message, nil)
}

func (p *socketImpl) SendCustom(message interface{}, options *MessageOptions) error {
	p.locker.Lock()
	defer p.locker.Unlock()
	if p.heart == 0 {
		return errors.New(fmt.Sprintf("socket#%s is closed", p.ctx.sid))
	}
	var err error = nil
	func() {
		defer func() {
			e := recover()
			if v, ok := e.(error); ok {
				err = v
			}
		}()
		p.outbox <- newPacketAuto(typeMessage, message)
	}()
	return err
}

func (p *socketImpl) Close() {
	p.locker.Lock()
	defer p.locker.Unlock()

	if p.heart == 0 {
		return
	}
	p.heart = 0
	close(p.inbox)
	close(p.outbox)
	p.engine.removeSocket(p)
	for _, fn := range p.onCloses {
		// TODO: add close reason.
		go fn("")
	}
}

func (p *socketImpl) fire() {
	go func() {
		for packet := range p.inbox {
			switch packet.typo {
			case typePing:
				// refresh heartbeat then pong it.
				p.locker.Lock()
				if p.heart != 0 {
					p.heart = uint32(time.Now().Unix())
				}
				p.locker.Unlock()
				pong := newPacket(typePong, packet.data)
				p.outbox <- pong
				break
			case typeMessage:
				for _, fn := range p.onMessages {
					go func() {
						defer func() {
							e := recover()
							if e != nil {
								glog.Errorln("handle message failed:", e)
							}
						}()
						fn(packet.data)
					}()
				}
				break
			default:
				break
			}
		}
	}()
}

func (p *socketImpl) isLost() bool {
	now := uint32(time.Now().Unix())
	d := 1000 * (now - p.heart)
	return d > p.engine.options.pingTimeout
}

func newSocket(ctx *context, engine *engineImpl, isize int, osize int) *socketImpl {
	socket := socketImpl{
		heart:      uint32(time.Now().Unix()),
		ctx:        ctx,
		engine:     engine,
		inbox:      make(chan *Packet, isize),
		outbox:     make(chan *Packet, osize),
		onMessages: make([]func([]byte), 0),
		onErrors:   make([]func(error), 0),
		locker:     new(sync.Mutex),
	}
	return &socket
}

package eio

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/jjeffcaii/engine.io/parser"
)

var (
	errHTTPMethod = errors.New("transport: illegal http method")
	errPollingEOF = errors.New("transport: polling EOF")
)

type xhrTransport struct {
	tinyTransport
	outbox chan *parser.Packet
	req    *http.Request
	res    http.ResponseWriter
}

func (p *xhrTransport) GetType() TransportType {
	return POLLING
}

func (p *xhrTransport) GetEngine() Engine {
	return p.eng
}

func (p *xhrTransport) GetSocket() Socket {
	return p.socket
}

func (p *xhrTransport) ready(writer http.ResponseWriter, request *http.Request) error {
	if request.Method != http.MethodGet {
		return errHTTPMethod
	}
	okMsg := messageOK{
		Sid:          p.socket.id,
		Upgrades:     make([]string, 0),
		PingInterval: p.eng.options.pingInterval,
		PingTimeout:  p.eng.options.pingTimeout,
	}
	for _, it := range p.eng.allowTransports {
		if it == WEBSOCKET {
			okMsg.Upgrades = append(okMsg.Upgrades, "websocket")
			break
		}
	}
	return p.write(parser.NewPacketByJSON(parser.OPEN, &okMsg))
}

func (p *xhrTransport) doReq(writer http.ResponseWriter, request *http.Request) {
	if p.eng.options.cookie {
		writer.Header().Set("Set-Cookie", fmt.Sprintf("io=%s; Path=/; HttpOnly", p.socket.id))
	}
	if _, ok := request.Header["User-Agent"]; ok {
		writer.Header().Set("Access-Control-Allow-Origin", "*")
	}
	switch request.Method {
	default:
		break
	case http.MethodGet:
		p.req = request
		p.res = writer
		defer func() {
			p.req = nil
			p.res = nil
		}()
		if err := p.flush(); err == errPollingEOF {
			bs, _ := parser.EncodePayload(parser.NewPacketCustom(parser.CLOSE, make([]byte, 0), 0))
			p.res.Write(bs)
			p.socket.Close()
			p.socket = nil
		}
		break
	case http.MethodPost:
		var err error
		defer func() {
			if err == nil {
				writer.Header().Set("Content-Type", "text/html; charset=UTF-8")
				writer.Write([]byte("ok"))
			} else {
				sendError(writer, err, http.StatusInternalServerError)
			}
		}()
		// read body
		var body []byte
		body, err = ioutil.ReadAll(request.Body)
		if err != nil {
			return
		}
		// extract packets
		var packets []*parser.Packet
		packets, err = parser.DecodePayload(body)
		if err != nil {
			return
		}
		// notify socket
		for _, pack := range packets {
			err = p.socket.accept(pack)
			if err != nil {
				return
			}
		}
		break
	}
}

func (p *xhrTransport) doUpgrade() error {
	p.write(parser.NewPacketCustom(parser.NOOP, make([]byte, 0), 0))
	return nil
}

func (p *xhrTransport) write(packet *parser.Packet) error {
	var err error
	func() {
		defer func() {
			e := recover()
			if e == nil {
				return
			}
			if e2, ok := e.(error); ok {
				err = e2
			}
		}()
		p.outbox <- packet
	}()
	if p.handlerWrite != nil {
		p.handlerWrite()
	}
	return nil
}

func (p *xhrTransport) flush() error {
	closeNotifier := p.res.(http.CloseNotifier)
	queue := make([]*parser.Packet, 0)
	// 1. check current packets inbox chan buffer.
	end := false
	for {
		select {
		case pk := <-p.outbox:
			if pk == nil {
				return errPollingEOF
			}
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
	if len(queue) < 1 {
		select {
		case <-closeNotifier.CloseNotify():
			glog.Warningln("notify close")
			return errPollingEOF
		case pk := <-p.outbox:
			if pk == nil {
				return errPollingEOF
			}
			queue = append(queue, pk)
			break
		case <-time.After(time.Millisecond * time.Duration(p.eng.options.pingTimeout)):
			return errPollingEOF
			//queue = append(queue, parser.NewPacketCustom(parser.CLOSE, make([]byte, 0), 0))
		}
	}
	bs, err := parser.EncodePayload(queue...)
	if err != nil {
		return err
	}
	_, err = p.res.Write(bs)
	return err
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

func newXhrTransport(server *engineImpl) Transport {
	trans := xhrTransport{
		tinyTransport: tinyTransport{
			eng:    server,
			locker: new(sync.RWMutex),
		},
		outbox: make(chan *parser.Packet, 1024),
	}
	return &trans
}

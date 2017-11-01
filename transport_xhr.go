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

const (
	noopDelay       = 1 * time.Second
	outboxThreshold = 64 // The smaller this value, the more GET will be requested.
)

var (
	errHTTPMethod      = errors.New("transport: illegal http method")
	errPollingEOF      = errors.New("transport: polling EOF")
	defaultPacketClose = parser.NewPacketCustom(parser.CLOSE, nil, 0)
	jsonpEnd           = []byte("\");")
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

func (p *xhrTransport) tryJSONP() (*string, bool) {
	j := p.req.URL.Query().Get("j")
	if len(j) < 1 {
		return nil, false
	}
	return &j, true
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
	if p.eng.options.allowUpgrades {
		for _, it := range p.eng.allowTransports {
			if it == WEBSOCKET {
				okMsg.Upgrades = append(okMsg.Upgrades, "websocket")
				break
			}
		}
	}
	return p.write(parser.NewPacketByJSON(parser.OPEN, &okMsg))
}

func (p *xhrTransport) doReq(writer http.ResponseWriter, request *http.Request) {
	if p.eng.options.cookie {
		var cookie string
		if p.eng.options.cookieHTTPOnly {
			cookie = fmt.Sprintf("io=%s; Path=%s; HttpOnly", p.socket.id, p.eng.options.cookiePath)
		} else {
			cookie = fmt.Sprintf("io=%s; Path=%s;", p.socket.id, p.eng.options.cookiePath)
		}
		writer.Header().Set("Set-Cookie", cookie)
	}
	origin := request.Header.Get("Origin")
	if len(origin) > 0 {
		writer.Header().Set("Access-Control-Allow-Credentials", "true")
		writer.Header().Set("Access-Control-Allow-Origin", origin)
	} else {
		writer.Header().Set("Access-Control-Allow-Origin", "*")
	}
	switch request.Method {
	default:
		break
	case http.MethodGet:
		p.doReqGet(writer, request)
		break
	case http.MethodPost:
		p.doReqPost(writer, request)
		break
	}
}

func (p *xhrTransport) doReqGet(writer http.ResponseWriter, request *http.Request) {
	p.req = request
	p.res = writer
	defer func() {
		p.req = nil
		p.res = nil
	}()
	j, jsonp := p.tryJSONP()
	if jsonp {
		if _, err := p.res.Write([]byte(fmt.Sprintf("___eio[%s](\"", *j))); err != nil {
			glog.Errorln("write jsonp prefix failed:", err)
			return
		}
	}
	var kill bool
	if err := p.flush(); err == errPollingEOF {
		kill = true
		if err := parser.WritePayloadTo(p.res, false, defaultPacketClose); err != nil {
			glog.Errorln("write close packet failed:", err)
			return
		}
	}
	if jsonp {
		if _, err := p.res.Write(jsonpEnd); err != nil {
			glog.Errorln("write jsonp suffix failed:", err)
			return
		}
	}
	if kill {
		p.socket.Close()
		p.socket = nil
	}
}
func (p *xhrTransport) doReqPost(writer http.ResponseWriter, request *http.Request) {
	var err error
	defer func() {
		request.Body.Close()
		if err == nil {
			writer.Header().Set("Content-Type", "text/html; charset=UTF-8")
			writer.Write([]byte("ok"))
		} else {
			sendError(writer, err, http.StatusInternalServerError)
		}
	}()
	// read body
	var body []byte
	switch request.Header.Get("Content-Type") {
	default:
		body, err = ioutil.ReadAll(request.Body)
		if err != nil {
			glog.Errorln("read request body failed:", err)
			return
		}
		break
	case "application/x-www-form-urlencoded":
		if err := request.ParseForm(); err != nil {
			glog.Errorln("parse post form failed:", err)
			return
		}
		body = []byte(request.PostFormValue("d"))
		break
	}
	// extract packets
	var packets []*parser.Packet
	packets, err = parser.DecodePayload(body)
	if err != nil {
		return
	}
	// notify socket
	go func() {
		for _, pack := range packets {
			err = p.socket.accept(pack)
			if err != nil {
				return
			}
		}
	}()
}

func (p *xhrTransport) upgradeStart(dest Transport) error {
	p.write(parser.NewPacketCustom(parser.NOOP, make([]byte, 0), 0))
	return nil
}

func (p *xhrTransport) upgradeEnd(dest Transport) error {
	end := false
	for {
		select {
		case pk := <-p.outbox:
			if pk != nil {
				dest.write(pk)
			}
			break
		default:
			end = true
			break
		}
		if end {
			break
		}
	}
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
			if pk.Type == parser.OPEN {
				end = true
			}
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
	_, jsonp := p.tryJSONP()
	if len(queue) == 1 {
		if queue[0].Type == parser.NOOP {
			time.Sleep(noopDelay)
		}
		return parser.WritePayloadTo(p.res, jsonp, queue[0])
	}
	nooped := false
	for _, v := range queue {
		if v.Type == parser.NOOP && !nooped {
			nooped = true
			p.write(v)
			continue
		}
		if err := parser.WritePayloadTo(p.res, jsonp, v); err != nil {
			return err
		}
	}
	return nil
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
		outbox: make(chan *parser.Packet, outboxThreshold),
	}
	return &trans
}

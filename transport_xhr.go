package eio

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/jjeffcaii/engine.io/internal/parser"
)

const (
	noopDelay       = 1 * time.Second
	outboxThreshold = 128 // The smaller this value, the more GET will be requested.
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
	cond   *sync.Cond
}

func (p *xhrTransport) GetRequest() *http.Request {
	p.cond.L.Lock()
	for p.req == nil {
		p.cond.Wait()
	}
	p.cond.L.Unlock()
	return p.req
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

func (p *xhrTransport) tryJSONP() (callback *string, ok bool) {
	if j := p.GetRequest().URL.Query().Get("j"); len(j) > 0 {
		callback = &j
		ok = true
	}
	return
}

func (p *xhrTransport) ready(writer http.ResponseWriter, request *http.Request) error {
	if request.Method != http.MethodGet {
		return errHTTPMethod
	}
	okMsg := messageOK{
		Sid:          p.socket.id,
		Upgrades:     make([]string, 0),
		PingInterval: int64(1000 * p.eng.options.pingInterval.Seconds()),
		PingTimeout:  int64(1000 * p.eng.options.pingTimeout.Seconds()),
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
	case http.MethodGet:
		p.doReqGet(writer, request)
	case http.MethodPost:
		p.doReqPost(writer, request)
	}
}

func (p *xhrTransport) doReqGet(writer http.ResponseWriter, request *http.Request) {
	p.cond.L.Lock()
	p.req = request
	p.res = writer
	p.cond.Broadcast()
	p.cond.L.Unlock()
	defer func() {
		p.req = nil
		p.res = nil
	}()
	j, jsonp := p.tryJSONP()
	if jsonp {
		if _, err := p.res.Write([]byte(fmt.Sprintf("___eio[%s](\"", *j))); err != nil {
			if p.eng.logErr != nil {
				p.eng.logErr("write jsonp prefix failed: %s\n", err)
			}
			return
		}
	}
	var kill bool
	if err := p.flush(); err == errPollingEOF {
		kill = true
		if err := parser.WritePayloadTo(p.res, false, defaultPacketClose); err != nil {
			if p.eng.logErr != nil {
				p.eng.logErr("write close packet failed: %s\n", err)
			}
			return
		}
	}
	if jsonp {
		if _, err := p.res.Write(jsonpEnd); err != nil {
			if p.eng.logErr != nil {
				p.eng.logErr("write jsonp suffix failed: %s\n", err)
			}
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
			if p.eng.logErr != nil {
				p.eng.logErr("read request body failed: %s\n", err)
			}
			return
		}
		break
	case "application/x-www-form-urlencoded":
		if err := request.ParseForm(); err != nil {
			if p.eng.logErr != nil {
				p.eng.logErr("parse post form failed: %s\n", err)
			}
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

func (p *xhrTransport) write(packet *parser.Packet) (err error) {
	defer func() {
		err = tryRecover(recover())
	}()
	p.outbox <- packet
	if p.handlerWrite != nil {
		p.handlerWrite()
	}
	return
}

func (p *xhrTransport) flush() error {
	closeNotifier := p.res.(http.CloseNotifier)
	queue := make([]*parser.Packet, 0)
	// 1. check current packets inbox chan buffer.
	end := false
	for {
		select {
		case pk, ok := <-p.outbox:
			if !ok {
				return errPollingEOF
			}
			/*if pk == nil {
				return errPollingEOF
			}*/
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
			if p.eng.logWarn != nil {
				p.eng.logWarn("client close connect\n")
			}
			return errPollingEOF
		case pk := <-p.outbox:
			if pk == nil {
				return errPollingEOF
			}
			queue = append(queue, pk)
			break
		case <-time.After(p.eng.options.pingTimeout):
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

func (p *xhrTransport) close() (err error) {
	defer func() {
		err = tryRecover(recover())
	}()
	close(p.outbox)
	return err
}

func newXhrTransport(server *engineImpl) Transport {
	trans := xhrTransport{
		tinyTransport: tinyTransport{
			eng:    server,
			locker: &sync.RWMutex{},
		},
		outbox: make(chan *parser.Packet, outboxThreshold),
		cond:   sync.NewCond(&sync.Mutex{}),
	}
	return &trans
}

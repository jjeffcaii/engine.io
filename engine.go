package engine_io

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/golang/glog"
)

var (
	transportWebsocket = "websocket"
	transportPolling   = "polling"
)

type context struct {
	eio       string
	transport string
	sid       string
	binary    bool
	t         string
	req       *http.Request
	res       http.ResponseWriter
}

type upgradeSuccess struct {
	Sid          string   `json:"sid"`
	Upgrades     []string `json:"upgrades"`
	PingInterval uint32   `json:"pingInterval"`
	PingTimeout  uint32   `json:"pingTimeout"`
}

type engineOptions struct {
	cookie       bool
	pingInterval uint32
	pingTimeout  uint32
}

type engineError struct {
	Code    int8   `json:"code"`
	Message string `json:"message"`
}

type transport interface {
	transport(ctx *context) error
}

func (ctx *context) validate() bool {
	if len(ctx.transport) < 1 {
		return false
	}
	if len(ctx.eio) < 1 {
		return false
	}
	return true
}

type engineImpl struct {
	path      string
	options   *engineOptions
	onSockets []func(Socket)
	sockets   map[string]*socketImpl
	locker    *sync.Mutex

	transports map[string]transport
}

func (p *engineImpl) Listen(addr string) error {
	http.HandleFunc(p.path, func(writer http.ResponseWriter, request *http.Request) {
		query := request.URL.Query()
		handshake := context{
			sid:       query.Get("sid"),
			eio:       query.Get("EIO"),
			transport: query.Get("transport"),
			binary:    query.Get("b64") == "1",
			t:         query.Get("t"),
			req:       request,
			res:       writer,
		}

		if !handshake.validate() {
			writer.WriteHeader(http.StatusBadRequest)
			writer.Header().Set("Content-Type", "application/json")
			e := engineError{Code: 0, Message: "Transport unknown"}
			bs, _ := json.Marshal(&e)
			writer.Write(bs)
			return
		}
		trans, ok := p.transports[handshake.transport]
		if !ok {
			writer.WriteHeader(http.StatusBadRequest)
			writer.Header().Set("Content-Type", "application/json")
			e := engineError{Code: 0, Message: "Transport unknown"}
			bs, _ := json.Marshal(&e)
			writer.Write(bs)
			return
		}
		trans.transport(&handshake)
	})

	tick := time.NewTicker(time.Millisecond * time.Duration(p.options.pingInterval))
	quit := make(chan uint8)

	defer close(quit)

	go func() {
		for {
			select {
			case <-tick.C:
				losts := make([]*socketImpl, 0)
				p.locker.Lock()
				for _, v := range p.sockets {
					if v.isLost() {
						glog.Warningln("********* find a lost socket:", v.Id())
						losts = append(losts, v)
					}
				}
				p.locker.Unlock()
				for _, it := range losts {
					it.Close()
				}
				break
			case <-quit:
				tick.Stop()
				return
			}
		}
	}()

	return http.ListenAndServe(addr, nil)
}

func (p *engineImpl) GetClients() map[string]Socket {
	snapshot := make(map[string]Socket)
	p.locker.Lock()
	defer p.locker.Unlock()
	for k, v := range p.sockets {
		snapshot[k] = v
	}
	return snapshot
}

func (p *engineImpl) OnConnect(onConn func(socket Socket)) Engine {
	p.onSockets = append(p.onSockets, onConn)
	return p
}

func (p *engineImpl) removeSocket(socket *socketImpl) {
	p.locker.Lock()
	defer p.locker.Unlock()
	delete(p.sockets, socket.Id())
}

func (p *engineImpl) putSocket(socket *socketImpl) {
	p.locker.Lock()
	defer p.locker.Unlock()
	sid := socket.Id()
	if _, ok := p.sockets[sid]; ok {
		panic(errors.New(fmt.Sprintf("socket#%s exists already", sid)))
	} else {
		p.sockets[sid] = socket
	}
}

func (p *engineImpl) getSocket(id string) *socketImpl {
	p.locker.Lock()
	defer p.locker.Unlock()
	return p.sockets[id]
}

func (p *engineImpl) hasSocket(id string) bool {
	p.locker.Lock()
	defer p.locker.Unlock()
	_, ok := p.sockets[id]
	keys := make([]string, 0)
	for k := range p.sockets {
		keys = append(keys, k)
	}
	return ok
}

type engineBuilder struct {
	cookie       bool
	pingInterval uint32
	pingTimeout  uint32
	path         string
}

func (p *engineBuilder) SetPath(path string) *engineBuilder {
	p.path = path
	return p
}

func (p *engineBuilder) SetCookie(enable bool) *engineBuilder {
	p.cookie = enable
	return p
}

func (p *engineBuilder) SetPingInterval(interval uint32) *engineBuilder {
	p.pingInterval = interval
	return p
}

func (p *engineBuilder) SetPingTimeout(timeout uint32) *engineBuilder {
	p.pingTimeout = timeout
	return p
}

func (p *engineBuilder) Build() Engine {
	opt := engineOptions{
		cookie:       p.cookie,
		pingInterval: p.pingInterval,
		pingTimeout:  p.pingTimeout,
	}
	eng := &engineImpl{
		onSockets:  make([]func(Socket), 0),
		options:    &opt,
		sockets:    make(map[string]*socketImpl, 0),
		locker:     new(sync.Mutex),
		transports: make(map[string]transport, 0),
		path:       p.path,
	}
	eng.transports[transportWebsocket] = newWebsocketTransport(eng)
	eng.transports[transportPolling] = newXhrTransport(eng)
	return eng
}

func NewEngineBuilder() *engineBuilder {
	builder := engineBuilder{
		path:         "/engine.io/",
		cookie:       false,
		pingInterval: 25000,
		pingTimeout:  60000,
	}
	return &builder
}

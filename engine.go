package engine_io

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang/glog"
)

var (
	protocolVersion = struct {
		n uint8
		s string
	}{3, "3",}
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

type initMsg struct {
	Sid          string   `json:"sid"`
	Upgrades     []string `json:"upgrades"`
	PingInterval uint32   `json:"pingInterval"`
	PingTimeout  uint32   `json:"pingTimeout"`
}

type engineOptions struct {
	allowUpgrades bool
	cookie        bool
	pingInterval  uint32
	pingTimeout   uint32
}

type engineError struct {
	Code    int8   `json:"code"`
	Message string `json:"message"`
}

type transport interface {
	transport(ctx *context) error
}

type engineImpl struct {
	sequence   uint32
	path       string
	options    *engineOptions
	onSockets  []func(Socket)
	sockets    map[string]*socketImpl
	locker     *sync.RWMutex
	transports map[string]transport
	sidGen     func(seq uint32) string
}

func (p *engineImpl) generateId() string {
	return p.sidGen(atomic.AddUint32(&(p.sequence), 1))
}

func (p *engineImpl) Listen(addr string) error {
	http.HandleFunc(p.path, func(writer http.ResponseWriter, request *http.Request) {
		query := request.URL.Query()
		ctx := context{
			sid:       query.Get("sid"),
			eio:       query.Get("EIO"),
			transport: query.Get("transport"),
			binary:    query.Get("b64") == "1",
			t:         query.Get("t"),
			req:       request,
			res:       writer,
		}

		isValidContext := true
		if len(ctx.transport) < 1 {
			isValidContext = false
		}
		if ctx.eio != protocolVersion.s {
			isValidContext = false
		}

		if !isValidContext {
			writer.WriteHeader(http.StatusBadRequest)
			writer.Header().Set("Content-Type", "application/json")
			e := engineError{Code: 0, Message: "Transport unknown"}
			bs, _ := json.Marshal(&e)
			writer.Write(bs)
			return
		}
		trans, ok := p.transports[ctx.transport]
		if !ok {
			writer.WriteHeader(http.StatusBadRequest)
			writer.Header().Set("Content-Type", "application/json")
			e := engineError{Code: 0, Message: "Transport unknown"}
			bs, _ := json.Marshal(&e)
			writer.Write(bs)
			return
		}
		trans.transport(&ctx)
	})

	tick := time.NewTicker(time.Millisecond * time.Duration(p.options.pingInterval))
	quit := make(chan uint8)

	defer close(quit)

	go func() {
		for {
			select {
			case <-tick.C:
				losts := make([]*socketImpl, 0)
				p.locker.RLock()
				for _, v := range p.sockets {
					if v.isLost() {
						glog.Warningln("********* find a lost socket:", v.Id())
						losts = append(losts, v)
					}
				}
				p.locker.RUnlock()
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

func (p *engineImpl) GetProtocol() uint8 {
	return protocolVersion.n
}

func (p *engineImpl) GetClients() map[string]Socket {
	snapshot := make(map[string]Socket)
	p.locker.RLock()
	for k, v := range p.sockets {
		snapshot[k] = v
	}
	p.locker.RUnlock()
	return snapshot
}

func (p *engineImpl) OnConnect(onConn func(socket Socket)) Engine {
	p.onSockets = append(p.onSockets, onConn)
	return p
}

func (p *engineImpl) removeSocket(socket *socketImpl) {
	p.locker.Lock()
	delete(p.sockets, socket.Id())
	p.locker.Unlock()
}

func (p *engineImpl) putSocket(socket *socketImpl) {
	sid := socket.Id()
	p.locker.Lock()
	defer p.locker.Unlock()
	if _, ok := p.sockets[sid]; ok {
		panic(errors.New(fmt.Sprintf("socket#%s exists already", sid)))
	} else {
		p.sockets[sid] = socket
	}
}

func (p *engineImpl) getSocket(id string) *socketImpl {
	p.locker.RLock()
	socket := p.sockets[id]
	p.locker.RUnlock()
	return socket
}

func (p *engineImpl) hasSocket(id string) bool {
	p.locker.RLock()
	_, ok := p.sockets[id]
	p.locker.RUnlock()
	return ok
}

type engineBuilder struct {
	options *engineOptions
	path    string
	gen     func(uint32) string
}

func (p *engineBuilder) SetGenerateId(gen func(uint32) string) *engineBuilder {
	p.gen = gen
	return p
}

func (p *engineBuilder) SetPath(path string) *engineBuilder {
	p.path = path
	return p
}

func (p *engineBuilder) SetCookie(enable bool) *engineBuilder {
	p.options.cookie = enable
	return p
}

func (p *engineBuilder) SetPingInterval(interval uint32) *engineBuilder {
	p.options.pingInterval = interval
	return p
}

func (p *engineBuilder) SetPingTimeout(timeout uint32) *engineBuilder {
	p.options.pingTimeout = timeout
	return p
}

func (p *engineBuilder) Build() Engine {
	clone := func(origin engineOptions) engineOptions {
		return origin
	}(*p.options)
	eng := &engineImpl{
		onSockets:  make([]func(Socket), 0),
		options:    &clone,
		sockets:    make(map[string]*socketImpl, 0),
		locker:     new(sync.RWMutex),
		transports: make(map[string]transport, 0),
		path:       p.path,
		sidGen:     p.gen,
	}
	eng.transports[transportWebsocket] = newWebsocketTransport(eng)
	eng.transports[transportPolling] = newXhrTransport(eng)
	return eng
}

func defaultIdGen(seed uint32) string {
	bf := new(bytes.Buffer)
	bf.Write(randStr(12))
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, seed)
	for i := 1; i < 4; i++ {
		bf.WriteByte(b[i])
	}
	bs := bf.Bytes()
	s := base64.StdEncoding.EncodeToString(bs)
	s = strings.Replace(s, "/", "_", -1)
	s = strings.Replace(s, "+", "-", -1)
	return s
}

func NewEngineBuilder() *engineBuilder {
	options := engineOptions{
		cookie:        false,
		pingInterval:  25000,
		pingTimeout:   60000,
		allowUpgrades: true,
	}
	builder := engineBuilder{
		path:    "/engine.io/",
		options: &options,
		gen:     defaultIdGen,
	}
	return &builder
}

package engine_io

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"strings"

	"github.com/golang/glog"
	"github.com/orcaman/concurrent-map"
)

var (
	protocolVersion = struct {
		n uint8
		s string
	}{3, "3",}
)

type engineOptions struct {
	allowUpgrades bool
	cookie        bool
	pingInterval  uint32
	pingTimeout   uint32
}

type engineImpl struct {
	allowTransports []TransportType
	sidGen          func(seq uint32) string
	sequence        uint32
	path            string
	options         *engineOptions
	onSockets       []func(Socket)
	sockets         cmap.ConcurrentMap
	junkKiller      chan struct{}
	junkTicker      *time.Ticker
	gets            int32
	posts           int32
}

func (p *engineImpl) Debug() string {
	m := make(map[string]int32)
	m["gets"] = atomic.LoadInt32(&(p.gets))
	m["posts"] = atomic.LoadInt32(&(p.posts))
	bs, _ := json.Marshal(m)
	return string(bs)
}

func (p *engineImpl) checkVersion(eio string) error {
	if eio != protocolVersion.s {
		return errors.New(fmt.Sprintf("illegal protocol version: EIO=%s", eio))
	}
	return nil
}

func (p *engineImpl) checkTransport(qTransport string) (TransportType, error) {
	t, err := parseTransportType(qTransport)
	if err != nil {
		return -1, err
	}
	for _, tt := range p.allowTransports {
		if t == tt {
			return t, nil
		}
	}
	return -1, errors.New(fmt.Sprintf("transport '%s' is forbiden", qTransport))
}

func (p *engineImpl) Router() func(http.ResponseWriter, *http.Request) {
	p.ensureCleaner()
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodOptions {
			writer.WriteHeader(http.StatusOK)
			return
		}
		if !(request.Method == http.MethodGet || request.Method == http.MethodPost) {
			writer.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		query := request.URL.Query()
		if request.Method == http.MethodGet {
			atomic.AddInt32(&(p.gets), 1)
			defer atomic.AddInt32(&(p.gets), -1)
		} else if request.Method == http.MethodPost {
			atomic.AddInt32(&(p.posts), 1)
			defer atomic.AddInt32(&(p.posts), -1)
		}

		var err error

		// check protocol version
		if err = p.checkVersion(query.Get("EIO")); err != nil {
			sendError(writer, err, http.StatusBadRequest)
			return
		}

		// check transport
		var ttype TransportType
		if ttype, err = p.checkTransport(query.Get("transport")); err != nil {
			sendError(writer, errors.New("transprot error"), http.StatusBadRequest, 0)
			return
		}

		var sid = query.Get("sid")
		var isNew = len(sid) < 1

		var socket *socketImpl
		var tp Transport

		if isNew {
			sid = p.generateId()
			tp, socket = newTransport(p, ttype), newSocket(sid, p)
			bind(tp, socket)
			if err = tp.ready(writer, request); err != nil {
				sendError(writer, err)
				return
			}
			socket.OnClose(func(reason string) {
				p.removeSocket(socket)
			})
			p.putSocket(socket)
			p.socketCreated(socket)
		} else if socket0, ok := p.getSocket(sid); !ok {
			sendError(writer, errors.New(fmt.Sprintf("socket#%s doesn't exist", sid)))
			return
		} else {
			socket = socket0
			tp0 := socket0.getTransport()
			ttype0 := tp0.GetType()
			if ttype > ttype0 {
				tp := newTransport(p, ttype)
				bind(tp, socket)
				// TODO: need upgrade
			} else if ttype < ttype0 {
				// TODO: use old transport
				tp = socket0.getTransportOld()
			} else {
				tp = tp0
			}
		}
		tp.doReq(writer, request)
	}
}

func (p *engineImpl) Close() {
	close(p.junkKiller)
}

func (p *engineImpl) Listen(addr string) error {
	http.HandleFunc(p.path, p.Router())
	return http.ListenAndServe(addr, nil)
}

func (p *engineImpl) GetProtocol() uint8 {
	return protocolVersion.n
}

func (p *engineImpl) GetClients() map[string]Socket {
	snapshot := make(map[string]Socket)
	for entry := range p.sockets.IterBuffered() {
		snapshot[entry.Key] = entry.Val.(Socket)
	}
	return snapshot
}

func (p *engineImpl) CountClients() int {
	return p.sockets.Count()
}

func (p *engineImpl) OnConnect(onConn func(socket Socket)) Engine {
	p.onSockets = append(p.onSockets, onConn)
	return p
}

func (p *engineImpl) removeSocket(socket *socketImpl) {
	p.sockets.Remove(socket.id)
}

func (p *engineImpl) generateId() string {
	return p.sidGen(atomic.AddUint32(&(p.sequence), 1))
}

func (p *engineImpl) putSocket(socket *socketImpl) {
	sid := socket.id
	if !p.sockets.SetIfAbsent(sid, socket) {
		panic(errors.New(fmt.Sprintf("socket#%s exists already", sid)))
	}
}

func (p *engineImpl) getSocket(id string) (*socketImpl, bool) {
	if socket, ok := p.sockets.Get(id); ok {
		return socket.(*socketImpl), ok
	} else {
		return nil, ok
	}
}

func (p *engineImpl) hasSocket(id string) bool {
	return p.sockets.Has(id)
}

func (p *engineImpl) ensureCleaner() {
	if p.junkTicker != nil {
		return
	}
	p.junkTicker = time.NewTicker(time.Millisecond * time.Duration(p.options.pingTimeout))
	// cron: check and kill lost socket.
	go func() {
		for {
			select {
			case <-p.junkTicker.C:
				losts := make([]*socketImpl, 0)
				for entry := range p.sockets.IterBuffered() {
					it := entry.Val.(*socketImpl)
					if it.isLost() {
						losts = append(losts, it)
					}
				}
				if len(losts) > 0 {
					lostIds := make([]string, 0)
					for _, it := range losts {
						it.Close()
						lostIds = append(lostIds, it.id)
					}
					glog.Infof("***** kill %d DEAD sockets: %s *****\n", len(losts), strings.Join(lostIds, ","))
				}
				break
			case <-p.junkKiller:
				p.junkTicker.Stop()
				return
			}
		}
	}()
}

func (p *engineImpl) socketCreated(socket *socketImpl) {
	if p.onSockets == nil {
		return
	}
	for _, fn := range p.onSockets {
		go func() {
			defer func() {
				if e := recover(); e != nil {
					glog.Errorln("handle socket connect failed:", e)
				}
			}()
			fn(socket)
		}()
	}
}

type engineBuilder struct {
	allowTransports []TransportType
	options         *engineOptions
	path            string
	gen             func(uint32) string
}

func (p *engineBuilder) SetTransports(transports ...TransportType) *engineBuilder {
	p.allowTransports = transports
	return p
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
		sockets:    cmap.New(),
		path:       p.path,
		sidGen:     p.gen,
		junkKiller: make(chan struct{}),
		junkTicker: nil,
	}
	if len(p.allowTransports) < 1 {
		eng.allowTransports = []TransportType{POLLING, WEBSOCKET}
	} else {
		allows := make([]TransportType, 0)
		copy(allows, p.allowTransports)
		eng.allowTransports = allows
	}

	return eng
}

func parseTransportType(s string) (TransportType, error) {
	switch s {
	default:
		return -1, errors.New("invalid transport " + s)
	case "polling":
		return POLLING, nil
	case "websocket":
		return WEBSOCKET, nil
	}
}

func bind(tp Transport, socket *socketImpl) {
	socket.setTransport(tp)
	tp.setSocket(socket)
}

func NewEngineBuilder() *engineBuilder {
	options := engineOptions{
		cookie:        false,
		pingInterval:  25000,
		pingTimeout:   60000,
		allowUpgrades: true,
	}
	builder := engineBuilder{
		path:    DEFAULT_PATH,
		options: &options,
		gen:     randomSessionId,
	}
	return &builder
}

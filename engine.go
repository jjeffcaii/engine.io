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

type context struct {
	sid    string
	binary bool
	t      string
	req    *http.Request
	res    http.ResponseWriter
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

type engineImpl struct {
	sidGen    func(seq uint32) string
	sequence  uint32
	path      string
	options   *engineOptions
	onSockets []func(Socket)
	sockets   cmap.ConcurrentMap
}

func (p *engineImpl) Listen(addr string) error {
	router := func(writer http.ResponseWriter, request *http.Request) {
		query := request.URL.Query()
		eio := query.Get("EIO")
		if eio != protocolVersion.s {
			panic(errors.New(fmt.Sprintf("illegal protocol version: EIO=%s", eio)))
		}
		var qSid, qTp = query.Get("sid"), query.Get("transport")
		var tp transport
		var err error

		if len(qSid) < 1 {
			tp, err = newTransport(p, qTp)
		} else if socket, ok := p.getSocket(qSid); ok {
			tp = socket.t
		} else {
			err = errors.New(fmt.Sprintf("no such socket#%s", qSid))
		}

		if err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			writer.Header().Set("Content-Type", "application/json")
			e := engineError{Code: 0, Message: "Transport unknown"}
			bs, _ := json.Marshal(&e)
			writer.Write(bs)
			return
		}
		ctx := context{
			sid:    qSid,
			binary: query.Get("b64") == "1",
			req:    request,
			res:    writer,
		}
		tp.transport(&ctx)
	}

	http.HandleFunc(p.path, router)

	// cron: check and kill lost socket.
	tick := time.NewTicker(time.Millisecond * time.Duration(p.options.pingInterval))
	quit := make(chan uint8)
	defer close(quit)
	go func() {
		for {
			select {
			case <-tick.C:
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
		onSockets: make([]func(Socket), 0),
		options:   &clone,
		sockets:   cmap.New(),
		path:      p.path,
		sidGen:    p.gen,
	}
	return eng
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
		gen:     randomSessionId,
	}
	return &builder
}

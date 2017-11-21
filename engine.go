package eio

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"log"
)

var (
	protocolVersion = struct {
		n uint8
		s string
	}{3, "3",}
)

type engineOptions struct {
	allowUpgrades  bool
	cookie         bool
	cookiePath     string
	cookieHTTPOnly bool
	pingInterval   uint32
	pingTimeout    uint32
	handleAsync    bool
}

type engineImpl struct {
	logInfo         *log.Logger
	logWarn         *log.Logger
	logErr          *log.Logger
	allowTransports []TransportType
	sidGen          func(seq uint32) string
	sequence        uint32
	path            string
	options         *engineOptions
	onSockets       []func(Socket)
	sockets         *socketMap
	junkKiller      chan struct{}
	junkTicker      *time.Ticker
	allowRequest    func(*http.Request) error
}

func (p *engineImpl) Router() func(http.ResponseWriter, *http.Request) {
	p.ensureCleaner()
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodOptions {
			writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			writer.WriteHeader(http.StatusOK)
			return
		}
		if !(request.Method == http.MethodGet || request.Method == http.MethodPost) {
			writer.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		query := request.URL.Query()
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

		// check allow request
		if p.allowRequest != nil {
			if err := p.allowRequest(request); err != nil {
				sendError(writer, err, http.StatusNotAcceptable, 0)
				return
			}
		}

		var sid = query.Get("sid")
		var isNew = len(sid) < 1

		var socket *socketImpl
		var tp Transport

		if isNew {
			sid = p.generateID()
			tp, socket = newTransport(p, ttype), newSocket(sid, p)
			socket.setTransport(tp)
			tp.setSocket(socket)
			if err = tp.ready(writer, request); err != nil {
				sendError(writer, err)
				return
			}
			socket.OnClose(func(reason string) {
				p.sockets.Remove(socket)
			})
			p.sockets.Put(socket)
			p.socketCreated(socket)
		} else if socket0, ok := p.sockets.Get(sid); !ok {
			sendError(writer, fmt.Errorf("%s:socket#%s doesn't exist", request.Method, sid))
			return
		} else {
			socket = socket0
			tp0 := socket0.getTransport()
			ttype0 := tp0.GetType()
			if ttype > ttype0 {
				tp = newTransport(p, ttype)
				tp.setSocket(socket)
				socket.setTransport(tp)
			} else if ttype < ttype0 {
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
	m := make(map[string]Socket)
	for _, it := range p.sockets.List(nil) {
		m[it.ID()] = it
	}
	return m
}

func (p *engineImpl) CountClients() int {
	return p.sockets.Count()
}

func (p *engineImpl) OnConnect(onConn func(socket Socket)) Engine {
	p.onSockets = append(p.onSockets, onConn)
	return p
}

func (p *engineImpl) checkVersion(v string) error {
	if v != protocolVersion.s {
		return fmt.Errorf("illegal protocol version: EIO=%s", v)
	}
	return nil
}

func (p *engineImpl) checkTransport(qTransport string) (TransportType, error) {
	var t TransportType
	switch qTransport {
	default:
		return -1, fmt.Errorf("invalid transport '%s'", qTransport)
	case "polling":
		t = POLLING
	case "websocket":
		t = WEBSOCKET
	}
	for _, it := range p.allowTransports {
		if t == it {
			return t, nil
		}
	}
	return -1, fmt.Errorf("transport '%s' is forbiden", qTransport)
}

func (p *engineImpl) generateID() string {
	if atomic.CompareAndSwapUint32(&(p.sequence), 0xFFFF, 0) {
		return p.sidGen(0)
	}
	return p.sidGen(atomic.AddUint32(&(p.sequence), 1))
}

func (p *engineImpl) ensureCleaner() {
	if p.junkTicker != nil {
		return
	}
	p.junkTicker = time.NewTicker(time.Millisecond * time.Duration(p.options.pingTimeout))
	// cron: check and kill lost socket.
	go func() {
		var end bool
		for {
			if end {
				break
			}
			select {
			case <-p.junkTicker.C:
				losts := p.sockets.List(func(val *socketImpl) bool {
					return val.isLost()
				})
				if len(losts) > 0 {
					for _, it := range losts {
						it.Close()
					}
					if p.logInfo != nil {
						p.logInfo.Printf("***** kill %d DEAD sockets *****\n", len(losts))
					}
				}
				break
			case <-p.junkKiller:
				p.junkTicker.Stop()
				end = true
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
		fn(socket)
	}
}

type socketMap struct {
	store *sync.Map
}

func (p *socketMap) Get(id string) (*socketImpl, bool) {
	val, ok := p.store.Load(id)
	if ok {
		return val.(*socketImpl), ok
	}
	return nil, ok
}

func (p *socketMap) Put(socket *socketImpl) {
	if _, ok := p.store.LoadOrStore(socket.ID(), socket); ok {
		panic(fmt.Errorf("socket#%s exists already", socket.ID()))
	}
}

func (p *socketMap) Remove(socket *socketImpl) {
	p.store.Delete(socket.ID())
}

func (p *socketMap) Count() int {
	var c int
	p.store.Range(func(_, _ interface{}) bool {
		c++
		return true
	})
	return c
}

func (p *socketMap) List(filter func(impl *socketImpl) bool) []*socketImpl {
	ret := make([]*socketImpl, 0)
	p.store.Range(func(key, value interface{}) bool {
		it := value.(*socketImpl)
		if filter == nil || filter(it) {
			ret = append(ret, it)
		}
		return true
	})
	return ret
}

package eio

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

var (
	protocolVersion = struct {
		n uint8
		s string
	}{3, "3"}
)

type engineOptions struct {
	allowUpgrades  bool
	cookie         bool
	cookiePath     string
	cookieHTTPOnly bool
	pingInterval   uint32
	pingTimeout    uint32
}

type engineImpl struct {
	infoLogger *log.Logger
	warnLogger *log.Logger
	errLogger  *log.Logger

	allowTransports []TransportType
	allowRequest    func(*http.Request) error
	sidGen          func(seq uint32) string
	sequence        uint32
	path            string
	options         *engineOptions
	onSockets       []func(Socket)
	sockets         *socketMap
	junkKiller      chan struct{}
	junkTicker      *time.Ticker
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
		var tTypeActive TransportType
		if tTypeActive, err = p.checkTransport(query.Get("transport")); err != nil {
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

		var socketActive *socketImpl
		var transportActive Transport

		if isNew {
			sid = p.generateID()
			transportActive, socketActive = newTransport(p, tTypeActive), newSocket(sid, p)
			socketActive.setTransport(transportActive)
			transportActive.setSocket(socketActive)
			if err = transportActive.init(writer, request); err != nil {
				sendError(writer, err)
				return
			}
			socketActive.OnClose(func(reason string) {
				p.sockets.Remove(socketActive)
			})
			p.sockets.Put(socketActive)
			p.socketCreated(socketActive)
		} else if socketOld, ok := p.sockets.Get(sid); !ok {
			sendError(writer, fmt.Errorf("%s:socketActive#%s doesn't exist", request.Method, sid))
			return
		} else {
			socketActive = socketOld
			transportOld := socketOld.getTransport()
			tTypeOld := transportOld.GetType()
			if tTypeActive > tTypeOld {
				transportActive = newTransport(p, tTypeActive)
				transportActive.setSocket(socketActive)
				socketActive.setTransport(transportActive)
			} else if tTypeActive < tTypeOld {
				transportActive = socketOld.getTransportBackup()
			} else {
				transportActive = transportOld
			}
		}
		transportActive.receive(writer, request)
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
		for {
			select {
			case <-p.junkTicker.C:
				losts := p.sockets.List(func(val *socketImpl) bool {
					return val.isLost()
				})
				if len(losts) > 0 {
					for _, it := range losts {
						it.Close()
					}
					if p.infoLogger != nil {
						p.infoLogger.Printf("***** kill %d DEAD sockets *****\n", len(losts))
					}
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

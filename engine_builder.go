package eio

import (
	"errors"
	"log"
	"math/rand"
	"net/http"
	"sync"
)

const (
	defaultPingTimeout  uint32 = 60000
	defaultPingInterval uint32 = 25000
)

var defaultTransports = []TransportType{POLLING, WEBSOCKET}

// EngineBuilder is a builder for Engine.
type EngineBuilder struct {
	l1              *log.Logger
	l2              *log.Logger
	l3              *log.Logger
	allowTransports []TransportType
	options         *engineOptions
	path            string
	gen             func(uint32) string
	allowRequest    func(*http.Request) error
	checkProtocol   bool
}

// ForceCheckProtocol force check eio protocol version in query EIO.
func (p *EngineBuilder) ForceCheckProtocol() *EngineBuilder {
	p.checkProtocol = true
	return p
}

// SetAllowRequest set a function that receives a given request, and can decide whether to continue or not.
func (p *EngineBuilder) SetAllowRequest(validator func(*http.Request) error) *EngineBuilder {
	p.allowRequest = validator
	return p
}

// SetLoggerInfo set logger for INFO
func (p *EngineBuilder) SetLoggerInfo(logger *log.Logger) *EngineBuilder {
	p.l1 = logger
	return p
}

// SetLoggerWarn set logger for WARN
func (p *EngineBuilder) SetLoggerWarn(logger *log.Logger) *EngineBuilder {
	p.l2 = logger
	return p
}

// SetLoggerError set logger for ERROR
func (p *EngineBuilder) SetLoggerError(logger *log.Logger) *EngineBuilder {
	p.l3 = logger
	return p
}

// SetTransports define transport types allow.
func (p *EngineBuilder) SetTransports(transports ...TransportType) *EngineBuilder {
	p.allowTransports = transports
	return p
}

// SetGenerateID define the method of creating SocketID.
func (p *EngineBuilder) SetGenerateID(gen func(uint32) string) *EngineBuilder {
	p.gen = gen
	return p
}

// SetPath define the http router path for Engine.
func (p *EngineBuilder) SetPath(path string) *EngineBuilder {
	p.path = path
	return p
}

// SetCookie can control enable/disable of cookie.
func (p *EngineBuilder) SetCookie(enable bool) *EngineBuilder {
	p.options.cookie = enable
	return p
}

// SetCookiePath define the path of cookie.
func (p *EngineBuilder) SetCookiePath(path string) *EngineBuilder {
	if len(path) < 1 {
		panic(errors.New("invalid cookie path: path is blank"))
	}
	if path[0] != '/' {
		panic(errors.New("cookie path must starts with '/'"))
	}
	p.options.cookiePath = path
	return p
}

// SetCookieHTTPOnly if set true HttpOnly io cookie cannot be accessed by client-side APIs,
// such as JavaScript. (true) This option has no effect
// if cookie or cookiePath is set to false.
func (p *EngineBuilder) SetCookieHTTPOnly(httpOnly bool) *EngineBuilder {
	p.options.cookieHTTPOnly = httpOnly
	return p
}

// SetAllowUpgrades define whether to allow transport upgrades. (default allow upgrades)
func (p *EngineBuilder) SetAllowUpgrades(enable bool) *EngineBuilder {
	p.options.allowUpgrades = enable
	return p
}

// SetPingInterval define ping time interval in millseconds for client.
func (p *EngineBuilder) SetPingInterval(interval uint32) *EngineBuilder {
	p.options.pingInterval = interval
	return p
}

// SetPingTimeout define ping timeout in millseconds for client.
func (p *EngineBuilder) SetPingTimeout(timeout uint32) *EngineBuilder {
	p.options.pingTimeout = timeout
	return p
}

// ForceAsyncHandle enable message handlers runing async. (default is sync)
func (p *EngineBuilder) ForceAsyncHandle() *EngineBuilder {
	p.options.handleAsync = true
	return p
}

// Build returns a new Engine.
func (p *EngineBuilder) Build() Engine {
	clone := func(origin engineOptions) engineOptions {
		return origin
	}(*p.options)
	sockets := socketMap{
		store: new(sync.Map),
	}
	eng := &engineImpl{
		logInfo:       p.l1,
		logWarn:       p.l2,
		logErr:        p.l3,
		sequence:      rand.Uint32(),
		onSockets:     make([]func(Socket), 0),
		options:       &clone,
		sockets:       &sockets,
		path:          p.path,
		sidGen:        p.gen,
		junkKiller:    make(chan struct{}),
		junkTicker:    nil,
		allowRequest:  p.allowRequest,
		checkProtocol: p.checkProtocol,
	}
	if len(p.allowTransports) < 1 {
		eng.allowTransports = defaultTransports
	} else {
		allows := make([]TransportType, 0)
		copy(allows, p.allowTransports)
		eng.allowTransports = allows
	}
	return eng
}

// NewEngineBuilder create a builder for Engine.
func NewEngineBuilder() *EngineBuilder {
	options := engineOptions{
		cookie:         false,
		cookiePath:     "/",
		cookieHTTPOnly: true,
		pingInterval:   defaultPingInterval,
		pingTimeout:    defaultPingTimeout,
		allowUpgrades:  true,
	}
	builder := EngineBuilder{
		path:    DefaultPath,
		options: &options,
		gen:     randomSessionID,
	}
	return &builder
}

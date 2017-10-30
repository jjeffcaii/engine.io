package eio

import (
	"math/rand"

	"sync"
)

const (
	defaultPingTimeout  uint32 = 60000
	defaultPingInterval uint32 = 25000
)

var defaultTransports = []TransportType{POLLING, WEBSOCKET}

// EngineBuilder is a builder for Engine.
type EngineBuilder struct {
	allowTransports []TransportType
	options         *engineOptions
	path            string
	gen             func(uint32) string
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

// Build returns a new Engine.
func (p *EngineBuilder) Build() Engine {
	clone := func(origin engineOptions) engineOptions {
		return origin
	}(*p.options)
	sockets := socketMap{
		store: new(sync.Map),
	}
	eng := &engineImpl{
		sequence:   rand.Uint32(),
		onSockets:  make([]func(Socket), 0),
		options:    &clone,
		sockets:    &sockets,
		path:       p.path,
		sidGen:     p.gen,
		junkKiller: make(chan struct{}),
		junkTicker: nil,
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
		cookie:        false,
		pingInterval:  defaultPingInterval,
		pingTimeout:   defaultPingTimeout,
		allowUpgrades: true,
	}
	builder := EngineBuilder{
		path:    DefaultPath,
		options: &options,
		gen:     randomSessionID,
	}
	return &builder
}

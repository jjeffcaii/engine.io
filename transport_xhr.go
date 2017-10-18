package engine_io

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

type xhrTransport struct {
	server      *engineImpl
	pollingTime time.Duration
}

func (p *xhrTransport) transport(ctx *context) error {
	defer ctx.req.Body.Close()
	doReq := func() ([]*Packet, error) {
		switch ctx.req.Method {
		default:
			return nil, errors.New(fmt.Sprintf("Unsupported Method: %s", ctx.req.Method))
		case http.MethodGet:
			if len(ctx.sid) < 1 {
				return p.asNewborn(ctx)
			} else if !p.server.hasSocket(ctx.sid) {
				return nil, errors.New(fmt.Sprintf("No such socket#%s", ctx.sid))
			} else {
				return p.asPolling(ctx)
			}
		case http.MethodPost:
			return p.asPushing(ctx)
		}
	}

	headers := ctx.res.Header()
	if p.server.options.cookie {
		headers.Set("Set-Cookie", fmt.Sprintf("io=%s; Path=/; HttpOnly", ctx.sid))
	}
	if _, ok := ctx.req.Header["User-Agent"]; ok {
		headers.Set("Access-Control-Allow-Origin", "*")
	}
	var data []byte
	if pks, err := doReq(); err != nil {
		ctx.res.WriteHeader(http.StatusInternalServerError)
		headers.Set("Content-Type", "text/plain; charset=UTF-8")
		ee := engineError{Code: 0, Message: err.Error()}
		bs, _ := json.Marshal(&ee)
		data = bs
	} else if pks == nil || len(pks) < 1 {
		ctx.res.WriteHeader(http.StatusOK)
		headers.Set("Content-Type", "text/html; charset=UTF-8")
		data = []byte("ok")
	} else {
		headers.Set("Content-Type", "text/plain; charset=UTF-8")
		if bs, err := marshallStringPayload(pks); err != nil {
			ctx.res.WriteHeader(http.StatusInternalServerError)
			ee := engineError{Code: 0, Message: err.Error()}
			bs, _ := json.Marshal(&ee)
			data = bs
		} else {
			ctx.res.WriteHeader(http.StatusOK)
			data = bs
		}
	}
	//	glog.Infof("==> response: %s\n", data)
	_, err := ctx.res.Write(data)
	return err
}

func (p *xhrTransport) asPolling(ctx *context) ([]*Packet, error) {
	closeNotifier := ctx.res.(http.CloseNotifier)
	socket := p.server.getSocket(ctx.sid)
	queue := make([]*Packet, 0)
	// 1. check current packets inbox chan buffer.
	end := false
	for {
		select {
		case pk := <-socket.outbox:
			queue = append(queue, pk)
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
			kill := Packet{
				typo: typeClose,
				data: make([]byte, 0),
			}
			queue = append(queue, &kill)
			go socket.Close()
			break
		case pk := <-socket.outbox:
			queue = append(queue, pk)
			break
		case <-time.After(p.pollingTime):
			kill := Packet{
				typo: typeClose,
				data: make([]byte, 0),
			}
			queue = append(queue, &kill)
			go socket.Close()
			break
		}
	}
	return queue, nil
}

func (p *xhrTransport) asPushing(ctx *context) ([]*Packet, error) {
	socket := p.server.getSocket(ctx.sid)
	if socket == nil {
		return nil, errors.New(fmt.Sprintf("No such socket#%s", ctx.sid))
	}
	body, _ := ioutil.ReadAll(ctx.req.Body)
	packets, err := unmarshallStringPayload(body)
	if err != nil {
		return nil, err
	}
	for _, pack := range packets {
		socket.inbox <- pack
	}
	return nil, nil
}

func (p *xhrTransport) asNewborn(ctx *context) ([]*Packet, error) {
	ctx.sid = newSocketId()
	up := p.newUpgradeSuccess(ctx.sid)
	socket := newSocket(ctx, p.server, 128, 128)
	p.server.putSocket(socket)
	for _, fn := range p.server.onSockets {
		go fn(socket)
	}
	socket.fire()
	return []*Packet{up}, nil
}

func (p *xhrTransport) newUpgradeSuccess(sid string) *Packet {
	us := upgradeSuccess{
		Sid:          sid,
		Upgrades:     []string{transportWebsocket},
		PingInterval: p.server.options.pingInterval,
		PingTimeout:  p.server.options.pingTimeout,
	}
	packet := new(Packet)
	if err := packet.fromJSON(typeOpen, &us); err != nil {
		panic(err)
	}
	return packet
}

func newXhrTransport(server *engineImpl) *xhrTransport {
	trans := xhrTransport{
		server:      server,
		pollingTime: time.Millisecond * time.Duration(server.options.pingTimeout),
	}
	return &trans
}

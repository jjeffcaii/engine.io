package engine_io

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"math/rand"
	"strings"
	"sync"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func randomSessionId(seed uint32) string {
	bf := new(bytes.Buffer)
	for i := 0; i < 12; i++ {
		bf.WriteByte(byte(rand.Int31n(256)))
	}
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

type queue struct {
	lock *sync.RWMutex
	q    []interface{}
}

func newQueue() *queue {
	foo := queue{
		lock: new(sync.RWMutex),
		q:    make([]interface{}, 0),
	}
	return &foo
}

func (p *queue) size() int {
	p.lock.RLock()
	p.lock.RUnlock()
	return len(p.q)
}

func (p *queue) append(item interface{}) {
	p.lock.Lock()
	p.q = append(p.q, item)
	p.lock.Unlock()
}

func (p *queue) pop() (interface{}, bool) {
	p.lock.Lock()
	defer p.lock.Unlock()
	if len(p.q) < 1 {
		return nil, false
	}
	var foo interface{}
	foo, p.q = p.q[0], p.q[1:]
	return foo, true
}

package eio

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	emptyStringArray = make([]string, 0)
	b64Rep           = strings.NewReplacer("/", "_", "+", "-")
	random           *rand.Rand
	randomLocker     sync.Mutex
)

func init() {
	random = rand.New(rand.NewSource(time.Now().UnixNano()))
}

func sendError(writer http.ResponseWriter, e error, codes ...int) {
	httpCode, bizCode := http.StatusInternalServerError, 0
	if len(codes) > 0 {
		httpCode = codes[0]
	}
	if len(codes) > 1 {
		bizCode = codes[1]
	}
	writer.Header().Set("Content-Type", "application/json; charset=UTF8")
	writer.WriteHeader(httpCode)
	foo := struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}{bizCode, e.Error()}
	if err := json.NewEncoder(writer).Encode(&foo); err != nil {
		panic(err)
	}
}

func randomSessionID(seed uint32) string {
	bf := new(bytes.Buffer)
	var b byte
	for i := 0; i < 12; i++ {
		randomLocker.Lock()
		b = byte(rand.Int31n(256))
		randomLocker.Unlock()
		bf.WriteByte(b)
	}
	_ = binary.Write(bf, binary.BigEndian, seed)
	bs := bf.Bytes()
	s := base64.StdEncoding.EncodeToString(bs)
	s = b64Rep.Replace(s)
	return s
}

type queue struct {
	lock sync.RWMutex
	q    []interface{}
}

func newQueue() *queue {
	return &queue{}
}

func (p *queue) size() (size int) {
	p.lock.RLock()
	size = len(p.q)
	p.lock.RUnlock()
	return
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

func (p *queue) reset() []interface{} {
	p.lock.Lock()
	defer p.lock.Unlock()
	var ret []interface{}
	ret, p.q = p.q[:], p.q[:0]
	return ret
}

func tryRecover(e interface{}) error {
	if e == nil {
		return nil
	}
	switch v := e.(type) {
	case error:
		return v
	case string:
		return errors.New(v)
	default:
		return fmt.Errorf("%s", v)
	}
}

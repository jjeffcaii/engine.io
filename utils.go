package eio

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
)

var emptyStringArray = make([]string, 0)

func init() {
	rand.Seed(time.Now().UnixNano())
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
	glog.Warningln("ERR:", e)
}

func randomSessionID(seed uint32) string {
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

func now32() uint32 {
	return uint32(time.Now().Unix())
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

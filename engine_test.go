package eio

import (
	"log"
	"testing"
	"time"
)

var server Engine

func init() {
	server = NewEngineBuilder().SetPingTimeout(time.Minute).SetPingInterval(30 * time.Second).Build()
}

func TestFunction(t *testing.T) {
	log.Printf("pointer: %p\n", server)

	t.Log("PASS")

}

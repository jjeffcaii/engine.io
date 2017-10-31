package eio

import (
	"log"
	"testing"
)

var server Engine

func init() {
	server = NewEngineBuilder().SetPingTimeout(3000).SetPingInterval(5000).Build()
}

func TestFunction(t *testing.T) {
	log.Printf("pointer: %p\n", server)

	t.Log("PASS")

}

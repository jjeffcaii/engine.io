package engine_io

import "testing"

var server Engine

func init() {
	server = NewEngineBuilder().SetPingTimeout(3000).SetPingInterval(5000).Build()
}

func TestFunction(t *testing.T) {

	t.Log("PASS")

}

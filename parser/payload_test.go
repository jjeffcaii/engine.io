package parser

import (
	"testing"
)

func TestNativeRead(t *testing.T) {
	var size int
	var left []byte
	var got []byte

	input := []byte("7:4你好，世界!1:5")

	size, left, _ = readPacketLength(input)
	if size != 7 {
		t.Error("size should be 7")
	}
	got, left, _ = readPacketString(left, size)

	if string(got) != "4你好，世界!" {
		t.Error("read err")
	}
	size, left, _ = readPacketLength(left)
	if size != 1 {
		t.Error("size should be 1")
	}

	got, left, _ = readPacketString(left, size)
	if string(got) != "5" {
		t.Error("read err2")
	}

}

func TestDecodePayload(t *testing.T) {
	input := "7:4你好，世界!1:5"
	packets, err := Payload.Decode([]byte(input))
	if err != nil {
		t.Error(err)
	}
	if len(packets) != 2 {
		t.Error("should 2 packets")
	}
	if string(packets[0].Data) != "你好，世界!" {
		t.Error("illegal result")
	}
	if string(packets[1].Data) != "" {
		t.Error("illegal result")
	}
}

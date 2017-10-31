package parser

import (
	"bytes"
	"log"
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
	input := []byte("7:4你好，世界!1:5")
	packets, err := DecodePayload(input)
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

func TestDecodePayloadBase64(t *testing.T) {
	input := []byte("22:b4YmluYXJ5IGZ1Y2sgMiE=")
	packets, err := DecodePayload(input)
	if err != nil {
		t.Error(err)
	}
	if len(packets) != 1 {
		t.Error("should 1 packets")
	}
	exp := "binary fuck 2!"
	if string(packets[0].Data) != exp {
		t.Error("data should be:", exp)
	}
}

func TestEncodeJSONP(t *testing.T) {
	m := make(map[string]interface{})
	m["id"] = 1
	m["content"] = "this is a test content.\nfuck fuck fuck!"
	packet := NewPacketByJSON(MESSAGE, m)
	bf := new(bytes.Buffer)
	if err := WritePayloadTo(bf, true, packet, packet); err != nil {
		t.Error(err)
	}
	log.Println(string(bf.Bytes()))
}

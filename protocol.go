package engine_io

import (
	"bytes"
	"encoding/json"
)

const (
	typeOpen    uint8 = 0
	typeClose   uint8 = 1
	typePing    uint8 = 2
	typePong    uint8 = 3
	typeMessage uint8 = 4
	typeUpgrade uint8 = 5
	typeNoop    uint8 = 6
)

type Packet struct {
	typo   uint8
	data   []byte
	option SendOption
}

func newPacket(packetType uint8, data []byte, opt SendOption) *Packet {
	packet := Packet{
		typo:   packetType,
		data:   data,
		option: opt,
	}
	return &packet
}

func newPacketByString(packetType uint8, data string) *Packet {
	return newPacket(packetType, []byte(data), 0)
}

func newPacketByJSON(packetType uint8, any interface{}) *Packet {
	bs, err := json.Marshal(any)
	if err != nil {
		panic(err)
	}
	return newPacket(packetType, bs, 0)
}

func newPacketAuto(ptype uint8, any interface{}) *Packet {
	switch any.(type) {
	default:
		return newPacketByJSON(ptype, any)
	case string:
		v, _ := any.(string)
		return newPacketByString(ptype, v)
	case []byte:
		v, _ := any.([]byte)
		return newPacket(ptype, v, BINARY)
	case *bytes.Buffer:
		v, _ := any.(*bytes.Buffer)
		return newPacket(ptype, v.Bytes(), BINARY)
	case bytes.Buffer:
		v, _ := any.(bytes.Buffer)
		return newPacket(ptype, v.Bytes(), BINARY)
	}
}

package parser

import (
	"bytes"
	"encoding/json"
)

type PacketType uint8
type PacketOption uint8

const (
	OPEN    PacketType = 0
	CLOSE   PacketType = 1
	PING    PacketType = 2
	PONG    PacketType = 3
	MESSAGE PacketType = 4
	UPGRADE PacketType = 5
	NOOP    PacketType = 6

	BINARY PacketOption = 0x01
	BASE64 PacketOption = 0x01 << 1
)

type Packet struct {
	Type   PacketType
	Data   []byte
	Option PacketOption
}

func NewPacketCustom(packetType PacketType, data []byte, opt PacketOption) *Packet {
	packet := Packet{
		Type:   packetType,
		Data:   data,
		Option: opt,
	}
	return &packet
}

func NewPacketByString(packetType PacketType, data string) *Packet {
	return NewPacketCustom(packetType, []byte(data), 0)
}

func NewPacketByJSON(packetType PacketType, any interface{}) *Packet {
	bs, err := json.Marshal(any)
	if err != nil {
		panic(err)
	}
	return NewPacketCustom(packetType, bs, 0)
}

func NewPacketAuto(ptype PacketType, any interface{}) *Packet {
	switch any.(type) {
	default:
		return NewPacketByJSON(ptype, any)
	case string:
		v, _ := any.(string)
		return NewPacketByString(ptype, v)
	case []byte:
		v, _ := any.([]byte)
		return NewPacketCustom(ptype, v, BINARY)
	case *bytes.Buffer:
		v, _ := any.(*bytes.Buffer)
		return NewPacketCustom(ptype, v.Bytes(), BINARY)
	case bytes.Buffer:
		v, _ := any.(bytes.Buffer)
		return NewPacketCustom(ptype, v.Bytes(), BINARY)
	}
}

func Decode(input []byte, option PacketOption) (*Packet, error) {
	if option&BINARY != BINARY {
		return stringEncoder.decode(input)
	} else if option&BASE64 != BASE64 {
		return binaryEncoder.decode(input)
	} else {
		return base64Encoder.decode(input)
	}
}

func Encode(packet *Packet) ([]byte, error) {
	if packet.Option&BINARY != BINARY {
		return stringEncoder.encode(packet)
	} else if packet.Option&BASE64 != BASE64 {
		return binaryEncoder.encode(packet)
	} else {
		return base64Encoder.encode(packet)
	}
}

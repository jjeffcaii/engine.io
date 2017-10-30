package parser

import (
	"bytes"
	"encoding/json"
)

// PacketType define type of packet.
type PacketType uint8

// PacketOption define options for a packet.
type PacketOption uint8

const (
	// OPEN packet is sent from the server when a new transport is opened (recheck).
	OPEN PacketType = 0
	// CLOSE packet is request the close of this transport but does not shutdown the connection itself.
	CLOSE PacketType = 1
	// PING packet is sent by the client.
	// Server should answer with a pong packet containing the same data.
	PING PacketType = 2
	// PONG packet is sent by the server to respond to ping packets.
	PONG PacketType = 3
	// MESSAGE packet is actual message, client and server should call their callbacks with the data.
	MESSAGE PacketType = 4
	// UPGRADE packet is sent by client when upgrade finish.
	// Before engine.io switches a transport, it tests, if server and client can communicate over this transport.
	// If this test succeed, the client sends an upgrade packets which requests the server to flush its cache
	// on the old transport and switch to the new transport.
	UPGRADE PacketType = 5
	// NOOP is used primarily to force a poll cycle when an incoming websocket connection is received.
	NOOP PacketType = 6

	// BINARY option define this packet data is binary.
	BINARY PacketOption = 0x01
	// BASE64 option define a base64-encode packet.
	BASE64 PacketOption = 0x01 << 1
)

// Packet is minimal transmission unit.
// An encoded packet can be UTF-8 string or binary data.
// The packet encoding format for a string is as follows
type Packet struct {
	Type   PacketType
	Data   []byte
	Option PacketOption
}

// NewPacketCustom create a packet with custom settings.
func NewPacketCustom(packetType PacketType, data []byte, opt PacketOption) *Packet {
	packet := Packet{
		Type:   packetType,
		Data:   data,
		Option: opt,
	}
	return &packet
}

// NewPacketByString create a packet from exist string.
func NewPacketByString(packetType PacketType, data string) *Packet {
	return NewPacketCustom(packetType, []byte(data), 0)
}

// NewPacketByJSON create a packet from a JSON object.
func NewPacketByJSON(packetType PacketType, any interface{}) *Packet {
	bs, err := json.Marshal(any)
	if err != nil {
		panic(err)
	}
	return NewPacketCustom(packetType, bs, 0)
}

// NewPacket create a packet according to your input data type.
func NewPacket(ptype PacketType, any interface{}) *Packet {
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

// Decode a packet from bytes.
func Decode(input []byte, option PacketOption) (*Packet, error) {
	if option&BINARY != BINARY {
		return stringEncoder.decode(input)
	} else if option&BASE64 != BASE64 {
		return binaryEncoder.decode(input)
	} else {
		return base64Encoder.decode(input)
	}
}

// Encode a packet to bytes.
func Encode(packet *Packet) ([]byte, error) {
	if packet.Option&BINARY != BINARY {
		return stringEncoder.encode(packet)
	} else if packet.Option&BASE64 != BASE64 {
		return binaryEncoder.encode(packet)
	} else {
		return base64Encoder.encode(packet)
	}
}

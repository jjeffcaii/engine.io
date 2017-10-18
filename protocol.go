package engine_io

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
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
	typo uint8
	data []byte
}

type packetEncoder interface {
	Decode(data []byte) (*Packet, error)
	Encode(packet *Packet) ([]byte, error)
}

type binaryPacketEncoder struct {
}

func (p *binaryPacketEncoder) Decode(data []byte) (*Packet, error) {
	if data == nil || len(data) < 1 {
		return nil, errors.New("invalid input data")
	}
	var t uint8 = data[0]
	switch t {
	default:
		return nil, errors.New(fmt.Sprintf("invalid packet type: %d", t))
	case typeOpen, typeClose, typePing, typePong, typeMessage, typeUpgrade, typeNoop:
		pack := Packet{
			typo: t,
			data: data[1:],
		}
		return &pack, nil
	}
}

func (p *binaryPacketEncoder) Encode(packet *Packet) ([]byte, error) {
	bf := new(bytes.Buffer)
	if err := bf.WriteByte(packet.typo); err != nil {
		return nil, err
	} else if _, err := bf.Write(packet.data); err != nil {
		return nil, err
	} else {
		return bf.Bytes(), nil
	}
}

type stringPacketEncoder struct {
}

func (p *stringPacketEncoder) Decode(data []byte) (*Packet, error) {
	if data == nil || len(data) < 1 {
		return nil, errors.New("invalid input data")
	}
	var t uint8
	switch data[0] {
	default:
		return nil, errors.New(fmt.Sprintf("invalid packet type: %s", string(data[0])))
	case 0x30:
		t = typeOpen
		break
	case 0x31:
		t = typeClose
		break
	case 0x32:
		t = typePing
		break
	case 0x33:
		t = typePong
		break
	case 0x34:
		t = typeMessage
		break
	case 0x35:
		t = typeUpgrade
		break
	case 0x36:
		t = typeNoop
		break
	}
	pack := Packet{
		typo: t,
		data: data[1:],
	}
	return &pack, nil
}
func (p *stringPacketEncoder) Encode(packet *Packet) ([]byte, error) {
	var c byte
	switch packet.typo {
	default:
		return nil, errors.New(fmt.Sprintf("invalid packet type %d", packet.typo))
	case typeOpen:
		c = 0x30
		break
	case typeClose:
		c = 0x31
		break
	case typePing:
		c = 0x32
		break
	case typePong:
		c = 0x33
		break
	case typeMessage:
		c = 0x34
		break
	case typeUpgrade:
		c = 0x35
		break
	case typeNoop:
		c = 0x36
		break
	}
	bf := new(bytes.Buffer)
	if err := bf.WriteByte(c); err != nil {
		return nil, err
	} else if _, err := bf.Write(packet.data); err != nil {
		return nil, err
	} else {
		return bf.Bytes(), nil
	}
}

func (p *Packet) fromBytes(typo uint8, data []byte) error {
	p.typo = typo
	p.data = data
	return nil
}

func (p *Packet) fromString(typo uint8, data string) error {
	return p.fromBytes(typo, []byte(data))
}

func (p *Packet) fromJSON(typo uint8, data interface{}) error {
	if bs, err := json.Marshal(data); err != nil {
		return err
	} else {
		return p.fromBytes(typo, bs)
	}
}

func (p *Packet) fromAny(message interface{}) error {
	switch message.(type) {
	default:
		if err := p.fromJSON(typeMessage, message); err != nil {
			return err
		}
		break
	case string:
		v, _ := message.(string)
		if err := p.fromString(typeMessage, v); err != nil {
			return err
		}
		break
	case []byte:
		v, _ := message.([]byte)
		if err := p.fromBytes(typeMessage, v); err != nil {
			return err
		}
		break
	case *bytes.Buffer:
		v, _ := message.(*bytes.Buffer)
		if err := p.fromBytes(typeMessage, v.Bytes()); err != nil {
			return err
		}
		break
	case bytes.Buffer:
		v, _ := message.(bytes.Buffer)
		if err := p.fromBytes(typeMessage, v.Bytes()); err != nil {
			return err
		}
		break
	}
	return nil
}

var (
	stringEncoder packetEncoder = new(stringPacketEncoder)
	binaryEncoder packetEncoder = new(binaryPacketEncoder)
)

func marshallStringPayload(packets []*Packet) ([]byte, error) {
	bf := new(bytes.Buffer)
	for _, pack := range packets {
		if bs, err := stringEncoder.Encode(pack); err != nil {
			return nil, err
		} else if _, err := bf.WriteString(fmt.Sprintf("%d:", len(bs))); err != nil {
			return nil, err
		} else if _, err := bf.Write(bs); err != nil {
			return nil, err
		}
	}
	return bf.Bytes(), nil
}

func unmarshallStringPayload(payload []byte) ([]*Packet, error) {
	var i int
	var o int
	packets := make([]*Packet, 0)
	for i < len(payload) {
		// equal with ':'
		if payload[i] == 0x3A {
			s := string(payload[o:i])
			if size, err := strconv.Atoi(s); err != nil {
				return nil, err
			} else if pack, err := stringEncoder.Decode(payload[i+1: i+1+size]); err != nil {
				return nil, err
			} else {
				packets = append(packets, pack)
				i += size + 1
				o = i
			}
		} else {
			i++
		}
	}
	return packets, nil
}

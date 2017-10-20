package engine_io

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
)

type packetCodec interface {
	decode(data []byte) (*Packet, error)
	encode(packet *Packet) ([]byte, error)
}

var (
	stringEncoder packetCodec = new(strCodec)
	binaryEncoder packetCodec = new(binCodec)
	base64Encoder packetCodec = new(b64Codec)
)

type binCodec struct {
}

func (p *binCodec) decode(data []byte) (*Packet, error) {
	if data == nil || len(data) < 1 {
		return nil, errors.New("packet bytes is empty")
	}
	t := data[0]
	switch t {
	default:
		return nil, errors.New(fmt.Sprintf("invalid packet type: %d", t))
	case typeOpen, typeClose, typePing, typePong, typeMessage, typeUpgrade, typeNoop:
		return newPacket(t, data[1:], BINARY), nil
	}
}

func (p *binCodec) encode(packet *Packet) ([]byte, error) {
	bf := new(bytes.Buffer)
	if err := bf.WriteByte(packet.typo); err != nil {
		return nil, err
	} else if _, err := bf.Write(packet.data); err != nil {
		return nil, err
	} else {
		return bf.Bytes(), nil
	}
}

type strCodec struct {
}

func (p *strCodec) decode(data []byte) (*Packet, error) {
	if data == nil || len(data) < 1 {
		return nil, errors.New("packet bytes is empty")
	}
	t, err := convertCharToType(data[0])
	if err != nil {
		return nil, err
	}
	return newPacket(t, data[1:], 0), nil
}

func (p *strCodec) encode(packet *Packet) ([]byte, error) {
	c, err := convertTypeToChar(packet.typo)
	if err != nil {
		return nil, err
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

type b64Codec struct {
}

func (p *b64Codec) decode(data []byte) (*Packet, error) {
	l := len(data)
	if l < 1 {
		return nil, errors.New("packet bytes is empty")
	}
	if l < 2 {
		t, err := convertCharToType(data[0])
		if err != nil {
			return nil, err
		}
		return newPacket(t, make([]byte, 0), BINARY), nil
	}
	if data[0] != 'b' {
		return nil, errors.New(fmt.Sprintf("invalid b64 packet: %s", data))
	}
	if t, err := convertCharToType(data[1]); err != nil {
		return nil, err
	} else if data, err := base64.StdEncoding.DecodeString(string(data[2:])); err != nil {
		return nil, err
	} else {
		return newPacket(t, data, BINARY), nil
	}
}

func (p *b64Codec) encode(packet *Packet) ([]byte, error) {
	c, err := convertTypeToChar(packet.typo)
	if err != nil {
		return nil, err
	}
	bf := new(bytes.Buffer)
	if packet.data == nil || len(packet.data) < 1 {
		if err := bf.WriteByte(c); err != nil {
			return nil, err
		} else {
			return bf.Bytes(), err
		}
	}
	if err := bf.WriteByte('b'); err != nil {
		return nil, err
	}
	if err := bf.WriteByte(c); err != nil {
		return nil, err
	}
	body := base64.StdEncoding.EncodeToString(packet.data)
	if _, err := bf.WriteString(body); err != nil {
		return nil, err
	}
	return bf.Bytes(), nil
}

func convertCharToType(c byte) (uint8, error) {
	switch c {
	default:
		return 0xff, errors.New(fmt.Sprintf("invalid packet type: %s", c))
	case '0':
		return typeOpen, nil
	case '1':
		return typeClose, nil
	case '2':
		return typePing, nil
	case '3':
		return typePong, nil
	case '4':
		return typeMessage, nil
	case '5':
		return typeUpgrade, nil
	case '6':
		return typeNoop, nil
	}
}

func convertTypeToChar(ptype uint8) (byte, error) {
	switch ptype {
	default:
		return 0, errors.New(fmt.Sprintf("invalid packet type: %d", ptype))
	case typeOpen:
		return '0', nil
	case typeClose:
		return '1', nil
	case typePing:
		return '2', nil
	case typePong:
		return '3', nil
	case typeMessage:
		return '4', nil
	case typeUpgrade:
		return '5', nil
	case typeNoop:
		return '6', nil
	}
}

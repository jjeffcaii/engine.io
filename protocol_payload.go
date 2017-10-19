package engine_io

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
)

type payloadCodec interface {
	encode(packets ...*Packet) ([]byte, error)
	decode(input []byte) ([]*Packet, error)
}

var strPayloadCodec payloadCodec
var b64PayloadCodec payloadCodec

func init() {
	foo := stdPayloadCodec{
		codec: stringEncoder,
	}
	bar := stdPayloadCodec{
		codec: base64Encoder,
	}
	strPayloadCodec = &foo
	b64PayloadCodec = &bar
}

type stdPayloadCodec struct {
	codec packetCodec
}

func (p *stdPayloadCodec) encode(packets ...*Packet) ([]byte, error) {
	if len(packets) < 1 {
		return nil, errors.New("packets is empty")
	}
	bf := new(bytes.Buffer)
	for _, pack := range packets {
		if pack == nil {
			return nil, errors.New("some packet is nil")
		}
		if bs, err := p.codec.encode(pack); err != nil {
			return nil, err
		} else if _, err := bf.WriteString(fmt.Sprintf("%d:", len(bs))); err != nil {
			return nil, err
		} else if _, err := bf.Write(bs); err != nil {
			return nil, err
		}
	}
	return bf.Bytes(), nil
}

func (p *stdPayloadCodec) decode(input []byte) ([]*Packet, error) {
	totals := len(input)
	if totals < 1 {
		return nil, errors.New("payload is empty")
	}
	packets := make([]*Packet, 0)
	var cursor int
	var offset int
	for cursor < totals {
		if input[cursor] != ':' {
			cursor++
			continue
		}
		s := string(input[offset:cursor])
		if size, err := strconv.Atoi(s); err != nil {
			return nil, err
		} else if packet, err := p.codec.decode(input[(cursor + 1): (cursor + 1 + size)]); err != nil {
			return nil, err
		} else {
			packets = append(packets, packet)
			cursor += size + 1
			offset = cursor
		}
	}
	return packets, nil
}

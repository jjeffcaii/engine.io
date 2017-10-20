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

var payloader payloadCodec = new(stdPayloadCodec)

type stdPayloadCodec struct {
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
		var cc packetCodec
		if pack.option&BINARY == BINARY {
			cc = base64Encoder
		} else {
			cc = stringEncoder
		}
		if bs, err := cc.encode(pack); err != nil {
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
		size, err := strconv.Atoi(s)
		if err != nil {
			return nil, err
		}
		bs := input[(cursor + 1): (cursor + 1 + size)]
		var packet *Packet
		if p1, err := base64Encoder.decode(bs); err != nil {
			if p2, err2 := stringEncoder.decode(bs); err2 != nil {
				return nil, err
			} else {
				packet = p2
			}
		} else {
			packet = p1
		}
		packets = append(packets, packet)
		cursor += size + 1
		offset = cursor
	}
	return packets, nil
}

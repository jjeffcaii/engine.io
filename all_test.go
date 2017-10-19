package engine_io

import (
	"fmt"

	"testing"
)

func TestPayload(t *testing.T) {
	packets := make([]*Packet, 0)
	for i := 0; i < 10; i++ {
		pack := newPacketByString(typeMessage, fmt.Sprintf("test:%d", i))
		packets = append(packets, pack)
	}

	payload, _ := strPayloadCodec.encode(packets...)
	fmt.Println(string(payload))

	ps, _ := strPayloadCodec.decode(payload)

	for _, p := range ps {
		fmt.Printf("type=%d, data=%s\n", p.typo, p.data)
	}
}

package engine_io

import (
	"fmt"

	"testing"
)

func TestPayload(t *testing.T) {
	packets := make([]*Packet, 0)
	for i := 0; i < 10; i++ {
		s := fmt.Sprintf("test:%d", i)
		var pack *Packet
		if i&1 != 0 {
			pack = newPacketAuto(typeMessage, s)
		} else {
			pack = newPacketAuto(typeMessage, []byte(s))
		}
		packets = append(packets, pack)
	}

	payload, _ := payloader.encode(packets...)
	fmt.Println(string(payload))

	ps, _ := payloader.decode(payload)

	for _, p := range ps {
		fmt.Printf("type=%d, data=%s\n", p.typo, p.data)
	}
}

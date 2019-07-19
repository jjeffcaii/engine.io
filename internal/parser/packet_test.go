package parser

import (
	"fmt"
	"testing"
	"time"
)

const totals int64 = 1000000

func TestEncode(t *testing.T) {
	packets := make([]*Packet, 0)
	var i int64
	for i = 0; i < totals; i++ {
		packets = append(packets, NewPacket(MESSAGE, time.Now().Format(time.RFC3339)))
	}
	ts := time.Now().UnixNano()
	for _, it := range packets {
		if bytes, err := Encode(it); err != nil {
			t.Error(err)
		} else {
			if _, err := Decode(bytes, 0); err != nil {
				t.Error(err)
			}
		}
	}
	ts = time.Now().UnixNano() - ts
	cost := ts / 1000000
	fmt.Println("totals:", totals, "packets")
	fmt.Println("cost:", cost, "ms")
	fmt.Println("ops:", 1000*totals/cost, "op/sec")
}

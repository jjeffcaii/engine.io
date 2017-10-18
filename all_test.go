package engine_io

import (
	"fmt"

	"sync"
	"testing"
	"time"
)

func TestPayload(t *testing.T) {
	packets := make([]*Packet, 0)
	for i := 0; i < 10; i++ {
		pack := new(Packet)
		pack.fromString(typeMessage, fmt.Sprintf("test:%d", i))
		packets = append(packets, pack)
	}

	payload, _ := marshallStringPayload(packets)
	fmt.Println(string(payload))

	ps, _ := unmarshallStringPayload(payload)

	for _, p := range ps {
		fmt.Printf("type=%d, data=%s\n", p.typo, p.data)
	}

}

func TestChan(t *testing.T) {
	ch := make(chan string, 10)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for {
			var exit bool
			select {
			default:
				exit = true
				break
			case s := <-ch:
				fmt.Println("wahaha:", s)
				break
			}
			if exit {
				break
			}
		}
		wg.Done()
	}()
	wg.Wait()
}

func TestChan2(t *testing.T) {
	ch := make(chan uint32, 10)
	go func() {
		select {
		default:
			break
		case <-ch:
			break
		}
		fmt.Println("end")
	}()

	time.Sleep(time.Second * 10)

}

func TestChan3(t *testing.T) {
	ch := make(chan uint32, 10)
	go func() {
		select {
		default:
			break
		case <-ch:
			break
		}
		fmt.Println("end")
	}()

	time.Sleep(time.Second * 10)

}

func Test4(t *testing.T) {
	ch := make(chan *Packet, 10)
	defer close(ch)
	ch <- nil
}

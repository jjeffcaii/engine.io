package main

import (
	"flag"
	"log"
	_ "net/http/pprof"

	"time"

	"fmt"

	eio "engine.io"
)

func main() {
	flag.Parse()
	server := eio.NewEngineBuilder().Build()

	tick := time.NewTicker(5 * time.Second)
	kill := make(chan uint8)
	go func() {
		for {
			select {
			case <-tick.C:
				for _, it := range server.GetClients() {
					it.Send(fmt.Sprintf("AUTO_BRD: %s", time.Now().Format(time.RFC3339)))
				}
				break
			case <-kill:
				tick.Stop()
				return
			}
		}
	}()

	defer func() {
		close(kill)
	}()

	server.OnConnect(func(socket eio.Socket) {
		log.Println("========> socket connect:", socket.Id())
		socket.OnMessage(func(data []byte) {
			socket.Send(fmt.Sprintf("ECHO1: %s", data))
			socket.Send(fmt.Sprintf("ECHO2: %s", data))
		})
		socket.OnClose(func(reason string) {
			log.Println("========> socket closed:", socket.Id())
		})
	})
	log.Fatalln(server.Listen(":3000"))
}

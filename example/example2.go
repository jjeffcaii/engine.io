package main

import (
	"flag"
	"log"
	_ "net/http/pprof"

	eio "github.com/jjeffcaii/engine.io"
)

func main() {
	flag.Parse()
	server := eio.NewEngineBuilder().Build()

	server.OnConnect(func(socket eio.Socket) {
		//log.Println("========> socket connect:", socket.Id())
		socket.OnMessage(func(data []byte) {
			// do nothing.
			log.Println("===> got message:", string(data))
		})
		socket.OnClose(func(reason string) {
			//log.Println("========> socket closed:", socket.Id())
		})
		socket.Send("test message string")
		socket.Send([]byte("test message binary"))
	})
	log.Fatalln(server.Listen(":3000"))
}

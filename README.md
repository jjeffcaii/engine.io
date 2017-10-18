# Engine.IO (WARNING: STILL WORKING!!!!)
Server-side [Engine.IO](https://github.com/socketio/engine.io) in Golang.

## Example

``` golang
package main

import (
	"flag"
    "fmt"
	"log"

	eio "github.com/jjeffcaii/engine.io"
)

func main() {
	flag.Parse()
	server := eio.NewEngineBuilder().Build()
	server.OnConnect(func(socket eio.Socket) {
		log.Printf("socket#%s connect!\n", socket.Id())
		socket.OnMessage(func(data []byte) {
			socket.Send(fmt.Sprintf("echo: %s", data))
		})
		socket.OnClose(func(reason string) {
			log.Printf("socket#%s closed: %s!\n", socket.Id(), reason)
		})
	})
	log.Fatalln(server.Listen(":3000"))
}

```

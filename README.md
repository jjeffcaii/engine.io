# Engine.IO (WARNING: STILL WORKING!!!!)

[![Build Status](https://travis-ci.org/jjeffcaii/engine.io.svg?branch=master)](https://travis-ci.org/jjeffcaii/engine.io)

Unofficial server-side [Engine.IO](https://github.com/socketio/engine.io) in Golang.

## Example

``` golang
package main

import (
	"flag"
	"log"

	"github.com/jjeffcaii/engine.io"
)

func main() {
	flag.Parse()
	server := eio.NewEngineBuilder().Build()
	server.OnConnect(func(socket eio.Socket) {
		socket.OnMessage(func(data []byte) {
			log.Println("recieve:", string(data))
		})
		socket.OnClose(func(reason string) {
			log.Println("socket closed:", socket.ID())
		})
		socket.Send("你好,世界!")
	})
	log.Fatalln(server.Listen(":3000"))
}

```

## Compatibility

| Key | Compatible | Remarks |
|------|-----|------|
| polling-xhr | Yes | |
| polling-jsonp | Yes | |
| websocket | Yes | |
| upgrade | Yes | |

> NOTICE: all compatibility tests are under engine.io-client^3.1.2

## Benchmarks

TODO

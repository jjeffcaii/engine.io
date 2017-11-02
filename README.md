# Engine.IO

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

## Documents

Please see [https://godoc.org/github.com/jjeffcaii/engine.io](https://godoc.org/github.com/jjeffcaii/engine.io).

## Benchmarks

## transport upgrade sequence
copy below text to the [textart](http://textart.io/sequence)  
```
object ClientXHR ClientWS ServerXHR ServerWS
ClientXHR->ServerXHR: GET with polling transport
ServerXHR->ClientXHR: return OK with sid and upgrades["websocket"]
ClientXHR->ServerXHR:GET with websocket transport
ClientXHR->ServerXHR: conntinue polling message with POST method
ClientXHR->ServerXHR: GET method waiting result of last polling.
ServerXHR->ClientXHR: return OK to the client as response of POST method
ServerXHR->ServerWS: websocket upgrade, we have a new transport.
ServerXHR->ClientXHR: HttpCode 101 switching protocols

ClientXHR->ClientWS: change transport to websocket
ClientWS->ServerXHR: 2probe (ping with probe)
ServerXHR->ClientWS: 3probe (pong with probe) 
ServerXHR->ServerXHR: upgradeStart() 
ServerXHR->ClientXHR: send a noop packet to stop polling transport of client.
ClientWS->ServerXHR: send packet "5" tell server that i have upgraded
ServerWS->ServerXHR: upgradeEnd() now we can close ServerXHR
ServerXHR->ServerXHR: close XHR transport
ServerWS->ClientWS: send rest packet before upgrade.

ClientWS->ServerWS: some message
ServerWS->ClientWS: response some message
```
TODO


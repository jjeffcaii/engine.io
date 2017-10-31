package example

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"testing"

	"github.com/jjeffcaii/engine.io"
)

var server eio.Engine

func init() {
	flag.Parse()
	server = eio.NewEngineBuilder().Build()
	http.HandleFunc("/conns", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		writer.Write([]byte(fmt.Sprintf("totals: %d", server.CountClients())))
	})
	var indexHtml = `
<html>
    <head>
        <script src="https://cdnjs.cloudflare.com/ajax/libs/engine.io-client/3.1.3/engine.io.min.js"></script>
        <script>
         function testConn(url,transports,jsonp){
             // eio = Socket
             let u = url || "ws://127.0.0.1:3000";
             let t = transports || ["polling","websocket"];
             let options = {
                 transports: t
             };
             if(jsonp){
                 options.forceJSONP = true;
             }
             let socket = eio(u,options);
             socket.on('open', () => {
                 let totals = 10000
                 let count = 0
                 socket.on('message', (data) => {
                     console.log("MSG#%d => %s", ++count, data.toString());
                     if(count == totals){
                         alert(totals + " message success!");
	                     socket.close();
                     }
                 });
                 socket.on('close', ()=>{
                     alert('socket close');
	             });
	             for(let i=0;i<totals;i++){
                     socket.send('你好，'+i+'世界!');
                 }
             });
         }
        </script>
    </head>
    <body>
        <a href="javascript:testConn(null,['polling'])" role="button">polling</a>
        <br>
        <a href="javascript:testConn(null,['polling'],true)" role="button">jsonp</a>
        <br>
        <a href="javascript:testConn(null,['websocket'])" role="button">websocket</a>
        <br>
        <a href="javascript:testConn(null,['polling','websocket'])" role="button">auto</a>
        <br>
    </body>
</html>
	`
	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=UTF-8")
		writer.Write([]byte(indexHtml))
	})
}

func TestEchoServer(t *testing.T) {
	server.OnConnect(func(socket eio.Socket) {
		socket.OnMessage(func(data []byte) {
			socket.Send(fmt.Sprintf("[ECHO] %s", data))
		})
	})
	log.Fatalln(server.Listen(":3000"))
}

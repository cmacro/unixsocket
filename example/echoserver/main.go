package main

import (
	"fmt"
	"log"
	"net/http"

	_ "net/http/pprof"

	"github.com/panjf2000/gnet/v2"
)

type echoServer struct {
	*gnet.BuiltinEventEngine
	address string
}

// OnBoot is triggered when the server starts.
func (es *echoServer) OnBoot(eng gnet.Engine) gnet.Action {
	log.Printf("Echo server is listening on %s\n", es.address)
	return gnet.None
}

// OnShutdown is triggered when the server is shutting down.
func (es *echoServer) OnShutdown(eng gnet.Engine) {
	log.Println("Echo server is shutting down")
}

// OnOpen is triggered when a new connection is opened.
func (es *echoServer) OnOpen(conn gnet.Conn) ([]byte, gnet.Action) {
	log.Printf("New connection from %s\n", conn.RemoteAddr().String())
	return nil, gnet.None
}

// OnClose is triggered when a connection is closed.
func (es *echoServer) OnClose(conn gnet.Conn, err error) gnet.Action {
	log.Printf("Connection from %s closed\n", conn.RemoteAddr().String())
	return gnet.None
}

var (
	echotag = []byte("echo:")
)

// OnTraffic is triggered when there is data to read from the connection.
func (es *echoServer) OnTraffic(conn gnet.Conn) gnet.Action {
	// Read incoming data
	buffer, _ := conn.Next(-1)

	// Log received data
	log.Printf("Received data: %s", string(buffer))

	// Echo the data back to the client
	sendBuf := make([]byte, len(buffer)+len(echotag))
	copy(sendBuf, echotag)
	copy(sendBuf[len(echotag):], buffer)
	conn.Write(sendBuf)

	// Return no action to continue the connection
	return gnet.None
}

func main() {
	fmt.Println("run pprof", ":8801")
	go http.ListenAndServe(":8801", nil)

	// Address to bind the server
	address := "unix:///tmp/codesocket.tmp"
	server := &echoServer{address: address}

	// Start the server
	err := gnet.Run(server, address, gnet.WithMulticore(true), gnet.WithReusePort(true))
	if err != nil {
		log.Fatalf("Failed to start server: %v\n", err)
	}
}

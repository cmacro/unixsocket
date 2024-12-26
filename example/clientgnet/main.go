package main

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log"
	"os"
	"sync/atomic"
	"time"

	"github.com/panjf2000/gnet/v2"
	"github.com/panjf2000/gnet/v2/pkg/logging"
)

type clientEvents struct {
	*gnet.BuiltinEventEngine
	client    *gnet.Client
	conn      gnet.Conn
	connected atomic.Int32
}

func (ev *clientEvents) OnBoot(e gnet.Engine) gnet.Action {
	log.Println("Echo client is listening")
	return gnet.None
}

func (ev *clientEvents) OnOpen(c gnet.Conn) (out []byte, action gnet.Action) {
	log.Println("New connection")
	c.Write([]byte("hello world"))
	return
}

func (ev *clientEvents) OnClose(gnet.Conn, error) gnet.Action {
	log.Println("Connection closed")
	go ev.reconnect()
	return gnet.None
}

func (ev *clientEvents) OnTraffic(c gnet.Conn) (action gnet.Action) {
	d, _ := c.Next(-1)
	log.Println("res ", string(d))
	return
}

func (ev *clientEvents) OnTick() (time.Duration, gnet.Action) {
	log.Println("tick")
	return time.Second * 10, gnet.None
}

// 重连处理
func (ev *clientEvents) reconnect() {
	log.Println("Attempting to reconnect...")
	for {
		c, err := ev.client.Dial("unix", "/tmp/codesocket.tmp")
		if err == nil {
			log.Println("Reconnected successfully.")
			ev.conn = c
			ev.connected.Swap(1)
			return
		}

		log.Printf("Failed to reconnect: %v. Retrying in 5 seconds...\n", err)
		time.Sleep(5 * time.Second)
	}
}

func (ev *clientEvents) Write(data []byte) (n int, err error) {
	if ev.connected.Load() == 0 {
		return 0, errors.New("client closed")
	}

	return ev.conn.Write(data)
}

func main() {
	clientEV := &clientEvents{}
	client, _ := gnet.NewClient(
		clientEV,
		gnet.WithLogLevel(logging.DebugLevel),
		gnet.WithLockOSThread(true),
		gnet.WithTicker(true),
	)
	clientEV.client = client

	client.Start()
	defer client.Stop() //nolint:errcheck

	go clientEV.reconnect()

	readIOStd(context.Background(), clientEV)

	// time.Sleep(100 * time.Second)
}

func readIOStd(_ context.Context, w io.Writer) {
	defer func() { log.Print("read IO std closed") }()
	reader := bufio.NewReader(os.Stdin)
	writer := bufio.NewWriter(w)

	for {
		log.Print("Enter message: ")
		msg, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("failed read stdin, %v", err)
			return
		}
		if len(msg) > 0 && msg[len(msg)-1] == '\n' {
			msg = msg[:len(msg)-1]
		}
		if len(msg) == 0 {
			continue
		}

		if msg == "q" {
			log.Print("close of command")
			return
		}

		_, err = writer.WriteString(msg)
		if err == nil {
			err = writer.Flush()
		}
		if err != nil {
			log.Printf("failed writer message: %s, error: %v", msg, err)
		}
	}
}

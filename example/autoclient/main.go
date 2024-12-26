package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

type ServerMessage struct{}

func (s *ServerMessage) Write(p []byte) (n int, err error) {
	fmt.Print("Server:", string(p))
	return len(p), nil
}

func main() {
	socketPath := "/tmp/codesocket.tmp"
	client := NewClient()

	svrMsg := ServerMessage{}

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	wg.Add(1)
	go func() { defer wg.Done(); client.Connect(ctx, socketPath, &svrMsg) }()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		readIOStd(ctx, client)
	}()

	<-ctx.Done()
	wg.Wait()
	log.Print("close auto connect client")
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
		if msg == "q\n" {
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

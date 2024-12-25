package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
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

	ctx, cancel := context.WithCancel(context.Background())
	client.Connect(ctx, socketPath, &svrMsg)

	go func() {
		defer cancel()
		readIOStd(ctx, client)
	}()

	<-ctx.Done()
}

func readIOStd(_ context.Context, w io.Writer) {
	reader := bufio.NewReader(os.Stdin)
	writer := bufio.NewWriter(w)

	for {
		fmt.Print("Enter message: ")
		msg, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("failed read stdin, %v", err)
			return
		}
		if msg == "q" {
			fmt.Print("close of command")
			return
		}

		_, err = writer.WriteString(msg)
		if err == nil {
			err = writer.Flush()
		}
		if err != nil {
			fmt.Printf("failed writer message, error: %v", err)
		}
	}
}

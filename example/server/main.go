package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
)

func main() {
	socketPath := "/tmp/codesocket.tmp"

	// 清理旧的 Unix Socket 文件
	if err := os.RemoveAll(socketPath); err != nil {
		fmt.Printf("Failed to remove old socket: %v\n", err)
		return
	}

	// 创建 Unix Socket Listener
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		fmt.Printf("Failed to create listener: %v\n", err)
		return
	}
	defer listener.Close()

	fmt.Printf("Server listening on %s\n", socketPath)

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("Failed to accept connection: %v\n", err)
			continue
		}

		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	fmt.Println("Client connected")
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	for {
		// 从客户端读取数据
		message, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Failed to read from client: %v\n", err)
			return
		}

		fmt.Printf("Received: %s", message)

		// 向客户端发送数据
		response := fmt.Sprintf("Echo: %s", message)
		_, err = writer.WriteString(response)
		if err != nil {
			fmt.Printf("Failed to write to client: %v\n", err)
			return
		}
		writer.Flush()
	}
}

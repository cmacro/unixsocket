package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
)

func main() {
	socketPath := "/tmp/codesocket.tmp"

	// 连接到服务端
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		fmt.Printf("Failed to connect to server: %v\n", err)
		return
	}
	defer conn.Close()

	fmt.Println("Connected to server")
	reader := bufio.NewReader(os.Stdin)
	writer := bufio.NewWriter(conn)

	// writer.ReadFrom(r io.Reader)

	for {
		// 从标准输入读取数据
		fmt.Print("Enter message: ")
		message, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Failed to read input: %v\n", err)
			return
		}

		// 发送数据到服务端
		_, err = writer.WriteString(message)
		if err != nil {
			fmt.Printf("Failed to send message: %v\n", err)
			return
		}
		writer.Flush()

		// 接收服务端响应
		response, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			fmt.Printf("Failed to read response: %v\n", err)
			return
		}

		fmt.Printf("Server response: %s", response)
	}
}

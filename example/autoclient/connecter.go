package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type Client struct {
	mu        sync.Mutex
	writer    io.Writer
	conn      net.Conn
	writeChan chan []byte
	closed    atomic.Int32
}

func NewClient() *Client {
	cli := Client{
		writeChan: make(chan []byte, 100),
	}
	return &cli
}

func (c *Client) Connect(ctx context.Context, addr string, w io.Writer) error {
	c.writer = w

	for {
		if c.closed.Load() == 1 {
			return errors.New("client closed")
		}

		conn, err := c.autoConnect(ctx, addr)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			log.Printf("Reconnect failed: %v, retrying...", err)
			continue
		}

		log.Printf("Connected to %s", addr)
		c.mu.Lock()
		c.conn = conn
		c.mu.Unlock()

		// 开启读写协程
		stopChan := make(chan struct{})
		go c.readLoop(ctx, conn, stopChan)
		c.writeLoop(ctx, conn, stopChan)

		// 当写协程退出时，重新尝试连接
		c.mu.Lock()
		c.conn = nil
		log.Printf("Disconnected to %s", addr)
		c.mu.Unlock()
	}
}

func (c *Client) autoConnect(ctx context.Context, addr string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			conn, err := dialer.DialContext(ctx, "unix", addr)
			if err == nil {
				return conn, nil
			}
			log.Printf("Connection failed, retrying in 5s: %v", err)
			time.Sleep(5 * time.Second)
		}
	}
}

func (c *Client) readLoop(ctx context.Context, conn net.Conn, stopChan chan<- struct{}) {
	defer close(stopChan)
	defer conn.Close()
	reader := bufio.NewReader(conn)

	var cache bytes.Buffer // 动态缓冲区
	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// 读取数据并写入 writer
			n, err := reader.Read(buf)
			if n > 0 {
				cache.Write(buf[:n]) // 追加数据到缓冲区

				// 写入 writer
				if _, writeErr := c.writer.Write(cache.Bytes()); writeErr != nil {
					log.Printf("Write error: %v", writeErr)
					return
				}

				cache.Reset() // 清空缓冲区
			}

			if err != nil {
				if errors.Is(err, io.EOF) {
					log.Println("Server closed the connection")
					return
				}
				log.Printf("Read error: %v", err)

				// 如果是临时错误，可以延迟重试
				if ne, ok := err.(net.Error); ok && ne.Temporary() {
					time.Sleep(50 * time.Millisecond)
					continue
				}

				// 非临时错误，退出
				return
			}
		}
	}
}

func (c *Client) writeLoop(ctx context.Context, conn net.Conn, stopChan <-chan struct{}) {
	defer conn.Close()
	writer := bufio.NewWriter(conn)

	for {
		select {
		case <-ctx.Done():
			log.Println("Write loop exiting due to context cancellation")
			return
		case <-stopChan:
			log.Println("Write loop exiting: stop signal received")
			return
		case data := <-c.writeChan:
			_, err := writer.Write(data)
			if err != nil {
				log.Printf("Write error: %v", err)
				return
			}
			writer.Flush()
		}
	}
}

func (c *Client) Write(data []byte) (n int, err error) {
	if c.closed.Load() == 1 {
		return 0, errors.New("client closed")
	}

	select {
	case c.writeChan <- data:
		return len(data), nil
	default:
		return 0, errors.New("write channel is full")
	}
}

func (c *Client) Close() {
	if !c.closed.CompareAndSwap(0, 1) {
		return
	}

	c.mu.Lock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.mu.Unlock()
	close(c.writeChan)
	log.Println("Client closed")
}

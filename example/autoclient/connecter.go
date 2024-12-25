package main

import (
	"bufio"
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
		go c.readLoop(ctx, conn)
		c.writeLoop(ctx, conn)

		// 当写协程退出时，重新尝试连接
		c.mu.Lock()
		c.conn = nil
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

func (c *Client) readLoop(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// 高效读取数据并写入 writer
			_, err := reader.WriteTo(c.writer)
			if err != nil {
				log.Printf("Read error: %v", err)
				return
			}
		}
	}
}

func (c *Client) writeLoop(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	writer := bufio.NewWriter(conn)

	for {
		select {
		case <-ctx.Done():
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

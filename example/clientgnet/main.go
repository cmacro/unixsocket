//go:build go1.20

package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/panjf2000/gnet/v2"
	"golang.org/x/exp/rand"
)

type clientEvents struct {
	*gnet.BuiltinEventEngine
	client *gnet.Client
	mu     sync.Mutex
	conn   gnet.Conn
	status atomic.Int32

	// reconnect
	ctx         context.Context
	addr        string
	reconnectWG sync.WaitGroup
}

const (
	state_Closed int32 = iota
	state_Opened
	state_Connecting
)

func (ev *clientEvents) OnOpen(c gnet.Conn) (out []byte, action gnet.Action) {
	log.Println("open connection")
	c.Write([]byte("hello world"))
	return
}

func (ev *clientEvents) OnClose(gnet.Conn, error) gnet.Action {
	log.Println("connection closed")
	ev.setConnect(nil)
	ev.tryConnect()
	return gnet.None
}

func (ev *clientEvents) OnTraffic(c gnet.Conn) (action gnet.Action) {
	d, _ := c.Next(-1)
	log.Println("res: ", B2S(d))
	return
}

func (ev *clientEvents) Connect(ctx context.Context, addr string) error {
	ev.ctx = ctx
	ev.addr = addr

	network, _ := parseAddr(addr)
	if network == "" {
		return fmt.Errorf("unable to connect, invalid addr %s ", addr)
	}
	ev.tryConnect()
	return nil
}

func randomJitter(baseDelay time.Duration) time.Duration {
	jitter := time.Duration(rand.Int63n(int64(baseDelay)))
	return baseDelay + jitter
}

func (ev *clientEvents) tryConnect() {
	if ev.status.Load() == state_Closed {
		ev.reconnectWG.Add(1)
		go func() {
			defer ev.reconnectWG.Done()
			ev.reconnect(ev.ctx, ev.addr)
		}()
	}
}

func (ev *clientEvents) setConnect(c gnet.Conn) {
	ev.mu.Lock()
	defer ev.mu.Unlock()

	if c == ev.conn {
		return
	}
	if ev.conn != nil {
		log.Println("close previous connection")
		ev.conn.Close()
	}
	ev.conn = c
	if ev.conn == nil {
		log.Println("update connect status : Closed")
		ev.status.Swap(state_Closed)
	} else {
		log.Println("update connect status : Opened")
		ev.status.Swap(state_Opened)
	}
}

// reconnect
func (ev *clientEvents) reconnect(ctx context.Context, addr string) {
	if ev.status.Load() != state_Closed {
		return
	}
	ev.status.Swap(state_Connecting)
	defer func() {
		if ev.status.Load() == state_Connecting {
			ev.status.Swap(state_Closed)
		}
	}()

	network, address := parseAddr(addr)
	if network == "" {
		log.Printf("unable to connect, invalid addr %s ", addr)
		return
	}

	baseDelay := time.Second
	maxDelay := 20 * time.Second
	for attempt := 1; ; attempt++ {
		conn, err := ev.client.Dial(network, address)
		if err == nil {
			ev.setConnect(conn)
			return
		}
		log.Printf("attempt #%d failed, retrying in %v: %v", attempt, baseDelay, err)

		delay := randomJitter(baseDelay)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			log.Println("context canceled of wait, stopping reconnect.")
			return
		case <-timer.C:
		}
		// add delay
		baseDelay *= 2
		if baseDelay > maxDelay {
			baseDelay = maxDelay
		}
	}
}

func (ev *clientEvents) Write(data []byte) (n int, err error) {
	if ev.status.Load() != state_Opened {
		return 0, errors.New("client closed, status?")
	}
	ev.mu.Lock()
	defer ev.mu.Unlock()
	if ev.conn == nil {
		return 0, errors.New("client closed, conn is empty?")
	}
	return ev.conn.Write(data)
}

func main() {
	clientEV := &clientEvents{}
	client, _ := gnet.NewClient(
		clientEV,
		gnet.WithLockOSThread(true),
	)
	clientEV.client = client

	client.Start()
	defer client.Stop() //nolint:errcheck

	ctx, cancel := context.WithCancel(context.Background())
	err := clientEV.Connect(ctx, "unix:///tmp/codesocket.tmp")
	if err != nil {
		cancel()
		log.Printf("connect failed: %v", err)
		clientEV.reconnectWG.Wait()
		return
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		readIOStd(ctx, clientEV)
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sigs:
		cancel()
	case <-ctx.Done():
	}

	wg.Wait()
	clientEV.setConnect(nil)
	clientEV.reconnectWG.Wait()
}

func readIOStd(ctx context.Context, w io.Writer) {
	defer func() { log.Print("read os stdin closed") }()
	reader := bufio.NewReader(os.Stdin)
	// WARN: cannot use bufio.NewWriter, reconnection will result in call failure
	// writer := bufio.NewWriter(w)
	for {
		select {
		case <-ctx.Done():
			log.Println("Context canceled, stopping os stdin read.")
			return
		default:

			fmt.Print("Enter message: ")
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

			if _, err = w.Write(S2B(msg)); err != nil {
				log.Printf("failed writer message: %s, error: %v", msg, err)
				continue
			}

		}
	}
}

// b2s converts byte slice to a string without memory allocation.
// See https://groups.google.com/forum/#!msg/Golang-Nuts/ENgbUzYvCuU/90yGx7GUAgAJ .
func B2S(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}

func S2B(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

func parseAddr(s string) (network string, path string) {
	idx := strings.Index(s, "://")
	if idx == -1 {
		return "", ""
	}
	return s[:idx], s[idx+3:]
}

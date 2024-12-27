# unixsocket
Unix Domain Socket 


example | note
--- | ---
server | 直接使用net实现连接服务
echoserver | 使用gnet库实现连接服务，解决在大量连接携程过多消耗内存  
client | 直接使用net实现连接  
autoclient | 增加服务端重启或断开自动连接处理。发送消息使用通道，解决大量消息堵塞情况  
clientgnet | 使用gnet库实现连接，包括自动连接处理


## net.Dial 与 gnet 

**事件驱动 vs 阻塞 I/O：**   

**gnet：** gnet 采用 事件驱动模型，所有的 I/O 操作都在事件循环中执行。它通过一个 事件循环 来处理连接的建立、关闭、消息的收发等。

- **优点：** 更高效，适合大规模并发连接；避免了每个连接都启动一个 goroutine 带来的高内存开销和上下文切换。  
- **适用场景：** 需要高并发的应用，如 WebSocket 服务、游戏服务器、实时通信等。


**net.Dial：** net.Dial 是同步阻塞的连接方式，每个连接都会阻塞当前 goroutine（大约 2KB 的栈空间），直到连接成功。每个连接都需要独立的 goroutine 来处理数据读写。

- **优点：** 简单易用，适合小规模连接。
- **缺点：** 在大量连接的情况下，会消耗大量的内存和 CPU，导致性能瓶颈。


**并发处理：**  

- **gnet：** 所有的连接都由事件循环管理，不需要为每个连接创建独立的 goroutine，适合处理大量并发连接。  
- **net.Dial：** 每个连接都需要一个独立的 goroutine，适合低并发的应用，但在大量并发情况下会造成资源浪费。

**连接重试与重连：**  

- **gnet：** 可以在 OnClosed 或其他事件中实现自动重连。事件循环会在检测到连接断开时进行处理，不会阻塞整个应用。  
- **net.Dial：** 可以通过 for 循环和 time.Sleep 来实现重试，但每次重试会阻塞当前 goroutine。

### 性能对比  

- **gnet：** 采用异步 I/O 模型，能够在一个或几个线程中高效处理成千上万的连接，因此能够在高并发场景下保持较低的延迟和较高的吞吐量。  
- **net.Dial：** 需要为每个连接创建一个独立的 goroutine，这在高并发场景下会消耗大量的内存和 CPU，性能较差。


### 总结
- **gnet：** 适用于高并发的网络应用，能够通过事件驱动和异步 I/O 实现高效的网络服务，处理大量连接时性能更优。
- **net.Dial：** 适用于低并发、简单的网络应用，但在高并发场景下可能会导致性能瓶颈。

## 开发测试环境


```bash
win11, wsl2

dev@KAIFAPC:~$ cat /etc/os-release
PRETTY_NAME="Debian GNU/Linux 12 (bookworm)"
NAME="Debian GNU/Linux"
VERSION_ID="12"
VERSION="12 (bookworm)"
VERSION_CODENAME=bookworm
ID=debian
HOME_URL="https://www.debian.org/"
SUPPORT_URL="https://www.debian.org/support"
BUG_REPORT_URL="https://bugs.debian.org/"
```


## 相关问题

bufio 中实现的reader和writer，当接口修改后会出现错误，会调用原有write函数的情况。

如下面代码，`clientEvents` 实现了 io.Writer，当调用 setConnect() 断开重连时出现异常。
直接使用io.Copy 或直接调用write不会出现这个问题


出现场景  

1. 启动服务端  
2. 启动客户端   
3. 客户端：发送消息（正常返回）  
4. 关闭服务端  
5. 客户端：发送消息（正常：无法发送）  
6. 启动客户端  
7. 客户端：发送消息（正常：等待连接中，无法发送）
8. 客户端：连接服务端（正常：OnOpen默认发送hello，能看到回执）    
9. 客户端：发送消息 （异常：无法正确发送）


> 注：autoclient 使用通道不会有这个问题  

```go
func readIOStd(ctx context.Context, w io.Writer) {
	defer func() { log.Print("read os stdin closed") }()
	reader := bufio.NewReader(os.Stdin)
	// WARN: cannot use bufio.NewWriter, reconnection will result in call failure
	// writer := bufio.NewWriter(w)
    ....
}

type clientEvents struct {
    ... ...
    mu     sync.Mutex
	conn   gnet.Conn
	status atomic.Int32
}

func (ev *clientEvents) OnOpen(c gnet.Conn) (out []byte, action gnet.Action) {
	c.Write([]byte("hello world"))
	return
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




```
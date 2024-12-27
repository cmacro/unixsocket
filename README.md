# unixsocket
Unix Domain Socket 


example | note
--- | ---
server | 直接使用net实现连接服务
echoserver | 使用gnet库实现连接服务，解决在大量连接时携程过多消耗内存  
client | 直接使用net实现连接  
autoclient | 增加服务端重启或断开自动连接处理。发送消息使用通道，解决大量消息堵塞情况  
clientgnet | 使用gnet库实现连接，包括自动连接处理

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
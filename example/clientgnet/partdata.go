package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"

	"log"

	"github.com/panjf2000/gnet/v2"
)

var (
	ErrBufferOverflow = errors.New("data exceeds 10M limit")
	ErrEmptySendData  = errors.New("empoty send data")
	ErrConnectClsoed  = errors.New("connect closed")
)

type DataDispatch interface {
	GetPartData(conn gnet.Conn) *PartData
	AddPartData(conn gnet.Conn, datalen int) *PartData
	RemovePartData(conn gnet.Conn)
	DispatchData(conn gnet.Conn, msg []byte) error
}

type PartData struct {
	DataLen int
	ReadLen int
	buf     bytes.Buffer
}

func (p *PartData) Put(data []byte) int {
	p.ReadLen += len(data)
	p.buf.Write(data)
	return p.ReadLen
}

func (p *PartData) Data() []byte {
	return p.buf.Bytes()
}

func (p *PartData) Clear() {
	p.DataLen = 0
	p.ReadLen = 0
	p.buf.Reset()
}

func TrafficData(svr DataDispatch, conn gnet.Conn) gnet.Action {
	var (
		dataLen int
		readLen int
		err     error
		part    *PartData
	)

	// 数据格式：uint32(4byte) + Data
	// xx xx xx xx | ......
	// 第一次读取时优先获取内容长度，读取内容不完整时暂存，下次触发时需要继续读取内容。
	part = svr.GetPartData(conn)

	for {
		readLen = 0
		if part == nil {
			if dataLen, err = readDataLen(conn); err != nil {
				if err != io.EOF {
					log.Printf("read conn data length, %v", err)
				}
				return gnet.Close
			}
			if dataLen <= 0 {
				return gnet.None
			}
		} else {
			dataLen = part.DataLen
			readLen = part.ReadLen
		}

		msg, err := conn.Next(dataLen - readLen)
		if err != nil {
			if err == io.ErrShortBuffer {
				// 数据读取不完整，需要等待下次触发读取完整
				// 读取当前剩余buffer内容
				msg, _ = conn.Next(-1)
				if part == nil {
					part = svr.AddPartData(conn, dataLen)
				}
				part.Put(msg)
				return gnet.None
			}
			log.Printf("read conn data, %v", err)
			return gnet.Close
		}

		// When there are data fragments, they must be concatenated before calling the message handler.
		if part == nil {
			if err = svr.DispatchData(conn, msg); err != nil {
				log.Printf("handler traffic data, %v", err)
				return gnet.Close
			}
		} else {
			// exist part message
			if part.Put(msg) == dataLen {
				msg = part.Data()
				if err = svr.DispatchData(conn, msg); err != nil {
					log.Printf("handler traffic data, %v", err)
					return gnet.Close
				}
				part = nil
				svr.RemovePartData(conn)
			}
		}
	}
}
func WritePackData(conn gnet.Conn, data []byte) (int, error) {
	n := len(data)
	if n == 0 {
		return 0, ErrEmptySendData
	}
	if conn == nil {
		return 0, ErrConnectClsoed
	}

	var buf bytes.Buffer
	length := uint32(len(data))
	if err := binary.Write(&buf, binary.BigEndian, length); err != nil {
		return 0, err
	}
	buf.Write(data)
	// buf.WriteByte(0)

	// 确保完整写入
	n, err := conn.Write(buf.Bytes())
	if err != nil {
		return -1, err
	}
	if err := conn.Flush(); err != nil {
		return -1, err
	}
	return n - 4, nil
}

func readDataLen(conn gnet.Conn) (int, error) {
	lenBuf, err := conn.Next(4)
	if err != nil {
		if err == io.ErrShortBuffer {
			return 0, nil
		}
		return 0, err
	}
	dataLen := int(binary.BigEndian.Uint32(lenBuf))
	return dataLen, err
}

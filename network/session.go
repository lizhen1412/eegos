package network

import (
	"github.com/lizhen1412/eegos/log"
	"github.com/lizhen1412/eegos/util"

	"io"
	"runtime/debug"
	"sync"
)

// 互斥锁，用于保护写入操作
var writeLock = &sync.Mutex{}

// 用于生成唯一会话标识符的计数器
var sessionCounter = util.Counter{Num: 0}

// Session 结构体表示一个网络会话
type Session struct {
	fd        uint16                       // 会话的文件描述符
	conn      io.ReadWriteCloser           // 会话的连接
	inData    chan *Data                   // 用于接收输入数据的通道
	outData   chan []byte                  // 用于发送输出数据的通道
	cClose    chan bool                    // 关闭通知通道
	state     int                          // 会话状态
	msgHandle func(uint16, uint16, []byte) // 处理消息的函数
}

// CreateSession 创建一个新的会话
func CreateSession(conn io.ReadWriteCloser, msgHandle func(uint16, uint16, []byte)) *Session {
	// 创建一个新的会话实例
	session := new(Session)
	// 为会话设置唯一的文件描述符编号
	session.fd = sessionCounter.GetNum()
	// 设置会话的连接对象
	session.conn = conn

	// 初始化会话的输入和输出通道，用于处理数据的收发
	session.inData = make(chan *Data, 1)
	session.outData = make(chan []byte, 1)

	// 创建一个用于通知关闭的通道
	session.cClose = make(chan bool)

	// 设置会话的状态为 NEW_CONNECTION，表示新连接
	session.state = NEW_CONNECTION

	// 设置消息处理函数，用于处理接收到的消息
	session.msgHandle = msgHandle

	// 返回创建的会话对象
	return session
}

// Start 用于启动会话的工作。一旦启动，会话将进入 WORKING 状态，
// 并创建独立的 goroutine 来处理读取和写入操作。
func (this *Session) Start() {
	// 将会话状态设置为 WORKING，表示会话正在工作中
	this.state = WORKING

	// 启动一个独立的 goroutine 来处理读取操作
	go this.handleRead()

	// 启动一个独立的 goroutine 来处理写入操作
	go this.handleWrite()

}

// Close 标记会话状态为 CLOSING
func (this *Session) Close() {
	// 将会话状态设置为 CLOSING，表示会话正在关闭中
	this.state = CLOSING
}

// Release 释放会话资源
func (this *Session) Release() {
	//log.Debug("release session")
	// 关闭输入数据通道
	close(this.inData)

	// 关闭输出数据通道
	close(this.outData)

	// 关闭通知关闭的通道
	close(this.cClose)

	// 将会话状态设置为 CLOSED，表示会话已关闭
	this.state = CLOSED
}

// Forward 更改会话的消息处理函数
func (this *Session) Forward(msgHandle func(uint16, uint16, []byte)) {
	// 更新会话的消息处理函数为传入的新函数
	this.msgHandle = msgHandle
}

// pack 将数据打包成特定格式
func (this *Session) pack(head uint16, dType uint8, body []byte) (pkg []byte) {
	// 计算消息体的长度
	length := len(body)

	// 如果消息体长度超过 65535，则警告并将长度限制为 65535
	if length > 65535 {
		log.Warn("package too big, limit 65535!!!!", length)
		length = 0
		body = []byte{}
	}

	// 初始化一个消息包切片，预分配足够的容量以减少内存分配
	pkg = make([]byte, 0, length+5)

	pkg = append(pkg, uint8(length), uint8(length>>8), dType, uint8(head), uint8(head>>8))

	// 向消息包中添加数据类型字段
	pkg = append(pkg, body...)

	// 返回构建好的消息包
	return pkg
}

// Reader 从连接中读取数据并解析成消息
func (this *Session) Reader() (err error) {
	// 创建一个长度为 5 的字节数组 b，用于存储消息包的头部信息
	var b [5]byte

	// 从连接中读取 5 个字节的数据，存储到字节数组 b 中
	if _, err = io.ReadFull(this.conn, b[:]); err != nil {
		return
	}

	// 解析头部信息，获取消息包的长度、数据类型和头部字段
	pkgLen := uint16(b[0]) + uint16(b[1])<<8
	pkg := &Data{}
	pkg.dType = b[2]
	pkg.head = uint16(b[3]) + uint16(b[4])<<8

	// 如果消息包长度大于 0，则创建一个相应长度的字节数组并读取消息体
	if pkgLen > 0 {
		pkg.body = make([]byte, pkgLen)
		if _, err = io.ReadFull(this.conn, pkg.body); err != nil {
			return
		}
	}

	// 将解析得到的消息包发送到会话的输入通道
	this.inData <- pkg

	// 返回读取操作的结果
	return nil
}

// handleRead 处理数据的读取
func (this *Session) handleRead() {
	//log.Debug("handleRead start")

	// 在函数执行完成后关闭连接和通知关闭通道
	defer func() {
		//log.Debug("connection close")
		this.conn.Close()
		this.cClose <- true
		if err := recover(); err != nil {
			log.Error(err, string(debug.Stack()))
		}
	}()

	// 循环读取数据，直到会话状态不再为 WORKING
	for {
		if this.state != WORKING {
			break
		}

		// 调用 Reader 方法读取数据，并处理可能的错误
		if err := this.Reader(); err != nil {
			if err != io.EOF && err != io.ErrUnexpectedEOF {
				log.Error(err)
			}
			break
		}
	}

	//log.Debug("handleRead stop")
}

// handleWrite 处理数据的写入
func (this *Session) handleWrite() {
	//log.Debug("handleWrite start")
	//defer log.Debug("handleWrite stop")

	// 循环写入数据，直到会话状态不再为 WORKING
	for {
		if this.state != WORKING {
			break
		}

		select {
		case pkg, ok := <-this.outData:
			if ok {
				// 将消息包写入连接
				this.conn.Write(pkg)
			}
		}
	}
	// 关闭连接
	this.conn.Close()
}

// doWrite 发送数据给客户端
func (this *Session) doWrite(head uint16, dType uint8, data []byte) {
	// 如果会话状态不再为 WORKING，则记录错误并不发送消息包
	if this.state != WORKING {
		log.Error("session not working state=", this.state, "pkg not send", head, dType)
		return
	}

	// 如果数据为空，则创建一个空的数据切片
	if data == nil {
		data = []byte{}
	}

	// 调用 pack 方法将数据打包成消息包，并将消息包写入输出通道
	pkg := this.pack(head, dType, data)
	this.outData <- pkg
}

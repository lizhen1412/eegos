package network

import (
	"github.com/lizhen1412/eegos/log"
	"github.com/lizhen1412/eegos/util"

	"net"
	"runtime/debug"
	"time"
)

// Handler 定义了RPC网络处理器的接口，包括连接、消息、心跳和关闭事件的处理方法。
type Handler interface {
	Connect(uint16, *Session)
	Message(uint16, uint16, []byte)
	Heartbeat(uint16, uint16)
	Close(uint16)
}

// TcpServer 表示RPC服务器，处理网络连接和消息传递。
type TcpServer struct {
	TcpConn
	addr *net.TCPAddr
}

// NewTcpServer 创建一个新的TCP服务器实例，监听指定地址。
func NewTcpServer(handle Handler, addr string) *TcpServer {
	// 解析TCP地址
	tcpAddr, err := net.ResolveTCPAddr("tcp4", addr)
	if err != nil {
		log.Error("gateserver.Open: net.ResolveTCPAddr: ", err)
		return nil
	}
	// 创建TCP服务器实例
	newServer := &TcpServer{TcpConn{isOpen: true, handle: handle},
		tcpAddr,
	}
	return newServer
}

// Start 启动TCP服务器，监听指定地址并接受客户端连接。
func (this *TcpServer) Start() {
	// 监听指定地址
	lis, err := net.ListenTCP("tcp", this.addr)

	log.Info("gateserver.Open: listening", this.addr)
	if err != nil {
		log.Error("gateserver.Open: net.ListenTCP: ", err)
		return
	}

	// 处理新连接
	func(l *net.TCPListener) {
		//defer log.Debug("listen stop")
		for {
			conn, err := l.Accept()
			if err != nil {
				log.Error("gateserver.Open: lis.Accept: ", err)
				return
			}
			//conn.SetKeepAlive(true)
			//conn.SetKeepAlivePeriod(5 * time.Second)
			go this.handleNewConn(conn)
		}
	}(lis)
}

// handleNewConn 处理新的客户端连接，创建并启动会话。
func (this *TcpServer) handleNewConn(conn net.Conn) {
	// 创建一个新的会话对象，并传入连接对象
	s := this.NewSession(conn)

	// 在函数执行完成后，处理可能的恢复错误，并关闭会话
	defer func() {
		if err := recover(); err != nil {
			log.Error(err, string(debug.Stack()))
			s.Close()
		}
	}()

	// 调用 handle 的 Connect 方法建立客户端会话
	this.handle.Connect(s.fd, s)
	// 启动一个独立的 goroutine 处理会话的输入数据
	go this.processInData(s)
}

// processInData 处理会话的传入数据。
// 该方法会不断处理会话的传入数据流，根据数据类型分发处理。
// 如果接收到心跳消息（HEARTBEAT），会响应并通知处理器进行心跳处理。
// 如果接收到普通数据消息（DATA），会将消息传递给消息处理器进行处理。
// 如果会话关闭，将会触发关闭事件。
func (this *TcpServer) processInData(s *Session) {
	//defer log.Debug("processInData stop")
	// 处理传入数据
	for this.isOpen {
		select {
		case data, ok := <-s.inData:
			if !ok {
				continue
			}
			switch data.dType {
			case HEARTBEAT:
				// 如果会话状态不是工作中，不处理心跳消息
				if s.state != WORKING {
					break
				}
				// 发送心跳响应并通知处理器处理心跳事件
				go s.doWrite(data.head, HEARTBEAT_RET, []byte{})
				go this.handle.Heartbeat(s.fd, data.head)
			case DATA:
				// 将普通数据消息传递给消息处理器进行处理
				go s.msgHandle(s.fd, data.head, data.body)
			}
		case <-s.cClose:
			// 如果会话关闭，触发关闭事件并处理
			this.Close(s)
			return
		}
	}
}

// TcpClient 表示RPC客户端，用于建立与服务器的连接并处理网络通信。
type TcpClient struct {
	TcpConn                  // 嵌入TcpConn以复用网络连接和关闭方法
	msgCounter *util.Counter // 用于生成会话ID的计数器
	cHeartbeat chan uint16   // 用于接收心跳响应的通道
	ticker     *time.Timer   // 用于发送心跳消息的定时器
	session    *Session      // 客户端会话实例
}

// NewTcpClient 创建一个新的TCP客户端实例。
// 参数 handle 是一个实现了Handler接口的对象，用于处理网络连接事件和消息。
// 返回一个新的TcpClient实例，用于建立与服务器的连接和处理通信。
func NewTcpClient(handle Handler) *TcpClient {
	// 创建一个新的TcpClient实例，初始化网络连接、会话ID计数器、心跳响应通道、定时器等属性
	newClient := &TcpClient{TcpConn{isOpen: true, handle: handle},
		&util.Counter{Num: 0},
		make(chan uint16),
		time.NewTimer(5 * time.Second),
		nil}
	return newClient
}

// Dial 建立与指定地址的TCP连接并初始化客户端会话。
// 参数 addr 是服务器的网络地址，如"host:port"。
func (this *TcpClient) Dial(addr string) {
	// 使用net.Dial函数建立TCP连接
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Error("net.Dial: ", err)
		return
	}

	// 创建一个新的会话实例，并将其与连接关联
	s := this.NewSession(conn)
	// 调用处理器的Connect方法，通知连接建立事件
	this.handle.Connect(s.fd, s)
	// 设置客户端的会话实例
	this.session = s
	// 启动处理客户端传入数据和心跳的协程
	go this.processInData()
	go this.heartbeat()
}

// GetSessionID 获取当前客户端会话的唯一标识符（会话ID）。
// 返回一个uint16类型的值，表示当前会话的唯一标识符。
func (this *TcpClient) GetSessionID() uint16 {
	// 调用msgCounter的GetNum方法获取会话ID
	return this.msgCounter.GetNum()
}

// processInData 处理从服务器接收的数据流，包括心跳响应和普通数据。
func (this *TcpClient) processInData() {
	//defer log.Debug("processInData stop")
	// 获取客户端会话实例
	s := this.session
	// 循环处理数据，直到客户端连接关闭
	for this.isOpen {
		select {
		case data, ok := <-s.inData:
			if !ok {
				continue
			}
			switch data.dType {
			// 处理心跳响应消息
			case HEARTBEAT_RET:
				go this.handleHeartbeatRet(s.fd, data.head)
			// 处理普通数据消息
			case DATA:
				go s.msgHandle(s.fd, data.head, data.body)
			}
			// 重置心跳定时器，以保持定时发送心跳消息
			go this.ticker.Reset(5 * time.Second)
		case <-s.cClose:
			// 当客户端连接关闭时，执行关闭操作并返回
			this.Close()
			return
		}
	}
}

// heartbeat 定期发送心跳消息以维持与服务器的连接。
func (this *TcpClient) heartbeat() {
	//defer log.Debug("heartbeat stop")
	// 获取客户端会话实例
	s := this.session
	// 循环发送心跳消息，直到客户端连接关闭
	for this.isOpen {
		// 等待定时器的触发
		<-this.ticker.C
		//log.Debug("heartbeat ticker")
		// 获取当前会话的唯一标识符（会话ID）
		sessionID := this.msgCounter.GetNum()

		// 发送心跳消息到服务器
		go s.doWrite(sessionID, HEARTBEAT, []byte{})

		// 监听心跳响应或超时
		select {
		case <-this.cHeartbeat:
			//log.Debug("heartbeat ret:", sid)
		// 收到心跳响应，继续下一次心跳
		case <-time.After(3 * time.Second):
			// 超时未收到心跳响应，记录警告信息，并重新设置心跳定时器
			log.Warn("heartbeat Timed out", sessionID)
			this.ticker.Reset(5 * time.Second)
		}

	}
}

// handleHeartbeatRet 处理收到的心跳响应消息，并将会话ID发送到心跳响应通道。
// 参数 fd 是会话的唯一标识符，sessionID 是心跳消息中包含的会话ID。
func (this *TcpClient) handleHeartbeatRet(fd uint16, sessionID uint16) {
	// 将会话ID发送到心跳响应通道，以表示成功接收到心跳响应
	this.cHeartbeat <- sessionID
}

// WriteData 向服务器发送自定义数据消息，并返回分配的会话ID。
// 参数 s 是会话实例，buff 是要发送的数据内容。
// 返回一个uint16类型的值，表示分配的会话ID。
func (this *TcpClient) WriteData(s *Session, buff []byte) uint16 {
	// 获取新的会话ID
	sessionID := this.msgCounter.GetNum()
	// 使用会话实例的doWrite方法发送数据消息
	s.doWrite(sessionID, DATA, buff)
	// 返回分配的会话ID
	return sessionID
}

// Close 关闭TCP客户端连接，停止心跳定时器和清理资源。
func (this *TcpClient) Close() {
	//log.Debug("TcpClient Close()")
	// 关闭客户端连接标志
	this.isOpen = false
	// 停止心跳定时器
	this.ticker.Stop()
	// 关闭心跳响应通道
	close(this.cHeartbeat)
	// 关闭与服务器的连接并释放会话资源
	this.TcpConn.Close(this.session)
}

// TcpConn 包含TCP连接相关的通用操作和处理器接口。
type TcpConn struct {
	isOpen bool    // TCP连接状态标志
	handle Handler // 处理器接口，用于处理网络连接事件和消息
}

// NewSession 创建一个新的会话实例，关联到指定的TCP连接。
// 参数 conn 是网络连接实例，msgHandler 是消息处理器接口。
// 返回一个新的会话实例。
func (this *TcpConn) NewSession(conn net.Conn) *Session {
	// 输出新连接的调试信息
	log.Debug("new connection from ", conn.RemoteAddr())
	// 创建并初始化会话实例，设置消息处理器
	session := CreateSession(conn, this.handle.Message)
	session.Start()
	// 返回新的会话实例
	return session
}

// Close 关闭指定会话并释放相关资源。
// 参数 s 是会话的唯一标识符。
func (this *TcpConn) Close(s *Session) {
	//log.Debug("session close")
	// 调用处理器的Close方法，通知会话关闭事件
	this.handle.Close(s.fd)
	// 释放会话资源
	s.Release()
}

// Write 向指定会话发送数据消息。
// 参数 s 是会话实例，sID 是会话的唯一标识符，buff 是要发送的数据内容。
func (this *TcpConn) Write(s *Session, sID uint16, buff []byte) {
	// 使用会话实例的doWrite方法发送数据消息
	s.doWrite(sID, DATA, buff)
}

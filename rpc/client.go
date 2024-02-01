package rpc

import (
	"github.com/lizhen1412/eegos/log"
	"github.com/lizhen1412/eegos/network"

	"encoding/json"
	"sync"
	"time"
)

// Client 表示与远程服务进行通信的客户端
type Client struct {
	callRet   map[uint16](chan []interface{}) // 存储调用结果的映射
	mapLocker *sync.RWMutex                   // 用于保护 callRet 的互斥锁
	tcpClient *network.TcpClient              // TCP 客户端
	session   *network.Session                // 网络会话
}

// NewClient 创建一个新的客户端
func NewClient() *Client {
	// 创建一个新的客户端实例
	newClient := &Client{}

	// 初始化调用结果映射，用于存储调用结果的通道和互斥锁
	newClient.callRet = make(map[uint16](chan []interface{}))
	newClient.mapLocker = &sync.RWMutex{}

	// 创建 TCP 客户端实例，将客户端自身作为参数传递
	newClient.tcpClient = network.NewTcpClient(newClient)

	// 返回新的客户端对象
	return newClient
}

// Dial 连接到远程服务器
func (this *Client) Dial(addr string) {
	// 调用 TCP 客户端的 Dial 方法来与指定地址建立连接
	this.tcpClient.Dial(addr)
}

// Connect 建立客户端会话
func (this *Client) Connect(fd uint16, s *network.Session) {
	// 设置客户端的会话（session）为传入的会话参数 s
	this.session = s
}

// Message 处理从服务器接收到的消息
func (this *Client) Message(fd uint16, sessionID uint16, body []byte) {
	// 使用读取锁来保护对调用结果映射的并发访问
	this.mapLocker.RLock()
	waitRet, ok := this.callRet[sessionID]
	this.mapLocker.RUnlock()

	// 如果未找到与会话ID关联的等待通道，则退出函数
	if !ok {
		return
	}

	// 创建一个空的接口切片用于解码消息体
	args := []interface{}{}

	// 将接收到的消息体解析为接口切片
	err := json.Unmarshal(body, &args)
	if err != nil {
		log.Error(err)
		return
	}

	// 将解析后的消息体发送到等待通道
	waitRet <- args

	// 使用写锁来删除映射中的对应条目，并关闭等待通道
	this.mapLocker.Lock()
	delete(this.callRet, sessionID)
	this.mapLocker.Unlock()
	close(waitRet)
}

// Heartbeat 心跳处理
func (this *Client) Heartbeat(fd uint16, sessionID uint16) {
	// 在这里可以添加心跳逻辑
	log.Debug("Client Heartbeat", fd, sessionID)
}

// Close 关闭客户端
func (this *Client) Close(uint16) {
	// 使用写锁来保护对调用结果映射的并发访问
	this.mapLocker.Lock()

	// 遍历调用结果映射，关闭所有等待通道并清除映射中的所有条目
	for sessionID, waitRet := range this.callRet {
		delete(this.callRet, sessionID)
		close(waitRet)
	}

	// 释放写锁
	this.mapLocker.Unlock()
	//this.tcpClient.Close()
}

// Call 发起远程调用并等待结果
func (this *Client) Call(v []interface{}) []interface{} {

	// 获取一个唯一的会话ID，通常用于标识远程调用
	sessionID := this.tcpClient.GetSessionID()
	//TODO make a channel list pool

	// 创建一个等待通道，用于接收远程调用的结果
	waitRet := make(chan []interface{})

	// 将传入的参数 v 编码为 JSON 格式的消息体
	body, err := json.Marshal(v)
	if err != nil {
		return nil
	}

	//log.Debug("call", sessionID)

	// 使用写锁来保护对调用结果映射的并发访问
	this.mapLocker.Lock()
	this.callRet[sessionID] = waitRet
	this.mapLocker.Unlock()

	//this.outData <- &network.Data{Head: sessionID, Body: body}
	//sessionID := this.tcpClient.WriteData(this.session, body)

	// 使用 TCP 客户端向服务器发送请求消息，包括会话ID和消息体
	this.tcpClient.Write(this.session, sessionID, body)

	// 使用 select 语句监听等待通道和超时条件
	select {
	case ret, ok := <-waitRet:
		// 如果成功从等待通道中接收到结果，则返回结果
		if ok {
			return ret
		}

	case <-time.After(3 * time.Second):
		// 如果超时，则记录超时信息
		log.Debug("Timed out", sessionID)
	}
	// 如果出现服务器问题或超时，返回 nil 或其他适当的错误信息
	log.Debug("some problems on server")
	return nil
}

// Send 发送数据到服务器，无需等待响应
func (this *Client) Send(v []interface{}) {
	//sessionID := this.tcpClient.GetSessionID()
	// 将传入的参数 v 编码为 JSON 格式的消息体
	body, err := json.Marshal(v)
	if err != nil {
		return
	}

	//this.outData <- &network.Data{Head: sessionID, Body: body}
	// 使用 TCP 客户端向服务器发送消息体
	this.tcpClient.WriteData(this.session, body)
	//log.Debug("send", sessionID)
}

// SendCallBack 发送数据到服务器，并在收到响应后执行回调函数
func (this *Client) SendCallBack(v []interface{}, callBack func(callRet interface{})) {
}

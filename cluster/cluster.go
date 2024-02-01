package cluster

import (
	"github.com/lizhen1412/eegos/rpc"

	"strings"
)

// ServerInfo 结构用于保存服务器信息和端口号
type ServerInfo struct {
	server *rpc.Server // rpc.Server 类型的服务器
	port   string      // 端口号
}

var cServer *ServerInfo

// Open 函数用于初始化服务器
func Open(addr string) {
	server := rpc.NewServer(addr) // 创建一个新的rpc服务器
	//server.Open(addr)
	indx := strings.LastIndex(addr, ":")
	cServer = &ServerInfo{server: server, port: addr[indx+1:]} // 初始化 cServer 变量
}

// Register 函数用于注册接收器
func Register(rcvr interface{}) {
	cServer.server.Register(rcvr) // 在服务器上注册接收器
}

// Start 函数用于启动服务器
func Start() {
	cServer.server.Start() // 启动服务器
}

var cClient map[string]*rpc.Client

// Connect 函数用于连接到服务器
func Connect(serverName string, addr string) {
	if cClient == nil {
		cClient = make(map[string]*rpc.Client) // 如果 cClient 为空，创建一个新的map
	}
	if cClient[serverName] == nil {
		client := rpc.NewClient()    // 创建一个新的rpc客户端
		client.Dial(addr)            // 连接到指定地址的服务器
		cClient[serverName] = client // 将客户端存储在 cClient 中
	}
}

// Call 函数用于调用远程服务器的方法
func Call(serverName string, v ...interface{}) []interface{} {
	client := cClient[serverName] // 获取指定服务器的客户端
	if client == nil {
		panic("cannot find server:" + serverName)
	} // 如果找不到客户端，触发 panic，报告无法找到服务器
	return client.Call(v) // 使用客户端向服务器发起调用请求，并返回服务器的响应
}

// Send 函数用于发送数据到远程服务器
func Send(serverName string, v ...interface{}) {
	client := cClient[serverName] // 获取指定服务器的客户端
	if client == nil {
		panic("cannot find server:" + serverName)
	} // 如果找不到客户端，触发 panic，报告无法找到服务器
	// 使用客户端向服务器发送数据
	client.Send(v)
}

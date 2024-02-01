package rpc

import (
	"github.com/lizhen1412/eegos/log"
	"github.com/lizhen1412/eegos/network"

	"encoding/json"
	"reflect"
	"strings"
)

// methodType 表示服务方法的类型，包括方法本身以及其参数类型。
type methodType struct {
	method reflect.Method // 方法的反射信息，包括方法的名称、类型等。
	args   []reflect.Type // 方法的参数类型列表。
}

// Service 表示一个RPC服务，它包含了一个接收器(receiver)对象，该对象拥有RPC方法。
// 服务通常由名称标识，并且可以包含多个注册的RPC方法。
type Service struct {
	name    string                 // 服务的名称，用于标识服务
	rcvr    reflect.Value          // 接收器对象，包含了RPC方法的实现
	typ     reflect.Type           // 接收器对象的类型
	methods map[string]*methodType // 注册的RPC方法，以方法名作为键
}

// Server 表示RPC服务器，用于处理远程过程调用请求。
// 它维护了一个服务映射(serviceMap)，包含了注册的RPC服务。
// 该服务器还包含一个TCP服务器(tcpServer)，用于处理网络连接。
// 以及一个会话映射(sessions)，用于跟踪和管理客户端会话。
type Server struct {
	serviceMap map[string]*Service         // 注册的RPC服务映射，以服务名称作为键
	tcpServer  *network.TcpServer          // TCP服务器，用于处理网络连接
	sessions   map[uint16]*network.Session // 客户端会话映射，以文件描述符(fd)作为键
}

// NewServer 创建一个新的RPC服务器实例。
// 参数 addr 是服务器监听的地址，通常是 IP 地址和端口号，例如 "localhost:8080"。
// 该函数会初始化一个服务映射(serviceMap)，一个TCP服务器(tcpServer)，以及一个会话映射(sessions)。
// 返回一个指向新服务器实例的指针。
func NewServer(addr string) *Server {
	// 创建一个空的服务映射
	services := make(map[string]*Service)
	// 创建一个新的服务器实例并初始化
	newServer := Server{serviceMap: services}
	// 创建一个新的TCP服务器实例并初始化
	newServer.tcpServer = network.NewTcpServer(&newServer, addr)
	// 创建一个空的会话映射
	newServer.sessions = make(map[uint16]*network.Session)
	// 返回服务器实例的指针
	return &newServer
}

// Start 启动服务器。
func (this *Server) Start() {
	// 启动TCP服务器
	this.tcpServer.Start()
}

// Connect 处理新连接。
func (this *Server) Connect(fd uint16, session *network.Session) {
	log.Debug("rpc server new connection", fd)
	// 将新建立的会话 session 与客户端的文件描述符 fd 关联起来
	this.sessions[fd] = session
}

// Message 处理接收到的消息。
func (this *Server) Message(fd uint16, sessionID uint16, body []byte) {
	// 查找与文件描述符对应的会话
	s, ok := this.sessions[fd]
	if !ok {
		log.Error("fd not found", fd, sessionID)
		return
	}

	// 解析接收到的消息内容为参数列表
	args := []interface{}{}
	err := json.Unmarshal(body, &args)
	if err != nil {
		log.Error(err)
		return
	}
	//log.Debug("get message", sessionID, args[0])
	// 检查是否有参数
	if len(args) == 0 {
		log.Error("no args")
		return
	}

	// 从参数中提取服务名和方法名
	info := args[0].(string)

	// 使用字符串函数 LastIndex 来查找字符串 "info" 中最后一个点号的索引位置。
	dot := strings.LastIndex(info, ".")
	if dot < 0 {
		log.Error("cannot find dot")
		return
	}

	// 使用切片操作将字符串 "info" 分割为服务名和方法名
	serviceName := info[:dot]  // 提取点号之前的部分，作为服务名
	methodName := info[dot+1:] // 提取点号之后的部分，作为方法名

	// 查找服务信息
	sInfo := this.serviceMap[serviceName]
	if sInfo == nil {
		log.Error("cannot find service")
		return
	}
	// 查找方法信息
	mInfo := sInfo.methods[methodName]
	if mInfo == nil {
		log.Error("cannot find method", methodName, len(methodName))
		return
	}

	// 准备调用方法的参数
	callArgs := make([]reflect.Value, len(mInfo.args)+1)
	callArgs[0] = sInfo.rcvr

	for i := 0; i < len(mInfo.args); i++ {
		/*
			if reflect.TypeOf(args[i+1]).Kind() != mInfo.args[i].Kind() {
				callArgs[i+1] = reflect.ValueOf(args[i+1]).Convert(mInfo.args[i])
				log.Println("arg type convert")
			} else {
				callArgs[i+1] = reflect.ValueOf(args[i+1])
			}
		*/
		callArgs[i+1] = reflect.ValueOf(args[i+1]).Convert(mInfo.args[i])
	}
	//log.Debug("run func", sessionID, args[0])
	// 调用方法并获取返回值
	ret := this.RunFunc(mInfo, callArgs)
	// 将返回值序列化为JSON并发送给客户端
	if ret != nil {
		retBody, err := json.Marshal(ret)
		if err != nil {
			log.Error(err)
			return
		}
		// 使用 TCP 服务器的 Write 方法将处理后的响应数据 retBody 发送回客户端
		this.tcpServer.Write(s, sessionID, retBody)
	} else {
		// 如果没有返回值，发送空响应
		this.tcpServer.Write(s, sessionID, nil)
	}
	//retPkg := &network.Data{Head: sessionID, Body: retBody}
	//log.Debug("return", sessionID, args[0])
	//this.tcpGate.Write(s, sessionID, retBody)
	//outc <- retPkg
}

// Heartbeat 处理心跳消息。
func (this *Server) Heartbeat(fd uint16, sessionID uint16) {
	log.Debug("server Heartbeat", fd, sessionID)
}

// Close 处理连接关闭。
func (this *Server) Close(fd uint16) {
	log.Debug("need close session", fd)
}

/*
func (this *Server) Open(addr string) {
	//this.tcpGate.Open(addr)
}
*/

// RunFunc 执行注册的RPC方法。
// 参数 m 表示要执行的方法信息，args 表示方法的参数值列表。
// 返回一个包含方法执行结果的切片，每个元素都是方法的返回值。
func (this *Server) RunFunc(m *methodType, args []reflect.Value) []interface{} {
	// 调用方法并获取返回值
	callRet := m.method.Func.Call(args)

	// 获取返回值的数量
	retLen := len(callRet)
	// 如果没有返回值，返回nil
	if retLen == 0 {
		return nil
	}
	// 创建一个包含方法执行结果的切片
	reply := make([]interface{}, retLen)
	for i := 0; i < retLen; i++ {
		// 将返回值转换为接口类型并存储在切片中
		reply[i] = callRet[i].Interface()
	}

	return reply
	//session.HandleWrite(sessionID, reply)
}

// Register 注册一个RPC服务。
// 参数 rcvr 是一个接收器(receiver)对象，该对象包含了实现RPC方法的函数。
// 该方法会为接收器对象创建一个服务实例，并将服务名称、方法信息等注册到服务器。
func (this *Server) Register(rcvr interface{}) {
	// 如果服务映射为空，创建一个新的服务映射
	if this.serviceMap == nil {
		this.serviceMap = make(map[string]*Service)
	}
	// 创建一个新的服务实例
	s := new(Service)
	// 获取接收器对象的类型和值
	s.typ = reflect.TypeOf(rcvr)
	s.rcvr = reflect.ValueOf(rcvr)
	// 从接收器对象中获取服务名称
	s.name = reflect.Indirect(s.rcvr).Type().Name()

	// 初始化服务的方法映射
	s.methods = make(map[string]*methodType)

	// 遍历接收器对象的方法，注册到服务的方法映射中
	for m := 0; m < s.typ.NumMethod(); m++ {
		method := s.typ.Method(m)

		// 获取方法的类型和名称
		mtype := method.Type
		mname := method.Name

		// 创建方法信息结构
		methodInfo := methodType{method: method}
		// 计算方法的参数数量
		argNum := mtype.NumIn() - 1
		if argNum > 0 {
			// 如果方法有参数，初始化参数类型切片
			methodInfo.args = make([]reflect.Type, argNum)
			//log.Println("arg num", argNum)
			// 遍历参数类型，并添加到参数类型切片中
			for a := 0; a < argNum; a++ {
				methodInfo.args[a] = mtype.In(a + 1)
				//log.Println(methodInfo.args[a].Kind())
			}
		}
		// 将方法信息注册到服务的方法映射中
		s.methods[mname] = &methodInfo
		// 打印已注册的方法信息
		log.Debug("registered method: ", mname, len(methodInfo.args))
	}
	// 将服务实例注册到服务映射中，以服务名称作为键
	this.serviceMap[s.name] = s
	// 打印已注册的服务信息
	log.Debug("registered Service: ", s.name)
}

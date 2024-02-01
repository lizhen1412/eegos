package network

// 连接状态常量
const (
	NEW_CONNECTION = iota // 新连接状态
	WORKING               // 工作中状态
	CLOSING               // 关闭中状态
	CLOSED                // 已关闭状态
)

// 数据包类型常量
const (
	PKG_TYPE      = iota // 数据包类型
	HEARTBEAT            // 心跳包类型
	HEARTBEAT_RET        // 心跳包响应类型
	DATA                 // 数据类型
)

// Data 结构体表示一个通用的数据包
type Data struct {
	dType uint8  // 数据包类型
	head  uint16 // 数据包头部
	body  []byte // 数据包主体
}

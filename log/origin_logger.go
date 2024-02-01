package log

import (
	"fmt"
	"io"
	olog "log"
)

// OriginLogger 结构表示一个基本的日志记录器，它实现了 Logger 接口。
type OriginLogger struct {
	w         io.Writer // 用于写入日志消息的 io.Writer
	head      string    // 用于日志消息的头部
	callDepth int       // 日志消息调用深度
}

// NewOriginLogger 创建一个新的 OriginLogger 实例，它将日志消息写入给定的 io.Writer。
func NewOriginLogger(w io.Writer) Logger {
	origin := OriginLogger{}
	origin.w = w
	origin.callDepth = 2 // 默认的日志消息调用深度，通常设置为 2，以跳过日志库的调用。

	return &origin
}

// VLog 实现了 Logger 接口的 VLog 方法，用于记录日志消息。
func (this *OriginLogger) VLog(v ...interface{}) error {
	// 使用 olog.Output 将日志消息写入 io.Writer。
	olog.Output(this.callDepth, this.head+fmt.Sprintln(v...))
	return nil
}

// KVLog 实现了 Logger 接口的 KVLog 方法，与 VLog 方法类似，用于记录日志消息。
func (this *OriginLogger) KVLog(v ...interface{}) error {
	olog.Output(this.callDepth, this.head+fmt.Sprintln(v...))
	return nil
}

// SetDepth 设置日志消息的调用深度，以便正确识别日志消息的来源位置。
func (this OriginLogger) SetDepth(depth int) Logger {
	this.callDepth = depth
	return &this
}

// SetFlags 设置日志库的标志。
func (this *OriginLogger) SetFlags(flag int) {
	olog.SetFlags(flag)
}

// WithHeader 添加一个带有指定头部信息的头部到日志消息。
func (this OriginLogger) WithHeader(keyvals ...interface{}) Logger {
	for i := 0; i < len(keyvals); i += 2 {
		this.head += "["
		this.head += keyvals[i+1].(string)
		this.head += "]: "
	}
	return &this
}

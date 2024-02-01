package log

import (
	"fmt"
	"os"
)

// Logger 是一个日志记录器的接口，定义了日志记录的方法和配置。
type Logger interface {
	// VLog 用于记录日志消息，可以接收可变数量的 keyvals 参数，并返回一个 error 表示记录是否成功。
	VLog(keyvals ...interface{}) error

	// KVLog 类似于 VLog，用于记录日志消息，但与 VLog 不同的是，它不需要明确指定消息级别，而是根据实际情况自动选择。
	KVLog(keyvals ...interface{}) error

	// WithHeader 返回一个新的 Logger，带有指定的头部信息（键值对）。这允许你为日志消息添加额外的元数据。
	WithHeader(keyvals ...interface{}) Logger

	// SetDepth 返回一个新的 Logger，设置日志消息的深度。深度表示从调用 SetDepth 方法的位置开始计算的堆栈深度，
	// 用于确定日志消息的源代码位置。
	SetDepth(depth int) Logger

	// SetFlags 设置日志记录器的标志，以控制日志消息的格式和内容。
	SetFlags(flag int)
}

// defaultLogger 是默认的日志记录器，将日志消息写入标准输出（os.Stdout）。
var defaultLogger = NewOriginLogger(os.Stdout)

// 以下是一些常用的日志标志和日志级别的常量。
const (
	Ldate         = 1 << iota     // the date in the local time zone: 2009/01/23
	Ltime                         // the time in the local time zone: 01:23:23
	Lmicroseconds                 // microsecond resolution: 01:23:23.123123.  assumes Ltime.
	Llongfile                     // full file name and line number: /a/b/c/d.go:23
	Lshortfile                    // final file name element and line number: d.go:23. overrides Llongfile
	LUTC                          // if Ldate or Ltime is set, use UTC rather than the local time zone
	LstdFlags     = Ldate | Ltime // initial values for the standard logger
)

const (
	DEBUG = iota // 调试级别，用于记录详细的调试信息。
	INFO         // 信息级别，用于记录一般的信息消息。
	WARN         // 警告级别，用于记录可能的问题或警告消息。
	ERROR        // 错误级别，用于记录错误和异常消息。
	NOLOG        // 无日志级别，表示不记录任何日志消息。
)

var (
	LEVEL_NAME map[int]string // 日志级别的名称映射
	LOG_LEVEL  int            // 当前日志级别
	OPEN_STACK bool           // 是否开启堆栈信息记录
)

// doLog 用于记录日志消息，根据日志级别和使用 VLog 还是 KVLog。
func doLog(level int, useV bool, v ...interface{}) {
	// 如果当前日志级别比指定的日志级别更高，不记录日志，直接返回。
	if LOG_LEVEL > level {
		return
	}

	// 创建一个带有 "level" 头部信息的新日志记录器模板。
	tmpl := defaultLogger.WithHeader("level", LEVEL_NAME[level])

	// 设置日志的堆栈深度为 4，以便记录日志的代码位置。
	tmpl = tmpl.SetDepth(4)

	// 根据是否使用 VLog 或 KVLog 方法，记录不同类型的日志消息。
	if useV {
		tmpl.VLog(v...)
	} else {
		tmpl.KVLog(v...)
	}
}

// doFormatLog 用于记录格式化的日志消息，根据日志级别。
func doFormatLog(level int, format string, v ...interface{}) {
	// 如果当前日志级别比指定的日志级别更高，不记录日志，直接返回。
	if LOG_LEVEL > level {
		return
	}

	// 创建一个带有 "level" 头部信息的新日志记录器模板。
	tmpl := defaultLogger.WithHeader("level", LEVEL_NAME[level])

	// 设置日志的堆栈深度为 4，以便记录日志的代码位置。
	tmpl = tmpl.SetDepth(4)

	// 使用 fmt.Sprintf 格式化日志消息，并通过 VLog 记录。
	tmpl.VLog(fmt.Sprintf(format, v...))
}

// Log 记录 DEBUG 级别的日志消息，使用 VLog 方法。
func Log(v ...interface{}) {
	doLog(DEBUG, true, v...)
}

// Logf 记录 DEBUG 级别的格式化日志消息。
func Logf(format string, v ...interface{}) {
	doFormatLog(DEBUG, format, v...)
}

// Debug 记录 DEBUG 级别的日志消息，使用 KVLog 方法。
func Debug(v ...interface{}) {
	doLog(DEBUG, false, v...)
}

// Debugf 记录 DEBUG 级别的格式化日志消息。
func Debugf(format string, v ...interface{}) {
	doFormatLog(DEBUG, format, v...)
}

// Info 记录 INFO 级别的日志消息，使用 KVLog 方法。
func Info(v ...interface{}) {
	doLog(INFO, false, v...)
}

// Infof 记录 INFO 级别的格式化日志消息。
func Infof(format string, v ...interface{}) {
	doFormatLog(INFO, format, v...)
}

// Warn 记录 WARN 级别的日志消息，使用 KVLog 方法。
func Warn(v ...interface{}) {
	doLog(WARN, false, v...)
}

// Warnf 记录 WARN 级别的格式化日志消息。
func Warnf(format string, v ...interface{}) {
	doFormatLog(WARN, format, v...)
}

// Error 记录 ERROR 级别的日志消息，使用 KVLog 方法。
func Error(v ...interface{}) {
	doLog(ERROR, false, v...)
}

// Errorf 记录 ERROR 级别的格式化日志消息。
func Errorf(format string, v ...interface{}) {
	doFormatLog(ERROR, format, v...)
}

// WithHeader 返回一个具有指定头部信息的新日志记录器。
func WithHeader(keyvals ...interface{}) Logger {
	return defaultLogger.WithHeader(keyvals...)
}

// SetFlags 设置默认日志记录器的标志。
func SetFlags(flag int) {
	defaultLogger.SetFlags(flag)
}

// OpenStack 开启堆栈信息记录。
func OpenStack() {
	OPEN_STACK = true
}

// SetLevel 设置当前日志级别。
func SetLevel(level int) {
	LOG_LEVEL = level
}

func init() {
	// 初始化默认的日志级别为 0，表示所有日志消息都会被记录。
	LOG_LEVEL = 0
	// 默认情况下，不开启堆栈信息记录。
	OPEN_STACK = false
	// 创建一个映射，用于将整数级别映射到字符串级别名称。
	LEVEL_NAME = make(map[int]string)
	// 将 "NOLOG" 级别映射到字符串 "NOLOG"，以便在日志消息中表示没有日志记录。
	LEVEL_NAME[NOLOG] = "NOLOG"
	// 将其他日志级别分别映射到相应的字符串级别名称，以便在日志消息中标识不同的日志级别。
	LEVEL_NAME[DEBUG] = "DEBUG"
	// 将 INFO 级别映射到字符串 "INFO"，以便在日志消息中标识 INFO 日志级别。
	LEVEL_NAME[INFO] = "INFO"
	// 将 WARN 级别映射到字符串 "WARN"，以便在日志消息中标识 WARN 日志级别。
	LEVEL_NAME[WARN] = "WARN"
	// 将 ERROR 级别映射到字符串 "ERROR"，以便在日志消息中标识 ERROR 日志级别。
	LEVEL_NAME[ERROR] = "ERROR"
}

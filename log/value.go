package log

import (
	"runtime"
	"strconv"
	"strings"
	"time"
)

// A Valuer generates a log value. When passed to With or WithPrefix in a
// value element (odd indexes), it represents a dynamic value which is re-
// evaluated with each log event.
// Valuer 表示生成日志值的函数类型。当作为值元素（奇数索引）传递给 With 或 WithPrefix 时，
// 它代表一个动态值，每次日志事件都会重新评估。
type Valuer func() interface{}

// bindValues replaces all value elements (odd indexes) containing a Valuer
// with their generated value.
// bindValues 用生成的值替换所有包含 Valuer 的值元素（奇数索引）。
func bindValues(keyvals []interface{}) {
	for i := 1; i < len(keyvals); i += 2 {
		if v, ok := keyvals[i].(Valuer); ok {
			keyvals[i] = v()
		}
	}
}

// containsValuer returns true if any of the value elements (odd indexes)
// contain a Valuer.
// containsValuer 如果任何值元素（奇数索引）包含 Valuer，则返回 true。
func containsValuer(keyvals []interface{}) bool {
	for i := 1; i < len(keyvals); i += 2 {
		if _, ok := keyvals[i].(Valuer); ok {
			return true
		}
	}
	return false
}

// Timestamp returns a timestamp Valuer. It invokes the t function to get the
// time; unless you are doing something tricky, pass time.Now.
//
// Most users will want to use DefaultTimestamp or DefaultTimestampUTC, which
// are TimestampFormats that use the RFC3339Nano format.
// Timestamp 返回一个时间戳 Valuer。它调用 t 函数以获取时间；除非您在做一些复杂的事情，否则传递 time.Now。
//
// 大多数用户将希望使用 DefaultTimestamp 或 DefaultTimestampUTC，它们是使用 RFC3339Nano 格式的 TimestampFormats。
func Timestamp(t func() time.Time) Valuer {
	return func() interface{} { return t() }
}

// TimestampFormat returns a timestamp Valuer with a custom time format. It
// invokes the t function to get the time to format; unless you are doing
// something tricky, pass time.Now. The layout string is passed to
// Time.Format.
//
// Most users will want to use DefaultTimestamp or DefaultTimestampUTC, which
// are TimestampFormats that use the RFC3339Nano format.
// TimestampFormat 返回具有自定义时间格式的时间戳 Valuer。它调用 t 函数以获取要格式化的时间；除非您在做一些复杂的事情，否则传递 time.Now。布局字符串传递给 Time.Format。
//
// 大多数用户将希望使用 DefaultTimestamp 或 DefaultTimestampUTC，它们是使用 RFC3339Nano 格式的 TimestampFormats。
func TimestampFormat(t func() time.Time, layout string) Valuer {
	return func() interface{} {
		return timeFormat{
			time:   t(),
			layout: layout,
		}
	}
}

// A timeFormat represents an instant in time and a layout used when
// marshaling to a text format.
// timeFormat 表示时间的瞬间和在文本格式化时使用的布局。
type timeFormat struct {
	time   time.Time
	layout string
}

// String 方法实现了 fmt.Stringer 接口，它返回格式化的时间字符串，
// 格式化方式由 timeFormat 结构体的 layout 字段指定。
func (tf timeFormat) String() string {
	return tf.time.Format(tf.layout)
}

// MarshalText implements encoding.TextMarshaller.
// MarshalText 实现了 encoding.TextMarshaller 接口。
func (tf timeFormat) MarshalText() (text []byte, err error) {
	// The following code adapted from the standard library time.Time.Format
	// method. Using the same undocumented magic constant to extend the size
	// of the buffer as seen there.
	b := make([]byte, 0, len(tf.layout)+10)
	b = tf.time.AppendFormat(b, tf.layout)
	return b, nil
}

// Caller returns a Valuer that returns a file and line from a specified depth
// in the callstack. Users will probably want to use DefaultCaller.
// Caller 返回一个 Valuer，它返回指定深度的调用堆栈中的文件和行号。用户可能希望使用 DefaultCaller。
func Caller(depth int) Valuer {
	return func() interface{} {
		_, file, line, _ := runtime.Caller(depth)
		idx := strings.LastIndexByte(file, '/')
		// using idx+1 below handles both of following cases:
		// idx == -1 because no "/" was found, or
		// idx >= 0 and we want to start at the character after the found "/".
		return file[idx+1:] + ":" + strconv.Itoa(line)
	}
}

var (
	// DefaultTimestamp is a Valuer that returns the current wallclock time,
	// respecting time zones, when bound.
	// DefaultTimestamp 是一个 Valuer，当绑定时返回当前墙钟时间，尊重时区。
	DefaultTimestamp = TimestampFormat(time.Now, time.RFC3339Nano)

	// DefaultTimestampUTC is a Valuer that returns the current time in UTC
	// when bound.
	// DefaultTimestampUTC 是一个 Valuer，在绑定时返回当前 UTC 时间。
	DefaultTimestampUTC = TimestampFormat(
		func() time.Time { return time.Now().UTC() },
		time.RFC3339Nano,
	)

	// DefaultCaller is a Valuer that returns the file and line where the Log
	// method was invoked. It can only be used with log.With.
	// DefaultCaller 是一个 Valuer，返回调用 Log 方法的文件和行号。它只能与 log.With 一起使用。
	DefaultCaller = Caller(3)
)

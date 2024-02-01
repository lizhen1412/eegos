package log

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// logfmtEncoder 用于编码 logfmt 格式的日志。
type logfmtEncoder struct {
	*Encoder
	buf bytes.Buffer
}

// Reset 重置 logfmtEncoder 的状态，用于复用。
// Reset 是 logfmtEncoder 结构体的一个方法。
// 这个方法没有返回值，并且接收一个指向 logfmtEncoder 实例的指针作为接收者。
func (l *logfmtEncoder) Reset() {

	// 调用 Encoder 结构体的 Reset 方法。
	// 这个操作通常用于清除 Encoder 中的所有状态和缓存的数据，使其恢复到初始状态。
	// 这里的 Encoder 是 logfmtEncoder 结构体中的一个字段，用于执行实际的编码操作。
	l.Encoder.Reset()

	// 调用 buf 的 Reset 方法。
	// buf 是 logfmtEncoder 结构体中的一个字段，通常是一个 bytes.Buffer 类型，用于存储编码后的数据。
	// Reset 方法清除 buf 中的所有数据，使其恢复到空状态。
	l.buf.Reset()
}

// logfmtEncoderPool 是一个池，用于复用 logfmtEncoder。
// 定义一个名为 logfmtEncoderPool 的 sync.Pool 对象。
// sync.Pool 是一个泛型对象池，它可以用来存储任何类型的对象。
var logfmtEncoderPool = sync.Pool{

	// New 是一个函数，当从 Pool 获取对象时，如果 Pool 中没有可用对象，就会调用这个函数来创建一个新对象。
	New: func() interface{} {

		// 定义一个 logfmtEncoder 结构体变量。
		// 这个结构体通常包含用于 logfmt 编码的字段和方法。
		var enc logfmtEncoder

		// 初始化 enc 的 Encoder 字段。
		// NewEncoder 是一个函数，它创建并返回一个新的 Encoder 实例。
		// &enc.buf 是一个指向 enc 中 buf 字段的指针，这个字段通常是一个 bytes.Buffer 类型，用于存储编码后的数据。
		enc.Encoder = NewEncoder(&enc.buf)

		// 返回 enc 的地址。
		// 在 Go 中，返回对象的地址意味着返回一个指向该对象的指针。
		// 这样做可以避免复制整个结构体，从而提高效率。
		return &enc
	},
}

// InfluxdbLogger 是一个实现了 Logger 接口的类型，用于将日志编码为 InfluxDB Line Protocol 格式。
type InfluxdbLogger struct {
	mu          sync.Mutex   // ensures atomic writes; protects the following fields 保证原子写入；保护以下字段
	w           io.Writer    // 输出日志的 Writer
	measurement string       // InfluxDB 中的测量名称
	tagSet      bytes.Buffer // 标签集
	buf         bytes.Buffer // 日志消息缓冲区
	callDepth   int          // 调用深度
	flag        int          // 标志位
}

// NewLogfmtLogger returns a logger that encodes keyvals to the Writer in
// logfmt format. Each log event produces no more than one call to w.Write.
// The passed Writer must be safe for concurrent use by multiple goroutines if
// the returned Logger will be used concurrently.
/*
func NewInfluxdbLogger(w io.Writer) Logger {
	influxdb := InfluxdbLogger{}
	influxdb.w = w
	arg := os.Args[0]
	idx := strings.LastIndex(arg, "/")
	influxdb.measurement = arg[idx+1:]

	return &influxdb
}
*/

// NewInfluxdbLogger 返回一个将 keyvals 编码为 InfluxDB Line Protocol 格式并写入 w 的 Logger。
// NewInfluxdbLogger 是一个函数，用于创建一个新的 InfluxDB 日志记录器。
// 它接受一个 io.Writer 接口用于写入日志，一个字符串作为测量名（measurement），以及一个可变长的键值对字符串数组。
func NewInfluxdbLogger(w io.Writer, measurement string, kv ...string) Logger {

	// 初始化一个 InfluxdbLogger 结构体。
	influxdb := InfluxdbLogger{}

	// 设置用于写入日志的 Writer。
	influxdb.w = w

	// 设置调用深度，默认值为 2。
	influxdb.callDepth = 2

	// 设置测量名称（measurement）。
	// 如果传入的 measurement 字符串为空，那么使用程序的名称作为测量名称。
	if measurement == "" {
		arg := os.Args[0]                  // 获取程序的完整路径和名称。
		idx := strings.LastIndex(arg, "/") // 找到路径中最后一个斜线("/")的位置。
		influxdb.measurement = arg[idx+1:] // 从最后一个斜线之后的部分作为测量名称。
	} else {
		influxdb.measurement = measurement // 如果提供了测量名称，直接使用该名称。
	}

	// 设置标签集。
	// 如果 kv 参数有值，则解析这些键值对，并将它们作为标签添加到日志记录器中。
	if len(kv) > 0 {
		influxdb.tagSet.WriteString(",") // 在标签集的开始添加一个逗号。
		for i := 0; i < len(kv); i += 2 {
			key, value := kv[i], kv[i+1]       // 获取键值对。
			influxdb.tagSet.WriteString(key)   // 添加键。
			influxdb.tagSet.WriteString("=")   // 添加等号。
			influxdb.tagSet.WriteString(value) // 添加值。
			if i < len(kv)-2 {
				influxdb.tagSet.WriteString(",") // 在键值对之间添加逗号。
			}
		}
	}

	// 将此 influxdb 实例设置为默认的日志记录器。
	defaultLogger = &influxdb

	// 返回这个新创建的 InfluxdbLogger 实例。
	return &influxdb
}

// VLog 将日志按照 InfluxDB Line Protocol 格式写入 w。
// VLog 方法用于记录具有可变参数的日志消息。它将一组任意类型的参数进行格式化，并将结果添加到日志记录中。
// 参数 v 是一个可变参数列表，您可以传递任意数量的参数给它，这些参数将在日志消息中被格式化和记录。
// 日志消息的格式为：measurement key1=value1 key2=value2 ... msg="formatted message" caller="file:line" timestamp
// 其中，measurement 是 InfluxDB 中的测量名称，key1、key2、... 是日志消息的键值对，msg 包含格式化后的消息，
// caller 显示调用日志记录方法的文件和行号，timestamp 显示时间戳。
// 该方法会将格式化后的日志消息写入到 InfluxDB 日志记录器的输出流中，并返回可能出现的错误。
func (l *InfluxdbLogger) VLog(v ...interface{}) error {
	// 获取调用位置信息
	caller := l.getCaller()

	// 使用互斥锁保护缓冲区写入，以防止多个 goroutine 同时写入日志
	l.mu.Lock()
	defer l.mu.Unlock()

	// 重置缓冲区，准备构建新的日志消息
	l.buf.Reset()

	// 将测量名称写入日志消息
	l.buf.WriteString(l.measurement)

	// 写入标签集合（如果有的话）
	l.buf.Write(l.tagSet.Bytes())

	// 写入消息字段键
	l.buf.Write([]byte(" msg="))

	// 将消息内容格式化并写入到消息字段值中
	l.buf.WriteString("\"")
	for i := 0; i < len(v); i++ {
		value := v[i]
		l.buf.WriteString(fmt.Sprint(value))
		if i < len(v)-1 {
			l.buf.WriteString(" ")
		}
	}
	l.buf.WriteString("\"")

	// 写入逗号和调用者信息
	l.buf.WriteString(",")
	l.buf.WriteString(caller)
	l.buf.WriteString(" ")

	// 写入时间戳
	l.buf.WriteString(fmt.Sprint(time.Now().UnixNano()))

	// 写入换行符表示一条日志记录的结束
	l.buf.WriteString("\n")

	// 将日志消息写入到日志记录器的输出流中
	if _, err := l.w.Write(l.buf.Bytes()); err != nil {
		return err
	}

	// 返回可能的错误（如果有）
	return nil
}

// KVLog 将键值对编码为 InfluxDB Line Protocol 格式写入 w。
func (l *InfluxdbLogger) KVLog(keyvals ...interface{}) error {
	enc := logfmtEncoderPool.Get().(*logfmtEncoder)
	enc.Reset()
	defer logfmtEncoderPool.Put(enc)
	enc.buf.WriteString(l.measurement)
	//enc.buf.WriteString(",")
	enc.buf.Write(l.tagSet.Bytes())
	enc.buf.Write([]byte(" "))

	if err := enc.EncodeKeyvalsWithQuoted(keyvals...); err != nil {
		return err
	}
	enc.buf.WriteString(",")
	enc.buf.WriteString(l.getCaller())
	enc.buf.WriteString(" ")

	//enc.buf.Write([]byte(" "))
	// 为了保证时间戳的唯一性，这里使用了纳秒级时间戳
	enc.buf.WriteString(fmt.Sprint(time.Now().UnixNano()))

	// Add newline to the end of the buffer
	// 在缓冲区末尾添加换行符
	if err := enc.EndRecord(); err != nil {
		return err
	}

	// The Logger interface requires implementations to be safe for concurrent
	// use by multiple goroutines. For this implementation that means making
	// only one call to l.w.Write() for each call to Log.
	// Logger 接口要求实现必须对多个 goroutine 并发使用是安全的。
	// 对于这个实现，这意味着对 Log 的每次调用只能调用一次 l.w.Write()。
	// 由于 l.w.Write() 是原子的，所以这个实现是安全的。
	if _, err := l.w.Write(enc.buf.Bytes()); err != nil {
		return err
	}
	return nil
}

// SetDepth 设置调用深度。
// SetDepth 用于设置当前 InfluxdbLogger 实例的调用深度。
// 调用深度表示在日志记录中标识调用位置的深度。例如，深度为 2 表示在日志记录中将显示调用 `SetDepth` 方法的调用者的位置。
// 这对于定位日志记录的来源非常有用，可以将其用于标识哪个函数或方法生成了日志记录。
// 通过设置不同的调用深度，您可以控制从哪个位置生成日志记录，以适应不同的应用程序需求。
// 注意：深度设置得太浅可能会显示不相关的调用位置，深度设置得太深可能会隐藏有用的信息，因此需要谨慎选择深度值。
// 返回一个新的 InfluxdbLogger 实例，以确保调用深度的更改不会影响到原始实例。
func (l InfluxdbLogger) SetDepth(depth int) Logger {
	// 设置新的调用深度并返回一个新的 InfluxdbLogger 实例。
	l.callDepth = depth
	return &l
}

// SetFlags 设置标志位。
// SetFlags 用于设置当前 InfluxdbLogger 实例的标志属性。
// 标志属性控制日志记录的格式和输出方式。可以使用位掩码来组合多个标志属性。
// 可用的标志属性包括：
//   - Ldate: 在日志中包含日期，格式为 "2009/01/23"。
//   - Ltime: 在日志中包含时间，格式为 "01:23:23"。
//   - Lmicroseconds: 在日志中包含微秒级时间戳，格式为 "01:23:23.123123"（需要 Ltime 属性）。
//   - Llongfile: 在日志中包含完整文件名和行号，格式为 "/a/b/c/d.go:23"。
//   - Lshortfile: 在日志中包含文件名和行号，格式为 "d.go:23"（会覆盖 Llongfile）。
//   - LUTC: 如果设置了 Ldate 或 Ltime 标志，将使用UTC时间而不是本地时间。
//   - LstdFlags: 默认的标志属性，包括 Ldate 和 Ltime。
//
// 设置标志属性后，新的日志记录将按照指定的格式输出。
func (l *InfluxdbLogger) SetFlags(flag int) {
	// 将传入的标志属性设置为当前 InfluxdbLogger 实例的标志属性。
	l.flag = flag
}

/*
func (l *InfluxdbLogger) WithHeader(keyvals ...interface{}) error {
	enc := logfmtEncoderPool.Get().(*logfmtEncoder)
	enc.Reset()
	defer logfmtEncoderPool.Put(enc)

	if l.tagFlag {
		l.tagSet.Write([]byte(","))
	}

	if err := enc.EncodeKeyvals(keyvals...); err != nil {
		return err
	}
	l.tagSet.Write(enc.buf.Bytes())
	l.tagFlag = true
	return nil
}
*/

// WithHeader 返回带有新标签的 Logger。
// WithHeader 为当前的 InfluxdbLogger 实例添加自定义标头信息。
// 该方法接受一系列 keyvals 参数，这些参数是成对出现的键值对，用于自定义标头。
// 例如：WithHeader("key1", "value1", "key2", "value2")。
// 每个键值对将被编码成字符串，并附加到已存在的标头信息之后。
// 返回一个新的 Logger 实例，该实例包含了添加的标头信息，原始 Logger 实例不受影响。
func (l InfluxdbLogger) WithHeader(keyvals ...interface{}) Logger {
	// 获取一个 logfmtEncoder 实例，用于编码键值对成为字符串。
	enc := logfmtEncoderPool.Get().(*logfmtEncoder)
	enc.Reset()
	defer logfmtEncoderPool.Put(enc)

	// 使用 logfmtEncoder 编码传入的键值对，生成标头信息的字符串。
	if err := enc.EncodeKeyvals(keyvals...); err != nil {
		fmt.Println(err)
		return nil
	}

	// 将编码后的标头信息追加到已有的标头信息之后。
	l.tagSet.Write([]byte(","))
	l.tagSet.Write(enc.buf.Bytes())

	// 返回一个新的 Logger 实例，该实例包含了添加的标头信息。
	// 原始 Logger 实例不受影响。
	return &l
}

/*
func (l *InfluxdbLogger) SetHeader(head string) {
	l.header.Reset()
	l.header.WriteString(head)
}
*/

// getCaller 获取调用者的信息。
// getCaller 获取当前日志记录的调用者信息。
func (l *InfluxdbLogger) getCaller() string {
	// 使用 runtime.Caller 获取调用栈信息，包括文件名、行号等。
	_, file, line, ok := runtime.Caller(l.callDepth)
	if !ok {
		// 如果无法获取调用栈信息，将文件名标记为 "???"，行号标记为 0。
		file = "???"
		line = 0
	}

	// 如果标志位中包含 Lshortfile，表示只保留文件名的最后一部分（不包括路径）。
	if l.flag&Lshortfile != 0 {
		short := file
		// 从文件路径的末尾向前搜索，找到最后一个斜杠 "/"，然后取其后的部分作为文件名。
		for i := len(file) - 1; i > 0; i-- {
			if file[i] == '/' {
				short = file[i+1:]
				break
			}
		}
		file = short
	}

	// 返回格式化后的调用者信息，包括文件名和行号。
	return "caller=\"" + file + ":" + fmt.Sprint(line) + "\""
}

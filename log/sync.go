package log

import (
	"io"
	"sync"
	"sync/atomic"
)

// SwapLogger wraps another logger that may be safely replaced while other
// goroutines use the SwapLogger concurrently. The zero value for a SwapLogger
// will discard all log events without error.
//
// SwapLogger serves well as a package global logger that can be changed by
// importers.
// SwapLogger 包装了另一个 logger，可以在其他 goroutine 使用 SwapLogger 时安全替换。
// SwapLogger 的零值将丢弃所有的日志事件而不返回错误。
//
// SwapLogger 适用于作为一个包全局的 logger，可以被导入者更改。
type SwapLogger struct {
	logger atomic.Value
}

type loggerStruct struct {
	Logger
}

// Log implements the Logger interface by forwarding keyvals to the currently
// wrapped logger. It does not log anything if the wrapped logger is nil.
// Log 通过将 keyvals 转发到当前包装的 logger 来实现 Logger 接口。
// 如果包装的 logger 为 nil，则不记录任何内容。
func (l *SwapLogger) Log(keyvals ...interface{}) error {
	s, ok := l.logger.Load().(loggerStruct)
	if !ok || s.Logger == nil {
		return nil
	}
	return s.VLog(keyvals...)
}

// Swap replaces the currently wrapped logger with logger. Swap may be called
// concurrently with calls to Log from other goroutines.
// Swap 用 logger 替换当前包装的 logger。Swap 可以与其他 goroutine 中的 Log 调用并发调用。
func (l *SwapLogger) Swap(logger Logger) {
	l.logger.Store(loggerStruct{logger})
}

// NewSyncWriter returns a new writer that is safe for concurrent use by
// multiple goroutines. Writes to the returned writer are passed on to w. If
// another write is already in progress, the calling goroutine blocks until
// the writer is available.
//
// If w implements the following interface, so does the returned writer.
//
//	interface {
//	    Fd() uintptr
//	}
//
// NewSyncWriter 返回一个新的 writer，可安全地由多个 goroutine 并发使用。
// 写入到返回的 writer 会传递到 w。如果另一个写入操作已经在进行中，
// 则调用 goroutine 将阻塞，直到 writer 可用。
//
// 如果 w 实现以下接口，则返回的 writer 也会实现它：
//
//	interface {
//	    Fd() uintptr
//	}
func NewSyncWriter(w io.Writer) io.Writer {
	switch w := w.(type) {
	case fdWriter:
		return &fdSyncWriter{fdWriter: w}
	default:
		return &syncWriter{Writer: w}
	}
}

// syncWriter synchronizes concurrent writes to an io.Writer.
// syncWriter 同步并发写入到 io.Writer。
type syncWriter struct {
	sync.Mutex
	io.Writer
}

// Write writes p to the underlying io.Writer. If another write is already in
// progress, the calling goroutine blocks until the syncWriter is available.
// Write 将 p 写入到底层的 io.Writer。如果另一个写入操作已经在进行中，
// 调用 goroutine 将阻塞，直到 syncWriter 可用。
func (w *syncWriter) Write(p []byte) (n int, err error) {
	w.Lock()
	n, err = w.Writer.Write(p)
	w.Unlock()
	return n, err
}

// fdWriter is an io.Writer that also has an Fd method. The most common
// example of an fdWriter is an *os.File.
// fdWriter 是一个同时具有 Fd 方法的 io.Writer。最常见的 fdWriter 示例是 *os.File。
type fdWriter interface {
	io.Writer
	Fd() uintptr
}

// fdSyncWriter synchronizes concurrent writes to an fdWriter.
// fdSyncWriter 同步并发写入到 fdWriter。
type fdSyncWriter struct {
	sync.Mutex
	fdWriter
}

// Write writes p to the underlying io.Writer. If another write is already in
// progress, the calling goroutine blocks until the fdSyncWriter is available.
// Write 将 p 写入到底层的 io.Writer。如果另一个写入操作已经在进行中，
// 调用 goroutine 将阻塞，直到 fdSyncWriter 可用。
func (w *fdSyncWriter) Write(p []byte) (n int, err error) {
	w.Lock()
	n, err = w.fdWriter.Write(p)
	w.Unlock()
	return n, err
}

// syncLogger provides concurrent safe logging for another Logger.
/*
type syncLogger struct {
	mu     sync.Mutex
	logger Logger
}

// NewSyncLogger returns a logger that synchronizes concurrent use of the
// wrapped logger. When multiple goroutines use the SyncLogger concurrently
// only one goroutine will be allowed to log to the wrapped logger at a time.
// The other goroutines will block until the logger is available.
func NewSyncLogger(logger Logger) Logger {
	return &syncLogger{logger: logger}
}

// Log logs keyvals to the underlying Logger. If another log is already in
// progress, the calling goroutine blocks until the syncLogger is available.
func (l *syncLogger) Log(keyvals ...interface{}) error {
	l.mu.Lock()
	err := l.logger.Log(keyvals...)
	l.mu.Unlock()
	return err
}
*/

package log

import (
	"bytes"
	"io"
	"strconv"
	"sync"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// Taken from Go's encoding/json and modified for use here.

// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// 从Go的encoding/json中提取并修改的代码。
var hex = "0123456789abcdef"

// bufferPool 是一个池，用于复用字节缓冲区。
var bufferPool = sync.Pool{
	New: func() interface{} {
		return &bytes.Buffer{}
	},
}

// getBuffer 返回一个从池中获取的字节缓冲区。
func getBuffer() *bytes.Buffer {
	return bufferPool.Get().(*bytes.Buffer)
}

// poolBuffer 将字节缓冲区重置并放回池中。
func poolBuffer(buf *bytes.Buffer) {
	// 重置字节缓冲区，清空其中的数据。
	buf.Reset()

	// 将重置后的字节缓冲区放回缓冲池，以便后续重复使用。
	bufferPool.Put(buf)
}

// NOTE: keep in sync with writeQuotedBytes below.
// writeQuotedString 将字符串 s 写入 w，并在字符串周围添加引号和进行转义。
// 这个函数用于将字符串写入 JSON 格式的字符串中。
func writeQuotedString(w io.Writer, s string) (int, error) {
	// 获取一个字节缓冲区，用于构建带引号的字符串。
	buf := getBuffer()

	// 在缓冲区中添加起始双引号。
	buf.WriteByte('"')

	// 初始化起始位置。
	start := 0

	// 遍历字符串的每个字符。
	for i := 0; i < len(s); {

		// 获取当前字符。
		if b := s[i]; b < utf8.RuneSelf {

			// 如果字符是ASCII字符（小于utf8.RuneSelf），并且不是空格、反斜杠或双引号，
			// 则直接添加到缓冲区。
			if 0x20 <= b && b != '\\' && b != '"' {
				i++
				continue
			}

			// 如果有需要，将之前的字符写入缓冲区。
			if start < i {
				buf.WriteString(s[start:i])
			}

			// 根据字符类型进行转义处理。
			switch b {
			case '\\', '"':
				// 对反斜杠和双引号进行转义。
				buf.WriteByte('\\')
				buf.WriteByte(b)
			case '\n':
				// 对换行符进行转义。
				buf.WriteByte('\\')
				buf.WriteByte('n')
			case '\r':
				// 对回车符进行转义。
				buf.WriteByte('\\')
				buf.WriteByte('r')
			case '\t':
				// 对制表符进行转义。
				buf.WriteByte('\\')
				buf.WriteByte('t')
			default:
				// This encodes bytes < 0x20 except for \n, \r, and \t.
				// 这会对小于 0x20 的字节进行编码，但排除了 \n、\r 和 \t 以外的情况。
				buf.WriteString(`\u00`)
				buf.WriteByte(hex[b>>4])
				buf.WriteByte(hex[b&0xF])
			}
			i++
			start = i
			continue
		}

		// 如果字符不是ASCII字符，使用 utf8.DecodeRuneInString 解码字符。
		c, size := utf8.DecodeRuneInString(s[i:])
		// 如果解码结果是 utf8.RuneError，表示不合法的 UTF-8 字符，进行转义处理。
		if c == utf8.RuneError {
			// 如果有需要，将之前的字符写入缓冲区。
			if start < i {
				buf.WriteString(s[start:i])
			}
			// 对不合法的 UTF-8 字符进行转义。
			buf.WriteString(`\ufffd`)
			// 更新索引位置。
			i += size
			start = i
			continue
		}
		// 更新索引位置。
		i += size
	}
	// 如果还有剩余的字符，将其写入缓冲区。
	if start < len(s) {
		buf.WriteString(s[start:])
	}

	// 在缓冲区中添加结束双引号。
	buf.WriteByte('"')

	// 将构建好的带引号的字符串写入到指定的 io.Writer 中。
	n, err := w.Write(buf.Bytes())

	// 将字节缓冲区归还给缓冲池。
	poolBuffer(buf)

	// 返回写入的字节数和可能的错误。
	return n, err
}

// NOTE: keep in sync with writeQuoteString above.
// writeQuotedBytes 将字节切片 s 写入 w，并在字节切片周围添加引号和进行转义。
// 这个函数用于将字节切片写入 JSON 格式的字符串中。
func writeQuotedBytes(w io.Writer, s []byte) (int, error) {
	// 获取一个字节缓冲区，用于构建带引号的字符串。
	buf := getBuffer()

	// 在缓冲区中添加起始双引号。
	buf.WriteByte('"')

	// 初始化起始位置。
	start := 0

	// 遍历字节切片的每个字节。
	for i := 0; i < len(s); {
		// 获取当前字节。
		if b := s[i]; b < utf8.RuneSelf {
			// 如果字节是ASCII字符（小于utf8.RuneSelf），并且不是空格、反斜杠或双引号，
			// 则直接添加到缓冲区。
			if 0x20 <= b && b != '\\' && b != '"' {
				i++
				continue
			}

			// 如果有需要，将之前的字节写入缓冲区。
			if start < i {
				buf.Write(s[start:i])
			}

			// 根据字节类型进行转义处理。
			switch b {
			case '\\', '"':
				// 对反斜杠和双引号进行转义。
				buf.WriteByte('\\')
				buf.WriteByte(b)
			case '\n':
				// 对换行符进行转义。
				buf.WriteByte('\\')
				buf.WriteByte('n')
			case '\r':
				// 对回车符进行转义。
				buf.WriteByte('\\')
				buf.WriteByte('r')
			case '\t':
				// 对制表符进行转义。
				buf.WriteByte('\\')
				buf.WriteByte('t')
			default:
				// This encodes bytes < 0x20 except for \n, \r, and \t.
				// 这会对小于 0x20 的字节进行编码，但排除了 \n、\r 和 \t 以外的情况。
				buf.WriteString(`\u00`)
				buf.WriteByte(hex[b>>4])
				buf.WriteByte(hex[b&0xF])
			}
			i++
			start = i
			continue
		}
		// 如果字节不是ASCII字符，使用 utf8.DecodeRune 解码字节。
		c, size := utf8.DecodeRune(s[i:])

		// 如果解码结果是 utf8.RuneError，表示不合法的 UTF-8 字节，进行转义处理。
		if c == utf8.RuneError {
			// 如果有需要，将之前的字节写入缓冲区。
			if start < i {
				buf.Write(s[start:i])
			}
			// 对不合法的 UTF-8 字节进行转义。
			buf.WriteString(`\ufffd`)

			// 更新索引位置。
			i += size
			start = i
			continue
		}
		// 更新索引位置。
		i += size
	}

	// 如果还有剩余的字节，将其写入缓冲区。
	if start < len(s) {
		buf.Write(s[start:])
	}

	// 在缓冲区中添加结束双引号。
	buf.WriteByte('"')

	// 将构建好的带引号的字符串写入到指定的 io.Writer 中。
	n, err := w.Write(buf.Bytes())

	// 将字节缓冲区归还给缓冲池。
	poolBuffer(buf)

	// 返回写入的字节数和可能的错误。
	return n, err
}

// getu4 decodes \uXXXX from the beginning of s, returning the hex value,
// or it returns -1.
// getu4 从字符串 s 的开头解码 \uXXXX，并返回其十六进制值，
// 或者返回 -1。
func getu4(s []byte) rune {

	// 检查输入字节切片的长度是否小于 6 个字节或者是否以 "\\u" 开头。
	if len(s) < 6 || s[0] != '\\' || s[1] != 'u' {
		return -1
	}

	// 使用 strconv.ParseUint 将字节切片中的 4 位十六进制数解析为 uint64。
	r, err := strconv.ParseUint(string(s[2:6]), 16, 64)

	if err != nil {
		// 如果解析失败，返回 -1 表示无效的 Unicode 转义序列。
		return -1
	}

	// 将解析得到的 uint64 转换为 Unicode 字符，并返回。
	return rune(r)
}

// unquoteBytes 将字节切片 s 解引用，去掉周围的引号和进行逆转义。
func unquoteBytes(s []byte) (t []byte, ok bool) {
	// 检查输入字节切片的长度是否小于 2，以及是否以引号开头和结尾。
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return
	}

	// 剥离引号，获取不带引号的子切片。
	s = s[1 : len(s)-1]

	// Check for unusual characters. If there are none,
	// then no unquoting is needed, so return a slice of the
	// original bytes.
	// 检查是否有不寻常的字符。如果没有，那么不需要解引用，直接返回原始字节切片。
	r := 0
	for r < len(s) {
		// 如果是反斜杠、双引号、或控制字符，则需要解除引号和转义。
		c := s[r]

		// 检查 c 是否是反斜杠 \、双引号 " 或控制字符（ASCII 码小于空格 ' ' 的字符）。
		if c == '\\' || c == '"' || c < ' ' {
			break
		}

		// 如果 c 是普通的 ASCII 字符（小于 utf8.RuneSelf），则将 r 递增，并继续下一个字符的检查。
		if c < utf8.RuneSelf {
			r++
			continue
		}

		// 如果 c 是一个多字节 UTF-8 字符（大于等于 utf8.RuneSelf），
		// 则使用 utf8.DecodeRune 函数来获取该字符的正确值，并将 r 增加多个字节的大小（size）。
		rr, size := utf8.DecodeRune(s[r:])

		// 如果 utf8.DecodeRune 返回 utf8.RuneError，表示解码错误，循环将终止。
		if rr == utf8.RuneError {
			break
		}
		// 增加 r 的值以继续下一个字符的检查。
		r += size
	}

	if r == len(s) {
		// 如果没有不寻常字符，直接返回解码后的字节切片和成功标志 (ok=true)。
		return s, true
	}

	// 创建一个缓冲字节切片来存储解码后的内容，预分配足够的容量以减少内存分配。
	b := make([]byte, len(s)+2*utf8.UTFMax)

	// 使用 copy 函数将 s 中的内容复制到 b 中，从索引 0 复制到索引 r，将 r 设置为复制结束的位置。
	w := copy(b, s[0:r])

	for r < len(s) {
		// Out of room?  Can only happen if s is full of
		// malformed UTF-8 and we're replacing each
		// byte with RuneError.
		// 没有空间了？只有在 s 充满了格式错误的 UTF-8 字节，并且我们将每个字节替换为 RuneError 时才会发生。
		if w >= len(b)-2*utf8.UTFMax {
			// 如果空间不足，扩展缓冲区。
			nb := make([]byte, (len(b)+utf8.UTFMax)*2)
			// 将当前缓冲区中的内容复制到新缓冲区中。
			copy(nb, b[0:w])
			// 将新缓冲区赋值给 b，扩展了缓冲区的容量。
			b = nb
		}

		// 检查当前位置 r 处的字符 c。
		switch c := s[r]; {
		case c == '\\':
			r++
			if r >= len(s) {
				return
			}
			switch s[r] {
			default:
				return
			case '"', '\\', '/', '\'':
				// 如果当前字符是转义字符，则将其解析并写入 b。
				b[w] = s[r]
				r++
				w++
			case 'b':
				// 如果当前字符是 '\b'，则将其替换为 ASCII 退格字符并写入 b。
				b[w] = '\b'
				r++
				w++
			case 'f':
				// 如果当前字符是 '\f'，则将其替换为 ASCII 换页字符并写入 b。
				b[w] = '\f'
				r++
				w++
			case 'n':
				// 如果当前字符是 '\n'，则将其替换为 ASCII 换行字符并写入 b。
				b[w] = '\n'
				r++
				w++
			case 'r':
				// 如果当前字符是 '\r'，则将其替换为 ASCII 回车字符并写入 b。
				b[w] = '\r'
				r++
				w++
			case 't':
				// 如果当前字符是 '\t'，则将其替换为 ASCII 制表符并写入 b。
				b[w] = '\t'
				r++
				w++
			case 'u':
				// 如果当前字符是 '\u'，则说明需要解析 Unicode 转义序列。
				r--
				// 调用 getu4 函数解析 4 位 Unicode 编码，获取对应的 Unicode 字符。
				rr := getu4(s[r:])
				if rr < 0 {
					return
				}
				// 将 r 后移 6 个位置，跳过 Unicode 转义序列。
				r += 6
				if utf16.IsSurrogate(rr) {
					// 如果 Unicode 字符是代理对（surrogate pair），需要解码成单一的 Unicode 字符。
					rr1 := getu4(s[r:])

					// 使用 utf16.DecodeRune 函数将代理对解码为单一 Unicode 字符。
					if dec := utf16.DecodeRune(rr, rr1); dec != unicode.ReplacementChar {
						// A valid pair; consume.
						r += 6
						w += utf8.EncodeRune(b[w:], dec)
						break
					}
					// Invalid surrogate; fall back to replacement rune.
					// 无效的代理对，回退到替代字符（ReplacementChar）。
					rr = unicode.ReplacementChar
				}
				// 将解析的 Unicode 字符编码为 UTF-8 字节序列，并写入 b。
				w += utf8.EncodeRune(b[w:], rr)
			}

		// Quote, control characters are invalid.
		case c == '"', c < ' ':
			// 如果字符是双引号 '"' 或小于空格的控制字符，则表示字符串未关闭或包含无效字符，返回错误。
			return

		// ASCII
		// ASCII 字符，直接写入新的字节数组 b。
		case c < utf8.RuneSelf:
			b[w] = c
			r++
			w++

		// Coerce to well-formed UTF-8.
		// 将字符强制转换为格式正确的 UTF-8 字符。
		default:
			// 使用 utf8.DecodeRune 函数从字符串 s 中解码一个合法的 Unicode 字符。
			rr, size := utf8.DecodeRune(s[r:])
			r += size

			// 使用 utf8.EncodeRune 函数将 Unicode 字符重新编码成 UTF-8 字节序列，并写入新的字节数组 b。
			w += utf8.EncodeRune(b[w:], rr)
		}
	}
	return b[0:w], true
}

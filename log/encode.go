package log

import (
	"bytes"
	"encoding"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"unicode/utf8"
)

// MarshalKeyvals returns the logfmt encoding of keyvals, a variadic sequence
// of alternating keys and values.
// MarshalKeyvals 将keyvals的logfmt编码返回，keyvals是一系列交替的键和值。
func MarshalKeyvals(keyvals ...interface{}) ([]byte, error) {
	// 使用bytes.Buffer缓冲区进行编码。
	buf := &bytes.Buffer{}
	// 使用NewEncoder函数创建一个编码器。
	if err := NewEncoder(buf).EncodeKeyvals(keyvals...); err != nil {
		// 如果编码时发生错误，返回nil字节切片和错误。
		return nil, err
	}
	// 返回编码后的字节切片。
	return buf.Bytes(), nil
}

// An Encoder writes logfmt data to an output stream.
// Encoder 用于将logfmt数据写入输出流的结构。
type Encoder struct {
	// w 表示要写入的 io.Writer 对象，用于将编码后的数据写入到指定的输出流中。
	w io.Writer
	// scratch 是一个临时的字节缓冲区，用于在编码过程中存储临时数据。
	scratch bytes.Buffer
	// needSep 表示是否需要添加分隔符，用于指示是否需要在写入多个数据项之间添加分隔符。
	needSep bool
}

// NewEncoder returns a new encoder that writes to w.
// NewEncoder 返回一个新的编码器，用于将数据写入w。
func NewEncoder(w io.Writer) *Encoder {
	// 返回一个编码器。
	return &Encoder{
		w: w,
	}
}

var (
	// space 包含一个空格字节切片，用于在logfmt中分隔键值对之间的空格。
	space = []byte(" ")
	// comma 包含一个逗号字节切片，用于在logfmt中分隔不同的键值对。
	comma = []byte(",")
	// equals 包含一个等号字节切片，用于在logfmt中分隔键和值。
	equals = []byte("=")
	// newline 包含一个换行符字节切片，用于在logfmt中表示记录的结束。
	newline = []byte("\n")
	// null 包含一个"null"字节切片，用于表示空值。
	null = []byte("null")
)

// EncodeKeyval writes the logfmt encoding of key and value to the stream. A
// single space is written before the second and subsequent keys in a record.
// Nothing is written if a non-nil error is returned.
// EncodeKeyval 将键和值的logfmt编码写入流。在记录中的第二个及后续键之前写入单个空格。
// 如果返回非nil错误，则不会写入任何内容。
func (enc *Encoder) EncodeKeyval(key, value interface{}, needQuoted bool) error {
	// 重置临时的字节缓冲区 scratch
	enc.scratch.Reset()

	// 如果需要添加分隔符，将逗号写入到缓冲区中
	if enc.needSep {
		if _, err := enc.scratch.Write(comma); err != nil {
			return err
		}
	}

	// 将键写入到缓冲区中
	if err := writeKey(&enc.scratch, key); err != nil {
		return err
	}

	// 将等号写入到缓冲区中
	if _, err := enc.scratch.Write(equals); err != nil {
		return err
	}

	// 将值写入到缓冲区中，并根据 needQuoted 参数决定是否需要对值进行引号包裹
	if err := writeValue(&enc.scratch, value, needQuoted); err != nil {
		return err
	}

	// 将缓冲区中的数据写入到 Encoder 的输出流中
	_, err := enc.w.Write(enc.scratch.Bytes())

	// 设置 needSep 标志为 true，表示下一个键值对需要添加分隔符
	enc.needSep = true

	// 返回可能的错误
	return err
}

// EncodeKeyvals writes the logfmt encoding of keyvals to the stream. Keyvals
// is a variadic sequence of alternating keys and values. Keys of unsupported
// type are skipped along with their corresponding value. Values of
// unsupported type or that cause a MarshalerError are replaced by their error
// but do not cause EncodeKeyvals to return an error. If a non-nil error is
// returned some key/value pairs may not have be written.
// EncodeKeyvals 将keyvals的logfmt编码写入流。Keyvals是一系列交替的键和值。
// 不支持的键类型及其对应的值将被跳过。不支持的类型或导致MarshalerError的值将被替换为它们的错误值，
// 但不会导致EncodeKeyvals返回错误。如果返回非nil错误，可能会有一些键/值对未写入。
func (enc *Encoder) EncodeKeyvals(keyvals ...interface{}) error {

	// 如果传入的参数个数为 0，直接返回 nil
	if len(keyvals) == 0 {
		return nil
	}

	// 如果传入的参数个数为奇数，追加一个 nil 作为最后一个值，以确保键值对成对出现
	if len(keyvals)%2 == 1 {
		keyvals = append(keyvals, nil)
	}

	// 循环处理传入的键值对
	for i := 0; i < len(keyvals); i += 2 {
		k, v := keyvals[i], keyvals[i+1]

		// 调用 EncodeKeyval 方法编码键值对，并指定 needQuoted 为 false
		err := enc.EncodeKeyval(k, v, false)

		// 如果出现不支持的键类型错误，继续下一轮循环
		if err == ErrUnsupportedKeyType {
			continue
		}

		// 如果出现编码器错误或不支持的值类型错误，将值设置为错误，并再次尝试编码
		if _, ok := err.(*MarshalerError); ok || err == ErrUnsupportedValueType {
			v = err
			err = enc.EncodeKeyval(k, v, false)
		}

		// 如果编码过程中出现其他错误，返回该错误
		if err != nil {
			return err
		}
	}
	// 所有键值对编码完成，返回 nil 表示成功
	return nil
}

// EncodeKeyvalsWithQuoted 与EncodeKeyvals类似，但引用需要引用的值。
func (enc *Encoder) EncodeKeyvalsWithQuoted(keyvals ...interface{}) error {
	// 如果传入的参数个数为 0，直接返回 nil
	if len(keyvals) == 0 {
		return nil
	}

	// 如果传入的参数个数为奇数，追加一个 nil 作为最后一个值，以确保键值对成对出现
	if len(keyvals)%2 == 1 {
		keyvals = append(keyvals, nil)
	}

	// 循环处理传入的键值对
	for i := 0; i < len(keyvals); i += 2 {
		k, v := keyvals[i], keyvals[i+1]

		// 调用 EncodeKeyval 方法编码键值对，并指定 needQuoted 为 true，表示需要对值进行引号包裹
		err := enc.EncodeKeyval(k, v, true)

		// 如果出现不支持的键类型错误，继续下一轮循环
		if err == ErrUnsupportedKeyType {
			continue
		}

		// 如果出现编码器错误或不支持的值类型错误，将值设置为错误，并再次尝试编码
		if _, ok := err.(*MarshalerError); ok || err == ErrUnsupportedValueType {
			v = err
			err = enc.EncodeKeyval(k, v, true)
		}

		// 如果编码过程中出现其他错误，返回该错误
		if err != nil {
			return err
		}
	}

	// 所有带引号的键值对编码完成，返回 nil 表示成功
	return nil
}

// MarshalerError represents an error encountered while marshaling a value.
// MarshalerError 表示在编组值时遇到的错误。
type MarshalerError struct {
	// Type 表示不支持的数据类型的反射类型信息。
	Type reflect.Type
	// Err 表示底层错误，通常用于描述为什么不支持该数据类型。
	Err error
}

// 错误实现
func (e *MarshalerError) Error() string {
	// 返回一个字符串，包含了不支持的数据类型的类型信息和底层错误信息。
	return "error marshaling value of type " + e.Type.String() + ": " + e.Err.Error()
}

// ErrNilKey is returned by Marshal functions and Encoder methods if a key is
// a nil interface or pointer value.
// ErrNilKey 是Marshal函数和Encoder方法的返回值，如果键是nil接口或指针值，则返回。
var ErrNilKey = errors.New("nil key")

// ErrInvalidKey is returned by Marshal functions and Encoder methods if, after
// dropping invalid runes, a key is empty.
// ErrInvalidKey 是Marshal函数和Encoder方法的返回值，如果在删除无效的字符后，键为空，则返回。
var ErrInvalidKey = errors.New("invalid key")

// ErrUnsupportedKeyType is returned by Encoder methods if a key has an
// unsupported type.
// ErrUnsupportedKeyType 是Encoder方法的返回值，如果键具有不支持的类型，则返回。
var ErrUnsupportedKeyType = errors.New("unsupported key type")

// ErrUnsupportedValueType is returned by Encoder methods if a value has an
// unsupported type.
// ErrUnsupportedValueType 是Encoder方法的返回值，如果值具有不支持的类型，则返回。
var ErrUnsupportedValueType = errors.New("unsupported value type")

// writeKey 根据key的类型进行处理并将其编码写入流。
func writeKey(w io.Writer, key interface{}) error {
	// 处理特殊情况：如果key为nil，返回ErrNilKey错误。
	if key == nil {
		return ErrNilKey
	}

	switch k := key.(type) {
	case string:
		// 如果key是字符串类型，则调用writeStringKey函数进行处理。
		return writeStringKey(w, k)
	case []byte:
		// 如果key是字节切片类型，检查是否为nil，然后调用writeBytesKey函数进行处理。
		if k == nil {
			return ErrNilKey
		}
		return writeBytesKey(w, k)
	case encoding.TextMarshaler:
		// 如果key实现了TextMarshaler接口，将其安全编码后写入流。
		kb, err := safeMarshal(k)
		if err != nil {
			return err
		}
		if kb == nil {
			return ErrNilKey
		}
		return writeBytesKey(w, kb)
	case fmt.Stringer:
		// 如果key实现了Stringer接口，将其安全转换为字符串后写入流。
		ks, ok := safeString(k)
		if !ok {
			return ErrNilKey
		}
		return writeStringKey(w, ks)
	default:
		// 对于其他类型的key，使用反射进行处理。
		rkey := reflect.ValueOf(key)
		switch rkey.Kind() {
		case reflect.Array, reflect.Chan, reflect.Func, reflect.Map, reflect.Slice, reflect.Struct:
			// 不支持的键类型，返回ErrUnsupportedKeyType错误。
			return ErrUnsupportedKeyType
		case reflect.Ptr:
			// 如果key是指针类型，检查是否为nil，然后递归调用writeKey处理其指向的值。
			if rkey.IsNil() {
				return ErrNilKey
			}
			return writeKey(w, rkey.Elem().Interface())
		}
		// 对于其他类型的key，将其安全转换为字符串后写入流。
		return writeStringKey(w, fmt.Sprint(k))
	}
}

// keyRuneFilter returns r for all valid key runes, and -1 for all invalid key
// runes. When used as the mapping function for strings.Map and bytes.Map
// functions it causes them to remove invalid key runes from strings or byte
// slices respectively.
// keyRuneFilter 为所有有效的键字符返回r，对于所有无效的键字符返回-1。
// 当用作strings.Map和bytes.Map函数的映射函数时，它会从字符串或字节切片中删除无效的键字符。
func keyRuneFilter(r rune) rune {
	// 检查字符是否小于或等于空白字符、等号、双引号或无效的 UTF-8 字符
	if r <= ' ' || r == '=' || r == '"' || r == utf8.RuneError {
		return -1
	}
	return r
}

// writeStringKey 将键字符串进行处理并写入流。
func writeStringKey(w io.Writer, key string) error {
	// 使用keyRuneFilter函数处理字符串，删除无效的键字符。
	k := strings.Map(keyRuneFilter, key)

	// 如果处理后的键为空字符串，表示键不合法，返回 ErrInvalidKey 错误。
	if k == "" {
		// 如果处理后的字符串为空，返回ErrInvalidKey错误。
		return ErrInvalidKey
	}

	// 将处理后的键写入到指定的 io.Writer 中。
	_, err := io.WriteString(w, k)

	// 返回可能的错误。
	return err
}

// writeBytesKey 将键字节切片进行处理并写入流。
func writeBytesKey(w io.Writer, key []byte) error {
	// 使用keyRuneFilter函数处理字节切片，删除无效的键字符。
	k := bytes.Map(keyRuneFilter, key)
	if len(k) == 0 {
		// 如果处理后的字节切片为空，返回ErrInvalidKey错误。
		return ErrInvalidKey
	}
	_, err := w.Write(k)
	return err
}

// writeValue 根据值的类型进行处理并将其编码写入流。
func writeValue(w io.Writer, value interface{}, needQuoted bool) error {
	switch v := value.(type) {
	case nil:
		// 如果值为nil，调用writeBytesValue将null写入流，根据needQuoted确定是否需要引号。
		return writeBytesValue(w, null, needQuoted)
	case int, uint, int8, uint8, int16, uint16, int32, uint32, int64, uint64, float32, float64:
		// 如果值是基本数值类型，将其转换为字符串后写入流，根据needQuoted确定是否需要引号。
		return writeStringValue(w, fmt.Sprint(v), true, false)
	case string:
		// 如果值是字符串类型，根据needQuoted确定是否需要引号。
		return writeStringValue(w, v, true, needQuoted)
	case []byte:
		// 如果值是字节切片类型，根据needQuoted确定是否需要引号。
		return writeBytesValue(w, v, needQuoted)
	case encoding.TextMarshaler:
		// 如果值实现了TextMarshaler接口，将其安全编码后写入流，根据needQuoted确定是否需要引号。
		vb, err := safeMarshal(v)
		if err != nil {
			return err
		}
		if vb == nil {
			vb = null
		}
		return writeBytesValue(w, vb, needQuoted)
	case error:
		// 如果值是error类型，将其安全处理为字符串后写入流，根据needQuoted确定是否需要引号。
		se, ok := safeError(v)
		return writeStringValue(w, se, ok, needQuoted)
	case fmt.Stringer:
		// 如果值实现了Stringer接口，将其安全转换为字符串后写入流，根据needQuoted确定是否需要引号。
		ss, ok := safeString(v)
		return writeStringValue(w, ss, ok, needQuoted)
	default:
		// 对于其他类型的值，使用反射进行处理。
		rvalue := reflect.ValueOf(value)
		switch rvalue.Kind() {
		case reflect.Array, reflect.Chan, reflect.Func, reflect.Map, reflect.Slice, reflect.Struct:
			// 不支持的值类型，返回ErrUnsupportedValueType错误。
			return ErrUnsupportedValueType
		case reflect.Ptr:
			// 如果值是指针类型，检查是否为nil，然后递归调用writeValue处理其指向的值。
			if rvalue.IsNil() {
				return writeBytesValue(w, null, needQuoted)
			}
			return writeValue(w, rvalue.Elem().Interface(), needQuoted)
		}
		// 对于其他类型的值，将其安全转换为字符串后写入流，根据needQuoted确定是否需要引号。
		return writeStringValue(w, fmt.Sprint(v), true, needQuoted)
	}
}

// needsQuotedValueRune 检查r是否需要引号。
func needsQuotedValueRune(r rune) bool {
	return r <= ' ' || r == '=' || r == '"' || r == utf8.RuneError
}

// writeStringValue 将字符串值进行处理并写入流。
func writeStringValue(w io.Writer, value string, ok bool, needQuoted bool) error {
	var err error
	if ok && value == "null" {
		// 如果值为"null"，写入带引号的"null"字符串。
		_, err = io.WriteString(w, `"null"`)
	} else if needQuoted {
		// 如果需要引号，将字符串值进行引号包装后写入流。
		_, err = writeQuotedString(w, value)
	} else if strings.IndexFunc(value, needsQuotedValueRune) != -1 {
		// 如果字符串值包含需要引号的字符，将其进行引号包装后写入流。
		_, err = writeQuotedString(w, value)
	} else {
		// 否则，直接写入字符串值。
		_, err = io.WriteString(w, value)
	}
	return err
}

// writeBytesValue 根据needQuoted确定是否需要引号，然后将字节切片值写入流。
func writeBytesValue(w io.Writer, value []byte, needQuoted bool) error {
	var err error
	if needQuoted {
		// 如果需要引号，将字节切片值进行引号包装后写入流。
		_, err = writeQuotedBytes(w, value)
	} else if bytes.IndexFunc(value, needsQuotedValueRune) != -1 {
		// 如果字节切片值包含需要引号的字符，将其进行引号包装后写入流。
		_, err = writeQuotedBytes(w, value)
	} else {
		// 否则，直接写入字节切片值。
		_, err = w.Write(value)
	}
	return err
}

// EndRecord writes a newline character to the stream and resets the encoder
// to the beginning of a new record.
func (enc *Encoder) EndRecord() error {
	_, err := enc.w.Write(newline)
	if err == nil {
		enc.needSep = false
	}
	return err
}

// Reset resets the encoder to the beginning of a new record.
// EndRecord 向流写入换行字符并重置编码器以开始新记录。
func (enc *Encoder) Reset() {
	enc.needSep = false
}

// safeError 将错误值安全处理为字符串。
func safeError(err error) (s string, ok bool) {
	defer func() {
		if panicVal := recover(); panicVal != nil {
			if v := reflect.ValueOf(err); v.Kind() == reflect.Ptr && v.IsNil() {
				// 如果错误是指针类型且为nil，将其处理为"null"字符串。
				s, ok = "null", false
			} else {
				// 否则，将恐慌值包装在字符串中。
				s, ok = fmt.Sprintf("PANIC:%v", panicVal), false
			}
		}
	}()
	// 将错误值转换为字符串。
	s, ok = err.Error(), true
	return
}

// safeString 将Stringer接口值安全处理为字符串。
func safeString(str fmt.Stringer) (s string, ok bool) {
	defer func() {
		if panicVal := recover(); panicVal != nil {
			if v := reflect.ValueOf(str); v.Kind() == reflect.Ptr && v.IsNil() {
				// 如果Stringer接口值是指针类型且为nil，将其处理为"null"字符串。
				s, ok = "null", false
			} else {
				// 否则，将恐慌值包装在字符串中。
				s, ok = fmt.Sprintf("PANIC:%v", panicVal), true
			}
		}
	}()
	// 将Stringer接口值转换为字符串。
	s, ok = str.String(), true
	return
}

// safeMarshal 将TextMarshaler接口值安全编码为字节切片。
func safeMarshal(tm encoding.TextMarshaler) (b []byte, err error) {
	defer func() {
		// 使用defer捕获可能发生的恐慌。
		if panicVal := recover(); panicVal != nil {
			// 如果恐慌值是nil，将其处理为nil字节切片。
			if v := reflect.ValueOf(tm); v.Kind() == reflect.Ptr && v.IsNil() {
				// 如果TextMarshaler接口值是指针类型且为nil，将其处理为nil字节切片。
				b, err = nil, nil
			} else {
				// 否则，将恐慌值包装在错误中。
				b, err = nil, fmt.Errorf("panic when marshalling: %s", panicVal)
			}
		}
	}()
	// 使用TextMarshaler接口编码值，并处理可能的错误。
	b, err = tm.MarshalText()
	// 如果编码后的字节切片为空，返回nil字节切片。
	if err != nil {
		// 如果编码时发生错误，将其包装在错误中。
		return nil, &MarshalerError{
			// 记录值的类型。
			Type: reflect.TypeOf(tm),
			// 记录错误。
			Err: err,
		}
	}
	// 如果编码后的字节切片为空，返回nil字节切片。
	return
}

package resp

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

var (
	ErrInvalidSyntax = errors.New("invalid RESP syntax")
)

// Reader deserializes RESP-encoded byte streams.
type Reader struct {
	reader *bufio.Reader
}

func NewReader(rd io.Reader) *Reader {
	return &Reader{reader: bufio.NewReader(rd)}
}

// Read parses the next complete RESP Value from the stream.
//
// Professor's Note:
// RESP is a binary-safe protocol because Bulk Strings are length-prefixed.
// Unlike C-strings which terminate at \0, RESP can carry arbitrary binary blobs,
// compressed images, or serialized objects without byte escaping.
func (r *Reader) Read() (Value, error) {
	typeByte, err := r.reader.ReadByte()
	if err != nil {
		return Value{}, err
	}

	switch typeByte {
	case TypeSimpleString:
		line, err := r.readLine()
		if err != nil {
			return Value{}, err
		}
		return SimpleString(line), nil

	case TypeError:
		line, err := r.readLine()
		if err != nil {
			return Value{}, err
		}
		return Error(line), nil

	case TypeInteger:
		line, err := r.readLine()
		if err != nil {
			return Value{}, err
		}
		n, err := strconv.Atoi(line)
		if err != nil {
			return Value{}, fmt.Errorf("invalid integer format: %w", err)
		}
		return Integer(n), nil

	case TypeBulkString:
		return r.readBulk()

	case TypeArray:
		return r.readArray()

	default:
		// Fallback for inline plain-text commands (e.g. from netcat: "PING\r\n")
		r.reader.UnreadByte()
		line, err := r.readLine()
		if err != nil {
			return Value{}, err
		}
		parts := strings.Fields(line)
		var arr []Value
		for _, part := range parts {
			arr = append(arr, BulkString(part))
		}
		return Array(arr), nil
	}
}

func (r *Reader) readLine() (string, error) {
	line, err := r.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func (r *Reader) readBulk() (Value, error) {
	lenStr, err := r.readLine()
	if err != nil {
		return Value{}, err
	}

	length, err := strconv.Atoi(lenStr)
	if err != nil {
		return Value{}, fmt.Errorf("invalid bulk string length: %w", err)
	}

	// Length -1 indicates a Null bulk string (e.g. key does not exist)
	if length == -1 {
		return NullBulk(), nil
	}

	buf := make([]byte, length)
	_, err = io.ReadFull(r.reader, buf)
	if err != nil {
		return Value{}, err
	}

	// Consume trailing CRLF
	_, _ = r.reader.ReadByte()
	_, _ = r.reader.ReadByte()

	return BulkString(string(buf)), nil
}

func (r *Reader) readArray() (Value, error) {
	countStr, err := r.readLine()
	if err != nil {
		return Value{}, err
	}

	count, err := strconv.Atoi(countStr)
	if err != nil {
		return Value{}, fmt.Errorf("invalid array count: %w", err)
	}

	if count == -1 {
		return Value{Type: TypeArray, Null: true}, nil
	}

	items := make([]Value, 0, count)
	for i := 0; i < count; i++ {
		val, err := r.Read()
		if err != nil {
			return Value{}, err
		}
		items = append(items, val)
	}

	return Array(items), nil
}

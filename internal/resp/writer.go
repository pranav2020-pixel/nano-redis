package resp

import (
	"fmt"
	"strconv"
)

// Marshal serializes a RESP Value into compliant wire bytes.
func (v Value) Marshal() []byte {
	switch v.Type {
	case TypeSimpleString:
		return []byte(fmt.Sprintf("+%s\r\n", v.Str))

	case TypeError:
		return []byte(fmt.Sprintf("-%s\r\n", v.Str))

	case TypeInteger:
		return []byte(fmt.Sprintf(":%d\r\n", v.Num))

	case TypeBulkString:
		if v.Null {
			return []byte("$-1\r\n")
		}
		return []byte(fmt.Sprintf("$%d\r\n%s\r\n", len(v.Bulk), v.Bulk))

	case TypeArray:
		if v.Null {
			return []byte("*-1\r\n")
		}
		res := []byte(fmt.Sprintf("*%d\r\n", len(v.Array)))
		for _, item := range v.Array {
			res = append(res, item.Marshal()...)
		}
		return res

	default:
		return []byte("-ERR unknown serialization type\r\n")
	}
}

// FormatString converts bytes length to string for fast buffering.
func itoa(n int) string {
	return strconv.Itoa(n)
}

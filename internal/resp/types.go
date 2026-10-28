package resp

// Protocol byte markers as defined in the Redis Serialization Protocol (RESP2) spec.
const (
	TypeSimpleString = '+'
	TypeError        = '-'
	TypeInteger      = ':'
	TypeBulkString   = '$'
	TypeArray        = '*'
)

// Value represents a typed RESP protocol element.
type Value struct {
	Type  byte     // '+', '-', ':', '$', '*'
	Str   string   // Used for SimpleString, Error, and BulkString
	Num   int      // Used for Integer
	Bulk  string   // Used for BulkString payload
	Array []Value  // Used for Array elements
	Null  bool     // True if BulkString or Array is null ($-1\r\n or *-1\r\n)
}

// Helper constructors for idiomatic value generation.
func SimpleString(s string) Value {
	return Value{Type: TypeSimpleString, Str: s}
}

func Error(msg string) Value {
	return Value{Type: TypeError, Str: msg}
}

func Integer(n int) Value {
	return Value{Type: TypeInteger, Num: n}
}

func BulkString(s string) Value {
	return Value{Type: TypeBulkString, Bulk: s}
}

func NullBulk() Value {
	return Value{Type: TypeBulkString, Null: true}
}

func Array(items []Value) Value {
	return Value{Type: TypeArray, Array: items}
}

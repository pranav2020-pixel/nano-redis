package resp

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseSimpleString(t *testing.T) {
	input := "+OK\r\n"
	r := NewReader(strings.NewReader(input))
	val, err := r.Read()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if val.Type != TypeSimpleString || val.Str != "OK" {
		t.Errorf("expected SimpleString 'OK', got %+v", val)
	}
}

func TestParseBulkString(t *testing.T) {
	input := "$5\r\nhello\r\n"
	r := NewReader(strings.NewReader(input))
	val, err := r.Read()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if val.Type != TypeBulkString || val.Bulk != "hello" {
		t.Errorf("expected BulkString 'hello', got %+v", val)
	}
}

func TestParseNullBulk(t *testing.T) {
	input := "$-1\r\n"
	r := NewReader(strings.NewReader(input))
	val, err := r.Read()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !val.Null {
		t.Errorf("expected Null BulkString, got %+v", val)
	}
}

func TestParseArray(t *testing.T) {
	// Represents command: SET mykey myval
	input := "*3\r\n$3\r\nSET\r\n$5\r\nmykey\r\n$5\r\nmyval\r\n"
	r := NewReader(strings.NewReader(input))
	val, err := r.Read()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if val.Type != TypeArray || len(val.Array) != 3 {
		t.Fatalf("expected array of 3, got %+v", val)
	}

	if val.Array[0].Bulk != "SET" || val.Array[1].Bulk != "mykey" || val.Array[2].Bulk != "myval" {
		t.Errorf("unexpected array contents: %+v", val.Array)
	}
}

func TestMarshalRoundTrip(t *testing.T) {
	original := Array([]Value{
		BulkString("GET"),
		BulkString("user:101"),
	})

	marshaled := original.Marshal()
	r := NewReader(bytes.NewReader(marshaled))
	parsed, err := r.Read()
	if err != nil {
		t.Fatalf("failed to re-parse marshaled bytes: %v", err)
	}

	if parsed.Type != TypeArray || len(parsed.Array) != 2 {
		t.Fatalf("mismatched round trip structure: %+v", parsed)
	}
	if parsed.Array[0].Bulk != "GET" || parsed.Array[1].Bulk != "user:101" {
		t.Errorf("mismatched values: %+v", parsed.Array)
	}
}

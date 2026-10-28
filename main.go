package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"

	"nano-redis/internal/resp"
	"nano-redis/internal/storage"
)

func main() {
	port := flag.Int("port", 6379, "Port for the Redis-compatible server")
	aofPath := flag.String("aof", "database.aof", "Path to Append-Only File durability log (empty string to disable)")
	flag.Parse()

	// 1. Initialize In-Memory Storage Engine with AOF Durability
	store, err := storage.NewStore(*aofPath)
	if err != nil {
		log.Fatalf("Failed to initialize storage engine: %v", err)
	}
	defer store.Close()

	// 2. Open TCP Socket on Port 6379
	addr := fmt.Sprintf(":%d", *port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to bind TCP listener on %s: %v", addr, err)
	}
	defer listener.Close()

	fmt.Printf("\n⚡ [nano-redis] In-Memory Store listening on port %d\n", *port)
	fmt.Printf("💾 [nano-redis] AOF Durability Log: %s\n", *aofPath)
	fmt.Printf("🔌 [nano-redis] Connect via official CLI: redis-cli -p %d\n\n", *port)

	// 3. Connection Accept Loop
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting client socket: %v", err)
			continue
		}

		go handleClient(conn, store)
	}
}

func handleClient(conn net.Conn, store *storage.Store) {
	defer conn.Close()
	reader := resp.NewReader(conn)

	for {
		val, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			log.Printf("[%s] Read error: %v", conn.RemoteAddr(), err)
			break
		}

		if val.Type != resp.TypeArray || len(val.Array) == 0 {
			_, _ = conn.Write(resp.Error("ERR syntax error").Marshal())
			continue
		}

		cmd := strings.ToUpper(val.Array[0].Bulk)
		args := val.Array[1:]

		var response resp.Value

		switch cmd {
		case "PING":
			if len(args) == 0 {
				response = resp.SimpleString("PONG")
			} else {
				response = resp.BulkString(args[0].Bulk)
			}

		case "COMMAND", "DOCS":
			// redis-cli sends 'COMMAND' on initialization to inspect cluster features
			response = resp.Array([]resp.Value{})

		case "SET":
			if len(args) < 2 {
				response = resp.Error("ERR wrong number of arguments for 'set' command")
			} else {
				key := args[0].Bulk
				value := args[1].Bulk
				ttl := 0

				// Support 'SET key val EX 60'
				if len(args) >= 4 && strings.ToUpper(args[2].Bulk) == "EX" {
					if sec, err := strconv.Atoi(args[3].Bulk); err == nil {
						ttl = sec
					}
				}

				store.Set(key, value, ttl)
				response = resp.SimpleString("OK")
			}

		case "GET":
			if len(args) != 1 {
				response = resp.Error("ERR wrong number of arguments for 'get' command")
			} else {
				key := args[0].Bulk
				if val, found := store.Get(key); found {
					response = resp.BulkString(val)
				} else {
					response = resp.NullBulk()
				}
			}

		case "DEL":
			if len(args) < 1 {
				response = resp.Error("ERR wrong number of arguments for 'del' command")
			} else {
				deletedCount := 0
				for _, arg := range args {
					if store.Del(arg.Bulk) {
						deletedCount++
					}
				}
				response = resp.Integer(deletedCount)
			}

		case "EXISTS":
			if len(args) < 1 {
				response = resp.Error("ERR wrong number of arguments for 'exists' command")
			} else {
				count := 0
				for _, arg := range args {
					if store.Exists(arg.Bulk) {
						count++
					}
				}
				response = resp.Integer(count)
			}

		case "EXPIRE":
			if len(args) != 2 {
				response = resp.Error("ERR wrong number of arguments for 'expire' command")
			} else {
				key := args[0].Bulk
				seconds, err := strconv.Atoi(args[1].Bulk)
				if err != nil {
					response = resp.Error("ERR value is not an integer or out of range")
				} else if store.Expire(key, seconds) {
					response = resp.Integer(1)
				} else {
					response = resp.Integer(0)
				}
			}

		case "TTL":
			if len(args) != 1 {
				response = resp.Error("ERR wrong number of arguments for 'ttl' command")
			} else {
				ttl := store.TTL(args[0].Bulk)
				response = resp.Integer(int(ttl))
			}

		case "KEYS":
			pattern := "*"
			if len(args) >= 1 {
				pattern = args[0].Bulk
			}
			matchedKeys := store.Keys(pattern)
			var arr []resp.Value
			for _, k := range matchedKeys {
				arr = append(arr, resp.BulkString(k))
			}
			response = resp.Array(arr)

		case "QUIT":
			_, _ = conn.Write(resp.SimpleString("OK").Marshal())
			return

		default:
			response = resp.Error(fmt.Sprintf("ERR unknown command '%s'", cmd))
		}

		if _, err := conn.Write(response.Marshal()); err != nil {
			break
		}
	}
}

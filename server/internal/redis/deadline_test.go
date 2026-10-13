package redis_test

import (
	projectredis "admin/server/internal/redis"
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRedisIOHonorsRequestDeadline(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan struct{})
	defer close(done)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		for {
			command, err := readRESPCommand(reader)
			if err != nil {
				return
			}
			switch strings.ToLower(command[0]) {
			case "hello":
				fmt.Fprint(conn, "-ERR unknown command 'HELLO'\r\n")
			case "ping":
				fmt.Fprint(conn, "+PONG\r\n")
			case "get":
				select {
				case <-done:
				case <-time.After(650 * time.Millisecond):
				}
				return
			default:
				fmt.Fprint(conn, "+OK\r\n")
			}
		}
	}()
	open, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	client, err := projectredis.Open(open, "redis://"+listener.Addr().String()+"/0")
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx, cancelRead := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancelRead()
	start := time.Now()
	_, _, err = client.GetString(ctx, "deadline-probe")
	if err == nil {
		t.Fatal("stalled read succeeded")
	}
	if elapsed := time.Since(start); elapsed > 350*time.Millisecond {
		t.Fatalf("Redis I/O exceeded request deadline: %v", elapsed)
	}
}
func readRESPCommand(reader *bufio.Reader) ([]string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(line, "*") {
		return nil, fmt.Errorf("not RESP array")
	}
	count, err := strconv.Atoi(strings.TrimSpace(line[1:]))
	if err != nil || count < 1 || count > 16 {
		return nil, fmt.Errorf("invalid RESP command")
	}
	parts := make([]string, count)
	for i := range parts {
		line, err = reader.ReadString('\n')
		if err != nil || !strings.HasPrefix(line, "$") {
			return nil, fmt.Errorf("invalid bulk")
		}
		n, err := strconv.Atoi(strings.TrimSpace(line[1:]))
		if err != nil || n < 0 || n > 1024 {
			return nil, fmt.Errorf("invalid bulk size")
		}
		raw := make([]byte, n+2)
		if _, err = io.ReadFull(reader, raw); err != nil {
			return nil, err
		}
		parts[i] = string(raw[:n])
	}
	return parts, nil
}

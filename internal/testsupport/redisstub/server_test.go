package redisstub

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func TestUnsupportedCommandKeepsConnectionOpen(t *testing.T) {
	server, err := Start(Options{})
	if err != nil {
		t.Fatalf("start Redis stub: %v", err)
	}
	t.Cleanup(func() { _ = server.Close() })

	conn, err := net.DialTimeout("tcp", server.Addr(), time.Second)
	if err != nil {
		t.Fatalf("dial Redis stub: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	reader := bufio.NewReader(conn)
	if _, err := fmt.Fprint(conn, "*1\r\n$11\r\nUNSUPPORTED\r\n"); err != nil {
		t.Fatalf("write unsupported command: %v", err)
	}
	if response, err := reader.ReadString('\n'); err != nil || !strings.HasPrefix(response, "-ERR") {
		t.Fatalf("unsupported response=%q err=%v", response, err)
	}

	if _, err := fmt.Fprint(conn, "*1\r\n$4\r\nPING\r\n"); err != nil {
		t.Fatalf("write PING: %v", err)
	}
	if response, err := reader.ReadString('\n'); err != nil || response != "+PONG\r\n" {
		t.Fatalf("PING response=%q err=%v", response, err)
	}
}

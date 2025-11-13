package chat

import (
	"bufio"
	"context"
	"net"
	"testing"
	"time"
)

type deadlineConn struct {
	net.Conn

	deadlineCh chan time.Time
}

func newDeadlineConn(base net.Conn) *deadlineConn {
	return &deadlineConn{Conn: base, deadlineCh: make(chan time.Time, 1)}
}

func (d *deadlineConn) SetReadDeadline(t time.Time) error {
	select {
	case d.deadlineCh <- t:
	default:
	}
	return nil
}

func (d *deadlineConn) nextDeadline(timeout time.Duration) (time.Time, bool) {
	select {
	case dl := <-d.deadlineCh:
		return dl, true
	case <-time.After(timeout):
		return time.Time{}, false
	}
}

func TestReadMessageDoesNotForceDefaultDeadline(t *testing.T) {
	t.Parallel()

	server, client := net.Pipe()
	t.Cleanup(func() {
		_ = server.Close()
		_ = client.Close()
	})

	dconn := newDeadlineConn(server)
	ws := &Conn{
		conn:   dconn,
		reader: bufio.NewReader(dconn),
		writer: bufio.NewWriter(dconn),
	}

	msgCh := make(chan []byte, 1)
	errCh := make(chan error, 1)
	go func() {
		msg, err := ws.ReadMessage(context.Background())
		if err != nil {
			errCh <- err
			return
		}
		msgCh <- msg
	}()

	dl, ok := dconn.nextDeadline(time.Second)
	if !ok {
		t.Fatalf("ReadMessage did not attempt to set a read deadline")
	}
	if !dl.IsZero() {
		t.Fatalf("expected zero deadline, got %v", dl)
	}

	select {
	case err := <-errCh:
		t.Fatalf("ReadMessage returned early: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	writeTextFrame(t, client, []byte("idle"))

	select {
	case err := <-errCh:
		t.Fatalf("ReadMessage returned error: %v", err)
	case msg := <-msgCh:
		if string(msg) != "idle" {
			t.Fatalf("unexpected payload %q", msg)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for message")
	}
}

func writeTextFrame(t testing.TB, conn net.Conn, payload []byte) {
	t.Helper()
	frame := []byte{0x81}
	if len(payload) >= 126 {
		t.Fatalf("payload too large for test frame: %d", len(payload))
	}
	frame = append(frame, byte(len(payload)))
	frame = append(frame, payload...)
	if _, err := conn.Write(frame); err != nil {
		t.Fatalf("failed to write frame: %v", err)
	}
}

package chat

import (
	"bufio"
	"context"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
)

// Conn represents a minimal WebSocket connection supporting text frames.
type Conn struct {
	conn   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer

	mu     sync.Mutex
	closed bool
}

// Accept upgrades the HTTP connection to a WebSocket and returns a Conn.
func Accept(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	if !headerContains(r.Header, "Connection", "upgrade") || !headerContains(r.Header, "Upgrade", "websocket") {
		return nil, fmt.Errorf("websocket upgrade required")
	}
	if r.Header.Get("Sec-WebSocket-Version") != "13" {
		return nil, fmt.Errorf("unsupported websocket version")
	}
	key := strings.TrimSpace(r.Header.Get("Sec-WebSocket-Key"))
	if key == "" {
		return nil, fmt.Errorf("missing websocket key")
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, fmt.Errorf("http server does not support hijacking")
	}
	conn, rw, err := hj.Hijack()
	if err != nil {
		return nil, err
	}

	accept := computeAcceptKey(key)
	response := fmt.Sprintf("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", accept)
	if _, err := rw.WriteString(response); err != nil {
		conn.Close()
		return nil, err
	}
	if err := rw.Flush(); err != nil {
		conn.Close()
		return nil, err
	}

	return &Conn{
		conn:   conn,
		reader: rw.Reader,
		writer: rw.Writer,
	}, nil
}

// Dial establishes a WebSocket connection to the given URL.
func Dial(ctx context.Context, rawURL string, header http.Header, tlsConfig *tls.Config) (*Conn, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "ws" && u.Scheme != "wss" {
		return nil, fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	host := u.Host
	if !strings.Contains(host, ":") {
		if u.Scheme == "wss" {
			host += ":443"
		} else {
			host += ":80"
		}
	}

	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", host)
	if err != nil {
		return nil, err
	}

	if u.Scheme == "wss" {
		cfg := &tls.Config{}
		if tlsConfig != nil {
			cfg = tlsConfig.Clone()
		}
		if cfg.ServerName == "" {
			cfg.ServerName = u.Hostname()
		}
		tlsConn := tls.Client(conn, cfg)
		if deadline, ok := ctx.Deadline(); ok {
			_ = tlsConn.SetDeadline(deadline)
			defer tlsConn.SetDeadline(time.Time{})
		}
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			conn.Close()
			return nil, err
		}
		conn = tlsConn
	}

	key := generateKey()
	path := u.RequestURI()
	if path == "" {
		path = "/"
	}
	req := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: %s\r\n", path, u.Host, key)
	for name, values := range header {
		for _, value := range values {
			req += fmt.Sprintf("%s: %s\r\n", name, value)
		}
	}
	req += "\r\n"
	if _, err := io.WriteString(conn, req); err != nil {
		conn.Close()
		return nil, err
	}

	reader := bufio.NewReader(conn)
	status, err := reader.ReadString('\n')
	if err != nil {
		conn.Close()
		return nil, err
	}
	if !strings.Contains(status, "101") {
		conn.Close()
		return nil, fmt.Errorf("handshake failed: %s", strings.TrimSpace(status))
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			conn.Close()
			return nil, err
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}

	return &Conn{
		conn:   conn,
		reader: reader,
		writer: bufio.NewWriter(conn),
	}, nil
}

func headerContains(header http.Header, name, expected string) bool {
	values := header.Values(name)
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), strings.ToLower(expected)) {
			return true
		}
	}
	return false
}

func computeAcceptKey(key string) string {
	hash := sha1.Sum([]byte(key + wsGUID))
	return base64.StdEncoding.EncodeToString(hash[:])
}

func generateKey() string {
	nonce := time.Now().UnixNano()
	raw := fmt.Sprintf("%d", nonce)
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

// ReadMessage reads the next text frame from the connection.
func (c *Conn) ReadMessage(ctx context.Context) ([]byte, error) {
	if c.closed {
		return nil, io.EOF
	}
	if ctx == nil {
		ctx = context.Background()
	}
	deadline, ok := ctx.Deadline()
	if ok {
		_ = c.conn.SetReadDeadline(deadline)
	} else {
		_ = c.conn.SetReadDeadline(time.Time{})
	}
	for {
		frame, err := readFrame(c.reader)
		if err != nil {
			return nil, err
		}
		switch frame.opcode {
		case opcodeText:
			return frame.payload, nil
		case opcodePing:
			if err := c.writeFrame(opcodePong, frame.payload); err != nil {
				return nil, err
			}
		case opcodeClose:
			c.Close()
			return nil, io.EOF
		default:
			// Ignore unsupported frames.
		}
	}
}

// WriteText sends a text frame.
func (c *Conn) WriteText(payload []byte) error {
	if c.closed {
		return io.ErrClosedPipe
	}
	return c.writeFrame(opcodeText, payload)
}

// Ping sends a ping control frame to the peer.
func (c *Conn) Ping(payload []byte) error {
	if c.closed {
		return io.ErrClosedPipe
	}
	return c.writeFrame(opcodePing, payload)
}

func (c *Conn) writeFrame(opcode byte, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return io.ErrClosedPipe
	}
	header := []byte{0x80 | opcode}
	length := len(payload)
	switch {
	case length < 126:
		header = append(header, byte(length))
	case length <= 65535:
		header = append(header, 126, byte(length>>8), byte(length))
	default:
		header = append(header, 127,
			byte(length>>56), byte(length>>48), byte(length>>40), byte(length>>32),
			byte(length>>24), byte(length>>16), byte(length>>8), byte(length))
	}
	if _, err := c.writer.Write(header); err != nil {
		return err
	}
	if _, err := c.writer.Write(payload); err != nil {
		return err
	}
	return c.writer.Flush()
}

// Close closes the underlying network connection.
func (c *Conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	return c.conn.Close()
}

type frame struct {
	fin     bool
	opcode  byte
	payload []byte
}

const (
	opcodeText   byte = 0x1
	opcodeBinary      = 0x2
	opcodeClose       = 0x8
	opcodePing        = 0x9
	opcodePong        = 0xA
)

func readFrame(reader *bufio.Reader) (frame, error) {
	first, err := reader.ReadByte()
	if err != nil {
		return frame{}, err
	}
	second, err := reader.ReadByte()
	if err != nil {
		return frame{}, err
	}
	fin := first&0x80 != 0
	opcode := first & 0x0F
	masked := second&0x80 != 0
	length := int(second & 0x7F)
	switch length {
	case 126:
		buf := make([]byte, 2)
		if _, err := io.ReadFull(reader, buf); err != nil {
			return frame{}, err
		}
		length = int(buf[0])<<8 | int(buf[1])
	case 127:
		buf := make([]byte, 8)
		if _, err := io.ReadFull(reader, buf); err != nil {
			return frame{}, err
		}
		length = int(buf[0])<<56 | int(buf[1])<<48 | int(buf[2])<<40 | int(buf[3])<<32 |
			int(buf[4])<<24 | int(buf[5])<<16 | int(buf[6])<<8 | int(buf[7])
	}
	var maskKey [4]byte
	if masked {
		if _, err := io.ReadFull(reader, maskKey[:]); err != nil {
			return frame{}, err
		}
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return frame{}, err
	}
	if masked {
		for i := 0; i < length; i++ {
			payload[i] ^= maskKey[i%4]
		}
	}
	return frame{fin: fin, opcode: opcode, payload: payload}, nil
}

// ErrUnexpectedFrame is returned when the connection encounters malformed
// frames.
var ErrUnexpectedFrame = errors.New("unexpected websocket frame")

package redis

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

type UniversalOptions struct {
	Addrs      []string
	MasterName string
	Username   string
	Password   string
	DB         int
	TLSConfig  *tls.Config

	DialTimeout     time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	PoolSize        int
	MaxRetries      int
	MinRetryBackoff time.Duration
	MaxRetryBackoff time.Duration
}

type Options struct {
	Addr      string
	Username  string
	Password  string
	DB        int
	TLSConfig *tls.Config

	DialTimeout     time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	PoolSize        int
	MaxRetries      int
	MinRetryBackoff time.Duration
	MaxRetryBackoff time.Duration
}

type UniversalClient interface {
	Close() error
	Do(ctx context.Context, args ...interface{}) (interface{}, error)
}

type Client struct {
	opt    Options
	pool   *connPool
	mu     sync.Mutex
	closed bool
}

func NewUniversalClient(opt *UniversalOptions) (UniversalClient, error) {
	if opt == nil {
		return nil, errors.New("redis: options required")
	}
	if len(opt.Addrs) == 0 {
		return nil, errors.New("redis: at least one address is required")
	}
	baseOpt := Options{
		Username:        opt.Username,
		Password:        opt.Password,
		DB:              opt.DB,
		TLSConfig:       opt.TLSConfig,
		DialTimeout:     opt.DialTimeout,
		ReadTimeout:     opt.ReadTimeout,
		WriteTimeout:    opt.WriteTimeout,
		PoolSize:        opt.PoolSize,
		MaxRetries:      opt.MaxRetries,
		MinRetryBackoff: opt.MinRetryBackoff,
		MaxRetryBackoff: opt.MaxRetryBackoff,
	}
	if opt.MasterName != "" {
		addr, err := resolveSentinel(context.Background(), opt.Addrs, opt.MasterName, baseOpt)
		if err != nil {
			return nil, err
		}
		baseOpt.Addr = addr
		return NewClient(&baseOpt), nil
	}
	if len(opt.Addrs) == 1 {
		baseOpt.Addr = opt.Addrs[0]
		return NewClient(&baseOpt), nil
	}
	// Very small cluster support: pick random node per dial.
	baseOpt.Addr = opt.Addrs[rand.Intn(len(opt.Addrs))]
	pool := newConnPool(func(ctx context.Context) (*conn, error) {
		addr := opt.Addrs[rand.Intn(len(opt.Addrs))]
		baseOpt.Addr = addr
		return dialConn(ctx, baseOpt)
	}, baseOpt.poolSize(), baseOpt.ReadTimeout, baseOpt.WriteTimeout)
	return &Client{opt: baseOpt, pool: pool}, nil
}

func NewClient(opt *Options) *Client {
	if opt == nil {
		panic("redis: options required")
	}
	pool := newConnPool(func(ctx context.Context) (*conn, error) {
		return dialConn(ctx, *opt)
	}, opt.poolSize(), opt.ReadTimeout, opt.WriteTimeout)
	return &Client{opt: *opt, pool: pool}
}

func (o Options) poolSize() int {
	if o.PoolSize > 0 {
		return o.PoolSize
	}
	return 16
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	if c.pool != nil {
		c.pool.close()
	}
	return nil
}

func (c *Client) Do(ctx context.Context, args ...interface{}) (interface{}, error) {
	if len(args) == 0 {
		return nil, errors.New("redis: missing command")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var lastErr error
	maxRetries := c.opt.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			sleep := backoffDuration(attempt, c.opt.MinRetryBackoff, c.opt.MaxRetryBackoff)
			if sleep > 0 {
				timer := time.NewTimer(sleep)
				select {
				case <-ctx.Done():
					timer.Stop()
					return nil, ctx.Err()
				case <-timer.C:
				}
			}
		}
		conn, err := c.pool.get(ctx)
		if err != nil {
			lastErr = err
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		reply, err := conn.do(ctx, args...)
		c.pool.put(conn, err == nil)
		if err == nil {
			return reply, nil
		}
		if errors.Is(err, io.EOF) || isNetworkError(err) {
			lastErr = err
			continue
		}
		return nil, err
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("redis: command failed")
}

type connPool struct {
	dial         func(ctx context.Context) (*conn, error)
	conns        chan *conn
	mu           sync.Mutex
	closed       bool
	readTimeout  time.Duration
	writeTimeout time.Duration
}

func newConnPool(dial func(ctx context.Context) (*conn, error), size int, readTimeout, writeTimeout time.Duration) *connPool {
	if size <= 0 {
		size = 16
	}
	return &connPool{
		dial:         dial,
		conns:        make(chan *conn, size),
		readTimeout:  readTimeout,
		writeTimeout: writeTimeout,
	}
}

func (p *connPool) get(ctx context.Context) (*conn, error) {
	select {
	case conn, ok := <-p.conns:
		if !ok {
			return nil, errors.New("redis: pool closed")
		}
		return conn, nil
	default:
	}
	return p.dial(ctx)
}

func (p *connPool) put(conn *conn, ok bool) {
	if conn == nil {
		return
	}
	if !ok {
		conn.close()
		return
	}
	select {
	case p.conns <- conn:
	default:
		conn.close()
	}
}

func (p *connPool) close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	close(p.conns)
	p.mu.Unlock()
	for conn := range p.conns {
		conn.close()
	}
}

type conn struct {
	netConn      net.Conn
	reader       *bufio.Reader
	writer       *bufio.Writer
	readTimeout  time.Duration
	writeTimeout time.Duration
}

func dialConn(ctx context.Context, opt Options) (*conn, error) {
	dialer := net.Dialer{Timeout: opt.DialTimeout}
	if opt.DialTimeout <= 0 {
		dialer.Timeout = 5 * time.Second
	}
	var rawConn net.Conn
	var err error
	if opt.TLSConfig != nil {
		tlsCfg := opt.TLSConfig.Clone()
		if tlsCfg.ServerName == "" {
			host := opt.Addr
			if h, _, splitErr := net.SplitHostPort(opt.Addr); splitErr == nil {
				host = h
			}
			tlsCfg.ServerName = host
		}
		rawConn, err = tls.DialWithDialer(&dialer, "tcp", opt.Addr, tlsCfg)
	} else {
		rawConn, err = dialer.DialContext(ctx, "tcp", opt.Addr)
	}
	if err != nil {
		return nil, err
	}
	c := &conn{netConn: rawConn, reader: bufio.NewReader(rawConn), writer: bufio.NewWriter(rawConn), readTimeout: opt.ReadTimeout, writeTimeout: opt.WriteTimeout}
	if err := c.init(ctx, opt); err != nil {
		rawConn.Close()
		return nil, err
	}
	return c, nil
}

func (c *conn) init(ctx context.Context, opt Options) error {
	if opt.Username != "" || opt.Password != "" {
		args := []interface{}{"AUTH"}
		if opt.Username != "" {
			args = append(args, opt.Username, opt.Password)
		} else {
			args = append(args, opt.Password)
		}
		if _, err := c.do(ctx, args...); err != nil {
			return err
		}
	}
	if opt.DB > 0 {
		if _, err := c.do(ctx, "SELECT", strconv.Itoa(opt.DB)); err != nil {
			return err
		}
	}
	return nil
}

func (c *conn) do(ctx context.Context, args ...interface{}) (interface{}, error) {
	if err := c.write(ctx, args...); err != nil {
		return nil, err
	}
	return c.read(ctx)
}

func (c *conn) write(ctx context.Context, args ...interface{}) error {
	if c.netConn == nil {
		return errors.New("redis: connection closed")
	}
	if deadline := deadlineFromContext(ctx, c.writeTimeout); !deadline.IsZero() {
		if err := c.netConn.SetWriteDeadline(deadline); err != nil {
			return err
		}
	}
	if err := writeRESP(c.writer, args...); err != nil {
		c.close()
		return err
	}
	return nil
}

func (c *conn) read(ctx context.Context) (interface{}, error) {
	if c.netConn == nil {
		return nil, errors.New("redis: connection closed")
	}
	if deadline := deadlineFromContext(ctx, c.readTimeout); !deadline.IsZero() {
		if err := c.netConn.SetReadDeadline(deadline); err != nil {
			return nil, err
		}
	}
	reply, err := parseRESP(c.reader)
	if err != nil {
		if errors.Is(err, io.EOF) {
			c.close()
		}
		return nil, err
	}
	return reply, nil
}

func (c *conn) close() {
	if c.netConn != nil {
		_ = c.netConn.Close()
	}
	c.netConn = nil
	c.reader = nil
	c.writer = nil
}

func resolveSentinel(ctx context.Context, addrs []string, master string, opt Options) (string, error) {
	for _, addr := range addrs {
		dialOpt := opt
		dialOpt.Addr = addr
		conn, err := dialConn(ctx, dialOpt)
		if err != nil {
			continue
		}
		reply, err := conn.do(ctx, "SENTINEL", "get-master-addr-by-name", master)
		conn.close()
		if err != nil {
			continue
		}
		array, ok := reply.([]interface{})
		if !ok || len(array) < 2 {
			continue
		}
		host, _ := asString(array[0])
		port, _ := asString(array[1])
		if host == "" || port == "" {
			continue
		}
		return net.JoinHostPort(host, port), nil
	}
	return "", fmt.Errorf("redis: sentinel master %s not found", master)
}

func backoffDuration(attempt int, min, max time.Duration) time.Duration {
	if min <= 0 {
		min = 50 * time.Millisecond
	}
	if max <= 0 {
		max = 500 * time.Millisecond
	}
	if min > max {
		min, max = max, min
	}
	delta := max - min
	if delta <= 0 {
		return min
	}
	jitter := time.Duration(rand.Int63n(int64(delta)))
	return min + jitter
}

func deadlineFromContext(ctx context.Context, timeout time.Duration) time.Time {
	if ctx == nil {
		if timeout > 0 {
			return time.Now().Add(timeout)
		}
		return time.Time{}
	}
	if dl, ok := ctx.Deadline(); ok {
		return dl
	}
	if timeout > 0 {
		return time.Now().Add(timeout)
	}
	return time.Time{}
}

func isNetworkError(err error) bool {
	if err == nil {
		return false
	}
	if _, ok := err.(net.Error); ok {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "network") || strings.Contains(msg, "connection")
}

func asString(v interface{}) (string, bool) {
	switch val := v.(type) {
	case string:
		return val, true
	case []byte:
		return string(val), true
	default:
		return "", false
	}
}

func writeRESP(w *bufio.Writer, args ...interface{}) error {
	if len(args) == 0 {
		return errors.New("redis: missing command")
	}
	if _, err := fmt.Fprintf(w, "*%d\r\n", len(args)); err != nil {
		return err
	}
	for _, arg := range args {
		switch v := arg.(type) {
		case string:
			if err := writeBulkString(w, v); err != nil {
				return err
			}
		case []byte:
			if err := writeBulkBytes(w, v); err != nil {
				return err
			}
		case fmt.Stringer:
			if err := writeBulkString(w, v.String()); err != nil {
				return err
			}
		default:
			if err := writeBulkString(w, fmt.Sprint(v)); err != nil {
				return err
			}
		}
	}
	return w.Flush()
}

func writeBulkString(w *bufio.Writer, s string) error {
	if _, err := fmt.Fprintf(w, "$%d\r\n%s\r\n", len(s), s); err != nil {
		return err
	}
	return nil
}

func writeBulkBytes(w *bufio.Writer, b []byte) error {
	if _, err := fmt.Fprintf(w, "$%d\r\n", len(b)); err != nil {
		return err
	}
	if _, err := w.Write(b); err != nil {
		return err
	}
	if _, err := w.WriteString("\r\n"); err != nil {
		return err
	}
	return nil
}

func parseRESP(r *bufio.Reader) (interface{}, error) {
	prefix, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	switch prefix {
	case '+':
		line, err := readLine(r)
		if err != nil {
			return nil, err
		}
		return line, nil
	case '-':
		line, err := readLine(r)
		if err != nil {
			return nil, err
		}
		return nil, errors.New(line)
	case ':':
		line, err := readLine(r)
		if err != nil {
			return nil, err
		}
		value, convErr := strconv.ParseInt(line, 10, 64)
		if convErr != nil {
			return nil, convErr
		}
		return value, nil
	case '$':
		lengthStr, err := readLine(r)
		if err != nil {
			return nil, err
		}
		length, convErr := strconv.Atoi(lengthStr)
		if convErr != nil {
			return nil, convErr
		}
		if length == -1 {
			return nil, nil
		}
		buf := make([]byte, length+2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		return string(buf[:length]), nil
	case '*':
		lengthStr, err := readLine(r)
		if err != nil {
			return nil, err
		}
		length, convErr := strconv.Atoi(lengthStr)
		if convErr != nil {
			return nil, convErr
		}
		if length == -1 {
			return nil, nil
		}
		items := make([]interface{}, 0, length)
		for i := 0; i < length; i++ {
			item, err := parseRESP(r)
			if err != nil {
				return nil, err
			}
			items = append(items, item)
		}
		return items, nil
	default:
		return nil, fmt.Errorf("redis: unexpected prefix %q", prefix)
	}
}

func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimSuffix(line, "\n")
	line = strings.TrimSuffix(line, "\r")
	return line, nil
}

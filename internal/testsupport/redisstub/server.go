package redisstub

import (
	"bufio"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Options struct {
	Password  string
	EnableTLS bool
}

type Server struct {
	opts     Options
	listener net.Listener
	addr     string
	mu       sync.Mutex
	streams  map[string]*redisStream
	kv       map[string]*kvEntry
	closed   chan struct{}
	tlsCert  tls.Certificate
	certPEM  []byte
	keyPEM   []byte
}

type redisStream struct {
	entries []streamEntry
	groups  map[string]*groupState
}

type streamEntry struct {
	id     string
	values map[string]string
}

type groupState struct {
	nextIndex int
	pending   map[string]struct{}
}

type kvEntry struct {
	value  int64
	expiry time.Time
}

func Start(opts Options) (*Server, error) {
	var ln net.Listener
	var err error
	server := &Server{
		opts:    opts,
		streams: make(map[string]*redisStream),
		kv:      make(map[string]*kvEntry),
		closed:  make(chan struct{}),
	}
	addr := "127.0.0.1:0"
	if opts.EnableTLS {
		certPEM, keyPEM, cert, err := generateSelfSignedCert()
		if err != nil {
			return nil, err
		}
		server.tlsCert = cert
		server.certPEM = certPEM
		server.keyPEM = keyPEM
		tlsCfg := &tls.Config{Certificates: []tls.Certificate{cert}}
		ln, err = tls.Listen("tcp", addr, tlsCfg)
	} else {
		ln, err = net.Listen("tcp", addr)
	}
	if err != nil {
		return nil, err
	}
	server.listener = ln
	server.addr = ln.Addr().String()
	go server.serve()
	return server, nil
}

func (s *Server) Addr() string {
	return s.addr
}

func (s *Server) CertPEM() []byte {
	return s.certPEM
}

func (s *Server) KeyPEM() []byte {
	return s.keyPEM
}

func (s *Server) Close() error {
	s.mu.Lock()
	select {
	case <-s.closed:
		s.mu.Unlock()
		return nil
	default:
	}
	close(s.closed)
	s.mu.Unlock()
	if s.listener != nil {
		_ = s.listener.Close()
	}
	return nil
}

func (s *Server) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.closed:
				return
			default:
			}
			continue
		}
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	authenticated := s.opts.Password == ""
	for {
		args, err := readArray(reader)
		if err != nil {
			return
		}
		if len(args) == 0 {
			writeError(writer, "ERR wrong number of arguments")
			continue
		}
		cmd := strings.ToUpper(args[0])
		switch cmd {
		case "PING":
			if err := writeSimpleString(writer, "PONG"); err != nil {
				return
			}
		case "AUTH":
			if len(args) == 2 {
				if s.opts.Password != "" && args[1] == s.opts.Password {
					authenticated = true
					if err := writeSimpleString(writer, "OK"); err != nil {
						return
					}
				} else if s.opts.Password == "" {
					authenticated = true
					if err := writeSimpleString(writer, "OK"); err != nil {
						return
					}
				} else {
					if err := writeError(writer, "WRONGPASS invalid username-password pair"); err != nil {
						return
					}
				}
			} else if len(args) == 3 {
				if s.opts.Password != "" && args[2] == s.opts.Password {
					authenticated = true
					if err := writeSimpleString(writer, "OK"); err != nil {
						return
					}
				} else {
					if err := writeError(writer, "WRONGPASS invalid username-password pair"); err != nil {
						return
					}
				}
			} else {
				if err := writeError(writer, "ERR wrong number of arguments for 'auth'"); err != nil {
					return
				}
			}
		case "SELECT":
			if err := writeSimpleString(writer, "OK"); err != nil {
				return
			}
		default:
			if !authenticated {
				if err := writeError(writer, "NOAUTH Authentication required."); err != nil {
					return
				}
				continue
			}
			if !s.dispatch(writer, args) {
				return
			}
		}
	}
}

func (s *Server) dispatch(writer *bufio.Writer, args []string) bool {
	if len(args) == 0 {
		_ = writeError(writer, "ERR command not provided")
		return false
	}
	cmd := strings.ToUpper(args[0])
	switch cmd {
	case "XADD":
		if len(args) < 5 {
			_ = writeError(writer, "ERR wrong number of arguments for 'xadd'")
			return false
		}
		stream := args[1]
		id := args[2]
		if id == "*" {
			id = fmt.Sprintf("%d-0", time.Now().UnixNano())
		}
		values := make(map[string]string)
		for i := 3; i+1 < len(args); i += 2 {
			values[args[i]] = args[i+1]
		}
		s.mu.Lock()
		strm := s.ensureStream(stream)
		strm.entries = append(strm.entries, streamEntry{id: id, values: values})
		s.mu.Unlock()
		if err := writeBulkString(writer, id); err != nil {
			return false
		}
		return true
	case "XGROUP":
		if len(args) < 5 {
			_ = writeError(writer, "ERR wrong number of arguments for 'xgroup'")
			return false
		}
		action := strings.ToUpper(args[1])
		if action != "CREATE" {
			_ = writeError(writer, "ERR only CREATE supported")
			return false
		}
		stream := args[2]
		group := args[3]
		s.mu.Lock()
		strm := s.ensureStream(stream)
		if strm.groups == nil {
			strm.groups = make(map[string]*groupState)
		}
		if _, exists := strm.groups[group]; exists {
			s.mu.Unlock()
			_ = writeError(writer, "BUSYGROUP Consumer Group name already exists")
			return false
		}
		strm.groups[group] = &groupState{pending: make(map[string]struct{})}
		s.mu.Unlock()
		if err := writeSimpleString(writer, "OK"); err != nil {
			return false
		}
		return true
	case "XREADGROUP":
		return s.handleXReadGroup(writer, args)
	case "XACK":
		if len(args) < 4 {
			_ = writeError(writer, "ERR wrong number of arguments for 'xack'")
			return false
		}
		stream := args[1]
		group := args[2]
		ids := args[3:]
		acked := s.ack(stream, group, ids)
		if err := writeInteger(writer, int64(acked)); err != nil {
			return false
		}
		return true
	case "INCR":
		if len(args) != 2 {
			_ = writeError(writer, "ERR wrong number of arguments for 'incr'")
			return false
		}
		value := s.incr(args[1])
		if err := writeInteger(writer, value); err != nil {
			return false
		}
		return true
	case "EXPIRE":
		if len(args) != 3 {
			_ = writeError(writer, "ERR wrong number of arguments for 'expire'")
			return false
		}
		seconds, err := strconv.ParseInt(args[2], 10, 64)
		if err != nil {
			_ = writeError(writer, "ERR invalid expire time")
			return false
		}
		s.expire(args[1], time.Duration(seconds)*time.Second)
		if err := writeInteger(writer, 1); err != nil {
			return false
		}
		return true
	case "TTL":
		if len(args) != 2 {
			_ = writeError(writer, "ERR wrong number of arguments for 'ttl'")
			return false
		}
		ttl := s.ttl(args[1])
		if err := writeInteger(writer, ttl); err != nil {
			return false
		}
		return true
	default:
		_ = writeError(writer, "ERR unsupported command")
		return false
	}
}

func (s *Server) ensureStream(name string) *redisStream {
	strm, ok := s.streams[name]
	if !ok {
		strm = &redisStream{}
		s.streams[name] = strm
	}
	if strm.groups == nil {
		strm.groups = make(map[string]*groupState)
	}
	return strm
}

func (s *Server) handleXReadGroup(writer *bufio.Writer, args []string) bool {
	if len(args) < 6 {
		_ = writeError(writer, "ERR wrong number of arguments for 'xreadgroup'")
		return false
	}
	var group, stream string
	count := 1
	blockMs := 0
	for i := 1; i < len(args); i++ {
		token := strings.ToUpper(args[i])
		switch token {
		case "GROUP":
			if i+2 >= len(args) {
				_ = writeError(writer, "ERR syntax error")
				return false
			}
			group = args[i+1]
			_ = args[i+2]
			i += 2
		case "COUNT":
			if i+1 >= len(args) {
				_ = writeError(writer, "ERR syntax error")
				return false
			}
			v, err := strconv.Atoi(args[i+1])
			if err != nil {
				_ = writeError(writer, "ERR invalid COUNT")
				return false
			}
			count = v
			i++
		case "BLOCK":
			if i+1 >= len(args) {
				_ = writeError(writer, "ERR syntax error")
				return false
			}
			v, err := strconv.Atoi(args[i+1])
			if err != nil {
				_ = writeError(writer, "ERR invalid BLOCK")
				return false
			}
			blockMs = v
			i++
		case "STREAMS":
			if i+2 >= len(args) {
				_ = writeError(writer, "ERR syntax error")
				return false
			}
			stream = args[i+1]
			i = len(args)
		}
	}
	if stream == "" || group == "" {
		_ = writeError(writer, "ERR missing stream or group")
		return false
	}
	deadline := time.Now().Add(time.Duration(blockMs) * time.Millisecond)
	for {
		items := s.readGroup(stream, group, count)
		if len(items) > 0 {
			if err := writeArray(writer, []interface{}{items}); err != nil {
				return false
			}
			return true
		}
		if blockMs <= 0 || time.Now().After(deadline) {
			if err := writeBulkNil(writer); err != nil {
				return false
			}
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (s *Server) readGroup(stream, group string, count int) []interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	strm := s.ensureStream(stream)
	state, ok := strm.groups[group]
	if !ok {
		state = &groupState{pending: make(map[string]struct{})}
		strm.groups[group] = state
	}
	start := state.nextIndex
	if start >= len(strm.entries) {
		return nil
	}
	end := start + count
	if end > len(strm.entries) {
		end = len(strm.entries)
	}
	records := make([]interface{}, 0, end-start)
	for i := start; i < end; i++ {
		entry := strm.entries[i]
		state.pending[entry.id] = struct{}{}
		records = append(records, []interface{}{
			entry.id,
			flatten(entry.values),
		})
	}
	state.nextIndex = end
	return []interface{}{stream, records}
}

func flatten(values map[string]string) []interface{} {
	out := make([]interface{}, 0, len(values)*2)
	for k, v := range values {
		out = append(out, k, v)
	}
	return out
}

func (s *Server) ack(stream, group string, ids []string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	strm, ok := s.streams[stream]
	if !ok {
		return 0
	}
	state, ok := strm.groups[group]
	if !ok {
		return 0
	}
	count := 0
	for _, id := range ids {
		if _, exists := state.pending[id]; exists {
			delete(state.pending, id)
			count++
		}
	}
	return count
}

func (s *Server) incr(key string) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.kv[key]
	if entry == nil || (entry.expiry.After(time.Time{}) && time.Now().After(entry.expiry)) {
		entry = &kvEntry{}
		s.kv[key] = entry
	}
	entry.value++
	return entry.value
}

func (s *Server) expire(key string, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.kv[key]
	if entry == nil {
		entry = &kvEntry{}
		s.kv[key] = entry
	}
	entry.expiry = time.Now().Add(ttl)
}

func (s *Server) ttl(key string) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.kv[key]
	if entry == nil || entry.expiry.Equal(time.Time{}) {
		return -1
	}
	remaining := time.Until(entry.expiry)
	if remaining <= 0 {
		delete(s.kv, key)
		return -2
	}
	return int64(remaining / time.Second)
}

func generateSelfSignedCert() ([]byte, []byte, tls.Certificate, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, tls.Certificate{}, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"127.0.0.1", "localhost"},
	}
	tmpl.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
	derBytes, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, tls.Certificate{}, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, nil, tls.Certificate{}, err
	}
	return certPEM, keyPEM, cert, nil
}

func readArray(r *bufio.Reader) ([]string, error) {
	prefix, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	if prefix != '*' {
		return nil, fmt.Errorf("unexpected prefix %q", prefix)
	}
	length, err := readLength(r)
	if err != nil {
		return nil, err
	}
	args := make([]string, 0, length)
	for i := 0; i < length; i++ {
		arg, err := readBulkString(r)
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
	}
	return args, nil
}

func readLength(r *bufio.Reader) (int, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return 0, err
	}
	line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	return strconv.Atoi(line)
}

func readBulkString(r *bufio.Reader) (string, error) {
	prefix, err := r.ReadByte()
	if err != nil {
		return "", err
	}
	if prefix != '$' {
		return "", fmt.Errorf("unexpected prefix %q", prefix)
	}
	length, err := readLength(r)
	if err != nil {
		return "", err
	}
	if length < 0 {
		return "", nil
	}
	buf := make([]byte, length+2)
	if _, err := r.Read(buf); err != nil {
		return "", err
	}
	return string(buf[:length]), nil
}

func writeSimpleString(w *bufio.Writer, value string) error {
	if _, err := fmt.Fprintf(w, "+%s\r\n", value); err != nil {
		return err
	}
	return w.Flush()
}

func writeBulkString(w *bufio.Writer, value string) error {
	if _, err := fmt.Fprintf(w, "$%d\r\n%s\r\n", len(value), value); err != nil {
		return err
	}
	return w.Flush()
}

func writeBulkNil(w *bufio.Writer) error {
	if _, err := w.WriteString("$-1\r\n"); err != nil {
		return err
	}
	return w.Flush()
}

func writeInteger(w *bufio.Writer, value int64) error {
	if _, err := fmt.Fprintf(w, ":%d\r\n", value); err != nil {
		return err
	}
	return w.Flush()
}

func writeArray(w *bufio.Writer, values []interface{}) error {
	if _, err := fmt.Fprintf(w, "*%d\r\n", len(values)); err != nil {
		return err
	}
	for _, value := range values {
		switch v := value.(type) {
		case string:
			if err := writeBulkStringRaw(w, v); err != nil {
				return err
			}
		case []byte:
			if err := writeBulkBytesRaw(w, v); err != nil {
				return err
			}
		case int64:
			if err := writeIntegerRaw(w, v); err != nil {
				return err
			}
		case []interface{}:
			if err := writeArray(w, v); err != nil {
				return err
			}
		default:
			if err := writeBulkStringRaw(w, fmt.Sprint(v)); err != nil {
				return err
			}
		}
	}
	return w.Flush()
}

func writeBulkStringRaw(w *bufio.Writer, value string) error {
	if _, err := fmt.Fprintf(w, "$%d\r\n%s\r\n", len(value), value); err != nil {
		return err
	}
	return nil
}

func writeBulkBytesRaw(w *bufio.Writer, value []byte) error {
	if _, err := fmt.Fprintf(w, "$%d\r\n", len(value)); err != nil {
		return err
	}
	if _, err := w.Write(value); err != nil {
		return err
	}
	if _, err := w.WriteString("\r\n"); err != nil {
		return err
	}
	return nil
}

func writeIntegerRaw(w *bufio.Writer, value int64) error {
	if _, err := fmt.Fprintf(w, ":%d\r\n", value); err != nil {
		return err
	}
	return nil
}

func writeError(w *bufio.Writer, msg string) error {
	if _, err := fmt.Fprintf(w, "-%s\r\n", msg); err != nil {
		return err
	}
	return w.Flush()
}

package server

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

type redisStore struct {
	addr     string
	password string
	timeout  time.Duration
}

func newRedisStore(addr, password string, timeout time.Duration) *redisStore {
	return &redisStore{addr: addr, password: password, timeout: timeout}
}

func (s *redisStore) Allow(key string, limit int, window time.Duration) (bool, time.Duration, error) {
	conn, err := net.DialTimeout("tcp", s.addr, s.timeout)
	if err != nil {
		return false, 0, err
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	if s.password != "" {
		if err := writeCommand(writer, "AUTH", s.password); err != nil {
			return false, 0, err
		}
		if _, err := readReply(reader); err != nil {
			return false, 0, err
		}
	}

	if err := writeCommand(writer, "INCR", key); err != nil {
		return false, 0, err
	}
	countReply, err := readReply(reader)
	if err != nil {
		return false, 0, err
	}
	count, err := asInt(countReply)
	if err != nil {
		return false, 0, err
	}
	if count == 1 {
		seconds := int64(window / time.Second)
		if seconds <= 0 {
			seconds = 1
		}
		if err := writeCommand(writer, "EXPIRE", key, strconv.FormatInt(seconds, 10)); err != nil {
			return false, 0, err
		}
		if _, err := readReply(reader); err != nil {
			return false, 0, err
		}
	}
	if count <= int64(limit) {
		return true, 0, nil
	}
	if err := writeCommand(writer, "TTL", key); err != nil {
		return false, 0, err
	}
	ttlReply, err := readReply(reader)
	if err != nil {
		return false, 0, err
	}
	ttl, err := asInt(ttlReply)
	if err != nil {
		return false, 0, err
	}
	if ttl < 0 {
		return false, window, nil
	}
	return false, time.Duration(ttl) * time.Second, nil
}

func writeCommand(w *bufio.Writer, args ...string) error {
	if len(args) == 0 {
		return errors.New("redis command requires arguments")
	}
	if _, err := fmt.Fprintf(w, "*%d\r\n", len(args)); err != nil {
		return err
	}
	for _, arg := range args {
		if _, err := fmt.Fprintf(w, "$%d\r\n%s\r\n", len(arg), arg); err != nil {
			return err
		}
	}
	return w.Flush()
}

func readReply(r *bufio.Reader) (interface{}, error) {
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
		return strconv.ParseInt(line, 10, 64)
	case '$':
		line, err := readLine(r)
		if err != nil {
			return nil, err
		}
		length, err := strconv.Atoi(line)
		if err != nil {
			return nil, err
		}
		if length < 0 {
			return nil, nil
		}
		buf := make([]byte, length+2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		return string(buf[:length]), nil
	default:
		return nil, fmt.Errorf("unexpected redis reply prefix %q", prefix)
	}
}

func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r"), nil
}

func asInt(v interface{}) (int64, error) {
	switch val := v.(type) {
	case int64:
		return val, nil
	case string:
		return strconv.ParseInt(val, 10, 64)
	case nil:
		return 0, errors.New("nil reply")
	default:
		return 0, fmt.Errorf("unexpected redis reply type %T", v)
	}
}

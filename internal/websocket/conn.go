package websocket

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
)

var errNotWebSocket = errors.New("not a websocket upgrade")

type Conn struct {
	rw     net.Conn
	reader *bufio.Reader
	mu     sync.Mutex
}

func Upgrade(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return nil, errNotWebSocket
	}
	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		return nil, errors.New("missing websocket key")
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, errors.New("hijacking unsupported")
	}
	conn, rw, err := hj.Hijack()
	if err != nil {
		return nil, err
	}
	accept := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	resp := "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: " + base64.StdEncoding.EncodeToString(accept[:]) + "\r\n\r\n"
	if _, err = rw.WriteString(resp); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err = rw.Flush(); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return &Conn{rw: conn, reader: bufio.NewReader(conn)}, nil
}

func (c *Conn) Close() error { return c.rw.Close() }

func (c *Conn) ReadText() ([]byte, error) {
	return readFrame(c.reader)
}

func readFrame(br *bufio.Reader) ([]byte, error) {
	h1, err := br.ReadByte()
	if err != nil {
		return nil, err
	}
	h2, err := br.ReadByte()
	if err != nil {
		return nil, err
	}
	opcode := h1 & 0x0f
	fin := h1&0x80 != 0
	masked := h2&0x80 != 0
	if !fin || opcode == 0x8 {
		return nil, io.EOF
	}
	if opcode != 0x1 {
		return nil, errors.New("only text websocket frames are supported")
	}
	length := uint64(h2 & 0x7f)
	if length == 126 {
		var b [2]byte
		if _, err = io.ReadFull(br, b[:]); err != nil {
			return nil, err
		}
		length = uint64(binary.BigEndian.Uint16(b[:]))
	}
	if length == 127 {
		var b [8]byte
		if _, err = io.ReadFull(br, b[:]); err != nil {
			return nil, err
		}
		length = binary.BigEndian.Uint64(b[:])
	}
	if length > 1<<20 {
		return nil, errors.New("message too large")
	}
	var mask [4]byte
	if masked {
		if _, err = io.ReadFull(br, mask[:]); err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("client frame must be masked")
	}
	payload := make([]byte, length)
	if _, err = io.ReadFull(br, payload); err != nil {
		return nil, err
	}
	for i := range payload {
		payload[i] ^= mask[i%4]
	}
	return payload, nil
}

func (c *Conn) WriteText(payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(payload) > 65535 {
		return errors.New("message too large")
	}
	var hdr []byte
	if len(payload) < 126 {
		hdr = []byte{0x81, byte(len(payload))}
	} else {
		hdr = []byte{0x81, 126, byte(len(payload) >> 8), byte(len(payload))}
	}
	if _, err := c.rw.Write(hdr); err != nil {
		return err
	}
	_, err := c.rw.Write(payload)
	return err
}

func (c *Conn) String() string { return fmt.Sprintf("websocket(%p)", c) }

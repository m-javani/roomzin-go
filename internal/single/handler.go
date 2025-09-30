// // SPDX-License-Identifier: BUSL-1.1
// // Copyright (c) 2026 M. Javani
// //
// // This file is part of roomzin-go.
// //
// // Use of this software is governed by the Business Source License 1.1
// // included in the LICENSE file in the root of this repository.

package single

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/roomzin/roomzin-go/internal/protocol"
)

type Config struct {
	Addr      string
	TCPPort   int
	AuthToken string
	Timeout   time.Duration
	KeepAlive time.Duration
}

type Handler struct {
	config *Config
	conn   net.Conn
	next   uint32

	mu          sync.Mutex
	closed      bool
	demux       map[uint32]chan protocol.RawResult
	ctx         context.Context
	OnReconnect func()
}

func NewHandler(cfg *Config, ctx context.Context) (*Handler, error) {
	c := &Handler{
		config: cfg,
		demux:  make(map[uint32]chan protocol.RawResult),
		ctx:    ctx,
	}
	if err := c.reconnect(); err != nil { // first dial
		return nil, err
	}
	return c, nil
}

func (c *Handler) reconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		_ = c.conn.Close()
	}
	host := ParseHost(c.config.Addr)
	addr := net.JoinHostPort(host, strconv.Itoa(int(c.config.TCPPort)))
	conn, err := dial(addr, c.config.AuthToken, c.config.Timeout, c.config.KeepAlive)
	if err != nil {
		return err
	}
	c.conn = conn
	// ----  start reader exactly here ----
	go c.readLoop()
	return nil
}

func dial(addr string, token string, timeout, keepAlive time.Duration) (*net.TCPConn, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, err
	}
	dialer := &net.Dialer{Timeout: timeout, KeepAlive: keepAlive}
	conn, err := dialer.Dial("tcp", tcpAddr.String())
	if err != nil {
		return nil, err
	}
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		conn.Close()
		return nil, fmt.Errorf("unexpected connection type for %s", addr)
	}

	// check authentication
	if err := handshake(tcpConn, token, timeout); err != nil {
		return nil, fmt.Errorf("%v, failed to handshake to %s", err, addr)
	}

	tcpConn.SetKeepAlive(true)
	tcpConn.SetKeepAlivePeriod(keepAlive)
	return tcpConn, nil
}

func handshake(conn *net.TCPConn, token string, timeout time.Duration) error {
	_ = conn.SetDeadline(time.Now().Add(timeout))
	defer conn.SetDeadline(time.Time{})

	// 1. send framed login
	payload, _ := protocol.BuildLoginPayload(token)
	frame := protocol.PrependHeader(0, payload)
	if _, err := conn.Write(frame); err != nil {
		return err
	}

	// 2. read plain-text reply
	buf := make([]byte, 32) // 12/13 bytes is enough
	n, err := conn.Read(buf)
	if err != nil {
		return err
	}
	switch string(buf[:n]) {
	case "LOGIN OK":
		return nil
	case "LOGIN FAILED":
		return errors.New("AUTH_ERROR: invalid token")
	default:
		return fmt.Errorf("RESPONSE_ERROR: unexpected login reply %q", buf[:n])
	}
}

func (c *Handler) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	_ = c.conn.Close()
	for _, ch := range c.demux {
		close(ch)
	}

	return nil
}

func (c *Handler) NextID() uint32 { return atomic.AddUint32(&c.next, 1) }

func (c *Handler) RoundTrip(clrid uint32, payload []byte) (protocol.RawResult, error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return protocol.RawResult{}, protocol.ErrConnClosed
	}
	// ---  added self-heal  ---
	if c.conn == nil {
		c.mu.Unlock()
		if err := c.reconnect(); err != nil {
			return protocol.RawResult{}, err
		}
		c.mu.Lock()
	}
	// ---  end self-heal  ---

	ch := make(chan protocol.RawResult, 1)
	c.demux[clrid] = ch
	c.mu.Unlock()

	if _, err := c.conn.Write(protocol.PrependHeader(clrid, payload)); err != nil {
		c.cleanup(clrid)
		_ = c.reconnect() // mark bad, retry next call
		return protocol.RawResult{}, err
	}

	select {
	case res := <-ch:
		return res, nil
	case <-time.After(c.config.Timeout):
		c.cleanup(clrid)
		_ = c.reconnect()
		return protocol.RawResult{}, protocol.ErrTimeout
	}
}

func (c *Handler) cleanup(clrid uint32) {
	c.mu.Lock()
	delete(c.demux, clrid)
	c.mu.Unlock()
}

func (c *Handler) readLoop() {
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}
		hdr, payload, err := protocol.DrainFrame(c.conn)
		if err != nil {
			c.failAll(err)
			// Connection lost - invalidate codecs
			if c.OnReconnect != nil {
				c.OnReconnect() // This sets codecs = nil
			}
			return
		}
		fields, _ := protocol.ParseFields(payload[1+len(hdr.Status)+2:], hdr.FieldCnt)

		c.mu.Lock()
		ch, ok := c.demux[hdr.ClrID]
		delete(c.demux, hdr.ClrID)
		c.mu.Unlock()

		if ok {
			ch <- protocol.RawResult{Status: hdr.Status, Fields: fields}
			close(ch)
		}
	}
}

func (c *Handler) failAll(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, ch := range c.demux {
		ch <- protocol.RawResult{}
		close(ch)
	}
	for k := range c.demux {
		delete(c.demux, k)
	}
}

func ParseHost(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// If SplitHostPort fails, addr is host-only (no port)
		host = addr
	}
	return host
}

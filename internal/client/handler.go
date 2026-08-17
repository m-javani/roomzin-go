// // SPDX-License-Identifier: BUSL-1.1
// // Copyright (c) 2026 M. Javani
// //
// // This file is part of roomzin-go.
// //
// // Use of this software is governed by the Business Source License 1.1
// // included in the LICENSE file in the root of this repository.

package client

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/m-javani/roomzin-go/internal/protocol"
)

// ConnectionMode defines how the client connects
type ConnectionMode int

const (
	StandaloneMode ConnectionMode = iota
	ClusterMode
)

type Config struct {
	Addr      string
	Port      int
	Timeout   time.Duration
	KeepAlive time.Duration
	Mode      ConnectionMode
}

type Handler struct {
	config        *Config
	conn          net.Conn
	nextID        uint32
	mu            sync.Mutex
	closed        bool
	demux         map[uint32]chan protocol.RawResult
	ctx           context.Context
	cancel        context.CancelFunc
	OnReconnect   func()
	keepaliveStop chan struct{}
}

func NewHandler(cfg *Config, ctx context.Context) (*Handler, error) {
	ctx, cancel := context.WithCancel(ctx)

	h := &Handler{
		config: cfg,
		demux:  make(map[uint32]chan protocol.RawResult),
		ctx:    ctx,
		cancel: cancel,
	}

	if err := h.reconnect(); err != nil {
		cancel()
		return nil, err
	}

	if cfg.Mode == ClusterMode {
		h.startKeepalive()
	}

	return h, nil
}

// Add keepalive methods
func (h *Handler) startKeepalive() {
	h.keepaliveStop = make(chan struct{})
	go h.keepaliveLoop()
}

func (h *Handler) stopKeepalive() {
	if h.keepaliveStop != nil {
		close(h.keepaliveStop)
	}
}

func (h *Handler) keepaliveLoop() {
	ticker := time.NewTicker(h.config.KeepAlive)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Send keepalive with a dummy clrID (0)
			frame := protocol.BuildKeepaliveFrame(0)
			h.mu.Lock()
			if h.conn != nil && !h.closed {
				_, err := h.conn.Write(frame)
				if err != nil {
					// Connection dead - reconnect will handle it
					h.mu.Unlock()
					continue
				}
			}
			h.mu.Unlock()

		case <-h.ctx.Done():
			return
		case <-h.keepaliveStop:
			return
		}
	}
}

func (h *Handler) reconnect() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.conn != nil {
		_ = h.conn.Close()
	}

	addr := net.JoinHostPort(h.config.Addr, strconv.Itoa(h.config.Port))
	conn, err := dial(addr, h.config.Timeout, h.config.KeepAlive)
	if err != nil {
		return err
	}

	h.conn = conn
	go h.readLoop()
	return nil
}

func dial(addr string, timeout, keepAlive time.Duration) (*net.TCPConn, error) {
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

	tcpConn.SetKeepAlive(true)
	tcpConn.SetKeepAlivePeriod(keepAlive)
	return tcpConn, nil
}

func (h *Handler) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return nil
	}
	h.closed = true
	h.cancel()
	h.stopKeepalive()
	_ = h.conn.Close()

	for _, ch := range h.demux {
		close(ch)
	}

	return nil
}

func (h *Handler) NextID() uint32 {
	return atomic.AddUint32(&h.nextID, 1)
}

// Execute sends a command and waits for response
// For standalone mode: segment and isWrite are ignored
// For cluster mode: segment and isWrite are used for routing
func (h *Handler) Execute(ctx context.Context, segment string, isWrite bool, payload []byte) (protocol.RawResult, error) {
	clrID := h.NextID()

	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return protocol.RawResult{}, protocol.ErrConnClosed
	}

	// Self-heal
	if h.conn == nil {
		h.mu.Unlock()
		if err := h.reconnect(); err != nil {
			return protocol.RawResult{}, err
		}
		h.mu.Lock()
	}

	ch := make(chan protocol.RawResult, 1)
	h.demux[clrID] = ch
	h.mu.Unlock()

	// Build the appropriate frame
	var frame []byte
	if h.config.Mode == ClusterMode {
		frame = protocol.PrependRouterHeader(segment, isWrite, clrID, payload)
	} else {
		frame = protocol.PrependHeader(clrID, payload)
	}

	// Send
	if _, err := h.conn.Write(frame); err != nil {
		h.cleanup(clrID)
		_ = h.reconnect()
		return protocol.RawResult{}, err
	}

	// Wait for response
	select {
	case res := <-ch:
		return res, nil
	case <-ctx.Done():
		h.cleanup(clrID)
		return protocol.RawResult{}, ctx.Err()
	case <-time.After(h.config.Timeout):
		h.cleanup(clrID)
		_ = h.reconnect()
		return protocol.RawResult{}, protocol.ErrTimeout
	}
}

func (h *Handler) cleanup(clrID uint32) {
	h.mu.Lock()
	delete(h.demux, clrID)
	h.mu.Unlock()
}

func (h *Handler) readLoop() {
	for {
		select {
		case <-h.ctx.Done():
			return
		default:
		}

		hdr, payload, err := protocol.DrainFrame(h.conn)
		if err != nil {
			h.failAll(err)
			if h.OnReconnect != nil {
				h.OnReconnect()
			}
			return
		}

		fields, _ := protocol.ParseFields(payload[1+len(hdr.Status)+2:], hdr.FieldCnt)

		h.mu.Lock()
		ch, ok := h.demux[hdr.ClrID]
		delete(h.demux, hdr.ClrID)
		h.mu.Unlock()

		if ok {
			ch <- protocol.RawResult{Status: hdr.Status, Fields: fields}
			close(ch)
		}
	}
}

func (h *Handler) failAll(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, ch := range h.demux {
		ch <- protocol.RawResult{}
		close(ch)
	}
	for k := range h.demux {
		delete(h.demux, k)
	}
}

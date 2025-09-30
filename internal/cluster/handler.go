// // SPDX-License-Identifier: BUSL-1.1
// // Copyright (c) 2026 M. Javani
// //
// // This file is part of roomzin-go.
// //
// // Use of this software is governed by the Business Source License 1.1
// // included in the LICENSE file in the root of this repository.

package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/roomzin/roomzin-go/internal/protocol"
	"github.com/roomzin/roomzin-go/types"
)

type DiscoveryMap struct {
	mu   sync.RWMutex
	data map[string]resolvedAddr
}

type resolvedAddr struct {
	addr    string
	tcpPort int
	apiPort int
}

// Resolve returns address, tcp port and api port for a nodeID.
// port falls back to cfg ports if not specified.
func (dm *DiscoveryMap) Resolve(nodeID string) (addr string, tcpPort int, apiPort int, ok bool) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	ra, exists := dm.data[nodeID]
	if !exists {
		return "", 0, 0, false
	}
	return ra.addr, ra.tcpPort, ra.apiPort, true
}

type Config struct {
	SeedNodeIDs       string // "host1,host2,host3"  (NO port, NO zone, NO shard)
	APIPort           int    // HTTP port for /peers /leader /node-info
	TCPPort           int    // TCP port for framed protocol
	AuthToken         string
	Timeout           time.Duration
	HttpTimeout       time.Duration
	KeepAlive         time.Duration
	MaxActiveConns    int           // hard cap on open TCP connections
	NodeProbeInterval time.Duration // how often to health-check

	// Discovery settings
	DiscoveryAddr   string           // if set → HTTP discovery mode
	StaticDiscovery []types.NodeAddr // used only when DiscoveryAddr is empty
}

type Handler struct {
	cfg              *Config
	discovery        *DiscoveryMap
	leaderHandler    *leaderHandler
	followersHandler *followersHandler
	respChanPool     *sync.Pool
	reqPool          *sync.Pool
}

type request struct {
	payload  []byte
	respChan chan protocol.RawResult
	ctx      context.Context
	clrID    uint32
}

type leaderHandler struct {
	cfg         *Config
	discovery   *DiscoveryMap
	reqChan     chan *request
	conn        *connection
	connMu      sync.RWMutex
	clrID       uint32
	OnReconnect func()
}

type followersHandler struct {
	cfg         *Config
	discovery   *DiscoveryMap
	reqChan     chan *request
	connections []*connection
	connMutex   sync.RWMutex
	clrID       uint32
	rrIndex     atomic.Uint32
}

type demuxMap struct {
	mu      sync.RWMutex
	entries map[uint32]demuxEntry
}

type demuxEntry struct {
	ch        chan protocol.RawResult
	send_time time.Time
}

func NewHandler(cfg *Config) *Handler {
	dm := buildDiscoveryMap(cfg)

	h := &Handler{
		cfg:       cfg,
		discovery: dm,
		leaderHandler: &leaderHandler{
			cfg:       cfg,
			discovery: dm,
			reqChan:   make(chan *request, 1024),
			conn:      nil,
		},
		followersHandler: &followersHandler{
			cfg:         cfg,
			discovery:   dm,
			reqChan:     make(chan *request, 1024),
			connections: make([]*connection, 0),
		},
		respChanPool: &sync.Pool{
			New: func() any { return make(chan protocol.RawResult, 1) },
		},
		reqPool: &sync.Pool{
			New: func() any { return &request{} },
		},
	}

	return h
}

// buildDiscoveryMap creates and populates the initial discovery map
func buildDiscoveryMap(cfg *Config) *DiscoveryMap {
	dm := &DiscoveryMap{
		data: make(map[string]resolvedAddr),
	}

	if cfg.DiscoveryAddr != "" {
		// HTTP mode: start empty, populated later by background task
		return dm
	}

	// Static mode
	if len(cfg.StaticDiscovery) == 0 {
		panic("static discovery enabled but StaticDiscovery is empty") // or return error if you prefer
	}

	dm.mu.Lock()
	for _, n := range cfg.StaticDiscovery {
		tcpPort := cfg.TCPPort
		if n.TcpPort > 0 {
			tcpPort = n.TcpPort
		}
		apiPort := cfg.APIPort
		if n.ApiPort > 0 {
			apiPort = n.ApiPort
		}
		dm.data[n.NodeID] = resolvedAddr{
			addr:    n.Host,
			tcpPort: tcpPort,
			apiPort: apiPort,
		}
	}
	dm.mu.Unlock()

	return dm
}

// updateDiscoveryMap replaces the map content (used by HTTP discovery)
func (h *Handler) updateDiscoveryMap(nodes []types.NodeAddr) {
	newData := make(map[string]resolvedAddr, len(nodes))

	for _, n := range nodes {
		tcpPort := h.cfg.TCPPort
		if n.TcpPort > 0 {
			tcpPort = n.TcpPort
		}
		apiPort := h.cfg.APIPort
		if n.ApiPort > 0 {
			apiPort = n.ApiPort
		}
		newData[n.NodeID] = resolvedAddr{
			addr:    n.Host,
			tcpPort: tcpPort,
			apiPort: apiPort,
		}
	}

	h.discovery.mu.Lock()
	h.discovery.data = newData
	h.discovery.mu.Unlock()
}

func (c *Handler) SetOnReconnectCallback(callback func()) {
	c.leaderHandler.OnReconnect = callback
}

func (c *Handler) Start(ctx context.Context) {
	go c.startDiscoveryTask(ctx)

	go c.leaderHandler.LeaderSyncWorker(ctx)
	go c.followersHandler.FollowerSyncWorker(ctx)

	go c.leaderHandler.LeaderSendWorker(ctx)
	go c.followersHandler.FollowerSendWroker(ctx)

	go func() {
		t := time.NewTicker(c.cfg.Timeout)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				conn := c.leaderHandler.getConnection()
				if conn != nil && conn.demuxMap != nil {
					conn.demuxMap.Cleanup(c.cfg.Timeout * 2)
				}
			}
		}
	}()
}

// ========================================================
//   demuxMap – with TTL-based cleanup
// ========================================================

func (m *demuxMap) Store(clrID uint32, ch chan protocol.RawResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.entries == nil {
		m.entries = make(map[uint32]demuxEntry)
	}
	m.entries[clrID] = demuxEntry{ch: ch, send_time: time.Now()}
}

func (m *demuxMap) LoadRemove(clrID uint32) (chan protocol.RawResult, time.Time, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entries[clrID]
	if ok {
		delete(m.entries, clrID)
	}
	return e.ch, e.send_time, ok
}

func (m *demuxMap) Cleanup(maxAge time.Duration) {
	threshold := time.Now().Add(-maxAge)
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, e := range m.entries {
		if e.send_time.Before(threshold) {
			close(e.ch)
			delete(m.entries, id)
		}
	}
}

// ========================================================
//   connection
// ========================================================

type connection struct {
	netConn   net.Conn
	demuxMap  *demuxMap
	sendQueue chan []byte
	closer    sync.Once
	cfg       *Config
	addr      string
	closed    atomic.Bool
}

func newConnection(host string, port int, cfg *Config, dm *demuxMap) (*connection, error) {
	dialer := &net.Dialer{
		Timeout:   cfg.Timeout,
		KeepAlive: cfg.KeepAlive,
	}
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	loginPayload, _ := protocol.BuildLoginPayload(cfg.AuthToken)
	frame := protocol.PrependHeader(0, loginPayload)
	if _, err := conn.Write(frame); err != nil {
		conn.Close()
		return nil, err
	}

	const LOGIN_RESP_OK string = "LOGIN OK"
	buf := make([]byte, len(LOGIN_RESP_OK))
	if _, err := conn.Read(buf); err != nil {
		conn.Close()
		return nil, err
	}
	if string(buf) != LOGIN_RESP_OK {
		conn.Close()
		return nil, errors.New("login failed")
	}

	c := &connection{
		netConn:   conn,
		demuxMap:  dm,
		sendQueue: make(chan []byte, 8192),
		cfg:       cfg,
		addr:      addr,
	}

	return c, nil
}

func (c *connection) activate() {
	go c.writeLoop()
	go c.readLoop()
}

func (c *connection) writeLoop() {
	// Now start reading from the queue
	for data := range c.sendQueue {
		if _, err := c.netConn.Write(data); err != nil {
			c.Close()
			return
		}
	}
}

// scoring is used for followers
func (c *connection) readLoop() {
	for {
		hdr, payload, err := protocol.DrainFrame(c.netConn)
		if err != nil {
			c.Close()
			return
		}

		ch, _, ok := c.demuxMap.LoadRemove(hdr.ClrID)
		if !ok {
			c.Close()
			return
		}

		fields, err := protocol.ParseFields(
			payload[1+len(hdr.Status)+2:],
			hdr.FieldCnt,
		)
		if err != nil {
			c.Close()
			return
		}

		// --- handle real-time error hints ---
		if hdr.Status == "ERROR" && len(fields) > 0 {
			switch string(fields[0].Data) {
			case "308": // leader changed
				c.Close() // force leaderHandler sync loop to reconnect

			case "405": // method not allowed - leader rejects reads
				c.Close() // force followerHandler to remove the connection from its list

			case "503": // unavailable
				c.Close() // leader unavailable, drop to trigger resync

			case "429": // busy
			}
		}

		ch <- protocol.RawResult{Status: hdr.Status, Fields: fields}
	}
}

func (c *connection) Close() error {
	var err error
	c.closer.Do(func() {
		if c.sendQueue != nil {
			close(c.sendQueue)
		}
		if c.netConn != nil {
			err = c.netConn.Close()
		}
		c.closed.Store(true)
	})
	return err
}

func (c *connection) IsClosed() bool {
	return c.closed.Load()
}

// ========================================================
//
//	leaderHandler
//
// ========================================================
func (lh *leaderHandler) updateConnection(newConn *connection) {
	lh.connMu.Lock()
	defer lh.connMu.Unlock()

	// Close old connection if it exists
	if lh.conn != nil {
		lh.conn.Close()
	}

	lh.conn = newConn
}

func (lh *leaderHandler) getConnection() *connection {
	lh.connMu.RLock()
	defer lh.connMu.RUnlock()
	return lh.conn
}

func (lh *leaderHandler) reconnectLeader() {
	curConn := lh.getConnection()

	leader, _, err := getClusterInfo(lh.cfg, lh.discovery)
	if err != nil {
		return
	}

	dm := &demuxMap{}
	if curConn != nil && curConn.demuxMap != nil {
		dm = curConn.demuxMap
	}

	conn, err := newConnection(leader.Host, leader.TcpPort, lh.cfg, dm)
	if err != nil {
		return
	}

	lh.updateConnection(conn)
	conn.activate()

	time.Sleep(10 * time.Millisecond) // Give goroutines time to start
}

func (lh *leaderHandler) LeaderSyncWorker(ctx context.Context) {
	backoff := 100 * time.Millisecond

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		// first check leader is healthy
		curConn := lh.getConnection()
		if curConn == nil || curConn.IsClosed() {
			// Invalidate codecs via callback
			if lh.OnReconnect != nil {
				lh.OnReconnect()
			}
			lh.reconnectLeader()
		}

		// backoff with cap + jitter
		time.Sleep(backoff + time.Duration(rand.Intn(50))*time.Millisecond)
		if backoff < time.Second {
			backoff *= 2
		}
		if backoff > 2*time.Second {
			backoff = 2 * time.Second
		}
	}
}

func (lh *leaderHandler) LeaderSendWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case req := <-lh.reqChan:
			var conn *connection
			// Wait for leader connection to be ready
			for {
				conn = lh.getConnection()
				if conn != nil && conn.netConn != nil && conn.sendQueue != nil {
					break
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(100 * time.Millisecond):
					// Keep waiting for connection
				}
			}

			clrID := atomic.AddUint32(&lh.clrID, 1)
			conn.demuxMap.Store(clrID, req.respChan)

			frame := protocol.PrependHeader(clrID, req.payload)

			// Try to send with detailed logging
			conn.sendQueue <- frame
		}
	}
}

func (fh *followersHandler) nextFollowerConnection() (*connection, error) {
	fh.connMutex.RLock()
	defer fh.connMutex.RUnlock()

	total := len(fh.connections)
	if total == 0 {
		return nil, errors.New("no follower connections available")
	}

	// Try each connection once
	for range total {
		idx := fh.rrIndex.Add(1) % uint32(total)
		conn := fh.connections[idx]

		if conn != nil && !conn.IsClosed() && conn.netConn != nil {
			return conn, nil
		}
		// Connection unhealthy, try next one
	}

	return nil, errors.New("no valid follower connection found")
}

func (fh *followersHandler) FollowerSendWroker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case req := <-fh.reqChan:
			var conn *connection
			var err error
			for {
				conn, err = fh.nextFollowerConnection()
				if err == nil && conn != nil && conn.netConn != nil && conn.sendQueue != nil {
					break
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(100 * time.Millisecond):
					// Keep waiting for connection
				}
			}
			clrID := atomic.AddUint32(&fh.clrID, 1)
			conn.demuxMap.Store(clrID, req.respChan)
			frame := protocol.PrependHeader(clrID, req.payload)
			conn.sendQueue <- frame
		}
	}
}

func (fh *followersHandler) reconnectFollower(host string, port int) {
	fh.connMutex.Lock()
	defer fh.connMutex.Unlock()

	// cold path is ok to have O(n) in favor of O(1) in load balancer
	curIdx := -1
	for idx, con := range fh.connections {
		addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))

		if con.addr == addr {
			if con != nil && !con.IsClosed() {
				return // already connected
			}
			fh.connections[idx].Close()
			curIdx = idx
			break
		}
	}

	conn, err := newConnection(host, port, fh.cfg, &demuxMap{})
	if err != nil || conn.netConn == nil || conn.sendQueue == nil {
		return
	}

	if curIdx != -1 {
		fh.connections[curIdx] = conn
	} else {
		fh.connections = append(fh.connections, conn)
	}

	conn.activate()
}

func (fh *followersHandler) syncFollowers() {
	_, followers, err := getClusterInfo(fh.cfg, fh.discovery)
	if err != nil {
		return
	}

	// --- add new followers we do not have yet ---
	for _, f := range followers {
		fh.reconnectFollower(f.Host, f.TcpPort)
	}
}

func (fh *followersHandler) FollowerSyncWorker(ctx context.Context) {
	ticker := time.NewTicker(fh.cfg.NodeProbeInterval)
	fastTick := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	defer fastTick.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-fastTick.C:
			active := len(fh.connections)
			for _, con := range fh.connections {
				if con == nil || con.IsClosed() {
					active -= 1
				}
			}
			if active == 0 {
				fh.syncFollowers()
			}
		case <-ticker.C:
			fh.syncFollowers()
		}
	}
}

// clrID is reset on every retry; uses config timeouts.
func (c *Handler) Execute(ctx context.Context, isWrite bool, payload []byte) (protocol.RawResult, error) {
	if len(payload) == 0 {
		return protocol.RawResult{}, errors.New("payload should not be empty")
	}

	if isWrite && c.leaderHandler.getConnection() == nil {
		return protocol.RawResult{}, errors.New("cluster has no leader")
	}

	reqAny := c.reqPool.Get()
	if reqAny == nil {
		return protocol.RawResult{}, errors.New("failed to get request from pool")
	}
	req := reqAny.(*request)
	defer c.reqPool.Put(req)

	respAny := c.respChanPool.Get()
	if respAny == nil {
		return protocol.RawResult{}, errors.New("failed to get respChan from pool")
	}
	respChan := respAny.(chan protocol.RawResult)
	defer c.respChanPool.Put(respChan)

	req.payload = payload
	req.ctx = ctx
	req.respChan = respChan
	req.clrID = 0 // will be set on send

	// choose handler
	handlerChan := c.followersHandler.reqChan
	if isWrite {
		handlerChan = c.leaderHandler.reqChan
	}

	// retry policy
	maxRetries := 5
	attempts := 0

	send := func() error {
		select {
		case handlerChan <- req:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// send first attempt
	if err := send(); err != nil {
		return protocol.RawResult{}, err
	}

	for {
		select {
		case <-ctx.Done():
			return protocol.RawResult{}, ctx.Err()

		case res := <-respChan:
			if res.Status == "SUCCESS" {
				return res, nil
			}

			// res.Status == "ERROR"
			var errMsg string
			if len(res.Fields) > 0 {
				errMsg = string(res.Fields[0].Data)
			} else {
				errMsg = res.Status
			}
			backoff := false
			switch errMsg {
			case "405", "308":
				// 405: follower node is promoted to leader and rejects reads
				// 308: leader changed
			case "503", "429": // unavailable / busy
				backoff = true
			default:
				return res, nil
			}

			if attempts >= maxRetries {
				return res, nil
			}
			attempts++

			// first retry immediately, then backoff
			if backoff {
				select {
				case <-time.After(time.Duration(attempts) * 100 * time.Millisecond):
				case <-ctx.Done():
					return protocol.RawResult{}, ctx.Err()
				}
			}

			req.clrID = 0
			if err := send(); err != nil {
				return res, nil
			}
		}
	}
}

func (h *Handler) startDiscoveryTask(ctx context.Context) {
	if h.cfg.DiscoveryAddr == "" {
		return // static mode
	}

	interval := h.cfg.NodeProbeInterval
	if interval <= 0 {
		interval = 2 * time.Second // fallback
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Initial fetch on startup
	if nodes, err := h.fetchExternalDiscovery(); err == nil && len(nodes) > 0 {
		h.updateDiscoveryMap(nodes)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if nodes, err := h.fetchExternalDiscovery(); err == nil && len(nodes) > 0 {
				h.updateDiscoveryMap(nodes)
			}
			// silent fail - keep using existing map
		}
	}
}

// fetchExternalDiscovery calls the external discovery service and returns nodes
func (h *Handler) fetchExternalDiscovery() ([]types.NodeAddr, error) {
	if h.cfg.DiscoveryAddr == "" {
		return nil, errors.New("discovery address not configured")
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(h.cfg.DiscoveryAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch discovery: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discovery service returned status: %d", resp.StatusCode)
	}

	var discovery struct {
		Nodes []types.NodeAddr `json:"nodes"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&discovery); err != nil {
		return nil, fmt.Errorf("failed to parse discovery response: %w", err)
	}

	if len(discovery.Nodes) == 0 {
		return nil, errors.New("discovery service returned empty node list")
	}

	return discovery.Nodes, nil
}

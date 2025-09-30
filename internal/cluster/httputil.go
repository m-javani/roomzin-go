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
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/roomzin/roomzin-go/types"
)

var ErrNoLeaderAvailable = errors.New("no leader found in cluster")

type NodeInfo struct {
	NodeID   string `json:"node_id"`
	ZoneID   string `json:"zone_id"`
	ShardID  string `json:"shard_id"`
	LeaderID string `json:"leader_id"`
}

func httpGet(host string, port int, path string, authToken string, timeout time.Duration, dst any) error {
	url := fmt.Sprintf("http://%s:%d%s", host, port, path)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("http %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

func getNodeInfo(host string, port int, authToken string, timeout time.Duration) (NodeInfo, error) {
	var out NodeInfo
	err := httpGet(host, port, "/node-info", authToken, timeout, &out)
	return out, err
}

func healthCheck(host string, port int, authToken string, timeout time.Duration) (string, error) {
	url := fmt.Sprintf("http://%s:%d/healthz", host, port)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("healthz %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
}

func getClusterInfo(cfg *Config, dmap *DiscoveryMap) (types.NodeAddr, []types.NodeAddr, error) {
	nodeIDs := parseNodeIDs(cfg.SeedNodeIDs)
	if len(nodeIDs) == 0 {
		return types.NodeAddr{}, nil, errors.New("no seed node IDs provided")
	}

	type nodeInfo struct {
		nodeID   string
		host     string
		tcpPort  int
		apiPort  int
		health   string
		leaderID string
	}

	var mu sync.Mutex
	nodes := make(map[string]nodeInfo) // keyed by resolved host
	existing := make(map[string]bool, len(nodeIDs))
	for _, id := range nodeIDs {
		existing[id] = true
	}
	discovered := make(map[string]bool)

	// First phase: seed nodes
	var wg sync.WaitGroup
	for _, nodeID := range nodeIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()

			host, tcpPort, apiPort, ok := dmap.Resolve(id)
			if !ok {
				return
			}

			health, err := healthCheck(host, apiPort, cfg.AuthToken, cfg.HttpTimeout)
			if err != nil || health == "unavailable" {
				return
			}

			info, err := getNodeInfo(host, apiPort, cfg.AuthToken, cfg.HttpTimeout)
			if err != nil {
				return
			}

			mu.Lock()
			nodes[host] = nodeInfo{
				nodeID:   id,
				host:     host,
				tcpPort:  tcpPort,
				apiPort:  apiPort,
				health:   health,
				leaderID: info.LeaderID,
			}
			mu.Unlock()

			var peers []string
			if err := httpGet(host, apiPort, "/peers", cfg.AuthToken, cfg.HttpTimeout, &peers); err == nil {
				mu.Lock()
				for _, peerID := range peers {
					if !existing[peerID] {
						discovered[peerID] = true
					}
				}
				mu.Unlock()
			}
		}(nodeID)
	}
	wg.Wait()

	// Second phase: discovered nodes
	var newWg sync.WaitGroup
	for nodeID := range discovered {
		newWg.Add(1)
		go func(id string) {
			defer newWg.Done()

			host, tcpPort, apiPort, ok := dmap.Resolve(id)
			if !ok {
				return
			}

			health, err := healthCheck(host, apiPort, cfg.AuthToken, cfg.HttpTimeout)
			if err != nil || health == "unavailable" {
				return
			}

			info, err := getNodeInfo(host, apiPort, cfg.AuthToken, cfg.HttpTimeout)
			if err != nil {
				return
			}
			mu.Lock()
			nodes[host] = nodeInfo{
				nodeID:   id,
				host:     host,
				tcpPort:  tcpPort,
				apiPort:  apiPort,
				health:   health,
				leaderID: info.LeaderID,
			}
			mu.Unlock()
		}(nodeID)
	}
	newWg.Wait()

	// Leader election logic
	votes := make(map[string]int, len(nodes))
	for _, node := range nodes {
		if node.leaderID != "" {
			votes[node.leaderID]++
		}
	}

	if len(votes) == 0 {
		return types.NodeAddr{}, nil, errors.New("no leader available")
	}

	var leaderID string
	maxVotes := 0
	for id, count := range votes {
		if count > maxVotes {
			maxVotes = count
			leaderID = id
		}
	}

	var leader types.NodeAddr
	var followers []types.NodeAddr
	for _, node := range nodes {
		if node.leaderID == leaderID {
			addr := types.NodeAddr{
				NodeID:  node.nodeID,
				Host:    node.host,
				TcpPort: node.tcpPort,
				ApiPort: node.apiPort,
			}
			switch node.health {
			case "active_leader":
				leader = addr
			case "active_follower":
				followers = append(followers, addr)
			}
		}
	}

	if leader.Host == "" {
		return types.NodeAddr{}, nil, errors.New("no leader available")
	}

	return leader, followers, nil
}

// parseNodeIDs splits and cleans node IDs
func parseNodeIDs(s string) []string {
	if s == "" {
		return nil
	}
	var ids []string
	for _, id := range strings.Split(s, ",") {
		id = strings.TrimSpace(id)
		if id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

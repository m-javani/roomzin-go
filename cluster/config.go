// // SPDX-License-Identifier: BUSL-1.1
// // Copyright (c) 2026 M. Javani
// //
// // This file is part of roomzin-go.
// //
// // Use of this software is governed by the Business Source License 1.1
// // included in the LICENSE file in the root of this repository.

package cluster

import (
	"errors"
	"strings"
	"time"

	"github.com/roomzin/roomzin-go/types"
)

type ClusterConfig struct {
	SeedNodeIDs    string // "host1,host2,host3"  (NO port, NO zone, NO shard)
	APIPort        int    // HTTP port for /peers /leader /node-info
	TCPPort        int    // TCP port for framed protocol
	AuthToken      string
	Timeout        time.Duration
	HttpTimeout    time.Duration
	KeepAlive      time.Duration
	MaxActiveConns int // hard cap on open TCP connections

	// Discovery settings
	DiscoveryAddr   string           // if set → HTTP discovery mode
	StaticDiscovery []types.NodeAddr // used only when DiscoveryAddr is empty
}

type ClusterConfigBuilder struct {
	config ClusterConfig
}

func NewConfigBuilder() *ClusterConfigBuilder {
	return &ClusterConfigBuilder{
		config: ClusterConfig{
			Timeout:   2 * time.Second,
			KeepAlive: 30 * time.Second,
		},
	}
}

func (b *ClusterConfigBuilder) WithSeedNodeIDs(seed string) *ClusterConfigBuilder {
	b.config.SeedNodeIDs = strings.TrimSpace(seed)
	return b
}

func (b *ClusterConfigBuilder) WithDiscoveryAddr(discoveryAddr string) *ClusterConfigBuilder {
	b.config.DiscoveryAddr = discoveryAddr
	return b
}

func (b *ClusterConfigBuilder) WithStaticDiscovery(staticDiscovery []types.NodeAddr) *ClusterConfigBuilder {
	b.config.StaticDiscovery = staticDiscovery
	return b
}

func (b *ClusterConfigBuilder) WithAPIPort(port int) *ClusterConfigBuilder {
	b.config.APIPort = port
	return b
}

func (b *ClusterConfigBuilder) WithTCPPort(port int) *ClusterConfigBuilder {
	b.config.TCPPort = port
	return b
}

func (b *ClusterConfigBuilder) WithToken(token string) *ClusterConfigBuilder {
	b.config.AuthToken = token
	return b
}

func (b *ClusterConfigBuilder) WithTimeout(d time.Duration) *ClusterConfigBuilder {
	b.config.Timeout = d
	return b
}

func (b *ClusterConfigBuilder) WithKeepAlive(d time.Duration) *ClusterConfigBuilder {
	b.config.KeepAlive = d
	return b
}

func (b *ClusterConfigBuilder) Build() (ClusterConfig, error) {
	if err := b.validate(); err != nil {
		return ClusterConfig{}, types.RzError(err, types.KindClient)
	}
	return b.config, nil
}

func (b *ClusterConfigBuilder) validate() error {
	var errs []error
	if b.config.SeedNodeIDs == "" {
		errs = append(errs, errors.New("at least one seed address is required"))
	}
	if b.config.TCPPort == 0 {
		errs = append(errs, errors.New("TCP port is required"))
	}
	if b.config.APIPort == 0 {
		errs = append(errs, errors.New("API port is required in clustered mode"))
	}
	if b.config.AuthToken == "" {
		errs = append(errs, errors.New("authentication requires a token"))
	}
	if len(errs) == 0 {
		return nil
	}
	return types.RzError(errors.Join(errs...), types.KindClient)
}

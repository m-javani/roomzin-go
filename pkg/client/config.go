// // SPDX-License-Identifier: BUSL-1.1
// // Copyright (c) 2026 M. Javani
// //
// // This file is part of roomzin-go.
// //
// // Use of this software is governed by the Business Source License 1.1
// // included in the LICENSE file in the root of this repository.

package client

import (
	"errors"
	"strings"
	"time"

	"github.com/m-javani/roomzin-go/pkg/types"
)

// ConnectionMode defines how the client connects to the server
type ConnectionMode int

const (
	// StandaloneMode connects directly to a single standalone server
	StandaloneMode ConnectionMode = iota
	// ClusterMode connects to a router which handles routing to cluster shards
	ClusterMode
)

// Config holds the configuration for the Roomzin client
type Config struct {
	// Addr is the server address (hostname or IP)
	Addr string

	// Port is the TCP port
	Port int

	// Timeout is the request timeout
	Timeout time.Duration

	// KeepAliveSec is the TCP keep-alive duration
	KeepAliveSec time.Duration

	// Mode determines how the client connects (standalone or cluster via router)
	Mode ConnectionMode
}

// ConfigBuilder provides a fluent interface for building Config
type ConfigBuilder struct {
	config Config
}

// NewConfigBuilder creates a new ConfigBuilder with default values
func NewConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{
		config: Config{
			Timeout:      2 * time.Second,
			KeepAliveSec: 30 * time.Second,
			Mode:         StandaloneMode, // default for backward compatibility
		},
	}
}

// WithAddr sets the server address
func (b *ConfigBuilder) WithAddr(addr string) *ConfigBuilder {
	b.config.Addr = strings.TrimSpace(addr)
	return b
}

// WithPort sets the TCP port
func (b *ConfigBuilder) WithPort(port int) *ConfigBuilder {
	b.config.Port = port
	return b
}

// WithTimeout sets the request timeout
func (b *ConfigBuilder) WithTimeout(d time.Duration) *ConfigBuilder {
	b.config.Timeout = d
	return b
}

// WithKeepAlive sets the TCP keep-alive duration
func (b *ConfigBuilder) WithKeepAlive(d time.Duration) *ConfigBuilder {
	b.config.KeepAliveSec = d
	return b
}

// WithMode sets the connection mode (standalone or cluster)
func (b *ConfigBuilder) WithMode(mode ConnectionMode) *ConfigBuilder {
	b.config.Mode = mode
	return b
}

// Build validates and returns the Config
func (b *ConfigBuilder) Build() (Config, error) {
	if err := b.validate(); err != nil {
		return Config{}, types.RzError(err, types.KindClient)
	}
	return b.config, nil
}

func (b *ConfigBuilder) validate() error {
	var errs []error

	if b.config.Addr == "" {
		errs = append(errs, errors.New("server address is required"))
	}

	if b.config.Port == 0 {
		errs = append(errs, errors.New("TCP port is required"))
	}

	if len(errs) == 0 {
		return nil
	}

	return types.RzError(errors.Join(errs...), types.KindClient)
}
